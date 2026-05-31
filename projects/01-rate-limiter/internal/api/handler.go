package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/ankitsriv89/rate-limiter/internal/metrics"
	"github.com/ankitsriv89/rate-limiter/internal/policy"
	"github.com/ankitsriv89/rate-limiter/internal/store"
)

type Handler struct {
	policies *policy.Cache
	pStore   *policy.Store
	limiter  *store.RedisLimiter
	log      *zap.Logger
}

func New(policies *policy.Cache, pStore *policy.Store, limiter *store.RedisLimiter, log *zap.Logger) *Handler {
	return &Handler{policies: policies, pStore: pStore, limiter: limiter, log: log}
}

func (h *Handler) Routes(r *mux.Router) {
	r.HandleFunc("/v1/limits/check", h.Check).Methods(http.MethodPost)
	r.HandleFunc("/v1/policies/{policy_id}", h.UpsertPolicy).Methods(http.MethodPut)
	r.HandleFunc("/v1/policies", h.ListPolicies).Methods(http.MethodGet)
	r.HandleFunc("/v1/policies/{policy_id}", h.GetPolicy).Methods(http.MethodGet)
	r.HandleFunc("/v1/usage/{subject}", h.Usage).Methods(http.MethodGet)
	r.HandleFunc("/healthz", h.Health).Methods(http.MethodGet)
}

func (h *OdysseyHandler) Routes(r *mux.Router) {
	r.HandleFunc("/v1/odyssey/state", h.State).Methods(http.MethodGet)
	r.HandleFunc("/v1/odyssey/question", h.Question).Methods(http.MethodGet)
	r.HandleFunc("/v1/odyssey/answer", h.Answer).Methods(http.MethodPost)
	r.HandleFunc("/v1/odyssey/hint", h.Hint).Methods(http.MethodGet)
	r.HandleFunc("/v1/odyssey/reset", h.Reset).Methods(http.MethodPost)
}

// CheckRequest is the body for POST /v1/limits/check.
// Subject is a namespaced identifier for the caller (e.g. "user:42", "ip:1.2.3.4").
// PolicyID selects which rate-limit rule to apply. Both are required.
type CheckRequest struct {
	Subject  string `json:"subject"`
	PolicyID string `json:"policy_id"`
}

type CheckResponse struct {
	Allowed      bool   `json:"allowed"`
	Remaining    int64  `json:"remaining"`
	RetryAfterMs int64  `json:"retry_after_ms,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

func (h *Handler) Check(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Subject == "" || req.PolicyID == "" {
		writeErr(w, http.StatusBadRequest, "subject and policy_id required")
		return
	}

	p, err := h.policies.Get(r.Context(), req.PolicyID)
	if err != nil {
		h.log.Warn("policy not found", zap.String("policy_id", req.PolicyID), zap.Error(err))
		writeErr(w, http.StatusNotFound, "policy not found")
		return
	}

	start := time.Now()
	// Key space: "<policy_id>:<subject>" keeps buckets isolated per policy,
	// so the same subject can have different limits under different policies.
	key := req.PolicyID + ":" + req.Subject

	var allowed bool
	var remaining, retryAfterMs int64

	switch p.Algorithm {
	case policy.AlgorithmTokenBucket:
		allowed, remaining, retryAfterMs, err = h.limiter.TokenBucketAllow(r.Context(), key, p.Capacity, p.RefillRate)
	case policy.AlgorithmSlidingWindow:
		var count int64
		allowed, count, err = h.limiter.SlidingWindowAllow(r.Context(), key, int64(p.Capacity), time.Duration(p.WindowMs)*time.Millisecond)
		remaining = int64(p.Capacity) - count
	default:
		writeErr(w, http.StatusBadRequest, "unsupported algorithm")
		return
	}

	dur := time.Since(start)
	metrics.CheckDuration.WithLabelValues(string(p.Algorithm)).Observe(dur.Seconds())

	if err != nil {
		metrics.RedisErrors.WithLabelValues("check").Inc()
		h.log.Error("redis check failed", zap.Error(err))
		// Fail-open: a Redis outage should not block all traffic. remaining=-1
		// signals to callers that the value is unknown rather than zero.
		allowed = true
		remaining = -1
	}

	allowedStr := "true"
	if !allowed {
		allowedStr = "false"
	}
	metrics.RequestsTotal.WithLabelValues(req.PolicyID, allowedStr).Inc()

	reason := ""
	if !allowed {
		reason = "rate_limit_exceeded"
	}

	// async audit (non-blocking)
	go func() {
		_ = h.pStore.RecordDecision(r.Context(), req.Subject, req.PolicyID, allowed, reason, retryAfterMs)
	}()

	status := http.StatusOK
	if !allowed {
		status = http.StatusTooManyRequests
		w.Header().Set("Retry-After", msToSeconds(retryAfterMs))
	}

	h.log.Info("check",
		zap.String("subject", req.Subject),
		zap.String("policy", req.PolicyID),
		zap.Bool("allowed", allowed),
		zap.Duration("dur", dur),
	)

	writeJSON(w, status, CheckResponse{
		Allowed:      allowed,
		Remaining:    remaining,
		RetryAfterMs: retryAfterMs,
		Reason:       reason,
	})
}

func (h *Handler) UpsertPolicy(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["policy_id"]
	var p policy.Policy
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	p.ID = id
	if err := h.pStore.Upsert(r.Context(), &p); err != nil {
		h.log.Error("upsert policy", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "failed to save policy")
		return
	}
	h.policies.Invalidate(id)
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["policy_id"]
	p, err := h.pStore.Get(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "policy not found")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := h.pStore.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list policies")
		return
	}
	writeJSON(w, http.StatusOK, policies)
}

// Usage returns the recent audit decisions for a subject.
func (h *Handler) Usage(w http.ResponseWriter, r *http.Request) {
	subject := mux.Vars(r)["subject"]
	decisions, err := h.pStore.RecentDecisions(r.Context(), subject, 30)
	if err != nil {
		h.log.Error("usage lookup", zap.String("subject", subject), zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "failed to load usage")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subject": subject, "decisions": decisions})
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// msToSeconds converts milliseconds to a whole-second string for the Retry-After header.
// RFC 7231 requires a positive integer; floor to 1 so clients always wait at least one second.
func msToSeconds(ms int64) string {
	if ms <= 0 {
		return "1"
	}
	s := ms / 1000
	if s < 1 {
		s = 1
	}
	return fmt.Sprintf("%d", s)
}

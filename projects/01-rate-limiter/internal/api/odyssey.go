package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"github.com/ankitsriv89/rate-limiter/internal/odyssey"
	"github.com/ankitsriv89/rate-limiter/internal/policy"
	"github.com/ankitsriv89/rate-limiter/internal/store"
)

const odysseyPolicy = "odyssey-hops"

// OdysseyHandler handles the /v1/odyssey/* routes.
type OdysseyHandler struct {
	oStore  *odyssey.Store
	limiter *store.RedisLimiter
	policies *policy.Cache
	groq    *odyssey.GroqClient // may be nil if no API key
	log     *zap.Logger
}

func NewOdysseyHandler(
	oStore *odyssey.Store,
	limiter *store.RedisLimiter,
	policies *policy.Cache,
	groq *odyssey.GroqClient,
	log *zap.Logger,
) *OdysseyHandler {
	return &OdysseyHandler{oStore: oStore, limiter: limiter, policies: policies, groq: groq, log: log}
}

// playerID returns the session ID for progress tracking (issued as a signed cookie)
// and the client IP for rate-limit keying. These are intentionally separate:
// - progress is tied to the session cookie (unforgeable, server-signed)
// - rate limit is tied to the real IP (network-layer identity)
func playerID(w http.ResponseWriter, r *http.Request) (sessionID, ip string) {
	sessionID = odyssey.IssueSession(w, r)
	ip = odyssey.ClientIP(r)
	return
}

// GET /v1/odyssey/state — returns player progress + real remaining hops from Redis
func (h *OdysseyHandler) State(w http.ResponseWriter, r *http.Request) {
	sid, ip := playerID(w, r)
	st, err := h.oStore.GetProgress(r.Context(), sid)
	if err != nil {
		h.log.Error("odyssey state", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "could not load progress")
		return
	}

	// Read real token count from Redis without debiting — peek only.
	p, pErr := h.policies.Get(r.Context(), odysseyPolicy)
	var remaining int64 = 6
	var retryAfterMs int64
	var rateLimited bool
	if pErr == nil {
		key := odysseyPolicy + ":ip:" + ip
		remaining, retryAfterMs = h.limiter.PeekTokenBucket(r.Context(), key, p.Capacity, p.RefillRate)
		rateLimited = remaining <= 0
	}

	dest := odyssey.Journey[st.DestinationIdx]
	writeJSON(w, http.StatusOK, map[string]any{
		"ip":              ip,
		"destination_idx": st.DestinationIdx,
		"destination":    dest,
		"hops_used":      st.HopsUsed,
		"hops_remaining":  remaining,
		"completed":      st.Completed,
		"total_stops":    len(odyssey.Journey) - 1,
		"rate_limited":   rateLimited,
		"retry_after_ms": retryAfterMs,
	})
}

// GET /v1/odyssey/question — returns a question for the current destination (no hop cost)
func (h *OdysseyHandler) Question(w http.ResponseWriter, r *http.Request) {
	sid, _ := playerID(w, r)
	st, err := h.oStore.GetProgress(r.Context(), sid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load progress")
		return
	}
	if st.Completed {
		writeJSON(w, http.StatusOK, map[string]any{"completed": true, "message": "You have reached Sagittarius A*. The odyssey is complete."})
		return
	}

	dest := odyssey.Journey[st.DestinationIdx]

	// Try DB first
	q, err := h.oStore.RandomQuestion(r.Context(), dest.ID)
	if errors.Is(err, sql.ErrNoRows) && h.groq != nil {
		// Generate from Groq and store it
		q, err = h.generateAndSave(r, dest)
	}
	if err != nil {
		h.log.Error("odyssey question", zap.String("dest", dest.ID), zap.Error(err))
		writeErr(w, http.StatusServiceUnavailable, "could not load question — try again")
		return
	}

	// Return choices but NOT the answer index (kept server-side)
	var choices []string
	_ = json.Unmarshal(q.Choices, &choices)

	writeJSON(w, http.StatusOK, map[string]any{
		"question_id":     q.ID,
		"destination_idx": st.DestinationIdx,
		"destination":     dest,
		"question":        q.Question,
		"choices":         choices,
		"source":          q.Source,
	})
}

type AnswerRequest struct {
	QuestionID int64 `json:"question_id"`
	Choice     int   `json:"choice"` // 0-3
}

// POST /v1/odyssey/answer — costs 1 hop, checks answer, advances destination if correct
func (h *OdysseyHandler) Answer(w http.ResponseWriter, r *http.Request) {
	sid, ip := playerID(w, r)

	var req AnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Choice < 0 || req.Choice > 3 {
		writeErr(w, http.StatusBadRequest, "choice must be 0-3")
		return
	}

	// Rate-limit by real IP (network identity) — not by session so spoofing
	// the cookie doesn't grant extra hops.
	p, err := h.policies.Get(r.Context(), odysseyPolicy)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "rate-limit policy not found — run seed script")
		return
	}

	// Rate-limit key is prefixed with "ip:" to match the policy's subject_type,
	// keeping the Redis namespace consistent with /v1/limits/check keys.
	subject := "ip:" + ip
	key := odysseyPolicy + ":" + subject
	allowed, remaining, retryAfterMs, rlErr := h.limiter.TokenBucketAllow(r.Context(), key, p.Capacity, p.RefillRate)
	if rlErr != nil {
		h.log.Error("redis rate limit", zap.Error(rlErr))
		// fail-open
		allowed = true
		remaining = -1
	}
	if !allowed {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"allowed":        false,
			"remaining":      0,
			"retry_after_ms": retryAfterMs,
			"retry_after_s":  retryAfterMs / 1000,
			"reason":         "hop_limit_exceeded",
			"message":        "You have used all your hops. Refuel and try again later.",
		})
		return
	}

	// Load player progress by session ID (cookie-bound, not IP-bound)
	st, err := h.oStore.GetProgress(r.Context(), sid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load progress")
		return
	}
	if st.Completed {
		writeJSON(w, http.StatusOK, map[string]any{"completed": true, "correct": true, "message": "Odyssey already complete!"})
		return
	}

	// Load the question to verify answer
	dest := odyssey.Journey[st.DestinationIdx]
	q, err := h.oStore.GetQuestionByID(r.Context(), req.QuestionID)
	if err != nil || q.DestinationID != dest.ID {
		writeErr(w, http.StatusBadRequest, "question not valid for current destination")
		return
	}

	correct := req.Choice == q.Answer
	st.HopsUsed++

	var nextDest *odyssey.Destination
	if correct {
		st.DestinationIdx++
		if st.DestinationIdx >= len(odyssey.Journey) {
			st.DestinationIdx = len(odyssey.Journey) - 1
			st.Completed = true
		} else {
			d := odyssey.Journey[st.DestinationIdx]
			nextDest = &d
		}
	}

	if err := h.oStore.SaveProgress(r.Context(), st); err != nil {
		h.log.Error("save progress", zap.Error(err))
	}

	var choices []string
	_ = json.Unmarshal(q.Choices, &choices)

	resp := map[string]any{
		"allowed":         true,
		"remaining_hops":  remaining,
		"correct":         correct,
		"correct_answer":  q.Answer,
		"correct_label":   choices[q.Answer],
		"hint":            q.Hint,
		"destination_idx": st.DestinationIdx,
		"completed":       st.Completed,
	}
	if nextDest != nil {
		resp["next_destination"] = nextDest
	}
	writeJSON(w, http.StatusOK, resp)
}

// GET /v1/odyssey/hint — returns hint for a question (costs 1 hop)
func (h *OdysseyHandler) Hint(w http.ResponseWriter, r *http.Request) {
	_, ip := playerID(w, r)
	qidStr := r.URL.Query().Get("question_id")
	var qid int64
	if _, err := parseID(qidStr, &qid); err != nil {
		writeErr(w, http.StatusBadRequest, "question_id required")
		return
	}

	// Debit 1 hop for a hint — rate-limited by IP
	p, err := h.policies.Get(r.Context(), odysseyPolicy)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "rate-limit policy not found")
		return
	}
	key := odysseyPolicy + ":ip:" + ip
	allowed, remaining, retryAfterMs, _ := h.limiter.TokenBucketAllow(r.Context(), key, p.Capacity, p.RefillRate)
	if !allowed {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"allowed":        false,
			"retry_after_ms": retryAfterMs,
			"reason":         "hop_limit_exceeded",
		})
		return
	}

	q, err := h.oStore.GetQuestionByID(r.Context(), qid)
	if err != nil {
		writeErr(w, http.StatusNotFound, "question not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"hint":           q.Hint,
		"remaining_hops": remaining,
	})
}

// POST /v1/odyssey/reset — resets progress for the calling session (for demo purposes)
func (h *OdysseyHandler) Reset(w http.ResponseWriter, r *http.Request) {
	sid, _ := playerID(w, r)
	// Reset journey progress only — hop counter is NOT reset.
	// Rate limiting is enforced by IP and persists across journey resets
	// so players can't bypass the limit by resetting their journey.
	st := &odyssey.State{IP: sid, DestinationIdx: 0, HopsUsed: 0, Completed: false}
	if err := h.oStore.SaveProgress(r.Context(), st); err != nil {
		writeErr(w, http.StatusInternalServerError, "reset failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reset": true})
}

// generateAndSave calls Groq to produce a question then persists it so future requests
// for the same destination are served from the DB instead of calling the LLM again.
// A DB save failure is non-fatal: the question is still returned to the player.
func (h *OdysseyHandler) generateAndSave(r *http.Request, dest odyssey.Destination) (*odyssey.Question, error) {
	q, err := h.groq.GenerateQuestion(r.Context(), dest)
	if err != nil {
		return nil, err
	}
	id, err := h.oStore.SaveQuestion(r.Context(), q)
	if err != nil {
		h.log.Warn("save groq question", zap.Error(err))
	}
	q.ID = id
	return q, nil
}

// parseID parses a string into int64.
func parseID(s string, out *int64) (bool, error) {
	if s == "" {
		return false, errors.New("empty")
	}
	var n int64
	_, err := parseIntStr(s, &n)
	if err != nil {
		return false, err
	}
	*out = n
	return true, nil
}

func parseIntStr(s string, out *int64) (int, error) {
	var n int64
	var neg bool
	i := 0
	if i < len(s) && s[i] == '-' {
		neg = true
		i++
	}
	if i == len(s) {
		return 0, errors.New("no digits")
	}
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, errors.New("non-digit")
		}
		n = n*10 + int64(c-'0')
	}
	if neg {
		n = -n
	}
	*out = n
	return len(s), nil
}


// Package api provides HTTP handlers for the load balancer control plane
// and the reverse-proxy data plane.
//
// Route split:
//   publicRouter  — /proxy/*, /healthz, /metrics, /static/*, /
//   adminRouter   — /v1/* (control plane, requires bearer token)
package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/ankitsriv89/06-load-balancer/balancer"
	"github.com/ankitsriv89/06-load-balancer/metrics"
	"github.com/ankitsriv89/06-load-balancer/store"
)

// Handler wires together control-plane and data-plane endpoints.
type Handler struct {
	lb          *balancer.LoadBalancer
	store       *store.Store
	ctx         context.Context
	log         *zap.Logger
	adminToken  string
	healthzBody []byte
}

// New registers routes on two separate routers:
//   - publicRouter: data plane + observability (no auth required)
//   - adminRouter:  control plane /v1/* (bearer token required when adminToken != "")
func New(
	ctx context.Context,
	lb *balancer.LoadBalancer,
	st *store.Store,
	log *zap.Logger,
	publicRouter *mux.Router,
	adminRouter *mux.Router,
	adminToken string,
) *Handler {
	h := &Handler{
		lb:          lb,
		store:       st,
		ctx:         ctx,
		log:         log,
		adminToken:  adminToken,
		healthzBody: []byte(`{"status":"ok"}`),
	}

	// ── Public router: data plane ────────────────────────────────
	publicRouter.HandleFunc("/healthz", h.healthz).Methods(http.MethodGet)
	publicRouter.PathPrefix("/proxy/{service}/").HandlerFunc(h.proxy)
	publicRouter.Handle("/metrics", promhttp.Handler())
	publicRouter.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("web"))),
	)
	publicRouter.HandleFunc("/", h.root).Methods(http.MethodGet)

	// ── Admin router: control plane ──────────────────────────────
	// All /v1/* routes are wrapped with auth middleware.
	admin := adminRouter.PathPrefix("/v1").Subrouter()
	admin.Use(h.requireToken)

	admin.HandleFunc("/backends", h.listBackends).Methods(http.MethodGet)
	admin.HandleFunc("/backends/{service}", h.addBackend).Methods(http.MethodPost)
	admin.HandleFunc("/backends/{service}/{backend}", h.removeBackend).Methods(http.MethodDelete)
	admin.HandleFunc("/backends/{service}/algorithm", h.setAlgorithm).Methods(http.MethodPut)
	admin.HandleFunc("/backends/{service}/health", h.healthHistory).Methods(http.MethodGet)
	admin.HandleFunc("/stats", h.stats).Methods(http.MethodGet)

	// Also expose read-only stats on the public router so the UI can poll it
	// without hitting the admin port. Write operations remain admin-only.
	publicRouter.HandleFunc("/v1/stats", h.stats).Methods(http.MethodGet)
	publicRouter.HandleFunc("/v1/backends/{service}/health", h.healthHistory).Methods(http.MethodGet)

	return h
}

// requireToken is a middleware that enforces Bearer token authentication on
// admin routes. If adminToken is empty the middleware is a no-op (dev mode).
func (h *Handler) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.adminToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		got := r.Header.Get("Authorization")
		want := "Bearer " + h.adminToken
		// Constant-time compare to prevent timing attacks.
		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid or missing admin token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) root(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(h.healthzBody)
}

// addBackend registers a backend in a service pool and persists it.
// POST /v1/backends/{service}
// Body: {"url":"http://...","weight":1}
func (h *Handler) addBackend(w http.ResponseWriter, r *http.Request) {
	service := mux.Vars(r)["service"]
	var body struct {
		URL    string `json:"url"`
		Weight int    `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.URL == "" {
		writeError(w, http.StatusBadRequest, "url required")
		return
	}
	if body.Weight <= 0 {
		body.Weight = 1
	}

	// SSRF validation happens inside lb.AddBackend via urlutil.Validate.
	if err := h.lb.AddBackend(h.ctx, service, body.URL, body.Weight); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.store != nil {
		if err := h.store.UpsertBackend(r.Context(), service, body.URL, body.Weight, "healthy"); err != nil {
			h.log.Error("store upsert backend", zap.Error(err))
		}
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"service": service,
		"url":     body.URL,
		"status":  "registered",
	})
}

// removeBackend deletes a backend from the pool.
// DELETE /v1/backends/{service}/{backend}  (backend = URL-encoded upstream URL)
func (h *Handler) removeBackend(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	service := vars["service"]
	backendURL, err := url.QueryUnescape(vars["backend"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid backend URL encoding")
		return
	}

	if err := h.lb.RemoveBackend(service, backendURL); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if h.store != nil {
		if err := h.store.DeleteBackend(r.Context(), service, backendURL); err != nil {
			h.log.Error("store delete backend", zap.Error(err))
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// setAlgorithm changes the routing algorithm for a service.
// PUT /v1/backends/{service}/algorithm
// Body: {"algorithm":"round_robin"}
func (h *Handler) setAlgorithm(w http.ResponseWriter, r *http.Request) {
	service := mux.Vars(r)["service"]
	var body struct {
		Algorithm balancer.Algorithm `json:"algorithm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.lb.SetAlgorithm(service, body.Algorithm); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"algorithm": string(body.Algorithm)})
}

// listBackends returns all backends across all services.
func (h *Handler) listBackends(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.lb.Stats())
}

// stats returns real-time backend statistics grouped by service.
func (h *Handler) stats(w http.ResponseWriter, _ *http.Request) {
	type serviceView struct {
		Service   string                  `json:"service"`
		Algorithm balancer.Algorithm      `json:"algorithm"`
		Backends  []balancer.BackendStats `json:"backends"`
	}
	services := h.lb.Services()
	out := make([]serviceView, 0, len(services))
	for _, svc := range services {
		pool := h.lb.ServicePool(svc)
		var bstats []balancer.BackendStats
		for _, b := range pool.Backends() {
			bstats = append(bstats, balancer.BackendStats{
				URL:         b.URL,
				Service:     b.Service,
				Status:      b.GetStatus(),
				Weight:      b.Weight,
				ActiveConns: atomic.LoadInt64(&b.ActiveConns),
				TotalConns:  atomic.LoadUint64(&b.TotalConns),
				LatencyEWMA: b.LatencyEWMA(),
			})
		}
		out = append(out, serviceView{
			Service:   svc,
			Algorithm: pool.Algorithm(),
			Backends:  bstats,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// healthHistory returns recent health-check events from the DB.
// GET /v1/backends/{service}/health?limit=50
func (h *Handler) healthHistory(w http.ResponseWriter, r *http.Request) {
	service := mux.Vars(r)["service"]
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if h.store == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	rows, err := h.store.HealthHistory(r.Context(), service, limit)
	if err != nil {
		h.log.Error("health history", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// proxy reverse-proxies requests to a healthy backend.
func (h *Handler) proxy(w http.ResponseWriter, r *http.Request) {
	service := mux.Vars(r)["service"]

	const maxRetries = 2
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		backend, err := h.lb.Next(service)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}

		if attempt > 0 {
			metrics.RetryTotal.WithLabelValues(service).Inc()
		}

		target, err := url.Parse(backend.URL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "invalid backend URL")
			return
		}

		atomic.AddInt64(&backend.ActiveConns, 1)
		atomic.AddUint64(&backend.TotalConns, 1)
		metrics.ActiveConnections.WithLabelValues(service, backend.URL).Inc()

		rw := &responseRecorder{ResponseWriter: w, code: http.StatusOK}
		start := time.Now()

		proxy := httputil.NewSingleHostReverseProxy(target)
		// Strip the /proxy/{service} prefix so the upstream sees the real path.
		r2 := r.Clone(r.Context())
		prefix := fmt.Sprintf("/proxy/%s", service)
		r2.URL.Path = r.URL.Path[len(prefix):]
		if r2.URL.Path == "" {
			r2.URL.Path = "/"
		}
		r2.URL.RawPath = ""

		// Strip hop-by-hop and sensitive headers before forwarding.
		sanitizeHeaders(r2)

		proxy.ServeHTTP(rw, r2)

		dur := time.Since(start)
		atomic.AddInt64(&backend.ActiveConns, -1)
		metrics.ActiveConnections.WithLabelValues(service, backend.URL).Dec()
		metrics.RequestDuration.WithLabelValues(service, backend.URL).Observe(dur.Seconds())
		backend.RecordLatency(float64(dur.Milliseconds()))
		metrics.RequestsTotal.WithLabelValues(service, backend.URL, strconv.Itoa(rw.code)).Inc()

		h.log.Debug("proxied request",
			zap.String("service", service),
			zap.String("backend", backend.URL),
			zap.Int("status", rw.code),
			zap.Duration("duration", dur),
		)

		if rw.code < 500 {
			return
		}
		lastErr = fmt.Errorf("backend %s returned %d", backend.URL, rw.code)
	}

	h.log.Warn("all retries exhausted", zap.String("service", service), zap.Error(lastErr))
	writeError(w, http.StatusBadGateway, "upstream error after retries")
}

// sanitizeHeaders removes headers that must not be forwarded to upstreams:
// spoofable X-Forwarded-* fields and the client's Authorization/Cookie so
// they are not leaked to an untrusted backend.
func sanitizeHeaders(r *http.Request) {
	for _, h := range []string{
		"X-Forwarded-For",
		"X-Forwarded-Host",
		"X-Forwarded-Proto",
		"X-Real-Ip",
		"Authorization",
		"Cookie",
	} {
		r.Header.Del(h)
	}
}

// responseRecorder captures the status code written by the upstream proxy.
type responseRecorder struct {
	http.ResponseWriter
	code    int
	written bool
}

func (rr *responseRecorder) WriteHeader(code int) {
	if !rr.written {
		rr.code = code
		rr.written = true
		rr.ResponseWriter.WriteHeader(code)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

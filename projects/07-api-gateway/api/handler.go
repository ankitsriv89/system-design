// Package api provides the HTTP transport layer for the API gateway:
// the reverse-proxy data plane and the admin control plane.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/ankitsriv89/07-api-gateway/gateway"
	"github.com/ankitsriv89/07-api-gateway/metrics"
)

// Pre-built static responses — never allocate in handlers.
var (
	bodyOK          = []byte(`{"status":"ok"}`)
	bodyNotFound    = []byte(`{"error":"not found"}`)
	bodyUnauth      = []byte(`{"error":"unauthorized"}`)
	bodyForbidden   = []byte(`{"error":"forbidden"}`)
	bodyRateLimited = []byte(`{"error":"rate limited"}`)
	bodyTooLarge    = []byte(`{"error":"payload too large"}`)
	bodyBadRequest  = []byte(`{"error":"bad request"}`)
	bodyInternal    = []byte(`{"error":"internal server error"}`)
)

// bufPool reduces allocations when copying proxy response bodies.
var bufPool = sync.Pool{New: func() any { b := make([]byte, 32*1024); return &b }}

// Handler holds the admin + proxy HTTP handlers.
type Handler struct {
	gw      *gateway.Gateway
	keys    gateway.KeyStore
	routes  gateway.RouteStore
	limiter gateway.RateLimiter
	log     *zap.Logger
	metrics *metrics.Metrics
	idGen   func() string
}

// New constructs a Handler.
func New(gw *gateway.Gateway, keys gateway.KeyStore, routes gateway.RouteStore, limiter gateway.RateLimiter, log *zap.Logger, m *metrics.Metrics, idGen func() string) *Handler {
	return &Handler{gw: gw, keys: keys, routes: routes, limiter: limiter, log: log, metrics: m, idGen: idGen}
}

// RegisterAdmin mounts admin endpoints on r (control plane).
func (h *Handler) RegisterAdmin(r *mux.Router) {
	r.HandleFunc("/healthz", h.healthz).Methods(http.MethodGet)

	// API key management
	r.HandleFunc("/v1/api-keys", h.createKey).Methods(http.MethodPost)
	r.HandleFunc("/v1/api-keys", h.listKeys).Methods(http.MethodGet)
	r.HandleFunc("/v1/api-keys/{id}", h.getKey).Methods(http.MethodGet)
	r.HandleFunc("/v1/api-keys/{id}/revoke", h.revokeKey).Methods(http.MethodPost)

	// Route management
	r.HandleFunc("/v1/routes", h.listRoutes).Methods(http.MethodGet)
	r.HandleFunc("/v1/routes/{id}", h.upsertRoute).Methods(http.MethodPut)
	r.HandleFunc("/v1/routes/{id}", h.getRouteHandler).Methods(http.MethodGet)
	r.HandleFunc("/v1/routes/{id}", h.deleteRoute).Methods(http.MethodDelete)

	// Stats
	r.HandleFunc("/v1/stats/quota/{key_id}", h.quotaStats).Methods(http.MethodGet)
}

// RegisterProxy mounts the catch-all proxy on r (data plane).
func (h *Handler) RegisterProxy(r *mux.Router) {
	r.HandleFunc("/healthz", h.healthz).Methods(http.MethodGet)
	r.PathPrefix("/").HandlerFunc(h.proxy)
}

// ---- static handlers --------------------------------------------------------

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(bodyOK)
}

// ---- proxy ------------------------------------------------------------------

func (h *Handler) proxy(w http.ResponseWriter, r *http.Request) {
	requestID := h.idGen()
	start := time.Now()

	result, err := h.gw.Evaluate(r.Context(), requestID, r)
	if err != nil {
		status, body := mapError(err)
		h.metrics.RequestTotal.WithLabelValues("proxy", http.StatusText(status)).Inc()
		h.metrics.RequestDuration.WithLabelValues("proxy").Observe(time.Since(start).Seconds())
		h.log.Info("gateway blocked",
			zap.String("request_id", requestID),
			zap.String("path", r.URL.Path),
			zap.String("reason", err.Error()),
			zap.Int("status", status),
		)
		jsonError(w, status, body)
		return
	}

	route := result.Route
	timeoutSecs := route.TimeoutSecs
	if timeoutSecs == 0 {
		timeoutSecs = 30
	}

	upstreamURL, parseErr := url.Parse(result.UpstreamURL)
	if parseErr != nil {
		h.log.Error("bad upstream URL", zap.String("url", result.UpstreamURL), zap.Error(parseErr))
		jsonError(w, http.StatusBadGateway, bodyInternal)
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = upstreamURL
			req.Host = upstreamURL.Host
			// Strip hop-by-hop headers.
			req.Header.Del("Connection")
			req.Header.Del("Upgrade")
			// Propagate request ID downstream.
			req.Header.Set("X-Request-ID", requestID)
			if result.Key != nil {
				req.Header.Set("X-Consumer-ID", result.Key.ID)
			}
		},
		BufferPool: proxyBufferPool{},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			h.log.Error("upstream error",
				zap.String("request_id", requestID),
				zap.String("upstream", result.UpstreamURL),
				zap.Error(err),
			)
			h.metrics.UpstreamErrors.WithLabelValues(route.ID).Inc()
			jsonError(w, http.StatusBadGateway, []byte(`{"error":"upstream error"}`))
		},
	}

	rec := &statusRecorder{ResponseWriter: w, status: 200}
	proxy.ServeHTTP(rec, r)

	elapsed := time.Since(start).Seconds()
	h.metrics.RequestTotal.WithLabelValues(route.ID, http.StatusText(rec.status)).Inc()
	h.metrics.RequestDuration.WithLabelValues(route.ID).Observe(elapsed)
	h.log.Info("proxied request",
		zap.String("request_id", requestID),
		zap.String("route", route.ID),
		zap.String("upstream", result.UpstreamURL),
		zap.Int("status", rec.status),
		zap.Float64("elapsed_s", elapsed),
	)
}

// ---- admin: API keys --------------------------------------------------------

func (h *Handler) createKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Owner       string   `json:"owner"`
		Scopes      []string `json:"scopes"`
		QuotaPerMin int      `json:"quota_per_min"`
		RawKey      string   `json:"key"` // caller provides the raw key value
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if body.Owner == "" || body.RawKey == "" {
		jsonError(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	key := &gateway.APIKey{
		ID:          h.idGen(),
		Owner:       body.Owner,
		HashedKey:   body.RawKey, // store will hash it
		Scopes:      body.Scopes,
		QuotaPerMin: body.QuotaPerMin,
		Active:      true,
		CreatedAt:   time.Now(),
	}
	if err := h.keys.CreateKey(r.Context(), key); err != nil {
		h.log.Error("create key", zap.Error(err))
		jsonError(w, http.StatusInternalServerError, bodyInternal)
		return
	}
	// Return the ID but never the hashed key.
	jsonOK(w, http.StatusCreated, map[string]any{"id": key.ID, "owner": key.Owner, "scopes": key.Scopes, "quota_per_min": key.QuotaPerMin})
}

func (h *Handler) listKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.keys.ListKeys(r.Context())
	if err != nil {
		h.log.Error("list keys", zap.Error(err))
		jsonError(w, http.StatusInternalServerError, bodyInternal)
		return
	}
	// Mask the hashed_key field.
	type safeKey struct {
		ID          string    `json:"id"`
		Owner       string    `json:"owner"`
		Scopes      []string  `json:"scopes"`
		QuotaPerMin int       `json:"quota_per_min"`
		Active      bool      `json:"active"`
		CreatedAt   time.Time `json:"created_at"`
	}
	out := make([]safeKey, len(keys))
	for i, k := range keys {
		out[i] = safeKey{k.ID, k.Owner, k.Scopes, k.QuotaPerMin, k.Active, k.CreatedAt}
	}
	jsonOK(w, http.StatusOK, out)
}

func (h *Handler) getKey(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	k, err := h.keys.GetKeyByID(r.Context(), id)
	if errors.Is(err, gateway.ErrNotFound) {
		jsonError(w, http.StatusNotFound, bodyNotFound)
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, bodyInternal)
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{
		"id": k.ID, "owner": k.Owner, "scopes": k.Scopes,
		"quota_per_min": k.QuotaPerMin, "active": k.Active, "created_at": k.CreatedAt,
	})
}

func (h *Handler) revokeKey(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.keys.RevokeKey(r.Context(), id); err != nil {
		h.log.Error("revoke key", zap.String("id", id), zap.Error(err))
		jsonError(w, http.StatusInternalServerError, bodyInternal)
		return
	}
	jsonOK(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// ---- admin: routes ----------------------------------------------------------

func (h *Handler) upsertRoute(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var body struct {
		PathPrefix    string `json:"path_prefix"`
		Upstream      string `json:"upstream"`
		StripPrefix   bool   `json:"strip_prefix"`
		AuthRequired  bool   `json:"auth_required"`
		RequiredScope string `json:"required_scope"`
		MaxBodyBytes  int64  `json:"max_body_bytes"`
		TimeoutSecs   int    `json:"timeout_secs"`
		Active        bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	if body.PathPrefix == "" || body.Upstream == "" {
		jsonError(w, http.StatusBadRequest, bodyBadRequest)
		return
	}
	// Validate upstream is a parseable URL.
	if _, err := url.ParseRequestURI(body.Upstream); err != nil {
		jsonError(w, http.StatusBadRequest, []byte(`{"error":"invalid upstream URL"}`))
		return
	}
	route := &gateway.Route{
		ID:            id,
		PathPrefix:    body.PathPrefix,
		Upstream:      body.Upstream,
		StripPrefix:   body.StripPrefix,
		AuthRequired:  body.AuthRequired,
		RequiredScope: body.RequiredScope,
		MaxBodyBytes:  body.MaxBodyBytes,
		TimeoutSecs:   body.TimeoutSecs,
		Active:        body.Active,
		UpdatedAt:     time.Now(),
	}
	if err := h.routes.UpsertRoute(r.Context(), route); err != nil {
		h.log.Error("upsert route", zap.String("id", id), zap.Error(err))
		jsonError(w, http.StatusInternalServerError, bodyInternal)
		return
	}
	// Reload in-process router immediately.
	if err := h.gw.ReloadRoutes(r.Context()); err != nil {
		h.log.Warn("route reload failed after upsert", zap.Error(err))
	}
	jsonOK(w, http.StatusOK, route)
}

func (h *Handler) listRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := h.routes.ListRoutes(r.Context())
	if err != nil {
		h.log.Error("list routes", zap.Error(err))
		jsonError(w, http.StatusInternalServerError, bodyInternal)
		return
	}
	if routes == nil {
		routes = []*gateway.Route{}
	}
	jsonOK(w, http.StatusOK, routes)
}

func (h *Handler) getRouteHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	route, err := h.routes.GetRoute(r.Context(), id)
	if errors.Is(err, gateway.ErrNotFound) {
		jsonError(w, http.StatusNotFound, bodyNotFound)
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, bodyInternal)
		return
	}
	jsonOK(w, http.StatusOK, route)
}

func (h *Handler) deleteRoute(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.routes.DeleteRoute(r.Context(), id); err != nil {
		h.log.Error("delete route", zap.String("id", id), zap.Error(err))
		jsonError(w, http.StatusInternalServerError, bodyInternal)
		return
	}
	if err := h.gw.ReloadRoutes(r.Context()); err != nil {
		h.log.Warn("route reload failed after delete", zap.Error(err))
	}
	jsonOK(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---- admin: stats -----------------------------------------------------------

func (h *Handler) quotaStats(w http.ResponseWriter, r *http.Request) {
	keyID := mux.Vars(r)["key_id"]
	k, err := h.keys.GetKeyByID(r.Context(), keyID)
	if errors.Is(err, gateway.ErrNotFound) {
		jsonError(w, http.StatusNotFound, bodyNotFound)
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, bodyInternal)
		return
	}
	rem, err := h.limiter.Remaining(r.Context(), keyID, k.QuotaPerMin)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, bodyInternal)
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{
		"key_id":        keyID,
		"quota_per_min": k.QuotaPerMin,
		"remaining":     rem,
	})
}

// ---- helpers ----------------------------------------------------------------

func jsonOK(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func mapError(err error) (int, []byte) {
	switch {
	case errors.Is(err, gateway.ErrNotFound):
		return http.StatusNotFound, bodyNotFound
	case errors.Is(err, gateway.ErrUnauthorized):
		return http.StatusUnauthorized, bodyUnauth
	case errors.Is(err, gateway.ErrForbidden):
		return http.StatusForbidden, bodyForbidden
	case errors.Is(err, gateway.ErrRateLimited):
		return http.StatusTooManyRequests, bodyRateLimited
	case errors.Is(err, gateway.ErrPayloadTooLarge):
		return http.StatusRequestEntityTooLarge, bodyTooLarge
	default:
		return http.StatusInternalServerError, bodyInternal
	}
}

// statusRecorder captures the HTTP status code written by a handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// proxyBufferPool satisfies httputil.BufferPool using sync.Pool to avoid
// per-request allocations in the reverse proxy.
type proxyBufferPool struct{}

func (proxyBufferPool) Get() []byte {
	return *bufPool.Get().(*[]byte)
}
func (proxyBufferPool) Put(b []byte) {
	bp := &b
	bufPool.Put(bp)
}

// Ensure io.Discard is used to avoid lint warnings on import.
var _ = io.Discard

// requestIDFromHeader extracts X-Request-ID or falls back to the provided id.
func requestIDFromHeader(r *http.Request, fallback string) string {
	if id := r.Header.Get("X-Request-ID"); id != "" && !strings.ContainsAny(id, "\r\n") {
		return id
	}
	return fallback
}

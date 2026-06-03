// Package api implements the HTTP transport layer for the caching system.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/ankitsriv89/09-caching-system/cache"
	"github.com/ankitsriv89/09-caching-system/metrics"
	"github.com/ankitsriv89/09-caching-system/store"
)

// Handler wires HTTP routes to the cache and AOF store.
type Handler struct {
	cache *cache.Cache
	aof   *store.AOF
	met   *metrics.Metrics
	log   *zap.Logger
}

// New creates a Handler.
func New(c *cache.Cache, aof *store.AOF, met *metrics.Metrics, log *zap.Logger) *Handler {
	return &Handler{cache: c, aof: aof, met: met, log: log}
}

// Register mounts all routes onto r.
func (h *Handler) Register(r *mux.Router) {
	r.HandleFunc("/healthz", h.healthz).Methods(http.MethodGet)
	r.HandleFunc("/v1/cache/{key}", h.getKey).Methods(http.MethodGet)
	r.HandleFunc("/v1/cache/{key}", h.setKey).Methods(http.MethodPut)
	r.HandleFunc("/v1/cache/{key}", h.deleteKey).Methods(http.MethodDelete)
	r.HandleFunc("/v1/cache", h.listKeys).Methods(http.MethodGet)
	r.HandleFunc("/v1/cache", h.flushAll).Methods(http.MethodDelete)
	r.HandleFunc("/v1/stats", h.stats).Methods(http.MethodGet)
	r.HandleFunc("/v1/entries", h.entries).Methods(http.MethodGet)

	// Static + SPA
	r.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("web"))),
	)
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})
}

// ─── request / response types ────────────────────────────────────────────────

type setRequest struct {
	Value string `json:"value"`
	TTLMs int64  `json:"ttl_ms"` // 0 = use cache default, -1 = no expiry
}

type getResponse struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	AccessCount int64     `json:"access_count"`
}

type errBody struct {
	Error string `json:"error"`
}

// ─── pre-built static responses ──────────────────────────────────────────────

var (
	healthOK  = []byte(`{"status":"ok"}`)
	notFound  = []byte(`{"error":"key not found"}`)
	deletedOK = []byte(`{"deleted":true}`)
)

// ─── handlers ─────────────────────────────────────────────────────────────────

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(healthOK)
}

func (h *Handler) getKey(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]
	start := time.Now()

	v, ok := h.cache.Get(key)
	h.met.LatencyGet.Observe(time.Since(start).Seconds())

	if !ok {
		h.met.Misses.Inc()
		jsonErr(w, "key not found", http.StatusNotFound)
		return
	}
	h.met.Hits.Inc()

	entry, _ := h.cache.Peek(key)
	jsonOK(w, getResponse{
		Key:         key,
		Value:       v,
		ExpiresAt:   entry.ExpiresAt,
		AccessCount: entry.AccessCount,
	})
}

func (h *Handler) setKey(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]

	var req setRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if key == "" || req.Value == "" {
		jsonErr(w, "key and value are required", http.StatusBadRequest)
		return
	}

	// honour optional ?ttl_ms= query param as override
	if ms := r.URL.Query().Get("ttl_ms"); ms != "" {
		if v, err := strconv.ParseInt(ms, 10, 64); err == nil {
			req.TTLMs = v
		}
	}

	ttl := time.Duration(req.TTLMs) * time.Millisecond

	start := time.Now()
	h.cache.Set(key, req.Value, ttl)
	h.met.LatencySet.Observe(time.Since(start).Seconds())
	h.met.Sets.Inc()

	// persist to AOF; errors are non-fatal (cache still serves)
	var expiresAt time.Time
	if req.TTLMs > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	if err := h.aof.AppendSet(key, req.Value, req.TTLMs, expiresAt); err != nil {
		h.met.AOFErrors.Inc()
		h.log.Warn("aof set error", zap.String("key", key), zap.Error(err))
	}

	// sync Prometheus gauges
	s := h.cache.Stats()
	h.met.MemoryBytes.Set(float64(s.MemoryBytes))
	h.met.KeyCount.Set(float64(s.Keys))

	entry, _ := h.cache.Peek(key)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}

func (h *Handler) deleteKey(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]
	if !h.cache.Delete(key) {
		jsonErr(w, "key not found", http.StatusNotFound)
		return
	}
	h.met.Deletes.Inc()

	if err := h.aof.AppendDelete(key); err != nil {
		h.met.AOFErrors.Inc()
		h.log.Warn("aof delete error", zap.String("key", key), zap.Error(err))
	}

	s := h.cache.Stats()
	h.met.MemoryBytes.Set(float64(s.MemoryBytes))
	h.met.KeyCount.Set(float64(s.Keys))

	w.Header().Set("Content-Type", "application/json")
	w.Write(deletedOK)
}

func (h *Handler) listKeys(w http.ResponseWriter, _ *http.Request) {
	jsonOK(w, h.cache.Keys())
}

func (h *Handler) flushAll(w http.ResponseWriter, _ *http.Request) {
	n := h.cache.Flush()
	if err := h.aof.AppendFlush(); err != nil {
		h.met.AOFErrors.Inc()
		h.log.Warn("aof flush error", zap.Error(err))
	}
	s := h.cache.Stats()
	h.met.MemoryBytes.Set(float64(s.MemoryBytes))
	h.met.KeyCount.Set(float64(s.Keys))
	jsonOK(w, map[string]int{"flushed": n})
}

func (h *Handler) stats(w http.ResponseWriter, _ *http.Request) {
	s := h.cache.Stats()
	h.met.MemoryBytes.Set(float64(s.MemoryBytes))
	h.met.KeyCount.Set(float64(s.Keys))
	jsonOK(w, s)
}

func (h *Handler) entries(w http.ResponseWriter, _ *http.Request) {
	jsonOK(w, h.cache.Entries())
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errBody{Error: msg})
}

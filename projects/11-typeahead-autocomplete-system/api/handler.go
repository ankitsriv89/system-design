// Package api provides HTTP handlers for the typeahead autocomplete service.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/ankitsriv89/11-typeahead-autocomplete-system/autocomplete"
	"github.com/ankitsriv89/11-typeahead-autocomplete-system/metrics"
	"github.com/ankitsriv89/11-typeahead-autocomplete-system/worker"
)

// Rebuilder is the subset of worker.Rebuilder the handler needs.
type Rebuilder interface {
	TriggerRebuild(ctx interface{ Done() <-chan struct{} }) (*autocomplete.IndexStats, error)
}

var (
	jsonPool = sync.Pool{New: func() interface{} { return new(strings.Builder) }}

	healthzBody = []byte(`{"status":"ok"}`)
)

// Handler holds dependencies for all HTTP handlers.
type Handler struct {
	store    autocomplete.Store
	rebuilder *worker.Rebuilder
	log      *zap.Logger
}

// New constructs the handler and returns a fully configured *mux.Router.
func New(store autocomplete.Store, rebuilder *worker.Rebuilder, log *zap.Logger) *mux.Router {
	h := &Handler{store: store, rebuilder: rebuilder, log: log}

	r := mux.NewRouter()
	r.Use(h.instrumentMiddleware)

	// Static assets and UI
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web"))))
	r.HandleFunc("/", h.serveIndex).Methods(http.MethodGet)

	// Core APIs
	r.HandleFunc("/v1/suggest", h.suggest).Methods(http.MethodGet)
	r.HandleFunc("/v1/corpus/items", h.addItem).Methods(http.MethodPost)
	r.HandleFunc("/v1/corpus/items", h.listItems).Methods(http.MethodGet)
	r.HandleFunc("/v1/corpus/items/{id}", h.getItem).Methods(http.MethodGet)
	r.HandleFunc("/v1/corpus/items/{id}", h.deleteItem).Methods(http.MethodDelete)
	r.HandleFunc("/v1/feedback/click", h.recordClick).Methods(http.MethodPost)

	// Admin
	r.HandleFunc("/v1/admin/rebuild-index", h.rebuildIndex).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/stats", h.getStats).Methods(http.MethodGet)

	// Observability
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/healthz", h.healthz).Methods(http.MethodGet)

	return r
}

func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(healthzBody)
}

// suggest handles GET /v1/suggest?q=<prefix>&locale=<locale>&limit=<n>
func (h *Handler) suggest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q := r.URL.Query()
	rawPrefix := q.Get("q")
	locale := q.Get("locale")
	if locale == "" {
		locale = "en"
	}
	limitStr := q.Get("limit")
	limit := 10
	if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
		limit = n
	}

	prefix := autocomplete.NormalizePrefix(rawPrefix)
	if prefix == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"suggestions": []*autocomplete.Suggestion{}})
		return
	}

	suggestions, err := h.store.Suggest(r.Context(), prefix, locale, limit)
	if err != nil {
		h.log.Error("api: suggest failed", zap.String("prefix", prefix), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "suggest failed")
		return
	}

	metrics.SuggestLatency.WithLabelValues(locale).Observe(time.Since(start).Seconds())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"prefix":      prefix,
		"locale":      locale,
		"suggestions": suggestions,
		"latency_ms":  time.Since(start).Milliseconds(),
	})
}

// addItem handles POST /v1/corpus/items
func (h *Handler) addItem(w http.ResponseWriter, r *http.Request) {
	var item autocomplete.SuggestItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(item.Text) == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}
	if item.Locale == "" {
		item.Locale = "en"
	}
	id, err := h.store.AddItem(r.Context(), &item)
	if err != nil {
		h.log.Error("api: add item failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "add item failed")
		return
	}
	item.ID = id
	metrics.CorpusItems.Inc()
	writeJSON(w, http.StatusCreated, item)
}

// listItems handles GET /v1/corpus/items?locale=&limit=&offset=
func (h *Handler) listItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	locale := q.Get("locale")
	limit := 50
	offset := 0
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 {
		limit = n
	}
	if n, err := strconv.Atoi(q.Get("offset")); err == nil && n >= 0 {
		offset = n
	}
	items, err := h.store.ListItems(r.Context(), locale, limit, offset)
	if err != nil {
		h.log.Error("api: list items failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "list items failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items, "limit": limit, "offset": offset})
}

// getItem handles GET /v1/corpus/items/{id}
func (h *Handler) getItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := h.store.GetItem(r.Context(), id)
	if err != nil {
		h.log.Error("api: get item failed", zap.Int64("id", id), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "get item failed")
		return
	}
	if item == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// deleteItem handles DELETE /v1/corpus/items/{id}
func (h *Handler) deleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.store.DeleteItem(r.Context(), id); err != nil {
		h.log.Error("api: delete item failed", zap.Int64("id", id), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	metrics.CorpusItems.Dec()
	w.WriteHeader(http.StatusNoContent)
}

// recordClick handles POST /v1/feedback/click
func (h *Handler) recordClick(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prefix         string `json:"prefix"`
		SelectedItemID *int64 `json:"selected_item_id"`
		LatencyMS      int64  `json:"latency_ms"`
		Locale         string `json:"locale"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Locale == "" {
		req.Locale = "en"
	}
	log := &autocomplete.QueryLog{
		Prefix:         req.Prefix,
		SelectedItemID: req.SelectedItemID,
		LatencyMS:      req.LatencyMS,
		Locale:         req.Locale,
	}
	if err := h.store.RecordQuery(r.Context(), log); err != nil {
		h.log.Warn("api: record click failed", zap.Error(err))
	}
	// Bump popularity on the selected item.
	if req.SelectedItemID != nil {
		if err := h.store.IncrementPopularity(r.Context(), *req.SelectedItemID, 1.0); err != nil {
			h.log.Warn("api: increment popularity failed", zap.Int64("id", *req.SelectedItemID), zap.Error(err))
		}
	}
	metrics.ClickFeedback.Inc()
	w.WriteHeader(http.StatusNoContent)
}

// rebuildIndex handles POST /v1/admin/rebuild-index
func (h *Handler) rebuildIndex(w http.ResponseWriter, r *http.Request) {
	stats, err := h.rebuilder.TriggerRebuild(r.Context())
	if err != nil {
		h.log.Error("api: rebuild index failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "rebuild failed")
		return
	}
	metrics.RebuildTotal.WithLabelValues("success").Inc()
	metrics.RebuildDuration.Observe(float64(stats.RebuildDuration) / 1000.0)
	writeJSON(w, http.StatusOK, stats)
}

// getStats handles GET /v1/admin/stats
func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.GetIndexStats(r.Context())
	if err != nil {
		h.log.Error("api: get stats failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "stats failed")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// instrumentMiddleware records per-request Prometheus metrics.
func (h *Handler) instrumentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		path := sanitizePath(r.URL.Path)
		status := strconv.Itoa(rw.status)
		metrics.HTTPRequests.WithLabelValues(r.Method, path, status).Inc()
		metrics.HTTPLatency.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func sanitizePath(p string) string {
	// Collapse /v1/corpus/items/123 → /v1/corpus/items/{id} etc. to avoid cardinality explosion.
	if strings.HasPrefix(p, "/v1/corpus/items/") {
		return "/v1/corpus/items/{id}"
	}
	return p
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func parseID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

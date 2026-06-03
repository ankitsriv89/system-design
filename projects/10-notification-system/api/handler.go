// Package api implements the HTTP transport layer for the notification system.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/ankitsriv89/10-notification-system/metrics"
	"github.com/ankitsriv89/10-notification-system/notification"
	"github.com/ankitsriv89/10-notification-system/store"
	"github.com/ankitsriv89/10-notification-system/worker"
)

// Handler wires HTTP routes to domain logic.
type Handler struct {
	store      *store.Store
	dispatcher *worker.Dispatcher
	met        *metrics.Metrics
	log        *zap.Logger
}

// New creates a Handler.
func New(st *store.Store, d *worker.Dispatcher, met *metrics.Metrics, log *zap.Logger) *Handler {
	return &Handler{store: st, dispatcher: d, met: met, log: log}
}

// Register mounts all routes onto r.
func (h *Handler) Register(r *mux.Router) {
	r.Use(h.metricsMiddleware)

	r.HandleFunc("/healthz", h.healthz).Methods(http.MethodGet)

	// Notifications
	r.HandleFunc("/v1/notifications", h.createNotification).Methods(http.MethodPost)
	r.HandleFunc("/v1/notifications", h.listNotifications).Methods(http.MethodGet)
	r.HandleFunc("/v1/notifications/{id}", h.getNotification).Methods(http.MethodGet)
	r.HandleFunc("/v1/notifications/{id}/attempts", h.listAttempts).Methods(http.MethodGet)

	// Preferences
	r.HandleFunc("/v1/preferences/{user_id}", h.upsertPreferences).Methods(http.MethodPut)
	r.HandleFunc("/v1/preferences/{user_id}", h.getPreferences).Methods(http.MethodGet)

	// Templates
	r.HandleFunc("/v1/templates", h.createTemplate).Methods(http.MethodPost)
	r.HandleFunc("/v1/templates", h.listTemplates).Methods(http.MethodGet)

	// Admin / demo controls
	r.HandleFunc("/v1/admin/queue/stats", h.queueStats).Methods(http.MethodGet)
	r.HandleFunc("/v1/admin/provider/{name}/failure-rate", h.setFailureRate).Methods(http.MethodPut)

	// Static + SPA
	r.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("web"))),
	)
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})
}

// ── pre-built static responses ────────────────────────────────────────────────

var healthOK = []byte(`{"status":"ok"}`)

// ── middleware ────────────────────────────────────────────────────────────────

func (h *Handler) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		dur := float64(time.Since(start).Milliseconds())
		status := strconv.Itoa(rw.statusCode)
		path := r.URL.Path
		h.met.HTTPRequests.WithLabelValues(r.Method, path, status).Inc()
		h.met.HTTPDuration.WithLabelValues(r.Method, path).Observe(dur)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// ── handlers ─────────────────────────────────────────────────────────────────

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(healthOK)
}

// ── Notifications ─────────────────────────────────────────────────────────────

type createNotificationRequest struct {
	UserID         string            `json:"user_id"`
	Channel        string            `json:"channel"`
	TemplateID     string            `json:"template_id"`
	Params         map[string]string `json:"params"`
	Priority       int               `json:"priority"`
	IdempotencyKey string            `json:"idempotency_key"`
}

func (h *Handler) createNotification(w http.ResponseWriter, r *http.Request) {
	var req createNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.UserID == "" || req.Channel == "" {
		writeError(w, http.StatusBadRequest, "user_id and channel are required")
		return
	}

	ch := notification.Channel(req.Channel)
	switch ch {
	case notification.ChannelEmail, notification.ChannelSMS, notification.ChannelPush:
	default:
		writeError(w, http.StatusBadRequest, "channel must be email, sms, or push")
		return
	}

	n := &notification.Notification{
		UserID:         req.UserID,
		Channel:        ch,
		TemplateID:     req.TemplateID,
		Params:         req.Params,
		Priority:       notification.Priority(req.Priority),
		Status:         notification.StatusPending,
		IdempotencyKey: req.IdempotencyKey,
	}

	// Resolve template if provided
	if req.TemplateID != "" {
		tmpl, err := h.store.GetTemplate(r.Context(), req.TemplateID)
		if err != nil {
			h.log.Error("get template", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to load template")
			return
		}
		if tmpl == nil {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		n.Subject, n.Body = notification.RenderTemplate(tmpl, req.Params)
	}

	// Check preferences
	pref, err := h.store.GetPreference(r.Context(), req.UserID, ch)
	if err != nil {
		h.log.Warn("get preference error", zap.Error(err))
	}
	if pref != nil && !pref.Enabled {
		n.Status = notification.StatusSkipped
		if err := h.store.CreateNotification(r.Context(), n); err != nil {
			h.log.Error("create notification", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to create notification")
			return
		}
		writeJSON(w, http.StatusCreated, n)
		return
	}
	if pref != nil && notification.IsQuietHour(pref, time.Now()) {
		n.Status = notification.StatusSkipped
		if err := h.store.CreateNotification(r.Context(), n); err != nil {
			h.log.Error("create notification", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to create notification")
			return
		}
		writeJSON(w, http.StatusCreated, n)
		return
	}

	n.Status = notification.StatusQueued
	if err := h.store.CreateNotification(r.Context(), n); err != nil {
		h.log.Error("create notification", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create notification")
		return
	}

	if err := h.dispatcher.Enqueue(n); err != nil {
		h.log.Warn("enqueue failed", zap.Error(err), zap.String("notification_id", n.ID))
		writeError(w, http.StatusServiceUnavailable, "queue full, try again")
		return
	}
	h.met.Enqueued.WithLabelValues(string(ch)).Inc()

	writeJSON(w, http.StatusCreated, n)
}

func (h *Handler) getNotification(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	n, err := h.store.GetNotification(r.Context(), id)
	if err != nil {
		h.log.Error("get notification", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if n == nil {
		writeError(w, http.StatusNotFound, "notification not found")
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (h *Handler) listNotifications(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)

	notifications, err := h.store.ListNotifications(r.Context(), userID, limit, offset)
	if err != nil {
		h.log.Error("list notifications", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if notifications == nil {
		notifications = []*notification.Notification{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"notifications": notifications,
		"count":         len(notifications),
	})
}

func (h *Handler) listAttempts(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	attempts, err := h.store.ListAttempts(r.Context(), id)
	if err != nil {
		h.log.Error("list attempts", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if attempts == nil {
		attempts = []*notification.DeliveryAttempt{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"notification_id": id,
		"attempts":        attempts,
		"count":           len(attempts),
	})
}

// ── Preferences ───────────────────────────────────────────────────────────────

type prefRequest struct {
	Channel    string `json:"channel"`
	Enabled    bool   `json:"enabled"`
	QuietStart int    `json:"quiet_start"`
	QuietEnd   int    `json:"quiet_end"`
}

func (h *Handler) upsertPreferences(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["user_id"]
	var prefs []prefRequest
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	for _, p := range prefs {
		pref := &notification.Preference{
			UserID:     userID,
			Channel:    notification.Channel(p.Channel),
			Enabled:    p.Enabled,
			QuietStart: p.QuietStart,
			QuietEnd:   p.QuietEnd,
		}
		if err := h.store.UpsertPreference(r.Context(), pref); err != nil {
			h.log.Error("upsert preference", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to save preference")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getPreferences(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["user_id"]
	prefs, err := h.store.ListPreferences(r.Context(), userID)
	if err != nil {
		h.log.Error("list preferences", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if prefs == nil {
		prefs = []*notification.Preference{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"user_id": userID, "preferences": prefs})
}

// ── Templates ─────────────────────────────────────────────────────────────────

type createTemplateRequest struct {
	ID      string `json:"id"`
	Channel string `json:"channel"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (h *Handler) createTemplate(w http.ResponseWriter, r *http.Request) {
	var req createTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.ID == "" || req.Channel == "" || req.Body == "" {
		writeError(w, http.StatusBadRequest, "id, channel, and body are required")
		return
	}
	t := &notification.Template{
		ID:      req.ID,
		Channel: notification.Channel(req.Channel),
		Subject: req.Subject,
		Body:    req.Body,
	}
	if err := h.store.UpsertTemplate(r.Context(), t); err != nil {
		h.log.Error("upsert template", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to save template")
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) listTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.store.ListTemplates(r.Context())
	if err != nil {
		h.log.Error("list templates", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if templates == nil {
		templates = []*notification.Template{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"templates": templates})
}

// ── Admin ─────────────────────────────────────────────────────────────────────

func (h *Handler) queueStats(w http.ResponseWriter, r *http.Request) {
	counts, err := h.store.CountNotificationsByStatus(r.Context())
	if err != nil {
		h.log.Error("count by status", zap.Error(err))
		counts = map[string]int64{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"queue_depth": h.dispatcher.QueueLen(),
		"dlq_depth":   h.dispatcher.DLQLen(),
		"by_status":   counts,
	})
}

type failureRateRequest struct {
	Rate float64 `json:"rate"`
}

func (h *Handler) setFailureRate(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	var req failureRateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Rate < 0 || req.Rate > 1 {
		writeError(w, http.StatusBadRequest, "rate must be 0.0–1.0")
		return
	}
	ratePtr, ok := h.dispatcher.FailureRates[name]
	if !ok {
		writeError(w, http.StatusNotFound, "provider not found; valid: email, sms, push")
		return
	}
	*ratePtr = req.Rate
	writeJSON(w, http.StatusOK, map[string]interface{}{"provider": name, "failure_rate": req.Rate})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func queryInt(r *http.Request, key string, def int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return def
	}
	return v
}

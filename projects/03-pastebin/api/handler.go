// Package api implements the HTTP transport layer for the pastebin service.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/ankitsriv89/pastebin/metrics"
	"github.com/ankitsriv89/pastebin/paste"
)

const (
	maxBodyBytes    = paste.MaxSizeBytes + 4096 // headroom for JSON envelope
	rateLimitPerMin = 60
	rateLimitWindow = time.Minute
)

// pasteService is the subset of paste.Service the handler needs.
type pasteService interface {
	Create(ctx context.Context, req paste.CreateRequest) (*paste.Paste, error)
	Get(ctx context.Context, id, requesterID string) (*paste.Paste, []byte, error)
	Delete(ctx context.Context, id, requesterID string) error
}

// rateLimiter is the subset of paste.Cache used for rate limiting.
type rateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// Handler holds all HTTP handler methods.
type Handler struct {
	svc    pasteService
	rl     rateLimiter
	log    *zap.Logger
}

func New(svc pasteService, rl rateLimiter, log *zap.Logger) *Handler {
	return &Handler{svc: svc, rl: rl, log: log}
}

// Register wires all routes onto the router.
func (h *Handler) Register(r *mux.Router) {
	r.Handle("/metrics", metrics.Handler()).Methods(http.MethodGet)
	r.HandleFunc("/healthz", h.healthz).Methods(http.MethodGet)
	r.HandleFunc("/", h.index).Methods(http.MethodGet)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web"))))

	v1 := r.PathPrefix("/v1").Subrouter()
	v1.HandleFunc("/pastes", h.createPaste).Methods(http.MethodPost)
	v1.HandleFunc("/pastes/{id}", h.getPaste).Methods(http.MethodGet)
	v1.HandleFunc("/pastes/{id}/raw", h.getRaw).Methods(http.MethodGet)
	v1.HandleFunc("/pastes/{id}", h.deletePaste).Methods(http.MethodDelete)
}

// --- handlers ---

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

type createRequest struct {
	Title         string `json:"title"`
	Language      string `json:"language"`
	Visibility    string `json:"visibility"`
	Content       string `json:"content"`
	TTLSeconds    *int   `json:"ttl_seconds"`
	BurnAfterRead bool   `json:"burn_after_read"`
}

type pasteResponse struct {
	ID            string     `json:"id"`
	Title         string     `json:"title,omitempty"`
	Language      string     `json:"language,omitempty"`
	Visibility    string     `json:"visibility"`
	SizeBytes     int64      `json:"size_bytes"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	BurnAfterRead bool       `json:"burn_after_read"`
	CreatedAt     time.Time  `json:"created_at"`
}

type pasteWithContent struct {
	pasteResponse
	Content string `json:"content"`
}

func (h *Handler) createPaste(w http.ResponseWriter, r *http.Request) {
	if !h.rateLimit(w, r) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}

	var req createRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		h.writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	vis := paste.Visibility(req.Visibility)
	if vis == "" {
		vis = paste.VisibilityPublic
	}
	if vis != paste.VisibilityPublic && vis != paste.VisibilityUnlisted && vis != paste.VisibilityPrivate {
		h.writeError(w, http.StatusBadRequest, "visibility must be public, unlisted, or private")
		return
	}

	// Auth is not yet implemented. Private pastes and owner-scoped operations
	// are rejected until a verified identity (JWT / session) is wired in.
	// Accepting a spoofable X-Owner-ID header would allow any client to claim
	// any identity and read or delete another user's private pastes.
	if vis == paste.VisibilityPrivate {
		h.writeError(w, http.StatusNotImplemented, "private pastes require authentication, which is not yet implemented")
		return
	}

	creq := paste.CreateRequest{
		Title:         req.Title,
		Language:      req.Language,
		Visibility:    vis,
		Content:       []byte(req.Content),
		BurnAfterRead: req.BurnAfterRead,
	}
	if req.TTLSeconds != nil {
		d := time.Duration(*req.TTLSeconds) * time.Second
		creq.TTL = &d
	}

	p, err := h.svc.Create(r.Context(), creq)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	metrics.PastesCreated.Inc()
	metrics.PasteSizeBytes.Observe(float64(p.SizeBytes))

	h.writeJSON(w, http.StatusCreated, pasteWithContent{
		pasteResponse: toResponse(p),
		Content:       req.Content,
	})
}

func (h *Handler) getPaste(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	// Pass empty owner — no verified identity yet. The service will return
	// ErrForbidden for private pastes, which is the correct safe default.
	p, content, err := h.svc.Get(r.Context(), id, "")
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, pasteWithContent{
		pasteResponse: toResponse(p),
		Content:       string(content),
	})
}

func (h *Handler) getRaw(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	_, content, err := h.svc.Get(r.Context(), id, "")
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(content)
}

func (h *Handler) deletePaste(w http.ResponseWriter, r *http.Request) {
	// Delete is only permitted on anonymous pastes (owner_id = "") until auth is implemented.
	// Passing "" means the service allows deletion only when owner_id is blank.
	id := mux.Vars(r)["id"]
	if err := h.svc.Delete(r.Context(), id, ""); err != nil {
		h.handleServiceError(w, err)
		return
	}
	metrics.PastesDeleted.Inc()
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

func (h *Handler) rateLimit(w http.ResponseWriter, r *http.Request) bool {
	ip := realIP(r)
	allowed, err := h.rl.Allow(r.Context(), ip, rateLimitPerMin, rateLimitWindow)
	if err != nil {
		h.log.Warn("rate limiter error, failing open", zap.Error(err))
		return true
	}
	if !allowed {
		metrics.RateLimitRejections.Inc()
		h.writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return false
	}
	return true
}

func (h *Handler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, paste.ErrNotFound):
		h.writeError(w, http.StatusNotFound, "paste not found")
	case errors.Is(err, paste.ErrExpired):
		h.writeError(w, http.StatusGone, "paste has expired")
	case errors.Is(err, paste.ErrForbidden):
		h.writeError(w, http.StatusForbidden, "access denied")
	case errors.Is(err, paste.ErrTooLarge):
		h.writeError(w, http.StatusRequestEntityTooLarge, "paste exceeds 10 MB limit")
	case errors.Is(err, paste.ErrInvalidInput):
		h.writeError(w, http.StatusBadRequest, "invalid input")
	default:
		h.log.Error("internal error", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, map[string]string{"error": msg})
}

func toResponse(p *paste.Paste) pasteResponse {
	return pasteResponse{
		ID:            p.ID,
		Title:         p.Title,
		Language:      p.Language,
		Visibility:    string(p.Visibility),
		SizeBytes:     p.SizeBytes,
		ExpiresAt:     p.ExpiresAt,
		BurnAfterRead: p.BurnAfterRead,
		CreatedAt:     p.CreatedAt,
	}
}

func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.SplitN(ip, ",", 2)[0]
	}
	return r.RemoteAddr
}

// Package api wires together the HTTP transport layer for the URL shortener.
// It owns request parsing, response encoding, and the cache-aside redirect path.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/ankitsriv89/url-shortener/internal/analytics"
	"github.com/ankitsriv89/url-shortener/internal/link"
	"github.com/ankitsriv89/url-shortener/internal/metrics"
	"github.com/ankitsriv89/url-shortener/internal/store"
)

const (
	// activeCacheTTL is how long a known-good URL is cached in memory.
	activeCacheTTL = 10 * time.Minute
	// missingCacheTTL is how long a 404 is negatively cached to absorb thundering-herd on bad codes.
	missingCacheTTL = 30 * time.Second

	redirectRoute   = "redirect"
	createLinkRoute = "create_link"
	linkStatsRoute  = "link_stats"
)

// dbStore is the subset of the store that Handler needs.
// Keeping it narrow lets us swap implementations in tests without touching handler logic.
type dbStore interface {
	GetLink(ctx context.Context, code string) (link.Link, error)
	RecordClick(ctx context.Context, code, referrer, userAgent, remoteAddr string) error
	Stats(ctx context.Context, code string) (analytics.Stats, error)
}

// Handler holds all HTTP handler methods for the URL shortener API.
type Handler struct {
	service *Service
	db      dbStore
	cache   store.Cache
	baseURL string
	log     *zap.Logger
}

// New constructs a Handler. baseURL is the public root used to build short URLs (e.g. https://s.example.com).
func New(service *Service, db dbStore, cache store.Cache, baseURL string, log *zap.Logger) *Handler {
	return &Handler{
		service: service,
		db:      db,
		cache:   cache,
		baseURL: strings.TrimRight(baseURL, "/"),
		log:     log,
	}
}

// Routes registers all routes on the provided router.
func (h *Handler) Routes(r *mux.Router) {
	r.HandleFunc("/v1/links", h.CreateLink).Methods(http.MethodPost)
	r.HandleFunc("/v1/links/{code}/stats", h.Stats).Methods(http.MethodGet)
	r.HandleFunc("/healthz", h.Health).Methods(http.MethodGet)
	r.HandleFunc("/{code}", h.Redirect).Methods(http.MethodGet)
}

// createLinkRequest is the JSON body for POST /v1/links.
type createLinkRequest struct {
	LongURL   string     `json:"long_url"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// linkResponse is the JSON shape returned after a successful link creation.
type linkResponse struct {
	Code      string     `json:"code"`
	ShortURL  string     `json:"short_url"`
	LongURL   string     `json:"long_url"`
	OwnerID   string     `json:"owner_id"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateLink handles POST /v1/links.
// Requires the X-Owner-ID header to identify the requesting owner.
func (h *Handler) CreateLink(w http.ResponseWriter, r *http.Request) {
	var req createLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		record(createLinkRoute, http.StatusBadRequest)
		return
	}
	l, err := h.service.CreateLink(r.Context(), r.Header.Get("X-Owner-ID"), req.LongURL, req.ExpiresAt)
	if err != nil {
		h.writeCreateError(w, err)
		return
	}
	record(createLinkRoute, http.StatusCreated)
	writeJSON(w, http.StatusCreated, h.toResponse(l))
}

// Redirect handles GET /{code}.
// Implements cache-aside: checks in-memory cache first, falls back to the database.
// Clicks are recorded asynchronously so the redirect latency is not affected.
func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	code := mux.Vars(r)["code"]
	if !link.IsValidCode(code) {
		writeErr(w, http.StatusNotFound, "link not found")
		record(redirectRoute, http.StatusNotFound)
		return
	}

	start := time.Now()

	// Fast path: serve from cache.
	if longURL, missing, found, err := h.cache.GetURL(r.Context(), code); err == nil && found {
		if missing {
			// Negative cache hit — code was recently 404'd, avoid DB round-trip.
			metrics.CacheLookups.WithLabelValues("negative_hit").Inc()
			metrics.RedirectDuration.WithLabelValues("negative_hit").Observe(time.Since(start).Seconds())
			writeErr(w, http.StatusNotFound, "link not found")
			record(redirectRoute, http.StatusNotFound)
			return
		}
		metrics.CacheLookups.WithLabelValues("hit").Inc()
		metrics.RedirectDuration.WithLabelValues("hit").Observe(time.Since(start).Seconds())
		h.recordClick(r.Context(), r, code)
		http.Redirect(w, r, longURL, http.StatusFound)
		record(redirectRoute, http.StatusFound)
		return
	} else if err != nil {
		h.log.Warn("cache lookup failed", zap.String("code", code), zap.Error(err))
		metrics.CacheLookups.WithLabelValues("error").Inc()
	} else {
		metrics.CacheLookups.WithLabelValues("miss").Inc()
	}

	// Slow path: query the database.
	l, err := h.db.GetLink(r.Context(), code)
	if err != nil {
		if errors.Is(err, link.ErrNotFound) {
			// Negatively cache the miss so repeated requests for a bad code don't hammer the DB.
			_ = h.cache.SetMissing(r.Context(), code, missingCacheTTL)
			writeErr(w, http.StatusNotFound, "link not found")
			record(redirectRoute, http.StatusNotFound)
			return
		}
		h.log.Error("link lookup failed", zap.String("code", code), zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "lookup failed")
		record(redirectRoute, http.StatusInternalServerError)
		return
	}
	if err := l.Active(time.Now()); err != nil {
		// 410 Gone signals the resource existed but is no longer available (expired or disabled).
		writeErr(w, http.StatusGone, "link expired or disabled")
		record(redirectRoute, http.StatusGone)
		return
	}

	// Populate cache for subsequent requests.
	_ = h.cache.SetURL(r.Context(), code, l.LongURL, activeCacheTTL)
	metrics.RedirectDuration.WithLabelValues("miss").Observe(time.Since(start).Seconds())
	h.recordClick(r.Context(), r, code)
	http.Redirect(w, r, l.LongURL, http.StatusFound)
	record(redirectRoute, http.StatusFound)
}

// Stats handles GET /v1/links/{code}/stats.
// Only the owner of the link may retrieve its analytics.
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Header.Get("X-Owner-ID")
	if ownerID == "" {
		writeErr(w, http.StatusBadRequest, link.ErrOwnerRequired.Error())
		record(linkStatsRoute, http.StatusBadRequest)
		return
	}
	stats, err := h.db.Stats(r.Context(), mux.Vars(r)["code"])
	if err != nil {
		if errors.Is(err, link.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "link not found")
			record(linkStatsRoute, http.StatusNotFound)
			return
		}
		h.log.Error("stats lookup failed", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "stats lookup failed")
		record(linkStatsRoute, http.StatusInternalServerError)
		return
	}
	if stats.OwnerID != ownerID {
		writeErr(w, http.StatusForbidden, "owner cannot access this link")
		record(linkStatsRoute, http.StatusForbidden)
		return
	}
	record(linkStatsRoute, http.StatusOK)
	writeJSON(w, http.StatusOK, stats)
}

// Health handles GET /healthz. Returns 200 OK when the server is reachable.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeCreateError maps service-layer errors to appropriate HTTP status codes.
func (h *Handler) writeCreateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, link.ErrOwnerRequired), errors.Is(err, link.ErrInvalidURL):
		writeErr(w, http.StatusBadRequest, err.Error())
		record(createLinkRoute, http.StatusBadRequest)
	case errors.Is(err, link.ErrQuotaExceeded):
		writeErr(w, http.StatusTooManyRequests, err.Error())
		record(createLinkRoute, http.StatusTooManyRequests)
	case errors.Is(err, link.ErrCollision):
		writeErr(w, http.StatusServiceUnavailable, err.Error())
		record(createLinkRoute, http.StatusServiceUnavailable)
	default:
		h.log.Error("create link failed", zap.Error(err))
		writeErr(w, http.StatusInternalServerError, "failed to create link")
		record(createLinkRoute, http.StatusInternalServerError)
	}
}

// toResponse converts a domain link to the API wire format.
func (h *Handler) toResponse(l link.Link) linkResponse {
	return linkResponse{
		Code:      l.Code,
		ShortURL:  h.baseURL + "/" + l.Code,
		LongURL:   l.LongURL,
		OwnerID:   l.OwnerID,
		ExpiresAt: l.ExpiresAt,
		CreatedAt: l.CreatedAt,
	}
}

// recordClick fires a goroutine to persist the click event without blocking the redirect.
func (h *Handler) recordClick(ctx context.Context, r *http.Request, code string) {
	go func() {
		if err := h.db.RecordClick(ctx, code, r.Referer(), r.UserAgent(), r.RemoteAddr); err != nil {
			metrics.AnalyticsErrors.Inc()
			h.log.Warn("record click failed", zap.String("code", code), zap.Error(err))
		}
	}()
}

// writeJSON encodes v as JSON and writes the given HTTP status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr sends a JSON error response with the given status and message.
func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// record increments the HTTP request counter for the given route and status.
func record(route string, status int) {
	metrics.HTTPRequestsTotal.WithLabelValues(route, http.StatusText(status)).Inc()
}

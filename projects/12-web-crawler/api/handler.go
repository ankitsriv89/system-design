// Package api implements HTTP transport for the web crawler service.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/ankitsriv89/12-web-crawler/crawler"
	"github.com/ankitsriv89/12-web-crawler/metrics"
	"github.com/ankitsriv89/12-web-crawler/store"
)

var healthBody = []byte(`{"status":"ok"}`)

// Handler holds all HTTP handler dependencies.
type Handler struct {
	db    *store.DB
	cache *store.Cache
	log   *zap.Logger
}

// NewHandler constructs the Handler.
func NewHandler(db *store.DB, cache *store.Cache, log *zap.Logger) *Handler {
	return &Handler{db: db, cache: cache, log: log}
}

// Router builds and returns the gorilla/mux router.
func (h *Handler) Router() *mux.Router {
	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/healthz", h.healthz).Methods(http.MethodGet)

	r.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("web"))),
	)
	r.HandleFunc("/", h.serveIndex).Methods(http.MethodGet)

	v1 := r.PathPrefix("/v1").Subrouter()
	v1.HandleFunc("/crawl-jobs", h.createJob).Methods(http.MethodPost)
	v1.HandleFunc("/crawl-jobs", h.listJobs).Methods(http.MethodGet)
	v1.HandleFunc("/crawl-jobs/{id:[0-9]+}", h.getJob).Methods(http.MethodGet)
	v1.HandleFunc("/pages/{url_hash}", h.getPage).Methods(http.MethodGet)
	v1.HandleFunc("/pages", h.listPages).Methods(http.MethodGet)
	v1.HandleFunc("/frontier/stats", h.frontierStats).Methods(http.MethodGet)
	v1.HandleFunc("/frontier/enqueue", h.enqueueURL).Methods(http.MethodPost)

	return r
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(healthBody)
}

func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

type createJobRequest struct {
	SeedURL  string `json:"seed_url"`
	MaxDepth int    `json:"max_depth"`
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.SeedURL == "" {
		h.writeError(w, http.StatusBadRequest, "seed_url required")
		return
	}
	if req.MaxDepth <= 0 {
		req.MaxDepth = 2
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	norm, err := crawler.NormalizeURL(req.SeedURL, nil)
	if err != nil || norm == "" {
		h.writeError(w, http.StatusBadRequest, "invalid seed_url")
		return
	}

	job, err := h.db.CreateJob(ctx, norm, req.MaxDepth)
	if err != nil {
		h.log.Error("create job", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "create job failed")
		return
	}

	parsed, _ := url.Parse(norm)
	host := ""
	if parsed != nil {
		host = parsed.Host
	}
	if err := h.db.EnqueueURL(ctx, norm, host, 10, 0); err != nil {
		h.log.Warn("enqueue seed", zap.Error(err))
	} else {
		metrics.URLsEnqueued.Inc()
	}

	if err := h.db.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		h.log.Warn("update job status", zap.Error(err))
	}
	job.Status = "running"

	h.writeJSON(w, http.StatusCreated, job)
}

func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	jobs, err := h.db.ListJobs(ctx, 20)
	if err != nil {
		h.log.Error("list jobs", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "list jobs failed")
		return
	}
	h.writeJSON(w, http.StatusOK, jobs)
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	job, err := h.db.GetJob(ctx, id)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "job not found")
		return
	}
	h.writeJSON(w, http.StatusOK, job)
}

func (h *Handler) getPage(w http.ResponseWriter, r *http.Request) {
	urlHash := mux.Vars(r)["url_hash"]
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	pf, err := h.db.GetPageFetch(ctx, urlHash)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "page not found")
		return
	}
	h.writeJSON(w, http.StatusOK, pf)
}

func (h *Handler) listPages(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	pages, err := h.db.RecentFetches(ctx, 50)
	if err != nil {
		h.log.Error("list pages", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "list pages failed")
		return
	}
	h.writeJSON(w, http.StatusOK, pages)
}

func (h *Handler) frontierStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	stats, err := h.db.FrontierStats(ctx)
	if err != nil {
		h.log.Error("frontier stats", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "stats failed")
		return
	}
	seenCount, _ := h.cache.SeenCount(ctx)
	type resp struct {
		Frontier map[string]int64 `json:"frontier"`
		Seen     int64            `json:"seen_count"`
	}
	h.writeJSON(w, http.StatusOK, resp{Frontier: stats, Seen: seenCount})
}

type enqueueRequest struct {
	URL      string `json:"url"`
	Priority int    `json:"priority"`
}

func (h *Handler) enqueueURL(w http.ResponseWriter, r *http.Request) {
	var req enqueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	norm, err := crawler.NormalizeURL(req.URL, nil)
	if err != nil || norm == "" {
		h.writeError(w, http.StatusBadRequest, "invalid url")
		return
	}
	if req.Priority == 0 {
		req.Priority = 1
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	parsed, _ := url.Parse(norm)
	host := ""
	if parsed != nil {
		host = parsed.Host
	}
	if err := h.db.EnqueueURL(ctx, norm, host, req.Priority, 0); err != nil {
		h.log.Error("enqueue url", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "enqueue failed")
		return
	}
	metrics.URLsEnqueued.Inc()
	h.writeJSON(w, http.StatusCreated, map[string]string{"url": norm, "status": "enqueued"})
}

func (h *Handler) writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) writeError(w http.ResponseWriter, code int, msg string) {
	h.writeJSON(w, code, map[string]string{"error": msg})
}

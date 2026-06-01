// Package metrics registers and exposes Prometheus metrics for the pastebin service.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pastebin_http_requests_total",
		Help: "Total HTTP requests by method, path, and status.",
	}, []string{"method", "path", "status"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pastebin_http_duration_seconds",
		Help:    "HTTP request latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	PastesCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pastebin_pastes_created_total",
		Help: "Total pastes successfully created.",
	})

	PastesDeleted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pastebin_pastes_deleted_total",
		Help: "Total pastes deleted (manual + expiry sweep).",
	})

	PasteSizeBytes = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "pastebin_paste_size_bytes",
		Help:    "Distribution of paste sizes in bytes.",
		Buckets: prometheus.ExponentialBuckets(1024, 4, 8), // 1KB → 16MB
	})

	CacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pastebin_cache_hits_total",
		Help: "Cache hits and misses.",
	}, []string{"result"}) // hit | miss

	RateLimitRejections = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pastebin_rate_limit_rejections_total",
		Help: "Requests rejected by the rate limiter.",
	})

	ExpirySweepDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "pastebin_expiry_sweep_duration_seconds",
		Help:    "Time taken by the background expiry sweep.",
		Buckets: prometheus.DefBuckets,
	})

	ExpiredPastesRemoved = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pastebin_expired_pastes_removed_total",
		Help: "Total pastes removed by the expiry sweeper.",
	})
)

// Handler returns the Prometheus HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// Middleware records request count and latency for every HTTP handler.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		dur := time.Since(start).Seconds()
		status := strconv.Itoa(rw.status)
		HTTPRequests.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		HTTPDuration.WithLabelValues(r.Method, r.URL.Path).Observe(dur)
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

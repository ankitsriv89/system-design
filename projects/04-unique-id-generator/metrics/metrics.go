// Package metrics registers and exposes Prometheus metrics for the unique-id-generator service.
//
// Key signals to watch:
//   - ids_generated_total: throughput counter; should grow linearly under load.
//   - generation_duration_seconds: latency distribution; p99 should be sub-millisecond.
//   - clock_rollback_total: any non-zero value is an alert condition.
//   - sequence_exhaustions_total: exhaustion means >4096 IDs were requested in one
//     millisecond on this worker; a non-zero value means throughput is near ceiling.
//   - lease_renewals_total / lease_failures_total: lease health.
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
	// HTTPRequests counts all HTTP requests by method, path, and status code.
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "uniqueid_http_requests_total",
		Help: "Total HTTP requests by method, path, and status.",
	}, []string{"method", "path", "status"})

	// HTTPDuration tracks HTTP request latency.
	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "uniqueid_http_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	// IDsGenerated counts successfully generated IDs (single and batch).
	IDsGenerated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uniqueid_ids_generated_total",
		Help: "Total unique IDs generated.",
	})

	// GenerationDuration tracks the time spent inside Next() / Batch().
	GenerationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "uniqueid_generation_duration_seconds",
		Help:    "Time spent generating a single ID or a batch.",
		Buckets: []float64{0.000001, 0.000005, 0.00001, 0.00005, 0.0001, 0.0005, 0.001, 0.005, 0.01},
	})

	// ClockRollbacks counts backward clock drift events. Any non-zero value
	// should trigger an alert — it means NTP is misbehaving or the host is
	// migrating between hypervisor clocks.
	ClockRollbacks = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uniqueid_clock_rollback_total",
		Help: "Number of clock rollback events detected.",
	})

	// ClockDriftMs tracks the magnitude of each clock rollback in milliseconds.
	ClockDriftMs = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "uniqueid_clock_drift_ms",
		Help:    "Magnitude of clock rollback events in milliseconds.",
		Buckets: []float64{1, 5, 10, 50, 100, 500, 1000},
	})

	// SequenceExhaustions counts how often a generator had to spin waiting
	// for the next millisecond because all 4096 sequence slots were used.
	SequenceExhaustions = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uniqueid_sequence_exhaustions_total",
		Help: "Number of times the per-millisecond sequence space was exhausted.",
	})

	// LeaseRenewals counts successful lease renewals.
	LeaseRenewals = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uniqueid_lease_renewals_total",
		Help: "Total successful lease renewals.",
	})

	// LeaseFailures counts failed lease renewals. A rising value means the
	// worker may lose its ID and must restart.
	LeaseFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uniqueid_lease_failures_total",
		Help: "Total failed lease renewals.",
	})
)

// Handler returns the Prometheus HTTP metrics handler.
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

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

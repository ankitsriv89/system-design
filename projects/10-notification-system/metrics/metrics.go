// Package metrics registers Prometheus metrics for the notification system.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus instruments.
type Metrics struct {
	// Dispatch pipeline
	Enqueued        *prometheus.CounterVec
	Delivered       *prometheus.CounterVec
	Failed          *prometheus.CounterVec
	Retries         *prometheus.CounterVec
	DLQ             *prometheus.CounterVec
	QueueDepth      prometheus.Gauge
	DeliveryLatency *prometheus.HistogramVec

	// HTTP
	HTTPRequests *prometheus.CounterVec
	HTTPDuration *prometheus.HistogramVec

	// Store
	DBErrors prometheus.Counter
}

// New registers and returns all metrics.
func New(reg prometheus.Registerer) *Metrics {
	f := promauto.With(reg)
	return &Metrics{
		Enqueued: f.NewCounterVec(prometheus.CounterOpts{
			Name: "notification_system_enqueued_total",
			Help: "Total notifications accepted and enqueued.",
		}, []string{"channel"}),

		Delivered: f.NewCounterVec(prometheus.CounterOpts{
			Name: "notification_system_delivered_total",
			Help: "Total notifications delivered successfully.",
		}, []string{"channel"}),

		Failed: f.NewCounterVec(prometheus.CounterOpts{
			Name: "notification_system_failed_total",
			Help: "Total delivery failures (before retry exhaustion).",
		}, []string{"channel"}),

		Retries: f.NewCounterVec(prometheus.CounterOpts{
			Name: "notification_system_retries_total",
			Help: "Total retry attempts.",
		}, []string{"channel"}),

		DLQ: f.NewCounterVec(prometheus.CounterOpts{
			Name: "notification_system_dlq_total",
			Help: "Total notifications moved to the dead-letter queue.",
		}, []string{"channel"}),

		QueueDepth: f.NewGauge(prometheus.GaugeOpts{
			Name: "notification_system_queue_depth",
			Help: "Current number of jobs in the dispatch queue.",
		}),

		DeliveryLatency: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "notification_system_delivery_latency_ms",
			Help:    "Provider send latency in milliseconds.",
			Buckets: []float64{10, 25, 50, 100, 250, 500, 1000},
		}, []string{"channel"}),

		HTTPRequests: f.NewCounterVec(prometheus.CounterOpts{
			Name: "notification_system_http_requests_total",
			Help: "Total HTTP requests by method and path.",
		}, []string{"method", "path", "status"}),

		HTTPDuration: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "notification_system_http_duration_ms",
			Help:    "HTTP request duration in milliseconds.",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250},
		}, []string{"method", "path"}),

		DBErrors: f.NewCounter(prometheus.CounterOpts{
			Name: "notification_system_db_errors_total",
			Help: "Total database errors.",
		}),
	}
}

// Package metrics registers and exposes Prometheus metrics for the key-value store.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus instruments for the key-value store.
type Metrics struct {
	opDuration *prometheus.HistogramVec
	opTotal    *prometheus.CounterVec
}

// New registers all metrics with the default Prometheus registry and returns
// the Metrics handle.  Call once at startup.
func New() *Metrics {
	return &Metrics{
		opDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "basic_key_value_store",
			Name:      "operation_duration_seconds",
			Help:      "Duration of KV operations in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"op", "result"}),

		opTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "basic_key_value_store",
			Name:      "operations_total",
			Help:      "Total number of KV operations.",
		}, []string{"op", "result"}),
	}
}

// RecordOp records one completed operation with its result label and duration.
func (m *Metrics) RecordOp(op, result string, durSeconds float64) {
	m.opDuration.WithLabelValues(op, result).Observe(durSeconds)
	m.opTotal.WithLabelValues(op, result).Inc()
}

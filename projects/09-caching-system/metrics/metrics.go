// Package metrics registers Prometheus metrics for the caching system.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus instruments.
type Metrics struct {
	Hits        prometheus.Counter
	Misses      prometheus.Counter
	Evictions   *prometheus.CounterVec
	Sets        prometheus.Counter
	Deletes     prometheus.Counter
	MemoryBytes prometheus.Gauge
	KeyCount    prometheus.Gauge
	LatencySet  prometheus.Histogram
	LatencyGet  prometheus.Histogram
	AOFErrors   prometheus.Counter
}

// New registers and returns a Metrics instance.
func New(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	f := promauto.With(reg)

	return &Metrics{
		Hits: f.NewCounter(prometheus.CounterOpts{
			Namespace: "caching_system",
			Name:      "hits_total",
			Help:      "Total cache hits.",
		}),
		Misses: f.NewCounter(prometheus.CounterOpts{
			Namespace: "caching_system",
			Name:      "misses_total",
			Help:      "Total cache misses.",
		}),
		Evictions: f.NewCounterVec(prometheus.CounterOpts{
			Namespace: "caching_system",
			Name:      "evictions_total",
			Help:      "Total evictions by reason.",
		}, []string{"reason"}),
		Sets: f.NewCounter(prometheus.CounterOpts{
			Namespace: "caching_system",
			Name:      "sets_total",
			Help:      "Total SET operations.",
		}),
		Deletes: f.NewCounter(prometheus.CounterOpts{
			Namespace: "caching_system",
			Name:      "deletes_total",
			Help:      "Total DELETE operations.",
		}),
		MemoryBytes: f.NewGauge(prometheus.GaugeOpts{
			Namespace: "caching_system",
			Name:      "memory_bytes",
			Help:      "Approximate bytes used by cached entries.",
		}),
		KeyCount: f.NewGauge(prometheus.GaugeOpts{
			Namespace: "caching_system",
			Name:      "key_count",
			Help:      "Current number of keys in the cache.",
		}),
		LatencySet: f.NewHistogram(prometheus.HistogramOpts{
			Namespace: "caching_system",
			Name:      "set_duration_seconds",
			Help:      "Latency of SET operations.",
			Buckets:   prometheus.ExponentialBuckets(0.00001, 2, 14),
		}),
		LatencyGet: f.NewHistogram(prometheus.HistogramOpts{
			Namespace: "caching_system",
			Name:      "get_duration_seconds",
			Help:      "Latency of GET operations.",
			Buckets:   prometheus.ExponentialBuckets(0.00001, 2, 14),
		}),
		AOFErrors: f.NewCounter(prometheus.CounterOpts{
			Namespace: "caching_system",
			Name:      "aof_errors_total",
			Help:      "Total errors writing to AOF log.",
		}),
	}
}

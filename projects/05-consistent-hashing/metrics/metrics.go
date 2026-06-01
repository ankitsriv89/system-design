// Package metrics registers all Prometheus metrics for the consistent-hashing service.
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	RingOpsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "consistent_hashing_ring_ops_total",
		Help: "Total number of ring mutation operations (add_node, remove_node, create_ring).",
	}, []string{"ring_id", "op"})

	LookupDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "consistent_hashing_lookup_duration_seconds",
		Help:    "Latency of key lookup operations.",
		Buckets: prometheus.ExponentialBuckets(0.000001, 2, 20),
	}, []string{"ring_id"})

	NodeCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "consistent_hashing_node_count",
		Help: "Current number of physical nodes in the ring.",
	}, []string{"ring_id"})

	VNodeCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "consistent_hashing_vnode_count",
		Help: "Current number of virtual nodes in the ring.",
	}, []string{"ring_id"})

	RingStdDev = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "consistent_hashing_ring_stddev",
		Help: "Standard deviation of arc lengths (0 = perfect balance).",
	}, []string{"ring_id"})

	KeyMovementPct = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "consistent_hashing_key_movement_pct",
		Help: "Estimated percentage of keys that moved on last topology change.",
	}, []string{"ring_id"})
)

func init() {
	prometheus.MustRegister(
		RingOpsTotal,
		LookupDuration,
		NodeCount,
		VNodeCount,
		RingStdDev,
		KeyMovementPct,
	)
}

// Package metrics registers Prometheus metrics for the load balancer.
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	RequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "load_balancer",
		Name:      "requests_total",
		Help:      "Total proxied requests by service and backend.",
	}, []string{"service", "backend", "code"})

	RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "load_balancer",
		Name:      "request_duration_seconds",
		Help:      "Proxied request latency distribution.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"service", "backend"})

	ActiveConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "load_balancer",
		Name:      "active_connections",
		Help:      "Current active connections per backend.",
	}, []string{"service", "backend"})

	BackendHealthy = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "load_balancer",
		Name:      "backend_healthy",
		Help:      "1 if backend is healthy, 0 otherwise.",
	}, []string{"service", "backend"})

	HealthCheckTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "load_balancer",
		Name:      "health_checks_total",
		Help:      "Total health check probes by service, backend, and result.",
	}, []string{"service", "backend", "result"})

	RetryTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "load_balancer",
		Name:      "retries_total",
		Help:      "Total request retries by service.",
	}, []string{"service"})
)

func init() {
	prometheus.MustRegister(
		RequestsTotal,
		RequestDuration,
		ActiveConnections,
		BackendHealthy,
		HealthCheckTotal,
		RetryTotal,
	)
}

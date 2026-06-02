// Package metrics registers Prometheus metrics for the API gateway.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all registered Prometheus collectors.
type Metrics struct {
	RequestTotal    *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	UpstreamErrors  *prometheus.CounterVec
	ActiveRoutes    prometheus.Gauge
	ActiveKeys      prometheus.Gauge
}

// New registers and returns all gateway metrics.
func New(reg prometheus.Registerer) *Metrics {
	factory := promauto.With(reg)
	return &Metrics{
		RequestTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "api_gateway_requests_total",
			Help: "Total proxied requests by route and HTTP status text.",
		}, []string{"route", "status"}),

		RequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "api_gateway_request_duration_seconds",
			Help:    "End-to-end request latency by route.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route"}),

		UpstreamErrors: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "api_gateway_upstream_errors_total",
			Help: "Total upstream errors by route.",
		}, []string{"route"}),

		ActiveRoutes: factory.NewGauge(prometheus.GaugeOpts{
			Name: "api_gateway_active_routes",
			Help: "Number of active proxy routes loaded in memory.",
		}),

		ActiveKeys: factory.NewGauge(prometheus.GaugeOpts{
			Name: "api_gateway_active_keys",
			Help: "Number of active API keys.",
		}),
	}
}

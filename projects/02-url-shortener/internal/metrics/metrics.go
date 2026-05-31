package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "urlshortener_http_requests_total",
		Help: "Total HTTP requests by route and status",
	}, []string{"route", "status"})

	RedirectDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "urlshortener_redirect_duration_seconds",
		Help:    "Redirect lookup latency",
		Buckets: prometheus.DefBuckets,
	}, []string{"cache"})

	CacheLookups = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "urlshortener_cache_lookups_total",
		Help: "Redis cache lookup results",
	}, []string{"result"})

	LinksCreated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "urlshortener_links_created_total",
		Help: "Links created by owner",
	}, []string{"owner_id"})

	AnalyticsErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "urlshortener_analytics_errors_total",
		Help: "Failed click event writes",
	})
)

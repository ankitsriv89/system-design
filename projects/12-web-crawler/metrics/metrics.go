// Package metrics registers Prometheus metrics for the web crawler service.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	URLsEnqueued = promauto.NewCounter(prometheus.CounterOpts{
		Name: "web_crawler_urls_enqueued_total",
		Help: "Total URLs added to the frontier.",
	})
	URLsFetched = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "web_crawler_urls_fetched_total",
		Help: "Total fetch attempts by status.",
	}, []string{"status"})
	FetchDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "web_crawler_fetch_duration_seconds",
		Help:    "HTTP fetch latency.",
		Buckets: prometheus.DefBuckets,
	})
	RobotsHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "web_crawler_robots_total",
		Help: "robots.txt cache decisions.",
	}, []string{"result"}) // hit / miss / disallowed
	WorkerQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "web_crawler_frontier_pending",
		Help: "Estimated pending URLs in the frontier.",
	})
	DedupeHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "web_crawler_dedupe_hits_total",
		Help: "URLs skipped because already seen.",
	})
	LinksExtracted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "web_crawler_links_extracted_total",
		Help: "Total outbound links extracted from pages.",
	})
)

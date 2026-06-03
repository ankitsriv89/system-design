// Package metrics registers Prometheus metrics for the typeahead service.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	SuggestRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "typeahead_autocomplete_suggest_requests_total",
		Help: "Total suggest requests by locale and cache source (redis|postgres).",
	}, []string{"locale", "source"})

	SuggestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "typeahead_autocomplete_suggest_latency_seconds",
		Help:    "Suggest endpoint latency in seconds.",
		Buckets: []float64{.001, .002, .005, .010, .025, .050, .100, .250, .500},
	}, []string{"locale"})

	CorpusItems = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "typeahead_autocomplete_corpus_items_total",
		Help: "Current count of items in the corpus.",
	})

	RebuildDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "typeahead_autocomplete_rebuild_duration_seconds",
		Help:    "Full index rebuild duration in seconds.",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
	})

	RebuildTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "typeahead_autocomplete_rebuilds_total",
		Help: "Total index rebuilds by result (success|failure).",
	}, []string{"result"})

	ClickFeedback = promauto.NewCounter(prometheus.CounterOpts{
		Name: "typeahead_autocomplete_click_feedback_total",
		Help: "Total click-through feedback events recorded.",
	})

	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "typeahead_autocomplete_http_requests_total",
		Help: "Total HTTP requests by method, path, and status.",
	}, []string{"method", "path", "status"})

	HTTPLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "typeahead_autocomplete_http_latency_seconds",
		Help:    "HTTP handler latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)

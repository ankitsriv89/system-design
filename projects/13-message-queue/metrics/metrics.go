// Package metrics registers and exposes Prometheus metrics for the message queue.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MessagesPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "message_queue_messages_published_total",
		Help: "Total messages published, labelled by topic and partition.",
	}, []string{"topic", "partition"})

	MessagesPolled = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "message_queue_messages_polled_total",
		Help: "Total messages polled, labelled by topic and consumer_group.",
	}, []string{"topic", "consumer_group"})

	MessagesAcked = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "message_queue_messages_acked_total",
		Help: "Total messages acknowledged, labelled by topic and consumer_group.",
	}, []string{"topic", "consumer_group"})

	MessagesDeadLettered = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "message_queue_messages_dead_lettered_total",
		Help: "Total messages moved to DLQ.",
	}, []string{"topic"})

	MessagesRestored = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "message_queue_messages_restored_total",
		Help: "Total messages restored after visibility timeout expiry.",
	}, []string{"topic"})

	QueueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "message_queue_depth",
		Help: "Current number of unacked messages per topic and partition.",
	}, []string{"topic", "partition"})

	DLQDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "message_queue_dlq_depth",
		Help: "Number of dead-lettered messages per topic.",
	}, []string{"topic"})

	PublishLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "message_queue_publish_duration_seconds",
		Help:    "Latency of the publish path.",
		Buckets: prometheus.DefBuckets,
	}, []string{"topic"})

	PollLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "message_queue_poll_duration_seconds",
		Help:    "Latency of the poll path.",
		Buckets: prometheus.DefBuckets,
	}, []string{"topic"})

	AckLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "message_queue_ack_duration_seconds",
		Help:    "Latency of the ack path.",
		Buckets: prometheus.DefBuckets,
	}, []string{"topic"})

	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "message_queue_http_requests_total",
		Help: "Total HTTP requests by method, path, and status code.",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "message_queue_http_request_duration_seconds",
		Help:    "HTTP request latency by method and path.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)

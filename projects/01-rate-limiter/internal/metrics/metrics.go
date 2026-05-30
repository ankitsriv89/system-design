package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ratelimiter_requests_total",
		Help: "Total rate-limit check requests",
	}, []string{"policy_id", "allowed"})

	CheckDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ratelimiter_check_duration_seconds",
		Help:    "Latency of /v1/limits/check",
		Buckets: prometheus.DefBuckets,
	}, []string{"algorithm"})

	PolicyCacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ratelimiter_policy_cache_total",
		Help: "Policy cache hits and misses",
	}, []string{"result"}) // hit | miss

	RedisErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ratelimiter_redis_errors_total",
		Help: "Redis operation errors",
	}, []string{"op"})
)

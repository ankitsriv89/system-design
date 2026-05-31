// Package algorithm provides rate-limiting algorithm implementations.
package algorithm

import (
	"sync"
	"time"
)

// SlidingWindow implements a sliding window log rate limiter.
// It tracks individual request timestamps and counts how many fall within [now-window, now].
// Memory grows with request volume (one entry per allowed request); use RedisLimiter for
// distributed or high-throughput deployments.
// Thread-safe for single-node use.
type SlidingWindow struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests []time.Time // sorted ascending; evicted lazily on each Allow/Count call
}

func NewSlidingWindow(limit int, window time.Duration) *SlidingWindow {
	return &SlidingWindow{
		limit:  limit,
		window: window,
	}
}

// Allow records a request and returns true if permitted within the window.
func (sw *SlidingWindow) Allow() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sw.window)

	// evict expired entries
	i := 0
	for i < len(sw.requests) && sw.requests[i].Before(cutoff) {
		i++
	}
	sw.requests = sw.requests[i:]

	if len(sw.requests) < sw.limit {
		sw.requests = append(sw.requests, now)
		return true
	}
	return false
}

// Count returns the number of requests in the current window.
func (sw *SlidingWindow) Count() int {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sw.window)
	i := 0
	for i < len(sw.requests) && sw.requests[i].Before(cutoff) {
		i++
	}
	sw.requests = sw.requests[i:]
	return len(sw.requests)
}

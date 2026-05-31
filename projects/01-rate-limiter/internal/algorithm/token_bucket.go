// Package algorithm provides rate-limiting algorithm implementations.
package algorithm

import (
	"sync"
	"time"
)

// TokenBucket implements the token bucket algorithm.
// Tokens refill at a fixed rate; each request consumes one token.
// Thread-safe for single-node use.
type TokenBucket struct {
	mu         sync.Mutex
	capacity   float64
	tokens     float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewTokenBucket creates a full bucket. capacity is the max token count;
// refillRate is tokens added per second.
func NewTokenBucket(capacity float64, refillRate float64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow consumes one token. Returns true if permitted.
func (tb *TokenBucket) Allow() bool {
	return tb.AllowN(1)
}

// AllowN consumes n tokens. Returns true if permitted.
func (tb *TokenBucket) AllowN(n float64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}
	return false
}

// State returns a snapshot of (current tokens, capacity).
func (tb *TokenBucket) State() (tokens, capacity float64) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return tb.tokens, tb.capacity
}

// refill adds tokens proportional to time elapsed since last call.
// Must be called under tb.mu.
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	added := elapsed * tb.refillRate
	if tb.tokens+added > tb.capacity {
		tb.tokens = tb.capacity
	} else {
		tb.tokens += added
	}
	tb.lastRefill = now
}

// Package store — Redis-backed sliding-window rate limiter.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter implements gateway.RateLimiter using a Redis sorted-set
// sliding window per key.
type RedisLimiter struct {
	rdb *redis.Client
}

// NewRedis creates a RedisLimiter. addr is "host:port".
func NewRedis(addr, password string, db int) (*RedisLimiter, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("store: redis ping: %w", err)
	}
	return &RedisLimiter{rdb: rdb}, nil
}

// Close releases the Redis connection pool.
func (r *RedisLimiter) Close() error { return r.rdb.Close() }

// Allow checks whether keyID is within limitPerMin requests in the last 60 s.
// Uses a sorted-set sliding window: members are request timestamps (nanoseconds),
// scored by the same value so range queries find the window efficiently.
func (r *RedisLimiter) Allow(ctx context.Context, keyID string, limitPerMin int) (bool, error) {
	now := time.Now()
	windowStart := now.Add(-time.Minute)
	key := "rl:" + keyID

	pipe := r.rdb.Pipeline()
	// Remove timestamps outside the window.
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))
	// Count remaining in window.
	countCmd := pipe.ZCard(ctx, key)
	// Add current timestamp.
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixNano()), Member: now.UnixNano()})
	// Expire the key after 2 minutes to avoid leaking memory on cold keys.
	pipe.Expire(ctx, key, 2*time.Minute)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("store: rate limiter pipeline: %w", err)
	}

	count := countCmd.Val()
	return count < int64(limitPerMin), nil
}

// Remaining returns how many requests remain in the current window.
func (r *RedisLimiter) Remaining(ctx context.Context, keyID string, limitPerMin int) (int, error) {
	windowStart := time.Now().Add(-time.Minute)
	key := "rl:" + keyID
	count, err := r.rdb.ZCount(ctx, key,
		fmt.Sprintf("%d", windowStart.UnixNano()), "+inf").Result()
	if err != nil {
		return 0, fmt.Errorf("store: remaining: %w", err)
	}
	rem := limitPerMin - int(count)
	if rem < 0 {
		rem = 0
	}
	return rem, nil
}

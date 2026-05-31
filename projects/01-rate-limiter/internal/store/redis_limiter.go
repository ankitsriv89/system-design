// Package store provides distributed rate-limit state via Redis.
package store

import (
	"context"
	_ "embed"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter executes atomic Lua scripts on Redis to implement
// token-bucket and sliding-window algorithms without races.
type RedisLimiter struct {
	client *redis.Client
}

func NewRedisLimiter(client *redis.Client) *RedisLimiter {
	return &RedisLimiter{client: client}
}

// TokenBucketAllow checks and debits one token from the bucket for (key).
// capacity is the max token count; refillRate is tokens-per-second.
// Returns (allowed, remaining, retryAfterMs).
//
// Lua script keeps atomicity: no WATCH/MULTI overhead, no race between
// read-compute-write steps.
var tokenBucketScript = redis.NewScript(`
local key       = KEYS[1]
local capacity  = tonumber(ARGV[1])
local refill    = tonumber(ARGV[2])   -- tokens per second
local now_ms    = tonumber(ARGV[3])   -- current unix ms
local cost      = tonumber(ARGV[4])   -- tokens to consume (usually 1)

local data = redis.call("HMGET", key, "tokens", "last_ms")
local tokens  = tonumber(data[1])
local last_ms = tonumber(data[2])

if tokens == nil then
  tokens  = capacity
  last_ms = now_ms
end

-- refill
local elapsed_s = (now_ms - last_ms) / 1000.0
local refilled = elapsed_s * refill
tokens = math.min(capacity, tokens + refilled)
last_ms = now_ms

local allowed = 0
local retry_ms = 0
if tokens >= cost then
  tokens = tokens - cost
  allowed = 1
else
  -- how many ms until one token is available
  local needed = cost - tokens
  retry_ms = math.ceil((needed / refill) * 1000)
end

-- TTL: evict buckets idle for 2× the refill window
local ttl_s = math.ceil((capacity / refill) * 2)
redis.call("HSET", key, "tokens", tokens, "last_ms", last_ms)
redis.call("EXPIRE", key, ttl_s)

return { allowed, math.floor(tokens), retry_ms }
`)

func (r *RedisLimiter) TokenBucketAllow(ctx context.Context, key string, capacity float64, refillRate float64) (allowed bool, remaining int64, retryAfterMs int64, err error) {
	nowMs := time.Now().UnixMilli()
	res, err := tokenBucketScript.Run(ctx, r.client,
		[]string{fmt.Sprintf("rl:tb:%s", key)},
		capacity, refillRate, nowMs, 1,
	).Int64Slice()
	if err != nil {
		return false, 0, 0, fmt.Errorf("redis token bucket: %w", err)
	}
	return res[0] == 1, res[1], res[2], nil
}

// SlidingWindowAllow uses a Redis sorted set to implement a sliding window log.
// window is the duration; limit is the max requests in that window.
var slidingWindowScript = redis.NewScript(`
local key     = KEYS[1]
local now_ms  = tonumber(ARGV[1])
local window  = tonumber(ARGV[2])   -- ms
local limit   = tonumber(ARGV[3])
local member  = ARGV[4]             -- unique request id

local cutoff  = now_ms - window

-- remove expired entries
redis.call("ZREMRANGEBYSCORE", key, "-inf", cutoff)

local count = redis.call("ZCARD", key)
local allowed = 0
if count < limit then
  redis.call("ZADD", key, now_ms, member)
  allowed = 1
  count = count + 1
end

-- keep the key alive for one extra window
redis.call("PEXPIRE", key, window + 1000)

return { allowed, count }
`)

func (r *RedisLimiter) SlidingWindowAllow(ctx context.Context, key string, limit int64, window time.Duration) (allowed bool, count int64, err error) {
	nowMs := time.Now().UnixMilli()
	member := fmt.Sprintf("%d-%s", nowMs, key) // good enough for demo; use UUID in prod
	res, err := slidingWindowScript.Run(ctx, r.client,
		[]string{fmt.Sprintf("rl:sw:%s", key)},
		nowMs, window.Milliseconds(), limit, member,
	).Int64Slice()
	if err != nil {
		return false, 0, fmt.Errorf("redis sliding window: %w", err)
	}
	return res[0] == 1, res[1], nil
}

// PeekTokenBucket returns (remaining, retryAfterMs) without debiting any token.
// Used by the state endpoint to show current hop count without consuming one.
func (r *RedisLimiter) PeekTokenBucket(ctx context.Context, key string, capacity float64, refillRate float64) (remaining int64, retryAfterMs int64) {
	redisKey := fmt.Sprintf("rl:tb:%s", key)
	vals, err := r.client.HMGet(ctx, redisKey, "tokens", "last_ms").Result()
	if err != nil || vals[0] == nil {
		// Key doesn't exist yet — bucket is full
		return int64(capacity), 0
	}

	var tokens, lastMs float64
	fmt.Sscanf(fmt.Sprint(vals[0]), "%f", &tokens)
	fmt.Sscanf(fmt.Sprint(vals[1]), "%f", &lastMs)

	nowMs := float64(time.Now().UnixMilli())
	elapsedS := (nowMs - lastMs) / 1000.0
	tokens = math.Min(capacity, tokens+elapsedS*refillRate)

	if tokens < 1 {
		needed := 1 - tokens
		retryAfterMs = int64(math.Ceil((needed / refillRate) * 1000))
		return 0, retryAfterMs
	}
	return int64(tokens), 0
}

// Ping checks Redis connectivity.
func (r *RedisLimiter) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// DeleteKey removes a rate-limit key from Redis (used for demo resets).
func (r *RedisLimiter) DeleteKey(ctx context.Context, key string) {
	_ = r.client.Del(ctx, "rl:tb:"+key).Err()
	_ = r.client.Del(ctx, "rl:sw:"+key).Err()
}

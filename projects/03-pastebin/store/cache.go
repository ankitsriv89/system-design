package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ankitsriv89/pastebin/paste"
)

// RedisCache implements paste.Cache using Redis.
type RedisCache struct {
	rdb *redis.Client
}

func NewRedisCache(addr string) *RedisCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		PoolSize:     20,
	})
	return &RedisCache{rdb: rdb}
}

func (c *RedisCache) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *RedisCache) SetPaste(ctx context.Context, p *paste.Paste, ttl time.Duration) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, pasteKey(p.ID), data, ttl).Err()
}

func (c *RedisCache) GetPaste(ctx context.Context, id string) (*paste.Paste, error) {
	data, err := c.rdb.Get(ctx, pasteKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p paste.Paste
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *RedisCache) DeletePaste(ctx context.Context, id string) error {
	return c.rdb.Del(ctx, pasteKey(id)).Err()
}

// Allow implements a sliding-window rate limiter using Redis INCR + EXPIRE.
// Returns true if the request is allowed.
func (c *RedisCache) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	rkey := "rl:" + key
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, rkey)
	pipe.Expire(ctx, rkey, window)
	if _, err := pipe.Exec(ctx); err != nil {
		// Fail open: if Redis is down don't block all traffic.
		return true, err
	}
	return incr.Val() <= int64(limit), nil
}

func pasteKey(id string) string {
	return fmt.Sprintf("paste:meta:%s", id)
}

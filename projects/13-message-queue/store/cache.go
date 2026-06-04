// Package store — cache.go provides Redis-backed topic metadata caching
// and a monotonic publish counter used for round-robin partition assignment.
package store

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// Cache wraps a Redis client for topic metadata and counters.
type Cache struct {
	client *redis.Client
}

// NewCache constructs a Cache with a single shared Redis client.
func NewCache(addr, password string, db int) *Cache {
	return &Cache{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}),
	}
}

// Close releases the Redis connection.
func (c *Cache) Close() error { return c.client.Close() }

// Ping verifies Redis connectivity.
func (c *Cache) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("cache: ping: %w", err)
	}
	return nil
}

// IncrPublishCounter atomically increments a per-topic counter used for
// round-robin partition selection when no message key is provided.
// Returns the new counter value.
func (c *Cache) IncrPublishCounter(ctx context.Context, topic string) (int64, error) {
	val, err := c.client.Incr(ctx, "mqctr:"+topic).Result()
	if err != nil {
		return 0, fmt.Errorf("cache: incr counter: %w", err)
	}
	return val, nil
}

// SetTopicPartitions caches the partition count for a topic.
func (c *Cache) SetTopicPartitions(ctx context.Context, topic string, partitions int) error {
	if err := c.client.Set(ctx, "mqpart:"+topic, strconv.Itoa(partitions), 0).Err(); err != nil {
		return fmt.Errorf("cache: set partitions: %w", err)
	}
	return nil
}

// GetTopicPartitions retrieves a cached partition count. Returns 0, nil on miss.
func (c *Cache) GetTopicPartitions(ctx context.Context, topic string) (int, error) {
	val, err := c.client.Get(ctx, "mqpart:"+topic).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("cache: get partitions: %w", err)
	}
	n, _ := strconv.Atoi(val)
	return n, nil
}

// DeleteTopicCache removes all cache keys associated with a topic.
func (c *Cache) DeleteTopicCache(ctx context.Context, topic string) error {
	keys := []string{"mqpart:" + topic, "mqctr:" + topic}
	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("cache: delete topic cache: %w", err)
	}
	return nil
}

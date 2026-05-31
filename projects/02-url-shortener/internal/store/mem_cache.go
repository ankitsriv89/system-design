package store

import (
	"context"
	"sync"
	"time"
)

const negativeCacheValue = "__missing__"

type cacheEntry struct {
	value     string
	expiresAt time.Time
}

// MemCache is a thread-safe in-process URL cache with TTL expiry.
// It replaces Redis for deployments where an external cache is not available.
type MemCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

func NewMemCache() *MemCache {
	c := &MemCache{entries: make(map[string]cacheEntry)}
	go c.janitor()
	return c
}

func (c *MemCache) GetURL(_ context.Context, code string) (url string, missing bool, found bool, err error) {
	c.mu.RLock()
	e, ok := c.entries[memKey(code)]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return "", false, false, nil
	}
	if e.value == negativeCacheValue {
		return "", true, true, nil
	}
	return e.value, false, true, nil
}

func (c *MemCache) SetURL(_ context.Context, code, longURL string, ttl time.Duration) error {
	c.set(memKey(code), longURL, ttl)
	return nil
}

func (c *MemCache) SetMissing(_ context.Context, code string, ttl time.Duration) error {
	c.set(memKey(code), negativeCacheValue, ttl)
	return nil
}

func (c *MemCache) set(k, v string, ttl time.Duration) {
	c.mu.Lock()
	c.entries[k] = cacheEntry{value: v, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
}

// janitor removes expired entries every minute to prevent unbounded growth.
func (c *MemCache) janitor() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for k, e := range c.entries {
			if now.After(e.expiresAt) {
				delete(c.entries, k)
			}
		}
		c.mu.Unlock()
	}
}

func memKey(code string) string {
	return "url:" + code
}

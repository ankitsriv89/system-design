// Package cache implements an in-memory cache with LRU and LFU eviction,
// per-entry TTL, and singleflight coalescing for stampede protection.
package cache

import (
	"container/heap"
	"container/list"
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// Policy selects the eviction algorithm.
type Policy string

const (
	PolicyLRU Policy = "lru"
	PolicyLFU Policy = "lfu"
)

// Entry is a single cached value with metadata.
type Entry struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	SizeBytes   int       `json:"size_bytes"`
	TTL         int64     `json:"ttl_ms"` // 0 = no expiry
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	LastAccess  time.Time `json:"last_access"`
	AccessCount int64     `json:"access_count"`
	CreatedAt   time.Time `json:"created_at"`
}

func (e *Entry) expired() bool {
	return !e.ExpiresAt.IsZero() && time.Now().After(e.ExpiresAt)
}

// EvictionRecord is emitted whenever a key is evicted.
type EvictionRecord struct {
	Key       string    `json:"key"`
	Reason    string    `json:"reason"` // "ttl" | "capacity" | "explicit"
	SizeBytes int       `json:"size_bytes"`
	At        time.Time `json:"at"`
}

// Stats is a point-in-time snapshot.
type Stats struct {
	Keys        int     `json:"keys"`
	HitRate     float64 `json:"hit_rate"`
	Hits        int64   `json:"hits"`
	Misses      int64   `json:"misses"`
	Evictions   int64   `json:"evictions"`
	MemoryBytes int64   `json:"memory_bytes"`
	Policy      string  `json:"policy"`
	MaxBytes    int64   `json:"max_bytes"`
}

// EvictionListener is called (in a goroutine) for every eviction.
type EvictionListener func(EvictionRecord)

// Config holds cache construction parameters.
type Config struct {
	Policy      Policy
	MaxBytes    int64
	DefaultTTL  time.Duration // 0 = no default TTL
	SweepPeriod time.Duration // passive expiry sweep interval
	Log         *zap.Logger
	OnEvict     EvictionListener
}

// Cache is the main thread-safe cache object.
type Cache struct {
	mu sync.Mutex

	policy     Policy
	maxBytes   int64
	defaultTTL time.Duration
	log        *zap.Logger
	onEvict    EvictionListener

	items map[string]*item

	// LRU structures
	lruList *list.List

	// LFU structures
	lfuHeap *lfuHeap

	hits      int64
	misses    int64
	evictions int64
	memBytes  int64

	sf     singleflight.Group
	cancel context.CancelFunc
}

type item struct {
	entry    Entry
	lruElem  *list.Element // non-nil only for LRU
	lfuIndex int           // heap index, only for LFU
}

// New creates and starts a Cache with the given Config.
func New(cfg Config) *Cache {
	if cfg.SweepPeriod == 0 {
		cfg.SweepPeriod = 30 * time.Second
	}
	if cfg.Log == nil {
		cfg.Log = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Cache{
		policy:     cfg.Policy,
		maxBytes:   cfg.MaxBytes,
		defaultTTL: cfg.DefaultTTL,
		log:        cfg.Log,
		onEvict:    cfg.OnEvict,
		items:      make(map[string]*item, 256),
		lruList:    list.New(),
		cancel:     cancel,
	}

	if cfg.Policy == PolicyLFU {
		h := make(lfuHeap, 0, 256)
		c.lfuHeap = &h
		heap.Init(c.lfuHeap)
	}

	go c.sweeper(ctx, cfg.SweepPeriod)
	return c
}

// Close stops background goroutines.
func (c *Cache) Close() {
	c.cancel()
}

// Set inserts or updates a key. ttl=0 uses the cache default; negative = no expiry.
func (c *Cache) Set(key, value string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	size := len(key) + len(value)

	// evict by size if needed before inserting
	if c.maxBytes > 0 {
		needed := int64(size)
		if it, exists := c.items[key]; exists {
			needed -= int64(it.entry.SizeBytes)
		}
		for c.maxBytes > 0 && c.memBytes+needed > c.maxBytes {
			if !c.evictOne("capacity") {
				break
			}
		}
	}

	// resolve TTL
	exp := resolveExpiry(ttl, c.defaultTTL)

	now := time.Now()

	if it, exists := c.items[key]; exists {
		// update in place
		c.memBytes -= int64(it.entry.SizeBytes)
		it.entry.Value = value
		it.entry.SizeBytes = size
		it.entry.ExpiresAt = exp
		it.entry.LastAccess = now
		it.entry.AccessCount++
		c.memBytes += int64(size)

		if c.policy == PolicyLRU {
			c.lruList.MoveToFront(it.lruElem)
		} else {
			heap.Fix(c.lfuHeap, it.lfuIndex)
		}
		return
	}

	e := Entry{
		Key:         key,
		Value:       value,
		SizeBytes:   size,
		TTL:         int64(ttl / time.Millisecond),
		ExpiresAt:   exp,
		LastAccess:  now,
		AccessCount: 1,
		CreatedAt:   now,
	}

	it := &item{entry: e}

	if c.policy == PolicyLRU {
		it.lruElem = c.lruList.PushFront(key)
	} else {
		it.lfuIndex = c.lfuHeap.Len()
		heap.Push(c.lfuHeap, it)
	}

	c.items[key] = it
	c.memBytes += int64(size)
}

// Get retrieves a value. Returns ("", false) on miss or expiry.
func (c *Cache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	it, ok := c.items[key]
	if !ok {
		c.misses++
		return "", false
	}
	if it.entry.expired() {
		c.evictKey(key, "ttl")
		c.misses++
		return "", false
	}

	it.entry.LastAccess = time.Now()
	it.entry.AccessCount++

	if c.policy == PolicyLRU {
		c.lruList.MoveToFront(it.lruElem)
	} else {
		heap.Fix(c.lfuHeap, it.lfuIndex)
	}

	c.hits++
	return it.entry.Value, true
}

// GetOrLoad retrieves a value, calling loader on miss with singleflight coalescing.
// loader is called outside the cache lock.
func (c *Cache) GetOrLoad(key string, ttl time.Duration, loader func() (string, error)) (string, bool, error) {
	if v, ok := c.Get(key); ok {
		return v, true, nil
	}

	val, err, _ := c.sf.Do(key, func() (interface{}, error) {
		// check again after acquiring the singleflight slot
		if v, ok := c.Get(key); ok {
			return v, nil
		}
		v, err := loader()
		if err != nil {
			return "", err
		}
		c.Set(key, v, ttl)
		return v, nil
	})
	if err != nil {
		return "", false, err
	}
	return val.(string), false, nil
}

// Delete removes a key explicitly.
func (c *Cache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.items[key]; !ok {
		return false
	}
	c.evictKey(key, "explicit")
	return true
}

// Keys returns all non-expired keys (snapshot).
func (c *Cache) Keys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.items))
	for k, it := range c.items {
		if !it.entry.expired() {
			out = append(out, k)
		}
	}
	return out
}

// Peek returns the Entry metadata without updating access order.
func (c *Cache) Peek(key string) (Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	it, ok := c.items[key]
	if !ok || it.entry.expired() {
		return Entry{}, false
	}
	return it.entry, true
}

// Stats returns a point-in-time snapshot.
func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()

	total := c.hits + c.misses
	var hr float64
	if total > 0 {
		hr = float64(c.hits) / float64(total)
	}
	return Stats{
		Keys:        len(c.items),
		HitRate:     hr,
		Hits:        c.hits,
		Misses:      c.misses,
		Evictions:   c.evictions,
		MemoryBytes: c.memBytes,
		Policy:      string(c.policy),
		MaxBytes:    c.maxBytes,
	}
}

// Flush removes all entries.
func (c *Cache) Flush() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(c.items)
	c.items = make(map[string]*item, 256)
	c.lruList.Init()
	if c.policy == PolicyLFU {
		h := make(lfuHeap, 0, 256)
		c.lfuHeap = &h
		heap.Init(c.lfuHeap)
	}
	c.memBytes = 0
	return n
}

// Entries returns a snapshot of all non-expired entries sorted by access time (newest first).
func (c *Cache) Entries() []Entry {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Entry, 0, len(c.items))
	for _, it := range c.items {
		if !it.entry.expired() {
			out = append(out, it.entry)
		}
	}
	return out
}

// evictKey removes the key and fires the listener. Must be called with mu held.
func (c *Cache) evictKey(key, reason string) {
	it, ok := c.items[key]
	if !ok {
		return
	}
	if c.policy == PolicyLRU && it.lruElem != nil {
		c.lruList.Remove(it.lruElem)
	} else if c.policy == PolicyLFU {
		heap.Remove(c.lfuHeap, it.lfuIndex)
	}
	c.memBytes -= int64(it.entry.SizeBytes)
	delete(c.items, key)
	c.evictions++

	if c.onEvict != nil {
		rec := EvictionRecord{
			Key:       key,
			Reason:    reason,
			SizeBytes: it.entry.SizeBytes,
			At:        time.Now(),
		}
		go c.onEvict(rec)
	}
}

// evictOne evicts the least recently / least frequently used item. Returns false if empty.
func (c *Cache) evictOne(reason string) bool {
	if len(c.items) == 0 {
		return false
	}
	var victim string
	if c.policy == PolicyLRU {
		back := c.lruList.Back()
		if back == nil {
			return false
		}
		victim = back.Value.(string)
	} else {
		if c.lfuHeap.Len() == 0 {
			return false
		}
		victim = (*c.lfuHeap)[0].entry.Key
	}
	c.evictKey(victim, reason)
	return true
}

// sweeper runs a passive TTL expiry sweep every period.
func (c *Cache) sweeper(ctx context.Context, period time.Duration) {
	t := time.NewTicker(period)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.sweepExpired()
		}
	}
}

func (c *Cache) sweepExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, it := range c.items {
		if it.entry.expired() {
			c.evictKey(k, "ttl")
		}
	}
}

func resolveExpiry(ttl, def time.Duration) time.Time {
	if ttl < 0 {
		return time.Time{}
	}
	if ttl == 0 {
		ttl = def
	}
	if ttl == 0 {
		return time.Time{}
	}
	return time.Now().Add(ttl)
}

// ─── LFU min-heap ────────────────────────────────────────────────────────────

type lfuHeap []*item

func (h lfuHeap) Len() int { return len(h) }
func (h lfuHeap) Less(i, j int) bool {
	// tie-break by last access (older = higher priority for eviction)
	if h[i].entry.AccessCount == h[j].entry.AccessCount {
		return h[i].entry.LastAccess.Before(h[j].entry.LastAccess)
	}
	return h[i].entry.AccessCount < h[j].entry.AccessCount
}
func (h lfuHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].lfuIndex = i
	h[j].lfuIndex = j
}
func (h *lfuHeap) Push(x interface{}) {
	it := x.(*item)
	it.lfuIndex = len(*h)
	*h = append(*h, it)
}
func (h *lfuHeap) Pop() interface{} {
	old := *h
	n := len(old)
	it := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return it
}

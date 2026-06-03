// Package cache tests LRU/LFU eviction, TTL, singleflight, and concurrent safety.
package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func newLRU(maxBytes int64) *Cache {
	return New(Config{
		Policy:      PolicyLRU,
		MaxBytes:    maxBytes,
		SweepPeriod: time.Hour, // don't sweep during tests
		Log:         zap.NewNop(),
	})
}

func newLFU(maxBytes int64) *Cache {
	return New(Config{
		Policy:      PolicyLFU,
		MaxBytes:    maxBytes,
		SweepPeriod: time.Hour,
		Log:         zap.NewNop(),
	})
}

func TestSetGet(t *testing.T) {
	c := newLRU(0)
	defer c.Close()

	c.Set("hello", "world", 0)
	v, ok := c.Get("hello")
	if !ok || v != "world" {
		t.Fatalf("expected 'world', got %q ok=%v", v, ok)
	}
}

func TestMiss(t *testing.T) {
	c := newLRU(0)
	defer c.Close()

	_, ok := c.Get("missing")
	if ok {
		t.Fatal("expected miss")
	}
}

func TestTTLExpiry(t *testing.T) {
	c := newLRU(0)
	defer c.Close()

	c.Set("k", "v", 50*time.Millisecond)
	if _, ok := c.Get("k"); !ok {
		t.Fatal("expected hit before expiry")
	}
	time.Sleep(70 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected miss after expiry")
	}
}

func TestLRUEviction(t *testing.T) {
	// capacity for ~3 small keys (each key+value ~4 bytes); set 4 to force eviction
	c := newLRU(12)
	defer c.Close()

	c.Set("a", "1", 0) // size 2
	c.Set("b", "2", 0) // size 2
	c.Set("c", "3", 0) // size 2 → total 6; max 12 — all fit
	// access 'a' to make it recently used
	c.Get("a")
	// add 'd' — won't trigger eviction yet since total is 8 ≤ 12
	c.Set("d", "4", 0)
	// add a large key to force eviction
	c.Set("e", "value-big-enough", 0) // size ~22, pushes total > 12
	s := c.Stats()
	if s.Evictions == 0 {
		t.Fatal("expected at least one eviction")
	}
}

func TestLFUEviction(t *testing.T) {
	// 3 keys each "k"+"v" = 2 bytes; max 4 → fits 2
	c := newLFU(4)
	defer c.Close()

	c.Set("a", "1", 0)
	c.Set("b", "2", 0)
	// access 'b' more
	c.Get("b")
	c.Get("b")
	// inserting 'c' should evict 'a' (least frequently used)
	c.Set("c", "3", 0)

	s := c.Stats()
	if s.Evictions == 0 {
		t.Fatal("expected eviction")
	}
}

func TestDelete(t *testing.T) {
	c := newLRU(0)
	defer c.Close()

	c.Set("x", "y", 0)
	if !c.Delete("x") {
		t.Fatal("expected delete to return true")
	}
	if _, ok := c.Get("x"); ok {
		t.Fatal("expected miss after delete")
	}
	if c.Delete("x") {
		t.Fatal("expected delete to return false on second call")
	}
}

func TestFlush(t *testing.T) {
	c := newLRU(0)
	defer c.Close()

	for i := 0; i < 10; i++ {
		c.Set(fmt.Sprintf("k%d", i), "v", 0)
	}
	n := c.Flush()
	if n != 10 {
		t.Fatalf("expected 10 flushed, got %d", n)
	}
	if s := c.Stats(); s.Keys != 0 {
		t.Fatalf("expected 0 keys after flush, got %d", s.Keys)
	}
}

func TestStats(t *testing.T) {
	c := newLRU(0)
	defer c.Close()

	c.Set("a", "1", 0)
	c.Get("a")
	c.Get("missing")

	s := c.Stats()
	if s.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", s.Hits)
	}
	if s.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", s.Misses)
	}
	if s.HitRate != 0.5 {
		t.Errorf("expected 0.5 hit rate, got %f", s.HitRate)
	}
}

func TestConcurrentSafety(t *testing.T) {
	c := newLRU(1024)
	defer c.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("k%d", i%10)
			c.Set(key, fmt.Sprintf("v%d", i), 50*time.Millisecond)
			c.Get(key)
		}(i)
	}
	wg.Wait()
}

func TestGetOrLoad_Singleflight(t *testing.T) {
	c := newLRU(0)
	defer c.Close()

	calls := 0
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := c.GetOrLoad("shared", time.Minute, func() (string, error) {
				mu.Lock()
				calls++
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				return "loaded", nil
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	c2 := calls
	mu.Unlock()
	// singleflight coalesces all concurrent requests into one loader call
	if c2 > 3 {
		t.Errorf("expected ≤3 loader calls (singleflight), got %d", c2)
	}
}

func BenchmarkSetLRU(b *testing.B) {
	c := newLRU(0)
	defer c.Close()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("k%d", i), "value", 0)
	}
}

func BenchmarkGetLRU(b *testing.B) {
	c := newLRU(0)
	defer c.Close()
	for i := 0; i < 1000; i++ {
		c.Set(fmt.Sprintf("k%d", i), "value", 0)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(fmt.Sprintf("k%d", i%1000))
	}
}

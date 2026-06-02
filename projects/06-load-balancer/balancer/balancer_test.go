// Package balancer implements the core load-balancing domain.
package balancer

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestRoundRobin(t *testing.T) {
	pool := NewPool(AlgoRoundRobin)
	for _, url := range []string{"http://a", "http://b", "http://c"} {
		pool.Add(&Backend{URL: url, Service: "svc", Weight: 1, status: StatusHealthy})
	}

	seen := make(map[string]int)
	for i := 0; i < 9; i++ {
		b := pool.Next()
		if b == nil {
			t.Fatal("Next() returned nil for healthy pool")
		}
		seen[b.URL]++
	}
	for url, count := range seen {
		if count != 3 {
			t.Errorf("url %s got %d requests, want 3", url, count)
		}
	}
}

func TestLeastConnections(t *testing.T) {
	pool := NewPool(AlgoLeastConnections)
	a := &Backend{URL: "http://a", Service: "svc", Weight: 1, status: StatusHealthy}
	b := &Backend{URL: "http://b", Service: "svc", Weight: 1, status: StatusHealthy}
	pool.Add(a)
	pool.Add(b)

	// Artificially load backend a.
	atomic.StoreInt64(&a.ActiveConns, 10)
	atomic.StoreInt64(&b.ActiveConns, 0)

	chosen := pool.Next()
	if chosen.URL != "http://b" {
		t.Errorf("expected least-conn to pick http://b, got %s", chosen.URL)
	}
}

func TestWeightedRoundRobin(t *testing.T) {
	pool := NewPool(AlgoWeightedRR)
	pool.Add(&Backend{URL: "http://heavy", Service: "svc", Weight: 3, status: StatusHealthy})
	pool.Add(&Backend{URL: "http://light", Service: "svc", Weight: 1, status: StatusHealthy})

	counts := make(map[string]int)
	for i := 0; i < 40; i++ {
		b := pool.Next()
		if b == nil {
			t.Fatal("Next() returned nil")
		}
		counts[b.URL]++
	}
	// heavy should receive ~3× more traffic than light
	if counts["http://heavy"] < counts["http://light"]*2 {
		t.Errorf("weighted distribution off: heavy=%d light=%d", counts["http://heavy"], counts["http://light"])
	}
}

func TestUnhealthyBackendsSkipped(t *testing.T) {
	pool := NewPool(AlgoRoundRobin)
	pool.Add(&Backend{URL: "http://dead", Service: "svc", Weight: 1, status: StatusUnhealthy})
	pool.Add(&Backend{URL: "http://alive", Service: "svc", Weight: 1, status: StatusHealthy})

	for i := 0; i < 10; i++ {
		b := pool.Next()
		if b == nil {
			t.Fatal("Next() returned nil when one backend is alive")
		}
		if b.URL == "http://dead" {
			t.Errorf("routed to unhealthy backend")
		}
	}
}

func TestAllUnhealthyReturnsNil(t *testing.T) {
	pool := NewPool(AlgoRoundRobin)
	pool.Add(&Backend{URL: "http://dead", Service: "svc", Weight: 1, status: StatusUnhealthy})
	if pool.Next() != nil {
		t.Error("expected nil when all backends are unhealthy")
	}
}

func TestConcurrentNext(t *testing.T) {
	pool := NewPool(AlgoRoundRobin)
	for i := 0; i < 5; i++ {
		pool.Add(&Backend{URL: "http://x", Service: "svc", Weight: 1, status: StatusHealthy})
	}
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pool.Next()
		}()
	}
	wg.Wait()
}

func BenchmarkRoundRobin(b *testing.B) {
	pool := NewPool(AlgoRoundRobin)
	for _, url := range []string{"http://a", "http://b", "http://c"} {
		pool.Add(&Backend{URL: url, Service: "svc", Weight: 1, status: StatusHealthy})
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = pool.Next()
		}
	})
}

func BenchmarkLeastConnections(b *testing.B) {
	pool := NewPool(AlgoLeastConnections)
	for _, url := range []string{"http://a", "http://b", "http://c"} {
		pool.Add(&Backend{URL: url, Service: "svc", Weight: 1, status: StatusHealthy})
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = pool.Next()
		}
	})
}

// Package ring tests for hash ring correctness, distribution, and race safety.
package ring

import (
	"fmt"
	"math"
	"testing"
)

func TestLookupEmptyRing(t *testing.T) {
	r := New(10)
	if got := r.Lookup("any-key"); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestAddNodeAndLookup(t *testing.T) {
	r := New(50)
	r.AddNode(Node{ID: "a", Weight: 1})
	r.AddNode(Node{ID: "b", Weight: 1})
	r.AddNode(Node{ID: "c", Weight: 1})

	seen := map[string]int{}
	for i := range 1000 {
		key := fmt.Sprintf("key-%d", i)
		owner := r.Lookup(key)
		if owner == "" {
			t.Fatalf("empty owner for key %q", key)
		}
		seen[owner]++
	}
	if len(seen) != 3 {
		t.Fatalf("expected all 3 nodes to own keys, got %v", seen)
	}
}

func TestLookupStability(t *testing.T) {
	r := New(150)
	r.AddNode(Node{ID: "a", Weight: 1})
	r.AddNode(Node{ID: "b", Weight: 1})
	r.AddNode(Node{ID: "c", Weight: 1})

	before := map[string]string{}
	for i := range 500 {
		k := fmt.Sprintf("key-%d", i)
		before[k] = r.Lookup(k)
	}

	// Add a 4th node — only ~25% of keys should move.
	r.AddNode(Node{ID: "d", Weight: 1})

	moved := 0
	for k, prev := range before {
		if r.Lookup(k) != prev {
			moved++
		}
	}
	pct := float64(moved) / float64(len(before)) * 100
	// With 4 nodes, expect roughly 25% to move; allow up to 35% for statistical variance.
	if pct > 35 {
		t.Fatalf("too many keys moved: %.1f%% (expected ~25%%)", pct)
	}
	t.Logf("keys moved after adding 4th node: %.1f%%", pct)
}

func TestRemoveNode(t *testing.T) {
	r := New(150)
	r.AddNode(Node{ID: "a", Weight: 1})
	r.AddNode(Node{ID: "b", Weight: 1})
	r.AddNode(Node{ID: "c", Weight: 1})

	_, ok := r.RemoveNode("b")
	if !ok {
		t.Fatal("RemoveNode returned false for existing node")
	}

	for i := range 300 {
		k := fmt.Sprintf("key-%d", i)
		owner := r.Lookup(k)
		if owner == "b" {
			t.Fatalf("key %q still routes to removed node b", k)
		}
	}
}

func TestWeightedDistribution(t *testing.T) {
	r := New(150)
	r.AddNode(Node{ID: "small", Weight: 1})
	r.AddNode(Node{ID: "large", Weight: 3})

	dist := r.SimulateKeys(100_000)
	smallPct := float64(dist["small"]) / 100_000.0
	largePct := float64(dist["large"]) / 100_000.0

	// large should own ~75%, small ~25%; allow 10% tolerance.
	if math.Abs(largePct-0.75) > 0.10 {
		t.Fatalf("large node owns %.1f%%, expected ~75%%", largePct*100)
	}
	if math.Abs(smallPct-0.25) > 0.10 {
		t.Fatalf("small node owns %.1f%%, expected ~25%%", smallPct*100)
	}
	t.Logf("weighted distribution: small=%.1f%%, large=%.1f%%", smallPct*100, largePct*100)
}

func TestVersionIncrement(t *testing.T) {
	r := New(10)
	if r.Version() != 0 {
		t.Fatal("initial version should be 0")
	}
	r.AddNode(Node{ID: "a"})
	if r.Version() != 1 {
		t.Fatal("version should be 1 after AddNode")
	}
	r.RemoveNode("a")
	if r.Version() != 2 {
		t.Fatal("version should be 2 after RemoveNode")
	}
}

func TestLookupN(t *testing.T) {
	r := New(50)
	r.AddNode(Node{ID: "a"})
	r.AddNode(Node{ID: "b"})
	r.AddNode(Node{ID: "c"})

	replicas := r.LookupN("some-key", 3)
	if len(replicas) != 3 {
		t.Fatalf("expected 3 replicas, got %d", len(replicas))
	}
	seen := map[string]bool{}
	for _, id := range replicas {
		if seen[id] {
			t.Fatalf("duplicate node %q in replica set", id)
		}
		seen[id] = true
	}
}

func TestStatsStdDev(t *testing.T) {
	r := New(200)
	r.AddNode(Node{ID: "a"})
	r.AddNode(Node{ID: "b"})
	r.AddNode(Node{ID: "c"})
	r.AddNode(Node{ID: "d"})

	s := r.Stats()
	// With 200 vnodes each, stddev should be small (< 0.05).
	if s.StdDev > 0.05 {
		t.Fatalf("stddev %.4f too high, ring is unbalanced", s.StdDev)
	}
	t.Logf("ring stddev with 200 vnodes/node: %.4f", s.StdDev)
}

func BenchmarkLookup(b *testing.B) {
	r := New(150)
	for i := range 10 {
		r.AddNode(Node{ID: fmt.Sprintf("node-%d", i), Weight: 1})
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			r.Lookup(fmt.Sprintf("bench-key-%d", i))
			i++
		}
	})
}

func BenchmarkAddNode(b *testing.B) {
	for b.Loop() {
		r := New(150)
		for i := range 10 {
			r.AddNode(Node{ID: fmt.Sprintf("node-%d", i)})
		}
	}
}

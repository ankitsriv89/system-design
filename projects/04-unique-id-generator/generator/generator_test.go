// Tests for the Snowflake generator: uniqueness, monotonicity, bit layout,
// clock rollback handling, and concurrent correctness.
package generator

import (
	"sync"
	"testing"
	"time"
)

// TestUniqueSequential verifies that successive IDs are strictly increasing.
func TestUniqueSequential(t *testing.T) {
	g, _ := New(1, nil)
	prev := g.Next()
	for i := 0; i < 10_000; i++ {
		id := g.Next()
		if id <= prev {
			t.Fatalf("ID not monotonically increasing: %d <= %d", id, prev)
		}
		prev = id
	}
}

// TestUniqueConcurrent generates IDs from multiple goroutines and asserts no duplicates.
func TestUniqueConcurrent(t *testing.T) {
	g, _ := New(2, nil)
	const goroutines = 50
	const perGoroutine = 200

	results := make(chan int64, goroutines*perGoroutine)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				results <- g.Next()
			}
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[int64]struct{}, goroutines*perGoroutine)
	for id := range results {
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID: %d", id)
		}
		seen[id] = struct{}{}
	}
}

// TestDecompose verifies that Decompose recovers the original fields.
func TestDecompose(t *testing.T) {
	g, _ := New(42, nil)
	id := g.Next()
	ts, wid, seq := Decompose(id)

	if wid != 42 {
		t.Errorf("worker_id: got %d, want 42", wid)
	}
	if seq < 0 || seq > maxSequence {
		t.Errorf("sequence out of range: %d", seq)
	}
	now := time.Now().UnixMilli()
	if ts < Epoch || ts > now+1000 {
		t.Errorf("timestamp out of expected range: %d", ts)
	}
}

// TestClockRollback verifies that the incident hook fires and IDs stay unique
// when the internal last-ms is artificially set into the future.
func TestClockRollback(t *testing.T) {
	var incidents int
	hook := func(_ int64, _ int64) { incidents++ }

	g, _ := New(3, hook)
	// Simulate a future lastMs so the next call sees "rollback".
	g.mu.Lock()
	g.lastMs = nowMs() + 5 // 5 ms ahead
	g.mu.Unlock()

	id1 := g.Next()
	if incidents == 0 {
		t.Error("expected clock incident to be recorded")
	}
	id2 := g.Next()
	if id2 <= id1 {
		t.Errorf("IDs not monotonic after rollback: %d <= %d", id2, id1)
	}
}

// TestBatch verifies that Batch returns the requested count with no duplicates.
func TestBatch(t *testing.T) {
	g, _ := New(5, nil)
	ids := g.Batch(500)
	if len(ids) != 500 {
		t.Fatalf("expected 500 IDs, got %d", len(ids))
	}
	seen := make(map[int64]struct{}, 500)
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate in batch: %d", id)
		}
		seen[id] = struct{}{}
	}
}

// TestWorkerIDValidation verifies that out-of-range worker IDs are rejected.
func TestWorkerIDValidation(t *testing.T) {
	if _, err := New(-1, nil); err == nil {
		t.Error("expected error for worker_id -1")
	}
	if _, err := New(1024, nil); err == nil {
		t.Error("expected error for worker_id 1024")
	}
	if _, err := New(0, nil); err != nil {
		t.Errorf("unexpected error for worker_id 0: %v", err)
	}
	if _, err := New(1023, nil); err != nil {
		t.Errorf("unexpected error for worker_id 1023: %v", err)
	}
}

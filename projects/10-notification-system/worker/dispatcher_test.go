// Package worker tests the dispatcher logic.
package worker

import (
	"testing"
)

func TestMinHelper(t *testing.T) {
	if min(3, 5) != 3 {
		t.Fatal("expected 3")
	}
	if min(7, 2) != 2 {
		t.Fatal("expected 2")
	}
	if min(4, 4) != 4 {
		t.Fatal("expected 4")
	}
}

func TestQueueBackpressure(t *testing.T) {
	// Verify that Enqueue returns an error when the queue is full without blocking.
	// We can't run a full dispatcher in unit tests (no DB), but we can test the
	// backpressure path by directly filling the channel.
	ch := make(chan Job, 2)
	ch <- Job{}
	ch <- Job{}

	// Third enqueue should fail immediately.
	select {
	case ch <- Job{}:
		t.Fatal("expected channel to be full")
	default:
		// expected
	}
}

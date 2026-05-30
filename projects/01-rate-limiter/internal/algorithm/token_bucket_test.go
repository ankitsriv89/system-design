package algorithm

import (
	"testing"
	"time"
)

func TestTokenBucket_BasicAllow(t *testing.T) {
	tb := NewTokenBucket(5, 1) // capacity=5, refill 1/s

	// First 5 requests should succeed
	for i := 0; i < 5; i++ {
		if !tb.Allow() {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	// 6th should be denied
	if tb.Allow() {
		t.Fatal("6th request should be denied")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	tb := NewTokenBucket(2, 10) // refill 10 tokens/s
	tb.Allow()
	tb.Allow()

	// After 200ms, should have ~2 tokens back
	time.Sleep(200 * time.Millisecond)
	if !tb.Allow() {
		t.Fatal("should be allowed after refill")
	}
}

func TestTokenBucket_State(t *testing.T) {
	tb := NewTokenBucket(10, 1)
	tb.Allow()
	tokens, cap := tb.State()
	if cap != 10 {
		t.Fatalf("expected capacity 10, got %f", cap)
	}
	if tokens >= 10 {
		t.Fatalf("expected tokens < 10 after Allow, got %f", tokens)
	}
}

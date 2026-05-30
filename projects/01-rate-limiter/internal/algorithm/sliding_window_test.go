package algorithm

import (
	"testing"
	"time"
)

func TestSlidingWindow_BasicAllow(t *testing.T) {
	sw := NewSlidingWindow(3, time.Second)

	for i := 0; i < 3; i++ {
		if !sw.Allow() {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	if sw.Allow() {
		t.Fatal("4th request should be denied")
	}
}

func TestSlidingWindow_WindowExpiry(t *testing.T) {
	sw := NewSlidingWindow(2, 100*time.Millisecond)
	sw.Allow()
	sw.Allow()

	time.Sleep(150 * time.Millisecond)

	if !sw.Allow() {
		t.Fatal("should be allowed after window expires")
	}
}

func TestSlidingWindow_Count(t *testing.T) {
	sw := NewSlidingWindow(10, time.Second)
	sw.Allow()
	sw.Allow()
	if c := sw.Count(); c != 2 {
		t.Fatalf("expected count 2, got %d", c)
	}
}

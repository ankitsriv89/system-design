// Package queue tests core domain logic.
package queue

import (
	"testing"
)

func TestPartitionFor_KeyBased(t *testing.T) {
	// Same key must always map to the same partition.
	p1 := PartitionFor("order-123", 4, 0)
	p2 := PartitionFor("order-123", 4, 99)
	if p1 != p2 {
		t.Fatalf("key-based routing is not deterministic: got %d and %d", p1, p2)
	}
	if p1 < 0 || p1 >= 4 {
		t.Fatalf("partition %d out of range [0,4)", p1)
	}
}

func TestPartitionFor_RoundRobin(t *testing.T) {
	// Empty key must distribute across all partitions.
	seen := make(map[int]bool)
	for i := int64(0); i < 8; i++ {
		p := PartitionFor("", 4, i)
		seen[p] = true
	}
	if len(seen) != 4 {
		t.Fatalf("expected 4 distinct partitions, got %d", len(seen))
	}
}

func TestPartitionFor_SinglePartition(t *testing.T) {
	if p := PartitionFor("any-key", 1, 0); p != 0 {
		t.Fatalf("expected 0 for single partition, got %d", p)
	}
}

func BenchmarkPartitionFor(b *testing.B) {
	for i := 0; i < b.N; i++ {
		PartitionFor("order-abc-123", 16, int64(i))
	}
}

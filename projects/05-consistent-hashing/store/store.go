// Package store holds all ring instances in memory, keyed by ring ID.
package store

import (
	"fmt"
	"sync"

	"github.com/ankitsriv89/consistent-hashing/ring"
)

// RingMeta is the metadata stored alongside each ring.
type RingMeta struct {
	ID       string
	HashFn   string
	Replicas int
	Ring     *ring.Ring
}

// Store holds all rings in memory.
type Store struct {
	mu    sync.RWMutex
	rings map[string]*RingMeta
}

// New returns an empty Store.
func New() *Store {
	return &Store{rings: make(map[string]*RingMeta)}
}

// CreateRing initialises a new ring and stores it. Returns ErrAlreadyExists if id taken.
func (s *Store) CreateRing(id, hashFn string, replicas int) (*RingMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rings[id]; ok {
		return nil, fmt.Errorf("ring %q already exists", id)
	}
	m := &RingMeta{
		ID:       id,
		HashFn:   hashFn,
		Replicas: replicas,
		Ring:     ring.New(replicas),
	}
	s.rings[id] = m
	return m, nil
}

// GetRing returns the ring metadata or an error if not found.
func (s *Store) GetRing(id string) (*RingMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.rings[id]
	if !ok {
		return nil, fmt.Errorf("ring %q not found", id)
	}
	return m, nil
}

// DeleteRing removes a ring. Returns false if not found.
func (s *Store) DeleteRing(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.rings[id]
	delete(s.rings, id)
	return ok
}

// ListRings returns metadata for all rings.
func (s *Store) ListRings() []*RingMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*RingMeta, 0, len(s.rings))
	for _, m := range s.rings {
		out = append(out, m)
	}
	return out
}

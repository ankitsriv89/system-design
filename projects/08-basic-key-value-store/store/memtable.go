// Package store implements the storage engine: WAL, memtable, SSTables, and compaction.
package store

import (
	"sort"
	"sync"
)

// entry holds a value plus a tombstone flag.
// A tombstone represents a deletion; its Val is nil.
type entry struct {
	Val       []byte
	Tombstone bool
}

// memtable is an in-memory sorted map that absorbs writes before they are
// flushed to an immutable SSTable on disk.  It is protected by a RWMutex
// so concurrent reads do not block each other.
type memtable struct {
	mu   sync.RWMutex
	data map[string]entry
	size int64 // approximate byte count of key+value data
}

func newMemtable() *memtable {
	return &memtable{data: make(map[string]entry, 64)}
}

func (m *memtable) set(key string, val []byte) {
	m.mu.Lock()
	old, had := m.data[key]
	m.data[key] = entry{Val: val}
	if had {
		m.size -= int64(len(key)) + int64(len(old.Val))
	}
	m.size += int64(len(key)) + int64(len(val))
	m.mu.Unlock()
}

func (m *memtable) del(key string) {
	m.mu.Lock()
	old, had := m.data[key]
	m.data[key] = entry{Tombstone: true}
	if had {
		m.size -= int64(len(key)) + int64(len(old.Val))
	}
	m.size += int64(len(key))
	m.mu.Unlock()
}

// get returns (value, found, tombstone).
func (m *memtable) get(key string) ([]byte, bool, bool) {
	m.mu.RLock()
	e, ok := m.data[key]
	m.mu.RUnlock()
	if !ok {
		return nil, false, false
	}
	return e.Val, true, e.Tombstone
}

// sortedKeys returns all keys in lexicographic order — used when flushing to SSTable.
func (m *memtable) sortedKeys() []string {
	m.mu.RLock()
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	m.mu.RUnlock()
	sort.Strings(keys)
	return keys
}

func (m *memtable) getEntry(key string) (entry, bool) {
	m.mu.RLock()
	e, ok := m.data[key]
	m.mu.RUnlock()
	return e, ok
}

func (m *memtable) byteSize() int64 {
	m.mu.RLock()
	s := m.size
	m.mu.RUnlock()
	return s
}

func (m *memtable) len() int {
	m.mu.RLock()
	n := len(m.data)
	m.mu.RUnlock()
	return n
}

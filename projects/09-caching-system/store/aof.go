// Package store provides AOF (append-only file) persistence for warm restarts.
// Each mutation is appended as a newline-delimited JSON record. On startup the
// log is replayed in order to reconstruct the last-known cache state.
package store

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// OpType classifies a WAL record.
type OpType string

const (
	OpSet    OpType = "set"
	OpDelete OpType = "del"
	OpFlush  OpType = "flush"
)

// Record is a single WAL entry.
type Record struct {
	Op        OpType    `json:"op"`
	Key       string    `json:"key,omitempty"`
	Value     string    `json:"value,omitempty"`
	TTLMs     int64     `json:"ttl_ms,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	At        time.Time `json:"at"`
}

// SetEntry is what the store hands back on replay.
type SetEntry struct {
	Key       string
	Value     string
	ExpiresAt time.Time
}

// AOF is an append-only file log.
type AOF struct {
	mu  sync.Mutex
	f   *os.File
	enc *json.Encoder
	log *zap.Logger
}

// Open opens or creates the AOF file at path.
func Open(path string, log *zap.Logger) (*AOF, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &AOF{f: f, enc: json.NewEncoder(f), log: log}, nil
}

// AppendSet records a SET operation.
func (a *AOF) AppendSet(key, value string, ttlMs int64, expiresAt time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.enc.Encode(Record{
		Op:        OpSet,
		Key:       key,
		Value:     value,
		TTLMs:     ttlMs,
		ExpiresAt: expiresAt,
		At:        time.Now(),
	})
}

// AppendDelete records a DELETE operation.
func (a *AOF) AppendDelete(key string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.enc.Encode(Record{Op: OpDelete, Key: key, At: time.Now()})
}

// AppendFlush records a FLUSH operation.
func (a *AOF) AppendFlush() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.enc.Encode(Record{Op: OpFlush, At: time.Now()})
}

// Replay reads the AOF file from the beginning and returns entries that are
// still valid (i.e. not deleted/flushed and not yet expired).
func (a *AOF) Replay() ([]SetEntry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, err := a.f.Seek(0, 0); err != nil {
		return nil, err
	}

	now := time.Now()
	state := make(map[string]SetEntry)

	scanner := bufio.NewScanner(a.f)
	for scanner.Scan() {
		var rec Record
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			a.log.Warn("aof: skip malformed record", zap.Error(err))
			continue
		}
		switch rec.Op {
		case OpSet:
			if rec.ExpiresAt.IsZero() || rec.ExpiresAt.After(now) {
				state[rec.Key] = SetEntry{
					Key:       rec.Key,
					Value:     rec.Value,
					ExpiresAt: rec.ExpiresAt,
				}
			}
		case OpDelete:
			delete(state, rec.Key)
		case OpFlush:
			state = make(map[string]SetEntry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	out := make([]SetEntry, 0, len(state))
	for _, e := range state {
		out = append(out, e)
	}
	return out, nil
}

// Close flushes and closes the underlying file.
func (a *AOF) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.f.Close()
}

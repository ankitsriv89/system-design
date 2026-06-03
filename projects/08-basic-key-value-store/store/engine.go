// Package store implements the storage engine: WAL, memtable, SSTables, and compaction.
package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

const (
	// flushThresholdBytes triggers a memtable flush when the memtable reaches this size.
	flushThresholdBytes = 4 * 1024 * 1024 // 4 MiB

	// l0CompactThreshold is the number of L0 SSTables that triggers a compaction.
	l0CompactThreshold = 4
)

// Stats holds observable counters that the metrics layer reads.
type Stats struct {
	Writes       atomic.Int64
	Reads        atomic.Int64
	Deletes      atomic.Int64
	Flushes      atomic.Int64
	Compactions  atomic.Int64
	MemtableSize atomic.Int64
	SSTCount     atomic.Int64
	WALEntries   atomic.Int64
}

// Engine is the top-level key-value store.  It coordinates the WAL,
// the active memtable, immutable SSTables, and background compaction.
//
// Concurrency model:
//   - mu guards all mutable fields (sstables, seqNum, flushing).
//   - Reads first check the memtable (RWMutex), then scan SSTables newest-first.
//   - Writes go to WAL then memtable; a flush is triggered when the memtable
//     exceeds flushThresholdBytes.
//   - Background compaction runs in its own goroutine and holds mu only when
//     swapping the SSTable list.
type Engine struct {
	mu       sync.RWMutex
	dataDir  string
	wal      *WAL
	mem      *memtable
	sstables []SSTableMeta // ordered newest-first
	seqNum   uint64
	flushing bool

	log   *zap.Logger
	Stats Stats

	cancel context.CancelFunc
	done   chan struct{}
}

// Open creates or reopens a key-value store rooted at dataDir.
// Any existing WAL is replayed to reconstruct the memtable before returning.
func Open(dataDir string, log *zap.Logger) (*Engine, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("engine: mkdir %s: %w", dataDir, err)
	}

	wal, err := openWAL(walPath(dataDir))
	if err != nil {
		return nil, err
	}

	e := &Engine{
		dataDir: dataDir,
		wal:     wal,
		mem:     newMemtable(),
		log:     log,
		done:    make(chan struct{}),
	}

	if err := e.recoverSSTables(); err != nil {
		return nil, err
	}
	if err := e.replayWAL(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	go e.compactionLoop(ctx)

	log.Info("engine open", zap.String("data_dir", dataDir), zap.Int("sst_count", len(e.sstables)))
	return e, nil
}

// replayWAL rebuilds the memtable from the WAL after a restart.
func (e *Engine) replayWAL() error {
	var n int64
	err := e.wal.Replay(func(entry walEntry) {
		switch entry.Op {
		case opSet:
			e.mem.set(entry.Key, entry.Val)
		case opDel:
			e.mem.del(entry.Key)
		}
		n++
	})
	if err != nil {
		return fmt.Errorf("engine: wal replay: %w", err)
	}
	e.Stats.WALEntries.Store(n)
	e.log.Info("wal replayed", zap.Int64("entries", n))
	return nil
}

// recoverSSTables discovers existing SSTable files on disk and loads their metadata.
func (e *Engine) recoverSSTables() error {
	entries, err := filepath.Glob(filepath.Join(e.dataDir, "sst-*.sst"))
	if err != nil {
		return fmt.Errorf("engine: glob sst: %w", err)
	}
	for _, p := range entries {
		meta, err := parseSSTMeta(p)
		if err != nil {
			e.log.Warn("skip corrupt sst", zap.String("path", p), zap.Error(err))
			continue
		}
		e.sstables = append(e.sstables, meta)
		if meta.SeqNum > e.seqNum {
			e.seqNum = meta.SeqNum
		}
	}
	// Sort newest-first (higher seqNum = newer).
	sort.Slice(e.sstables, func(i, j int) bool {
		return e.sstables[i].SeqNum > e.sstables[j].SeqNum
	})
	e.Stats.SSTCount.Store(int64(len(e.sstables)))
	return nil
}

// Set writes key=value durably.
func (e *Engine) Set(key string, val []byte) error {
	if err := e.wal.Append(walEntry{Op: opSet, Key: key, Val: val}); err != nil {
		return fmt.Errorf("engine: set wal: %w", err)
	}
	e.mem.set(key, val)
	e.Stats.Writes.Add(1)
	e.Stats.WALEntries.Add(1)
	e.Stats.MemtableSize.Store(e.mem.byteSize())

	if e.mem.byteSize() >= flushThresholdBytes {
		go e.triggerFlush()
	}
	return nil
}

// Delete marks key as deleted.
func (e *Engine) Delete(key string) error {
	if err := e.wal.Append(walEntry{Op: opDel, Key: key, Val: nil}); err != nil {
		return fmt.Errorf("engine: del wal: %w", err)
	}
	e.mem.del(key)
	e.Stats.Deletes.Add(1)
	e.Stats.WALEntries.Add(1)
	return nil
}

// Get returns (value, found, error).
// Search order: memtable → SSTables newest-first.
func (e *Engine) Get(key string) ([]byte, bool, error) {
	e.Stats.Reads.Add(1)

	// Check active memtable first.
	if val, ok, tombstone := e.mem.get(key); ok {
		if tombstone {
			return nil, false, nil
		}
		return val, true, nil
	}

	// Scan SSTables newest-first.
	e.mu.RLock()
	ssts := make([]SSTableMeta, len(e.sstables))
	copy(ssts, e.sstables)
	e.mu.RUnlock()

	for _, sst := range ssts {
		val, found, tombstone, err := lookupSST(sst.Path, key)
		if err != nil {
			e.log.Warn("sst lookup error", zap.String("path", sst.Path), zap.Error(err))
			continue
		}
		if found {
			if tombstone {
				return nil, false, nil
			}
			return val, true, nil
		}
	}
	return nil, false, nil
}

// triggerFlush initiates a memtable flush if one is not already in progress.
func (e *Engine) triggerFlush() {
	e.mu.Lock()
	if e.flushing {
		e.mu.Unlock()
		return
	}
	e.flushing = true
	e.mu.Unlock()

	if err := e.flushMemtable(); err != nil {
		e.log.Error("flush failed", zap.Error(err))
	}

	e.mu.Lock()
	e.flushing = false
	e.mu.Unlock()
}

// flushMemtable writes the active memtable to a new SSTable and resets it.
func (e *Engine) flushMemtable() error {
	e.mu.Lock()
	e.seqNum++
	seq := e.seqNum
	oldMem := e.mem
	e.mem = newMemtable()
	e.mu.Unlock()

	meta, err := writeSSTFromMemtable(e.dataDir, seq, oldMem)
	if err != nil {
		// Restore the old memtable — the write was not acknowledged to clients.
		e.mu.Lock()
		e.seqNum--
		e.mem = oldMem
		e.mu.Unlock()
		return fmt.Errorf("engine: flush: %w", err)
	}

	// Truncate the WAL to entries not yet flushed (the new empty memtable has none).
	if err := e.wal.Truncate(nil); err != nil {
		e.log.Warn("wal truncate failed", zap.Error(err))
	}

	e.mu.Lock()
	// Insert newest-first.
	e.sstables = append([]SSTableMeta{meta}, e.sstables...)
	e.mu.Unlock()

	e.Stats.Flushes.Add(1)
	e.Stats.SSTCount.Store(int64(len(e.sstables)))
	e.Stats.MemtableSize.Store(0)
	e.log.Info("memtable flushed", zap.Uint64("seq", seq), zap.Int("keys", meta.Count))
	return nil
}

// Compact manually triggers a compaction regardless of thresholds.
func (e *Engine) Compact() error {
	return e.runCompaction()
}

// compactionLoop runs background compaction whenever L0 has enough files.
func (e *Engine) compactionLoop(ctx context.Context) {
	defer close(e.done)
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.mu.RLock()
			l0Count := 0
			for _, s := range e.sstables {
				if s.Level == 0 {
					l0Count++
				}
			}
			e.mu.RUnlock()
			if l0Count >= l0CompactThreshold {
				if err := e.runCompaction(); err != nil {
					e.log.Error("compaction failed", zap.Error(err))
				}
			}
		}
	}
}

// runCompaction merges all L0 SSTables into a single L1 SSTable.
func (e *Engine) runCompaction() error {
	e.mu.Lock()
	var l0 []SSTableMeta
	var rest []SSTableMeta
	for _, s := range e.sstables {
		if s.Level == 0 {
			l0 = append(l0, s)
		} else {
			rest = append(rest, s)
		}
	}
	if len(l0) < 2 {
		e.mu.Unlock()
		return nil
	}
	e.seqNum++
	seq := e.seqNum
	e.mu.Unlock()

	// Build path list oldest-first (reverse of our newest-first slice).
	paths := make([]string, len(l0))
	for i, m := range l0 {
		paths[len(l0)-1-i] = m.Path
	}

	merged, err := mergeCompact(e.dataDir, seq, 1, paths)
	if err != nil {
		return fmt.Errorf("engine: compact: %w", err)
	}

	e.mu.Lock()
	// Remove old L0 files and prepend the new L1 file.
	newList := []SSTableMeta{merged}
	newList = append(newList, rest...)
	e.sstables = newList
	e.mu.Unlock()

	// Delete old L0 files from disk after the manifest is updated.
	for _, m := range l0 {
		if err := os.Remove(m.Path); err != nil {
			e.log.Warn("remove old sst", zap.String("path", m.Path), zap.Error(err))
		}
	}

	e.Stats.Compactions.Add(1)
	e.Stats.SSTCount.Store(int64(len(e.sstables)))
	e.log.Info("compaction done",
		zap.Int("l0_merged", len(l0)),
		zap.Uint64("out_seq", seq),
		zap.Int("out_keys", merged.Count),
	)
	return nil
}

// SSTables returns a snapshot of current SSTable metadata for the API.
func (e *Engine) SSTables() []SSTableMeta {
	e.mu.RLock()
	out := make([]SSTableMeta, len(e.sstables))
	copy(out, e.sstables)
	e.mu.RUnlock()
	return out
}

// MemtableLen returns the number of entries in the active memtable.
func (e *Engine) MemtableLen() int {
	return e.mem.len()
}

// Close flushes the active memtable and shuts down background goroutines.
func (e *Engine) Close() error {
	e.cancel()
	<-e.done

	// Flush any remaining memtable entries so no data is lost on graceful shutdown.
	if e.mem.len() > 0 {
		if err := e.flushMemtable(); err != nil {
			e.log.Error("final flush on close", zap.Error(err))
		}
	}
	return e.wal.Close()
}

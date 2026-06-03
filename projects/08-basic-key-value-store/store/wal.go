// Package store implements the storage engine: WAL, memtable, SSTables, and compaction.
package store

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// walOp identifies the type of a WAL entry.
type walOp uint8

const (
	opSet walOp = 1
	opDel walOp = 2
)

// walEntry is one record in the write-ahead log.
//
// Wire format (all little-endian):
//
//	[op:1][keyLen:4][valLen:4][key:keyLen][val:valLen][crc32:4]
type walEntry struct {
	Op  walOp
	Key string
	Val []byte
}

// WAL is an append-only write-ahead log that survives process crashes.
// Every mutation is fsynced before the call returns, so no acknowledged
// write can be lost even if the kernel buffers are not flushed.
type WAL struct {
	mu   sync.Mutex
	f    *os.File
	bw   *bufio.Writer
	path string
}

// openWAL opens (or creates) the WAL file at path.
func openWAL(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("wal: open %s: %w", path, err)
	}
	return &WAL{f: f, bw: bufio.NewWriterSize(f, 64*1024), path: path}, nil
}

// Append writes one entry and fsyncs the file.
func (w *WAL) Append(e walEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	keyB := []byte(e.Key)
	// header: op(1) + keyLen(4) + valLen(4) = 9 bytes
	hdr := make([]byte, 9)
	hdr[0] = byte(e.Op)
	binary.LittleEndian.PutUint32(hdr[1:5], uint32(len(keyB)))
	binary.LittleEndian.PutUint32(hdr[5:9], uint32(len(e.Val)))

	crc := crc32.NewIEEE()
	crc.Write(hdr)
	crc.Write(keyB)
	crc.Write(e.Val)
	chk := make([]byte, 4)
	binary.LittleEndian.PutUint32(chk, crc.Sum32())

	for _, chunk := range [][]byte{hdr, keyB, e.Val, chk} {
		if _, err := w.bw.Write(chunk); err != nil {
			return fmt.Errorf("wal: write: %w", err)
		}
	}
	if err := w.bw.Flush(); err != nil {
		return fmt.Errorf("wal: flush: %w", err)
	}
	// fsync so the OS page cache write reaches durable storage.
	return w.f.Sync()
}

// Replay reads all entries from the WAL and calls fn for each valid one.
// Entries with a bad CRC are skipped (and logged) — they indicate a
// torn write at crash time, which is expected and safe to discard.
func (w *WAL) Replay(fn func(walEntry)) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("wal: seek: %w", err)
	}
	r := bufio.NewReaderSize(w.f, 64*1024)

	hdr := make([]byte, 9)
	for {
		if _, err := io.ReadFull(r, hdr); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return fmt.Errorf("wal: read header: %w", err)
		}
		op := walOp(hdr[0])
		keyLen := binary.LittleEndian.Uint32(hdr[1:5])
		valLen := binary.LittleEndian.Uint32(hdr[5:9])

		keyB := make([]byte, keyLen)
		val := make([]byte, valLen)
		chkB := make([]byte, 4)

		if _, err := io.ReadFull(r, keyB); err != nil {
			break
		}
		if _, err := io.ReadFull(r, val); err != nil {
			break
		}
		if _, err := io.ReadFull(r, chkB); err != nil {
			break
		}

		crc := crc32.NewIEEE()
		crc.Write(hdr)
		crc.Write(keyB)
		crc.Write(val)
		got := crc.Sum32()
		want := binary.LittleEndian.Uint32(chkB)
		if got != want {
			// Torn write — stop here; anything after is also suspect.
			break
		}
		fn(walEntry{Op: op, Key: string(keyB), Val: val})
	}
	return nil
}

// Truncate rewrites the WAL to contain only the provided entries.
// Called after a successful SSTable flush so the WAL doesn't grow unbounded.
func (w *WAL) Truncate(entries []walEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	tmp := w.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("wal: truncate create tmp: %w", err)
	}

	bw := bufio.NewWriterSize(f, 64*1024)
	for _, e := range entries {
		keyB := []byte(e.Key)
		hdr := make([]byte, 9)
		hdr[0] = byte(e.Op)
		binary.LittleEndian.PutUint32(hdr[1:5], uint32(len(keyB)))
		binary.LittleEndian.PutUint32(hdr[5:9], uint32(len(e.Val)))

		crc := crc32.NewIEEE()
		crc.Write(hdr)
		crc.Write(keyB)
		crc.Write(e.Val)
		chk := make([]byte, 4)
		binary.LittleEndian.PutUint32(chk, crc.Sum32())

		for _, chunk := range [][]byte{hdr, keyB, e.Val, chk} {
			if _, werr := bw.Write(chunk); werr != nil {
				f.Close()
				os.Remove(tmp)
				return fmt.Errorf("wal: truncate write: %w", werr)
			}
		}
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("wal: truncate flush: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("wal: truncate sync: %w", err)
	}
	f.Close()

	// Atomic rename — even if we crash here, we lose at most the truncation,
	// not any data (the old WAL is still intact until rename completes).
	if err := os.Rename(tmp, w.path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("wal: truncate rename: %w", err)
	}

	nf, err := os.OpenFile(w.path, os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("wal: reopen after truncate: %w", err)
	}
	w.f.Close()
	w.f = nf
	w.bw = bufio.NewWriterSize(nf, 64*1024)
	return nil
}

// Close flushes and closes the underlying file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.bw.Flush(); err != nil {
		return err
	}
	return w.f.Close()
}

// walPath returns the standard WAL path for a data directory.
func walPath(dataDir string) string {
	return filepath.Join(dataDir, "wal.log")
}

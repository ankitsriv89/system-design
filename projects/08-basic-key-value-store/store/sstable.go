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
	"sort"
	"strconv"
	"strings"
)

// SSTable is an immutable, sorted file of key-value pairs written when the
// memtable exceeds the flush threshold.  Each entry is self-describing with
// a CRC so corruption is detected on read.
//
// Wire format per entry (little-endian):
//
//	[flags:1][keyLen:4][valLen:4][key:keyLen][val:valLen][crc32:4]
//
// flags bit 0 = tombstone.

const (
	sstFlagTombstone byte = 0x01
)

// SSTableMeta describes a single SSTable file without loading its data.
type SSTableMeta struct {
	Path   string
	SeqNum uint64
	Level  int
	MinKey string
	MaxKey string
	Count  int
}

// writeSSTFromMemtable flushes the memtable to a new SSTable file and
// returns the metadata.  seqNum must be globally monotone — callers hold
// the engine lock when calling this.
func writeSSTFromMemtable(dataDir string, seqNum uint64, mem *memtable) (SSTableMeta, error) {
	path := sstPath(dataDir, seqNum, 0)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return SSTableMeta{}, fmt.Errorf("sst: create %s: %w", path, err)
	}
	defer f.Close()

	bw := bufio.NewWriterSize(f, 128*1024)
	keys := mem.sortedKeys()
	var minKey, maxKey string
	count := 0

	for i, k := range keys {
		e, _ := mem.getEntry(k)
		if i == 0 {
			minKey = k
		}
		maxKey = k

		var flags byte
		if e.Tombstone {
			flags |= sstFlagTombstone
		}
		keyB := []byte(k)
		hdr := make([]byte, 9)
		hdr[0] = flags
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
				return SSTableMeta{}, fmt.Errorf("sst: write entry: %w", werr)
			}
		}
		count++
	}

	if err := bw.Flush(); err != nil {
		return SSTableMeta{}, fmt.Errorf("sst: flush: %w", err)
	}
	if err := f.Sync(); err != nil {
		return SSTableMeta{}, fmt.Errorf("sst: sync: %w", err)
	}

	return SSTableMeta{
		Path:   path,
		SeqNum: seqNum,
		Level:  0,
		MinKey: minKey,
		MaxKey: maxKey,
		Count:  count,
	}, nil
}

// scanSST reads all entries from an SSTable and calls fn for each.
func scanSST(path string, fn func(key string, val []byte, tombstone bool)) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("sst: open %s: %w", path, err)
	}
	defer f.Close()

	r := bufio.NewReaderSize(f, 128*1024)
	hdr := make([]byte, 9)
	for {
		if _, err := io.ReadFull(r, hdr); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			return fmt.Errorf("sst: read header: %w", err)
		}
		flags := hdr[0]
		keyLen := binary.LittleEndian.Uint32(hdr[1:5])
		valLen := binary.LittleEndian.Uint32(hdr[5:9])

		keyB := make([]byte, keyLen)
		val := make([]byte, valLen)
		chkB := make([]byte, 4)

		if _, err := io.ReadFull(r, keyB); err != nil {
			return fmt.Errorf("sst: read key: %w", err)
		}
		if _, err := io.ReadFull(r, val); err != nil {
			return fmt.Errorf("sst: read val: %w", err)
		}
		if _, err := io.ReadFull(r, chkB); err != nil {
			return fmt.Errorf("sst: read crc: %w", err)
		}

		crc := crc32.NewIEEE()
		crc.Write(hdr)
		crc.Write(keyB)
		crc.Write(val)
		if crc.Sum32() != binary.LittleEndian.Uint32(chkB) {
			return fmt.Errorf("sst: crc mismatch at key %q", string(keyB))
		}
		fn(string(keyB), val, flags&sstFlagTombstone != 0)
	}
}

// lookupSST performs a linear scan of a single SSTable for key.
// For a production engine this would use a sparse index + binary search;
// a linear scan is correct and sufficient for the MVP.
func lookupSST(path, key string) ([]byte, bool, bool, error) {
	var foundVal []byte
	var foundTombstone bool
	found := false

	err := scanSST(path, func(k string, val []byte, tombstone bool) {
		if k == key {
			v := make([]byte, len(val))
			copy(v, val)
			foundVal = v
			foundTombstone = tombstone
			found = true
		}
	})
	return foundVal, found, foundTombstone, err
}

// mergeCompact merges the given SSTable paths into a single new SSTable at
// the target level, dropping superseded versions and resolved tombstones.
// Earlier paths (lower index) are older; later paths take precedence.
func mergeCompact(dataDir string, seqNum uint64, level int, paths []string) (SSTableMeta, error) {
	// Merge newest-wins: scan all SSTables and build a map, processing
	// oldest first so newer writes overwrite older ones.
	merged := make(map[string]entry)
	var orderedKeys []string

	for _, p := range paths {
		err := scanSST(p, func(k string, val []byte, tombstone bool) {
			v := make([]byte, len(val))
			copy(v, val)
			if _, exists := merged[k]; !exists {
				orderedKeys = append(orderedKeys, k)
			}
			merged[k] = entry{Val: v, Tombstone: tombstone}
		})
		if err != nil {
			return SSTableMeta{}, fmt.Errorf("compact: scan %s: %w", p, err)
		}
	}

	// Deduplicate orderedKeys and sort.
	seen := make(map[string]struct{}, len(orderedKeys))
	uniqueKeys := orderedKeys[:0]
	for _, k := range orderedKeys {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			uniqueKeys = append(uniqueKeys, k)
		}
	}
	sort.Strings(uniqueKeys)

	outPath := sstPath(dataDir, seqNum, level)
	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return SSTableMeta{}, fmt.Errorf("compact: create output: %w", err)
	}
	defer f.Close()

	bw := bufio.NewWriterSize(f, 256*1024)
	var minKey, maxKey string
	count := 0

	for i, k := range uniqueKeys {
		e := merged[k]
		// Drop tombstones during compaction — they are no longer needed once
		// all older SSTables that could have the key are being merged away.
		// We keep tombstones only at level 0 (not yet fully compacted).
		if e.Tombstone && level > 0 {
			continue
		}
		if count == 0 || i == 0 {
			minKey = k
		}
		maxKey = k

		var flags byte
		if e.Tombstone {
			flags |= sstFlagTombstone
		}
		keyB := []byte(k)
		hdr := make([]byte, 9)
		hdr[0] = flags
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
				return SSTableMeta{}, fmt.Errorf("compact: write: %w", werr)
			}
		}
		count++
	}
	if err := bw.Flush(); err != nil {
		return SSTableMeta{}, fmt.Errorf("compact: flush: %w", err)
	}
	if err := f.Sync(); err != nil {
		return SSTableMeta{}, fmt.Errorf("compact: sync: %w", err)
	}

	return SSTableMeta{
		Path:   outPath,
		SeqNum: seqNum,
		Level:  level,
		MinKey: minKey,
		MaxKey: maxKey,
		Count:  count,
	}, nil
}

// sstPath returns the canonical file name for an SSTable.
// Format: <dataDir>/sst-<seqNum>-L<level>.sst
func sstPath(dataDir string, seqNum uint64, level int) string {
	return filepath.Join(dataDir, fmt.Sprintf("sst-%020d-L%d.sst", seqNum, level))
}

// parseSSTMeta reads metadata from an SSTable file on disk by scanning it once.
func parseSSTMeta(path string) (SSTableMeta, error) {
	base := filepath.Base(path)
	// base: sst-<seqNum>-L<level>.sst
	parts := strings.Split(strings.TrimSuffix(base, ".sst"), "-")
	if len(parts) < 3 {
		return SSTableMeta{}, fmt.Errorf("sst: bad filename %q", base)
	}
	seq, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return SSTableMeta{}, fmt.Errorf("sst: parse seqnum: %w", err)
	}
	levelStr := strings.TrimPrefix(parts[2], "L")
	level, err := strconv.Atoi(levelStr)
	if err != nil {
		return SSTableMeta{}, fmt.Errorf("sst: parse level: %w", err)
	}

	meta := SSTableMeta{Path: path, SeqNum: seq, Level: level}
	err = scanSST(path, func(k string, _ []byte, _ bool) {
		if meta.MinKey == "" {
			meta.MinKey = k
		}
		meta.MaxKey = k
		meta.Count++
	})
	return meta, err
}

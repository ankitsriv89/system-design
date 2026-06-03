// Package store implements the storage engine: WAL, memtable, SSTables, and compaction.
package store

import (
	"fmt"
	"os"
	"testing"

	"go.uber.org/zap"
)

func testEngine(t *testing.T) (*Engine, string) {
	t.Helper()
	dir := t.TempDir()
	log, _ := zap.NewDevelopment()
	e, err := Open(dir, log)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { e.Close() })
	return e, dir
}

func TestSetGet(t *testing.T) {
	e, _ := testEngine(t)
	if err := e.Set("hello", []byte("world")); err != nil {
		t.Fatal(err)
	}
	val, ok, err := e.Get("hello")
	if err != nil || !ok || string(val) != "world" {
		t.Fatalf("Get: val=%q ok=%v err=%v", val, ok, err)
	}
}

func TestDelete(t *testing.T) {
	e, _ := testEngine(t)
	e.Set("foo", []byte("bar"))
	e.Delete("foo")
	_, ok, err := e.Get("foo")
	if err != nil || ok {
		t.Fatalf("expected key deleted, got ok=%v err=%v", ok, err)
	}
}

func TestMissingKey(t *testing.T) {
	e, _ := testEngine(t)
	_, ok, err := e.Get("nonexistent")
	if err != nil || ok {
		t.Fatalf("expected not found, got ok=%v err=%v", ok, err)
	}
}

func TestWALRecovery(t *testing.T) {
	dir := t.TempDir()
	log, _ := zap.NewDevelopment()

	// Write some keys.
	e1, err := Open(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	e1.Set("k1", []byte("v1"))
	e1.Set("k2", []byte("v2"))
	e1.Delete("k1")
	// Close without flushing — WAL must survive.
	e1.wal.Close()
	e1.cancel()
	<-e1.done

	// Reopen and verify recovery.
	e2, err := Open(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	defer e2.Close()

	_, ok, _ := e2.Get("k1")
	if ok {
		t.Error("k1 should be deleted after recovery")
	}
	val, ok, _ := e2.Get("k2")
	if !ok || string(val) != "v2" {
		t.Errorf("k2 recovery: val=%q ok=%v", val, ok)
	}
}

func TestFlushAndReadFromSST(t *testing.T) {
	e, _ := testEngine(t)

	// Write enough data to trigger a flush.
	const n = 500
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("key-%05d", i)
		val := fmt.Sprintf("value-%05d", i)
		e.Set(key, []byte(val))
	}

	// Manually flush.
	if err := e.flushMemtable(); err != nil {
		t.Fatal(err)
	}

	// Keys should still be readable from the SSTable.
	for i := 0; i < n; i += 50 {
		key := fmt.Sprintf("key-%05d", i)
		val, ok, err := e.Get(key)
		if err != nil || !ok {
			t.Errorf("key %s not found after flush: err=%v", key, err)
			continue
		}
		if string(val) != fmt.Sprintf("value-%05d", i) {
			t.Errorf("key %s: got %q", key, val)
		}
	}
}

func TestCompaction(t *testing.T) {
	e, _ := testEngine(t)

	// Write and flush multiple times to create several L0 SSTables.
	for round := 0; round < 3; round++ {
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key-%04d", i)
			val := fmt.Sprintf("round-%d-val-%04d", round, i)
			e.Set(key, []byte(val))
		}
		e.flushMemtable()
	}

	if err := e.Compact(); err != nil {
		t.Fatal(err)
	}

	// After compaction, only the latest value per key survives.
	for i := 0; i < 100; i += 10 {
		key := fmt.Sprintf("key-%04d", i)
		val, ok, err := e.Get(key)
		if err != nil || !ok {
			t.Errorf("key %s missing after compaction: err=%v", key, err)
			continue
		}
		want := fmt.Sprintf("round-2-val-%04d", i)
		if string(val) != want {
			t.Errorf("key %s: got %q want %q", key, val, want)
		}
	}
}

func TestWAL_AppendAndReplay(t *testing.T) {
	dir := t.TempDir()
	path := walPath(dir)

	w, err := openWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	entries := []walEntry{
		{Op: opSet, Key: "a", Val: []byte("1")},
		{Op: opSet, Key: "b", Val: []byte("2")},
		{Op: opDel, Key: "a"},
	}
	for _, e := range entries {
		if err := w.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	w2, _ := openWAL(path)
	var replayed []walEntry
	w2.Replay(func(e walEntry) { replayed = append(replayed, e) })
	w2.Close()

	if len(replayed) != len(entries) {
		t.Fatalf("replayed %d entries, want %d", len(replayed), len(entries))
	}
	for i, e := range replayed {
		if e.Op != entries[i].Op || e.Key != entries[i].Key || string(e.Val) != string(entries[i].Val) {
			t.Errorf("entry %d mismatch: got %+v want %+v", i, e, entries[i])
		}
	}
}

func BenchmarkSet(b *testing.B) {
	dir := b.TempDir()
	log, _ := zap.NewNop(), (*zap.Logger)(nil)
	_ = log
	l, _ := zap.NewProduction()
	e, _ := Open(dir, l)
	defer e.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Set(fmt.Sprintf("bench-key-%d", i), []byte("bench-value"))
	}
}

func BenchmarkGet(b *testing.B) {
	dir := b.TempDir()
	l, _ := zap.NewProduction()
	e, _ := Open(dir, l)
	defer e.Close()

	for i := 0; i < 1000; i++ {
		e.Set(fmt.Sprintf("bench-key-%d", i), []byte("bench-value"))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Get(fmt.Sprintf("bench-key-%d", i%1000))
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

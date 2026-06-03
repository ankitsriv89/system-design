# Code Flow — Basic Key-Value Store (08)

## Write (SET) Path

```mermaid
flowchart TD
    A["PUT /v1/kv/{key}"] --> B["handler.handleSet"]
    B --> C{"key empty?"}
    C -->|yes| BADREQ["400 Bad Request"]
    C -->|no| D["io.ReadAll (limit 1 MiB)"]
    D --> E{"> 1 MiB?"}
    E -->|yes| TOO["413 Too Large"]
    E -->|no| F["engine.Set(key, val)"]
    F --> G["wal.Append(opSet, key, val)"]
    G --> H["bufio.Write header + key + val + crc32"]
    H --> I["bufio.Flush → os.File.Sync (fsync)"]
    I -->|error| WERR["return error → 500"]
    I -->|ok| J["memtable.set(key, val)"]
    J --> K["Stats.Writes.Add(1)"]
    K --> L{"memtable.byteSize ≥ 4 MiB?"}
    L -->|yes| M["go triggerFlush()"]
    L -->|no| OK["200 OK"]
    M --> OK
```

### Why each step

- **WAL first, then memtable**: the WAL fsync is the durability guarantee. If we updated the memtable first and then crashed, the write would be visible to readers but unrecoverable.
- **bufio.Writer**: amortises syscall overhead; `Flush` triggers the actual write then `Sync` ensures the data reaches durable storage.
- **CRC32 per entry**: detects torn writes (e.g., partial last entry after a crash). The WAL replayer stops at the first bad CRC.
- **Async flush**: the HTTP handler returns before the flush completes. The write is already durable (it's in the WAL), so there is no correctness issue.

---

## Read (GET) Path

```mermaid
flowchart TD
    A["GET /v1/kv/{key}"] --> B["handler.handleGet"]
    B --> C["engine.Get(key)"]
    C --> D["memtable.get(key)"]
    D -->|found + tombstone| NDEL["nil, false"]
    D -->|found + value| HIT["return value"]
    D -->|not found| E["acquire engine.mu.RLock, copy sst slice"]
    E --> F["for each SST newest-first"]
    F --> G["lookupSST(path, key)"]
    G --> H["scanSST: linear scan, CRC verify"]
    H -->|found + tombstone| NDEL
    H -->|found + value| HIT
    H -->|not found| F
    F -->|exhausted| MISS["nil, false → 404"]
    HIT --> OK["200 + value bytes"]
```

### Why each step

- **Memtable first**: the memtable always has the newest version of any recently written key, so checking it first both speeds up the common case and avoids stale reads from older SSTables.
- **Newest-first SSTable scan**: SSTables are ordered by sequence number descending. The first SSTable that contains the key is the authoritative version; earlier files are older.
- **Tombstone check at every level**: a deletion record (tombstone) in a newer SSTable supersedes a value in an older SSTable. We must honour it even if the value is readable.
- **RLock copy**: we copy the SSTable slice under a read lock then release before doing I/O, so compaction can proceed concurrently without holding the lock during slow disk reads.

---

## Memtable Flush

```mermaid
flowchart TD
    A["triggerFlush"] --> B{"engine.flushing?"}
    B -->|yes| EXIT["return (already in progress)"]
    B -->|no| C["flushing = true"]
    C --> D["engine.mu.Lock"]
    D --> E["seqNum++, swap mem = newMemtable()"]
    E --> F["engine.mu.Unlock"]
    F --> G["writeSSTFromMemtable(dataDir, seq, oldMem)"]
    G --> H["iterate sortedKeys → write entries to sst-N-L0.sst"]
    H --> I["Flush + Sync new SSTable file"]
    I --> J["wal.Truncate(nil) — reset WAL"]
    J --> K["engine.mu.Lock → prepend meta to sstables"]
    K --> L["engine.mu.Unlock"]
    L --> M["flushing = false"]
```

### Why each step

- **Swap memtable under lock**: we hold the engine lock only for the in-memory pointer swap (nanoseconds). The actual disk write happens outside the lock so writes can continue into the new memtable concurrently.
- **WAL truncation after flush**: once the SSTable is safely on disk, the WAL entries it covers are redundant. Truncation keeps the WAL small and speeds up crash recovery.
- **Prepend (newest-first)**: the read path scans newest-first; prepending the new SSTable ensures reads immediately see fresh data without re-sorting.

---

## Compaction

```mermaid
flowchart TD
    A["compactionLoop (every 10s)"] --> B{"L0 count ≥ 4?"}
    B -->|no| A
    B -->|yes| C["runCompaction"]
    C --> D["engine.mu.Lock: collect L0 metas, seqNum++"]
    D --> E["engine.mu.Unlock"]
    E --> F["mergeCompact(dataDir, seq, level=1, paths oldest-first)"]
    F --> G["scanSST each input file → merged map, newest wins"]
    G --> H["write sst-N-L1.sst (drop tombstones at level>0)"]
    H --> I["engine.mu.Lock: replace sstables list"]
    I --> J["engine.mu.Unlock"]
    J --> K["os.Remove each old L0 file"]
    K --> L["Stats.Compactions.Add(1)"]
```

### Why each step

- **Oldest-first scan**: scanning older files first then letting newer files overwrite in the merged map gives newest-wins semantics cheaply — no per-key version comparison needed.
- **Drop tombstones at L1**: a tombstone in an L1 file means all older L0 files that could have contained the key are now merged away. The tombstone record is safe to drop, reclaiming the space.
- **Delete after manifest update**: we remove the old SSTable files only after the new SSTable and the new sstables slice are in place. A crash between the slice update and the file deletes leaves orphaned (but harmless) files; they will be ignored on next open because they are not in the in-memory list.

---

## Call Graph Summary

```mermaid
graph LR
    main --> engine.Open
    main --> api.New
    main --> http.ListenAndServe
    api.handleSet --> engine.Set
    api.handleGet --> engine.Get
    api.handleDelete --> engine.Delete
    api.handleCompact --> engine.Compact
    engine.Set --> wal.Append
    engine.Set --> memtable.set
    engine.Get --> memtable.get
    engine.Get --> lookupSST
    engine.triggerFlush --> engine.flushMemtable
    engine.flushMemtable --> writeSSTFromMemtable
    engine.flushMemtable --> wal.Truncate
    engine.compactionLoop --> engine.runCompaction
    engine.runCompaction --> mergeCompact
    engine.runCompaction --> os.Remove
```

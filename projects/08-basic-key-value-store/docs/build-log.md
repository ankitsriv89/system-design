# Build Log — Basic Key-Value Store (08)

## Environment

| Item | Value |
|---|---|
| Go version | go1.22 |
| Module path | `github.com/ankitsriv89/08-basic-key-value-store` |
| Build platform | linux/arm64 (Oracle Cloud free-tier) |
| Build date | 2026-06-03 |

## Direct Dependencies

| Package | Version | Role |
|---|---|---|
| `github.com/gorilla/mux` | v1.8.1 | HTTP router with named URL parameters |
| `github.com/prometheus/client_golang` | v1.19.1 | Prometheus metrics instrumentation |
| `go.uber.org/zap` | v1.27.0 | Structured, high-performance logging |

All other entries in `go.sum` are transitive dependencies of the above.

## Build Output

```
$ go build ./...
(no output — clean build)

$ go vet ./...
(no output — no issues)
```

## Test Output

```
$ go test -race -v -timeout 120s ./...

?   	github.com/ankitsriv89/08-basic-key-value-store	[no test files]
?   	github.com/ankitsriv89/08-basic-key-value-store/api	[no test files]
?   	github.com/ankitsriv89/08-basic-key-value-store/metrics	[no test files]
=== RUN   TestSetGet
--- PASS: TestSetGet (0.14s)
=== RUN   TestDelete
--- PASS: TestDelete (0.14s)
=== RUN   TestMissingKey
--- PASS: TestMissingKey (0.00s)
=== RUN   TestWALRecovery
--- PASS: TestWALRecovery (0.21s)
=== RUN   TestFlushAndReadFromSST
--- PASS: TestFlushAndReadFromSST (19.03s)
=== RUN   TestCompaction
--- PASS: TestCompaction (14.83s)
=== RUN   TestWAL_AppendAndReplay
--- PASS: TestWAL_AppendAndReplay (0.24s)
PASS
ok  	github.com/ankitsriv89/08-basic-key-value-store/store	35.615s
```

No races detected.

## Notable Build Decisions

### WAL fsync per write
Every `wal.Append` calls `os.File.Sync()` before returning.  This gives strong
durability (acknowledged writes survive a crash) but limits write throughput to
the number of fsyncs the storage device supports per second — typically 2 000–5 000/s
on an SSD.  The alternative (group commit / batch fsync) is the natural next step
for a production design.

### TestFlushAndReadFromSST is slow (19s)
The test writes 500 keys, each individually fsynced through the WAL.  On the
Oracle ARM VM this takes ~19s.  The test is correct — it proves that after a
flush, keys survive and are readable from disk.  A benchmark (`BenchmarkSet`)
captures the throughput number without the test timeout concern.

### bufio.Writer wrapping os.File in WAL
Each `Append` writes header + key + value + CRC into the bufio buffer, then
flushes the buffer to the OS (one syscall), then fsyncs.  Without the bufio layer,
each chunk would be a separate `write(2)` syscall — measurably slower on small values.

### Memtable pointer swap under engine.mu
The memtable flush swaps the active memtable pointer while holding `engine.mu`
for only the duration of the swap (< 1 µs).  The actual SSTable write happens
outside the lock, so write throughput is not blocked by I/O during flush.

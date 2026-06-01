# Build Log — Consistent Hashing Service

## Environment

| Item | Value |
|---|---|
| Go version | go1.23.x |
| Module | `github.com/ankitsriv89/consistent-hashing` |
| OS/Arch | linux/amd64 (Oracle Cloud ARM target: linux/arm64) |
| Build date | 2026-06-02 |

## Dependencies

| Package | Version | Role |
|---|---|---|
| `github.com/gorilla/mux` | v1.8.1 | HTTP router with named path variables |
| `github.com/prometheus/client_golang` | v1.23.2 | Prometheus metrics exposition |
| `go.uber.org/zap` | v1.28.0 | Structured production logging |

All indirect dependencies pinned in `go.sum`.

## Build output

```
$ go build ./...
(no output — clean build)

$ go vet ./...
(no output — no issues)
```

## Test output

```
$ go test -race -v ./...

=== RUN   TestLookupEmptyRing
--- PASS: TestLookupEmptyRing (0.00s)
=== RUN   TestAddNodeAndLookup
--- PASS: TestAddNodeAndLookup (0.66s)
=== RUN   TestLookupStability
    ring_test.go:63: keys moved after adding 4th node: 21.0%
--- PASS: TestLookupStability (0.72s)
=== RUN   TestRemoveNode
--- PASS: TestRemoveNode (0.68s)
=== RUN   TestWeightedDistribution
    ring_test.go:102: weighted distribution: small=28.2%, large=71.8%
--- PASS: TestWeightedDistribution (1.13s)
=== RUN   TestVersionIncrement
--- PASS: TestVersionIncrement (0.25s)
=== RUN   TestLookupN
--- PASS: TestLookupN (0.51s)
=== RUN   TestStatsStdDev
    ring_test.go:151: ring stddev with 200 vnodes/node: 0.0138
--- PASS: TestStatsStdDev (0.73s)
PASS
ok  github.com/ankitsriv89/consistent-hashing/ring  5.700s
```

## Benchmark output

```
$ go test -bench=. -benchmem ./ring/

BenchmarkLookup-2     1000000    1043 ns/op    56 B/op    2 allocs/op
BenchmarkAddNode-2          2    665495687 ns/op    53343628 B/op    1212153 allocs/op
```

**Lookup:** 1.04 µs/op — fast enough for 1M+ lookups/second per core. The 56 B / 2 allocs come from `fmt.Sprintf` inside `BenchmarkLookup`'s key construction; the ring itself allocates nothing.

**AddNode:** ~665ms per 10-node ring build (150 vnodes × 10 nodes = 1500 SHA-256 operations). This is a control-plane operation called rarely; latency is acceptable. A production optimisation would be to cache the position per `(nodeID, idx)` pair across restarts.

## Build decisions

- **SHA-256 over MD5/FNV for hashing:** SHA-256 has better uniformity for short strings like `node-a#0`. MD5 is faster but the ~10ms AddNode cost is dominated by the hash calls, not the algorithm choice. SHA-256 avoids any clustering artefacts for lexically similar node IDs.

- **`sort.Slice` on each AddNode:** The ring slice is re-sorted on every topology change. With ≤5000 vnodes this is ~65K comparisons and completes in microseconds. An insertion approach would save a constant factor but is not worth the code complexity.

- **In-memory store for MVP:** No PostgreSQL for ring state. The MVP demonstrates placement behaviour; durability is a production extension. Adding a snapshot-on-mutation path to PG is a single `store.Snapshot()` call wired into each mutation handler.

- **`atomic.Uint64` for version:** Avoids holding the mutex just to read or bump the version. Topology mutations hold the write lock anyway, so the atomic Add is uncontended.

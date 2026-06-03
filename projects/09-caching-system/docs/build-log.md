# Build Log — 09 Caching System

## Environment

| Field | Value |
|---|---|
| Go version | go1.24.2 linux/amd64 |
| Module path | `github.com/ankitsriv89/09-caching-system` |
| Build target | `CGO_ENABLED=0 GOOS=linux GOARCH=arm64` |

## Direct dependencies

| Module | Version | Role |
|---|---|---|
| `github.com/gorilla/mux` | v1.8.1 | HTTP router |
| `github.com/prometheus/client_golang` | v1.19.0 | Prometheus metrics |
| `go.uber.org/zap` | v1.27.0 | Structured logging |
| `golang.org/x/sync` | v0.7.0 | `singleflight.Group` for stampede protection |

## Indirect dependencies

| Module | Version | Role |
|---|---|---|
| `github.com/beorn7/perks` | v1.0.1 | Prometheus histogram quantiles |
| `github.com/cespare/xxhash/v2` | v2.2.0 | Prometheus label hashing |
| `github.com/prometheus/client_model` | v0.5.0 | Prometheus protobuf model |
| `github.com/prometheus/common` | v0.48.0 | Prometheus text format |
| `github.com/prometheus/procfs` | v0.12.0 | Linux /proc scraping |
| `go.uber.org/multierr` | v1.10.0 | zap dependency |
| `golang.org/x/sys` | v0.16.0 | Prometheus procfs |
| `google.golang.org/protobuf` | v1.32.0 | Prometheus protobuf |

## Build output

```
$ go build ./...
(no output — clean build)

$ go vet ./...
(no output — no issues)

$ go test -race ./... -v
=== RUN   TestSetGet
--- PASS: TestSetGet (0.00s)
=== RUN   TestMiss
--- PASS: TestMiss (0.00s)
=== RUN   TestTTLExpiry
--- PASS: TestTTLExpiry (0.07s)
=== RUN   TestLRUEviction
--- PASS: TestLRUEviction (0.00s)
=== RUN   TestLFUEviction
--- PASS: TestLFUEviction (0.00s)
=== RUN   TestDelete
--- PASS: TestDelete (0.00s)
=== RUN   TestFlush
--- PASS: TestFlush (0.00s)
=== RUN   TestStats
--- PASS: TestStats (0.00s)
=== RUN   TestConcurrentSafety
--- PASS: TestConcurrentSafety (0.00s)
=== RUN   TestGetOrLoad_Singleflight
--- PASS: TestGetOrLoad_Singleflight (0.01s)
PASS
ok    github.com/ankitsriv89/09-caching-system/cache  1.115s
```

**10/10 tests pass. Race detector: clean.**

## Build decisions

- `singleflight` sourced from `golang.org/x/sync` rather than a local implementation — it is the standard, well-tested coalescing primitive in the Go ecosystem.
- LFU uses `container/heap` (standard library); the heap index is stored inside each `item` so `heap.Fix` is O(log n) without a secondary map.
- LRU uses `container/list` (doubly-linked list) + a map for O(1) move-to-front on access.
- AOF uses `json.Encoder` per record (NDJSON) rather than a binary format — simpler to debug and inspect with `jq`.
- `CGO_ENABLED=0` ensures a fully static binary; `GOARCH=arm64` matches the Oracle Cloud ARM instance.

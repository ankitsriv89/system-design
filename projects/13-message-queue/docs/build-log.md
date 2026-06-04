# Build Log — Message Queue (Project 13)

## Environment

| Item | Value |
|------|-------|
| Go version | go1.22 |
| Module path | `github.com/ankitsriv89/13-message-queue` |
| OS/Arch | linux/amd64 |
| CPU | Intel Celeron 2955U @ 1.40GHz (Oracle Cloud free-tier ARM) |

## Dependencies

| Module | Version | Role |
|--------|---------|------|
| `github.com/gorilla/mux` | v1.8.1 | HTTP router with path variables |
| `github.com/lib/pq` | v1.10.9 | PostgreSQL driver |
| `github.com/prometheus/client_golang` | v1.19.0 | Prometheus metrics exposition |
| `github.com/redis/go-redis/v9` | v9.5.1 | Redis client (partition counter + cache) |
| `go.uber.org/zap` | v1.27.0 | Structured JSON logging |

## Build Output

```
$ go build ./...
(no output — clean build)

$ go vet ./...
(no output — no issues)

$ go test -race ./... -v
?       github.com/ankitsriv89/13-message-queue [no test files]
?       github.com/ankitsriv89/13-message-queue/api [no test files]
?       github.com/ankitsriv89/13-message-queue/metrics [no test files]
=== RUN   TestPartitionFor_KeyBased
--- PASS: TestPartitionFor_KeyBased (0.00s)
=== RUN   TestPartitionFor_RoundRobin
--- PASS: TestPartitionFor_RoundRobin (0.00s)
=== RUN   TestPartitionFor_SinglePartition
--- PASS: TestPartitionFor_SinglePartition (0.00s)
PASS
ok      github.com/ankitsriv89/13-message-queue/queue   1.037s
?       github.com/ankitsriv89/13-message-queue/store [no test files]
?       github.com/ankitsriv89/13-message-queue/worker [no test files]
```

## Benchmark Output

```
$ go test -race -bench=. ./queue/
goos: linux
goarch: amd64
cpu: Intel(R) Celeron(R) 2955U @ 1.40GHz
BenchmarkPartitionFor-2   37328978   31.15 ns/op
PASS
ok  github.com/ankitsriv89/13-message-queue/queue   3.159s
```

`PartitionFor` runs in 31 ns — well under the publish-path budget. The FNV-1a hash is computed inline without allocation.

## Build Decisions

- **`BIGSERIAL` for offset**: PostgreSQL sequences are non-blocking and monotonic per-session. They are not gap-free, but that is acceptable — message ordering is enforced by reading in `ORDER BY offset` rather than by offset contiguity.
- **`FOR UPDATE SKIP LOCKED` in poll**: standard PostgreSQL pattern for work-queue fan-out. Avoids explicit locking tables and scales horizontally to multiple API replicas.
- **No `init()` functions**: all I/O happens in explicit `NewDB` / `NewCache` constructors called from `main.go`, following the project rule against `init()` I/O.
- **`sync.Pool` not used here**: message structs are allocated per-request. At the expected throughput (<1 000 msg/s on free-tier), GC pressure is negligible. A pool would be appropriate above ~10 000 msg/s.
- **Redis fallback on failure**: `cache.GetTopicPartitions` returns `0, nil` on a Redis miss. The handler falls back to a PostgreSQL read. The publish path continues even if Redis is down.

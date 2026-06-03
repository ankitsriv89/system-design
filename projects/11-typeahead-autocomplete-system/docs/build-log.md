# Build Log — Typeahead Autocomplete System

## Environment

| Item | Value |
|------|-------|
| Go version | go1.22 |
| Module path | `github.com/ankitsriv89/11-typeahead-autocomplete-system` |
| Platform | linux/amd64 |
| Build date | 2026-06-04 |

## Direct Dependencies

| Module | Version | Role |
|--------|---------|------|
| `github.com/gorilla/mux` | v1.8.1 | HTTP router |
| `github.com/lib/pq` | v1.10.9 | PostgreSQL driver |
| `github.com/prometheus/client_golang` | v1.19.0 | Prometheus metrics |
| `github.com/redis/go-redis/v9` | v9.5.1 | Redis client |
| `go.uber.org/zap` | v1.27.0 | Structured logging |

## Build Output

```
$ go build ./...
(no output — clean build)

$ go vet ./...
(no output — no issues)

$ go test -race ./... -v
?   github.com/ankitsriv89/11-typeahead-autocomplete-system [no test files]
?   github.com/ankitsriv89/11-typeahead-autocomplete-system/api [no test files]
=== RUN   TestNormalizePrefix
--- PASS: TestNormalizePrefix (0.00s)
=== RUN   TestGeneratePrefixes
--- PASS: TestGeneratePrefixes (0.00s)
=== RUN   TestGeneratePrefixesMaxLen
--- PASS: TestGeneratePrefixesMaxLen (0.00s)
=== RUN   TestScoreItem
--- PASS: TestScoreItem (0.00s)
PASS
ok  github.com/ankitsriv89/11-typeahead-autocomplete-system/autocomplete 1.035s
?   github.com/ankitsriv89/11-typeahead-autocomplete-system/metrics [no test files]
?   github.com/ankitsriv89/11-typeahead-autocomplete-system/store [no test files]
?   github.com/ankitsriv89/11-typeahead-autocomplete-system/worker [no test files]
```

## Benchmarks

```
$ go test -race ./... -bench=. -benchtime=1s
goos: linux
goarch: amd64
cpu: Intel(R) Celeron(R) 2955U @ 1.40GHz
BenchmarkNormalizePrefix-2     235248     4738 ns/op
BenchmarkGeneratePrefixes-2    185187     6717 ns/op
```

Both hot-path helpers are under 7 µs/op on a constrained ARM-equivalent host (Celeron 2955U); well within the 2 ms p50 budget.

## Build Decisions

- **Redis sorted sets over a server-side trie**: A trie requires server memory proportional to the corpus and complex concurrent access control. Sorted sets distribute naturally, support `ZREVRANGE` O(log N + K), and Redis handles eviction. The trade-off is that prefix keys must be managed (TTL, rebuild) but the operational simplicity pays off.
- **maxPrefixLen = 20**: Caps Redis key space. Prefixes beyond 20 chars have negligible traffic and would double memory. Users typing >20 chars fall back to the PostgreSQL `LIKE` query.
- **topK = 20**: Each sorted set holds at most 20 members, enforced by `ZREMRANGEBYRANK 0 -21` on every write. Prevents any single hot prefix from consuming unbounded memory.
- **bsm/redislock removed**: The rebuild job runs on a single worker goroutine; distributed locking would be needed only when horizontal scaling is added.

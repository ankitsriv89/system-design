# Build Log — Web Crawler (Project 12)

## Environment

| Item | Value |
|---|---|
| Go version | go1.22 |
| Module path | `github.com/ankitsriv89/12-web-crawler` |
| OS / arch | linux/amd64 (Oracle Cloud ARM free-tier host) |

## Direct Dependencies

| Module | Version | Role |
|---|---|---|
| `github.com/gorilla/mux` | v1.8.1 | HTTP router |
| `github.com/lib/pq` | v1.10.9 | PostgreSQL driver |
| `github.com/prometheus/client_golang` | v1.19.0 | Prometheus metrics |
| `github.com/redis/go-redis/v9` | v9.5.1 | Redis client |
| `go.uber.org/zap` | v1.27.0 | Structured logging |
| `golang.org/x/net` | v0.24.0 | HTML parser (`html.Parse`) |

## Build Output

```
$ go build ./...
(no output — clean build)

$ go vet ./...
(no output — clean)
```

## Test Output

```
$ go test -race ./... -v

=== RUN   TestNormalizeURL
--- PASS: TestNormalizeURL (0.00s)
=== RUN   TestURLHash
--- PASS: TestURLHash (0.00s)
=== RUN   TestContentHash
--- PASS: TestContentHash (0.00s)
=== RUN   TestExtractLinks
--- PASS: TestExtractLinks (0.00s)
=== RUN   TestParseRobotsTxt
--- PASS: TestParseRobotsTxt (0.00s)
=== RUN   TestIsAllowed
--- PASS: TestIsAllowed (0.00s)
PASS
ok   github.com/ankitsriv89/12-web-crawler/crawler   1.028s
```

## Benchmark Output

```
$ go test -race -bench=. ./crawler/

BenchmarkURLHash-2        170463     6795 ns/op
BenchmarkContentHash-2     10000   101411 ns/op
```

SHA-256 of a 4KB body ≈101µs; hashing is not on the hot request path (once per fetch).

## Build Decisions

- **No Kafka/NATS**: The plan called for optional Kafka. For MVP the frontier is a PostgreSQL table with `SKIP LOCKED` — this gives equivalent fan-out semantics with zero extra infrastructure. Message bus can be added later by publishing to a topic after each enqueue.
- **No raw HTML storage**: Page bodies are not stored in PostgreSQL — only the content hash and metadata. Storing snapshots would require object storage (MinIO); that's noted as a production extension.
- **`golang.org/x/net`** used for HTML parsing via `html.Parse` — the standard library has no HTML parser.
- **`CHAR(64)` for hashes** in PostgreSQL: SHA-256 produces exactly 64 hex characters; fixed-width avoids TOAST overhead on the primary key column.

# Build Log — 07 API Gateway

## Environment

| Item | Value |
|---|---|
| Go version | go1.25.0 linux/amd64 |
| Module path | `github.com/ankitsriv89/07-api-gateway` |
| Platform | linux/amd64 (Oracle Cloud ARM → cross-compiles with `GOOS=linux`) |

## Direct Dependencies

| Package | Version | Role |
|---|---|---|
| `github.com/gorilla/mux` | v1.8.1 | HTTP router with path variables for admin endpoints |
| `github.com/lib/pq` | v1.12.3 | PostgreSQL driver (pure Go) |
| `github.com/prometheus/client_golang` | v1.23.2 | Prometheus metrics counters, histograms, gauges |
| `github.com/redis/go-redis/v9` | v9.10.0 | Redis client with pipeline support for rate-limit sorted sets |
| `go.uber.org/zap` | v1.28.0 | Structured JSON logging |

## Build Output

```
$ go build ./...
(no output — clean build)

$ go vet ./...
(no output — no issues)

$ go test -race ./... -v
?     github.com/ankitsriv89/07-api-gateway      [no test files]
?     github.com/ankitsriv89/07-api-gateway/api   [no test files]
=== RUN   TestRouterLongestPrefixWins
--- PASS: TestRouterLongestPrefixWins (0.00s)
=== RUN   TestRouterNoMatchReturnsNotFound
--- PASS: TestRouterNoMatchReturnsNotFound (0.00s)
=== RUN   TestEvaluateAllowedNoAuth
--- PASS: TestEvaluateAllowedNoAuth (0.00s)
=== RUN   TestEvaluateAuthRequired_MissingToken
--- PASS: TestEvaluateAuthRequired_MissingToken (0.00s)
=== RUN   TestEvaluateAuthRequired_ValidKey
--- PASS: TestEvaluateAuthRequired_ValidKey (0.00s)
=== RUN   TestEvaluateRateLimited
--- PASS: TestEvaluateRateLimited (0.00s)
=== RUN   TestEvaluateForbiddenScope
--- PASS: TestEvaluateForbiddenScope (0.00s)
=== RUN   TestStripPrefix
--- PASS: TestStripPrefix (0.00s)
PASS
ok    github.com/ankitsriv89/07-api-gateway/gateway  1.049s
?     github.com/ankitsriv89/07-api-gateway/metrics  [no test files]
?     github.com/ankitsriv89/07-api-gateway/store    [no test files]
```

## Build Decisions

- **`httputil.ReverseProxy` over custom TCP proxy**: the standard library reverse proxy handles hop-by-hop header stripping, chunked transfer encoding, and WebSocket upgrades correctly. A custom implementation would replicate this work without meaningful benefit at this scale.
- **`sync.Pool` for proxy copy buffers**: `httputil.ReverseProxy` accepts a `BufferPool` interface. Injecting one backed by `sync.Pool` eliminates per-request 32 KiB allocations on the hot path.
- **SHA-256 for API key hashing**: a fast non-keyed hash is intentional here. The raw key is a random secret; SHA-256 provides preimage resistance. For credential storage (passwords), bcrypt/scrypt would be correct; for random tokens, SHA-256 is sufficient and orders of magnitude faster.
- **Fail-open on Redis error**: a Redis outage should not take down the entire gateway. The rate limiter returns `allowed=true` on error, trading quota enforcement for availability. The error is logged at `Warn` level for alerting.
- **Two HTTP servers (proxy + admin)**: separating ports allows network-level access control (firewall rules / Caddy upstreams) to ensure the admin API is never accidentally exposed to the public internet.
- **`promauto.With(reg)`**: using a custom `prometheus.Registry` (not the default global) means the binary can safely run multiple test instances without colliding on global metric names.

# Build Log — 06 Load Balancer

## Environment

| Key | Value |
|---|---|
| Go version | go1.25.0 linux/amd64 |
| Module path | github.com/ankitsriv89/06-load-balancer |
| Platform | linux/arm64 (Oracle Cloud free-tier) |

## Direct Dependencies

| Module | Version | Role |
|---|---|---|
| github.com/gorilla/mux | v1.8.1 | HTTP router with path variables |
| github.com/lib/pq | v1.12.3 | PostgreSQL driver (libpq-compatible) |
| github.com/prometheus/client_golang | v1.23.2 | Prometheus metrics instrumentation |
| go.uber.org/zap | v1.28.0 | Structured, levelled production logger |

## Build Output

```
$ go build ./...
(no output — clean build)

$ go vet ./...
(no output — no issues)

$ go test -race -v ./...
?   	github.com/ankitsriv89/06-load-balancer	[no test files]
?   	github.com/ankitsriv89/06-load-balancer/api	[no test files]
=== RUN   TestRoundRobin
--- PASS: TestRoundRobin (0.00s)
=== RUN   TestLeastConnections
--- PASS: TestLeastConnections (0.00s)
=== RUN   TestWeightedRoundRobin
--- PASS: TestWeightedRoundRobin (0.00s)
=== RUN   TestUnhealthyBackendsSkipped
--- PASS: TestUnhealthyBackendsSkipped (0.00s)
=== RUN   TestAllUnhealthyReturnsNil
--- PASS: TestAllUnhealthyReturnsNil (0.00s)
=== RUN   TestConcurrentNext
--- PASS: TestConcurrentNext (0.00s)
PASS
ok  	github.com/ankitsriv89/06-load-balancer/balancer	1.034s
?   	github.com/ankitsriv89/06-load-balancer/metrics	[no test files]
?   	github.com/ankitsriv89/06-load-balancer/store	[no test files]
```

## Design Decisions

- **Go 1.25** — jumped from 1.24.2 when `prometheus/common v0.68` required it; no code changes needed.
- **gorilla/mux** — chosen over `net/http.ServeMux` for named path variable support (`{service}`, `{backend}`).
- **httputil.ReverseProxy** — standard library reverse proxy avoids an external dependency; path stripping handled by cloning the request and rewriting `r.URL.Path`.
- **Atomic counters for ActiveConns/TotalConns** — hot path; avoids a mutex acquire on every request.
- **EWMA for latency** — α=0.2 weights recent samples more than historical ones; smooths out transient spikes without a full sliding window.
- **Buffered health event channel (512)** — health events are best-effort; a full channel drops the event rather than blocking the health-check goroutine.

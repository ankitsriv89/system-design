# Build Log — Notification System (Project 10)

## Environment

| Item | Value |
|---|---|
| Go version | go1.24.2 linux/amd64 |
| Module | `github.com/ankitsriv89/10-notification-system` |
| Target platform | linux/arm64 (Oracle Cloud free-tier ARM) |
| Build flags | `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 -ldflags="-s -w"` |

## Direct Dependencies

| Package | Version | Role |
|---|---|---|
| `github.com/gorilla/mux` | v1.8.1 | HTTP router |
| `github.com/lib/pq` | v1.10.9 | PostgreSQL driver |
| `github.com/prometheus/client_golang` | v1.19.0 | Prometheus metrics |
| `go.uber.org/zap` | v1.27.0 | Structured logging |

## Build Output

```
$ go build ./...
(no output — clean build)

$ go vet ./...
(no output — no vet issues)

$ go test -race ./... -v
?   github.com/ankitsriv89/10-notification-system            [no test files]
?   github.com/ankitsriv89/10-notification-system/api        [no test files]
?   github.com/ankitsriv89/10-notification-system/metrics    [no test files]
=== RUN   TestRenderTemplate
--- PASS: TestRenderTemplate (0.00s)
=== RUN   TestRenderTemplate_MissingParam
--- PASS: TestRenderTemplate_MissingParam (0.00s)
=== RUN   TestIsQuietHour_NormalWindow
--- PASS: TestIsQuietHour_NormalWindow (0.00s)
=== RUN   TestIsQuietHour_Disabled
--- PASS: TestIsQuietHour_Disabled (0.00s)
=== RUN   TestIsQuietHour_SameDay
--- PASS: TestIsQuietHour_SameDay (0.00s)
PASS
ok  github.com/ankitsriv89/10-notification-system/notification   1.031s
=== RUN   TestMinHelper
--- PASS: TestMinHelper (0.00s)
=== RUN   TestQueueBackpressure
--- PASS: TestQueueBackpressure (0.00s)
PASS
ok  github.com/ankitsriv89/10-notification-system/worker         1.032s
```

## Build Decisions

**In-process channels instead of Kafka**: The plan.md recommended Java/Spring Boot + Kafka. This project uses Go + buffered channels for consistency with the entire series (01–09 all use Go), lower resource consumption on the shared Oracle Cloud instance, and to avoid a heavy broker dependency in docker-compose. The dispatch semantics are identical: buffered queue, worker pool, retry with backoff, dead-letter queue. Kafka would be the production evolution.

**`lib/pq` instead of `pgx`**: Simpler API, sufficient for the query patterns here (no pgx-specific features needed), and consistent with projects that already use it elsewhere in the series.

**`gen_random_uuid()` in PostgreSQL**: UUIDs are generated DB-side using `pgcrypto`, so the Go layer receives them after INSERT. This guarantees uniqueness even if the application is multi-instance.

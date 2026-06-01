# Build Log — Unique ID Generator

## Environment

| Key | Value |
|---|---|
| Go version | go1.24.2 linux/amd64 |
| Module | `github.com/ankitsriv89/uniqueid` |
| Build date | 2026-06-01 |
| Platform | linux/amd64 |

## Dependencies

| Package | Version | Role |
|---|---|---|
| `github.com/gorilla/mux` | v1.8.1 | HTTP router |
| `github.com/lib/pq` | v1.10.9 | PostgreSQL driver |
| `github.com/prometheus/client_golang` | v1.20.5 | Prometheus metrics |
| `go.uber.org/zap` | v1.27.0 | Structured logging |

## `go build`

```
$ go build ./...
(no output — clean build)
```

Exit code: 0

## `go vet`

```
$ go vet ./...
(no output — no issues)
```

Exit code: 0

## `go test`

```
$ go test ./... -v

?       github.com/ankitsriv89/uniqueid         [no test files]
?       github.com/ankitsriv89/uniqueid/api     [no test files]
=== RUN   TestUniqueSequential
--- PASS: TestUniqueSequential (0.00s)
=== RUN   TestUniqueConcurrent
--- PASS: TestUniqueConcurrent (0.01s)
=== RUN   TestDecompose
--- PASS: TestDecompose (0.00s)
=== RUN   TestClockRollback
--- PASS: TestClockRollback (0.01s)
=== RUN   TestBatch
--- PASS: TestBatch (0.00s)
=== RUN   TestWorkerIDValidation
--- PASS: TestWorkerIDValidation (0.00s)
PASS
ok      github.com/ankitsriv89/uniqueid/generator    0.020s
?       github.com/ankitsriv89/uniqueid/lease        [no test files]
?       github.com/ankitsriv89/uniqueid/metrics      [no test files]
```

**6/6 tests passed. 0 failures.**

## Build decisions

- `CGO_ENABLED=0` — statically linked binary; no libc dependency in the Docker image.
- `-trimpath` — strips local file paths from the binary for reproducibility and security.
- `-ldflags="-s -w"` — strips debug symbols and DWARF info to reduce binary size.
- `go 1.23.0` in `go.mod` — minimum version required; actual toolchain is 1.24.2.
- No `go.sum` checked in from scratch — `go mod tidy` generated it from the module graph.

---

## v0.2.0 build — 2026-06-01 (memory/performance optimisations)

```
$ go build ./...
(no output — clean build)

$ go vet ./...
(no output — no issues)

$ go test ./... -v -count=1
?       github.com/ankitsriv89/uniqueid         [no test files]
?       github.com/ankitsriv89/uniqueid/api     [no test files]
=== RUN   TestUniqueSequential
--- PASS: TestUniqueSequential (0.00s)
=== RUN   TestUniqueConcurrent
--- PASS: TestUniqueConcurrent (0.01s)
=== RUN   TestDecompose
--- PASS: TestDecompose (0.00s)
=== RUN   TestClockRollback
--- PASS: TestClockRollback (0.01s)
=== RUN   TestBatch
--- PASS: TestBatch (0.00s)
=== RUN   TestWorkerIDValidation
--- PASS: TestWorkerIDValidation (0.00s)
PASS
ok      github.com/ankitsriv89/uniqueid/generator    0.021s
?       github.com/ankitsriv89/uniqueid/lease        [no test files]
?       github.com/ankitsriv89/uniqueid/metrics      [no test files]
```

**6/6 tests passed. 0 failures.**

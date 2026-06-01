# System Design Projects — Claude Instructions

This file is read by Claude Code at the start of every session. All instructions here are mandatory for every project under `projects/`.

---

## Performance and memory — shared-host rules

All 50 projects run on the same Oracle Cloud free-tier ARM instance. Every allocation, goroutine, and file descriptor competes with every other service. These rules are non-negotiable.

### Allocations
- Use `sync.Pool` for any object allocated per-request (JSON buffers, byte slices, scratch structs).
- Pre-build static responses (healthz, constant error bodies) as `[]byte` package-level vars — never inside a handler.
- Pre-render immutable derived values (e.g. `workerIDStr`) at construction time, not on every call.
- Size slice/map literals with a capacity hint when the size is known: `make([]T, 0, n)`, `make(map[K]V, n)`.
- Never append to a nil slice inside a hot loop — pre-allocate.

### Goroutines
- Every goroutine must have a clear owner and a documented exit condition.
- Use a passed `context.Context` for cancellation — never a raw `done chan struct{}` unless the context model genuinely does not fit.
- Background goroutines (sweepers, renewers, watchers) must stop when their context is cancelled and must not leak on shutdown.
- Do not spawn goroutines inside request handlers unless the work is genuinely fire-and-forget and bounded.

### Connections and pools
- DB pools: set `MaxOpenConns` to the minimum needed, set `MaxIdleConns == MaxOpenConns` so connections are not destroyed between infrequent ticks, and set `SetConnMaxIdleTime` to reclaim idle connections after quiet periods.
- HTTP clients: always reuse a package-level or injected `*http.Client` — never create one per request.
- Redis/cache clients: one client instance per service, shared via constructor injection.

### Struct layout
- Pad hot structs to a cache-line boundary (64 bytes) when multiple instances may be allocated adjacently and the struct is mutated on every call.
- Group immutable fields (set once at construction) before mutable fields. Add a `_pad [N]byte` between them when the mutable fields would otherwise share a cache line with the immutable ones.

### `defer` in hot paths
- Do not use `defer` inside loops or functions called millions of times per second — the overhead is measurable. Use explicit cleanup instead.
- `defer mu.Unlock()` is fine in functions called occasionally; avoid it in the inner generation loop of a Snowflake generator or similar.

### What never to do
- No `fmt.Println` or `log.Printf` in production code — use `zap.Logger` everywhere.
- No unbounded maps that grow forever (e.g. per-request caches without eviction).
- No `time.Sleep` in request handlers.
- No blocking calls without a context deadline.
- No `init()` functions that do I/O or allocate large objects.

---

## Go code conventions

### Error handling
- Wrap errors with context using `fmt.Errorf("package: operation: %w", err)`.
- Never silently discard errors with `_` unless the error is genuinely irrelevant and a comment explains why.
- At service boundaries (HTTP handlers, gRPC handlers) translate domain errors to HTTP/gRPC status codes explicitly — no raw `err.Error()` strings to clients.

### Logging
- Use `go.uber.org/zap` everywhere. No `fmt.Println`, no `log.Printf`.
- Every log entry must include enough context to identify the request/resource: `request_id`, `worker_id`, `resource_id`, etc.
- Use `log.Debug` for per-operation noise, `log.Info` for lifecycle events, `log.Warn` for recoverable anomalies, `log.Error` for unexpected failures.

### Interfaces
- Define interfaces at the point of use (consumer package), not in the producer package.
- Keep interfaces narrow — only the methods the consumer actually calls.
- Use interfaces to decouple packages and to allow test stubs.

### Package structure
Every project follows this layout:

```
<project-slug>/
├── main.go           — wiring only: construct deps, start server, handle signals
├── <domain>/         — core domain logic, no HTTP/DB imports
│   ├── <domain>.go
│   └── <domain>_test.go
├── api/              — HTTP transport: handlers, request/response types, middleware
│   └── handler.go
├── store/            — storage adapters: DB, cache, object store
├── metrics/          — Prometheus metric registrations
├── scripts/          — migrate.sql, seed.sh, integration_test.sh, load_test.sh
├── docs/             — architecture.md, code-flow.md, build-log.md, changelog.md
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

### Testing
- Every domain package must have a `_test.go` file.
- Unit tests must not require network, DB, or filesystem.
- Run tests with the race detector: `go test -race ./...` — fix all races before committing.
- Add benchmark functions (`BenchmarkX`) for any hot path (ID generation, hashing, routing).
- Integration tests live in `scripts/integration_test.sh` and require a live Docker Compose stack.

---

## Workflow: after every successful build + test

After `go build ./...`, `go vet ./...`, and `go test -race ./...` all pass, produce the following artefacts **before** committing:

### 1. `docs/architecture.md`
- Mermaid `graph TD` diagram of the full system (client → API → domain → storage → async workers → observability).
- Mermaid `sequenceDiagram` for the primary happy-path request flow.
- Written prose: components, responsibilities, data flows, external dependencies.
- Capacity table: throughput, storage growth, key limits.

### 2. `docs/code-flow.md`
- Mermaid `flowchart TD` tracing every significant function call from `main()` through to the storage layer and back.
- One section per major operation (e.g. "Generate ID", "Acquire Lease", "Renew Lease").
- Explain *why* each call is made, not just *what* it does.
- Include a "Call graph summary" section as a `graph LR`.

### 3. `docs/build-log.md`
- Go version, module path, all direct dependencies pinned with versions and roles.
- Actual output of `go build ./...`, `go vet ./...`, `go test -race ./... -v`.
- Any build decisions, workarounds, or notable compiler flags explained.

### 4. `docs/changelog.md`
- Follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) format.
- Version starts at `0.1.0` for a new project.
- Sections: Added, Changed, Fixed, Performance.

### 5. `docs/api.md` (if the README API section is short)
- Full `curl` examples for every endpoint.
- Request and response field descriptions.
- All error responses with HTTP status codes and bodies.

### Commit and push
After all docs are written:
1. `git add` only project files — never `.env`, `*.secret`, or binaries.
2. Commit message: `feat(<project-slug>): initial implementation + docs`
3. `git push origin main`

For a performance/optimisation-only change, use: `perf(<project-slug>): <short description>`

---

## Project conventions

### Ports (do not reuse)
| Project | Port |
|---|---|
| 01-rate-limiter | 8081 |
| 02-url-shortener | 8082 (tentative) |
| 03-pastebin | 8082 |
| 04-unique-id-generator | 8083 |
| 05-consistent-hashing | 8084 |
| 06 onwards | 8085, 8086 … |

### Naming
- Module path: `github.com/ankitsriv89/<project-slug>`
- Docker image tag: `<project-slug>:latest`
- Prometheus metric prefix: `<project_slug_underscored>_`

### Shared infra
- PostgreSQL, Redis, Prometheus, Grafana, MinIO live in `infra/docker-compose.yml`.
- Every project connects via the external Docker network named `infra`.
- Never bundle a redundant Postgres or Redis in a project's own `docker-compose.yml` unless the project specifically requires isolation.

### Source file comments
- Every `.go` file must have a `// Package <name> ...` comment.
- No comments explaining *what* the code does — only *why* when non-obvious (hidden constraint, subtle invariant, workaround).
- Do not leave TODO/FIXME comments in committed code — either fix it or open a tracked issue.

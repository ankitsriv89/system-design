---
description: Go code conventions — applies only to Go projects
paths: ["projects/**/*.go", "projects/**/go.mod"]
---

# Go code conventions

These rules apply to projects whose plan.md recommends a Go stack (e.g. 01-13, 25, 32, 40, 45).

## Error handling
- Wrap errors with context using `fmt.Errorf("package: operation: %w", err)`.
- Never silently discard errors with `_` unless the error is genuinely irrelevant and a comment explains why.
- At service boundaries (HTTP handlers, gRPC handlers) translate domain errors to HTTP/gRPC status codes explicitly — no raw `err.Error()` strings to clients.

## Logging
- Use `go.uber.org/zap` everywhere. No `fmt.Println`, no `log.Printf`.
- Every log entry must include enough context to identify the request/resource: `request_id`, `worker_id`, `resource_id`, etc.
- Use `log.Debug` for per-operation noise, `log.Info` for lifecycle events, `log.Warn` for recoverable anomalies, `log.Error` for unexpected failures.

## Interfaces
- Define interfaces at the point of use (consumer package), not in the producer package.
- Keep interfaces narrow — only the methods the consumer actually calls.
- Use interfaces to decouple packages and to allow test stubs.

## Package structure (Go projects)
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
├── web/              — frontend assets
├── scripts/          — migrate.sql, integration_test.sh, load_test.sh
├── docs/
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

## Testing
- Every domain package must have a `_test.go` file.
- Unit tests must not require network, DB, or filesystem.
- Run tests with the race detector: `go test -race ./...` — fix all races before committing.
- Add benchmark functions (`BenchmarkX`) for any hot path (ID generation, hashing, routing).
- Integration tests live in `scripts/integration_test.sh` and require a live Docker Compose stack.

## Source file comments
- Every `.go` file must have a `// Package <name> ...` comment.
- No comments explaining *what* the code does — only *why* when non-obvious.
- Do not leave TODO/FIXME comments in committed code.

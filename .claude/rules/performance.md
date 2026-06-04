---
description: Go performance and memory rules — applies only to Go projects
paths: ["projects/**/*.go", "projects/**/go.mod"]
---

# Performance and memory (Go)

These rules apply to Go projects regardless of deployment target.

## Allocations
- Use `sync.Pool` for any object allocated per-request (JSON buffers, byte slices, scratch structs).
- Pre-build static responses (healthz, constant error bodies) as `[]byte` package-level vars — never inside a handler.
- Pre-render immutable derived values (e.g. `workerIDStr`) at construction time, not on every call.
- Size slice/map literals with a capacity hint when the size is known: `make([]T, 0, n)`, `make(map[K]V, n)`.
- Never append to a nil slice inside a hot loop — pre-allocate.

## Goroutines
- Every goroutine must have a clear owner and a documented exit condition.
- Use a passed `context.Context` for cancellation — never a raw `done chan struct{}` unless the context model genuinely does not fit.
- Background goroutines (sweepers, renewers, watchers) must stop when their context is cancelled and must not leak on shutdown.
- Do not spawn goroutines inside request handlers unless the work is genuinely fire-and-forget and bounded.

## Connections and pools
- DB pools: set `MaxOpenConns` to the minimum needed, `MaxIdleConns == MaxOpenConns`, and `SetConnMaxIdleTime` to reclaim idle connections.
- HTTP clients: always reuse a package-level or injected `*http.Client` — never create one per request.
- Redis/cache clients: one client instance per service, shared via constructor injection.

## Struct layout
- Pad hot structs to a cache-line boundary (64 bytes) when multiple instances are allocated adjacently and mutated on every call.
- Group immutable fields before mutable fields; add `_pad [N]byte` when mutable fields would otherwise share a cache line with immutable ones.

## `defer` in hot paths
- Do not use `defer` inside loops or functions called millions of times per second — use explicit cleanup instead.
- `defer mu.Unlock()` is fine in functions called occasionally; avoid it in tight inner loops (e.g. ID generation).

## What never to do
- No `fmt.Println` or `log.Printf` in production code — use `zap.Logger` everywhere.
- No unbounded maps that grow forever.
- No `time.Sleep` in request handlers.
- No blocking calls without a context deadline.
- No `init()` functions that do I/O or allocate large objects.

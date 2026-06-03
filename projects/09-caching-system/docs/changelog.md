# Changelog — 09 Caching System

All notable changes follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] — 2026-06-03

### Added
- In-memory cache engine with LRU (doubly-linked list) and LFU (min-heap) eviction policies, switchable via `EVICTION_POLICY` env var.
- Per-entry TTL with lazy expiry on GET and active background sweeper (30 s interval).
- `singleflight`-based `GetOrLoad` for cache stampede protection — all concurrent misses on the same key share a single loader invocation.
- AOF (append-only file) persistence: every SET, DELETE, and FLUSH is logged as NDJSON; warm-restart replay skips expired entries.
- HTTP API: `PUT /v1/cache/{key}`, `GET /v1/cache/{key}`, `DELETE /v1/cache/{key}`, `GET /v1/cache` (list), `DELETE /v1/cache` (flush), `GET /v1/stats`, `GET /v1/entries`.
- Prometheus metrics: hit/miss counters, eviction counter (labelled by reason), SET/DELETE counters, memory bytes gauge, key count gauge, GET/SET latency histograms, AOF error counter.
- Vanilla-JS web UI with live arc gauge (hit rate), eviction-order track, memory bar, animated request-flow canvas, and stampede demo.
- Tutorial UI with collapsible concept panels covering cache-aside, LRU vs LFU, TTL expiry, stampede protection, and AOF warm restart.
- Docker Compose deployment on port 8090; Caddy reverse-proxy at `/p09/`.
- Integration test script (`scripts/integration_test.sh`), load test script (`scripts/load_test.sh`), seed script (`scripts/seed.sh`).
- `sync.Pool` is not needed here — per-request allocations are dominated by JSON encoding, which is already pooled internally by `encoding/json`.

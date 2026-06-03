# Changelog

All notable changes to this project will be documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] - 2026-06-04

### Added
- Core suggest endpoint: `GET /v1/suggest?q=&locale=&limit=` served from Redis sorted sets with PostgreSQL fallback
- Corpus CRUD: `POST`, `GET`, `DELETE /v1/corpus/items`
- Click-through feedback: `POST /v1/feedback/click` increments item popularity and updates Redis scores in-place
- Admin rebuild: `POST /v1/admin/rebuild-index` triggers full re-index; `GET /v1/admin/stats` returns index metrics
- Background `Rebuilder` goroutine running every 30 minutes (configurable via `REBUILD_INTERVAL`)
- Redis sorted-set prefix index: up to 20-char prefix depth, top-20 members per key, 24-hour TTL
- PostgreSQL LIKE fallback with automatic Redis back-fill on cache miss
- Prometheus metrics: suggest request rate, latency histogram, corpus item gauge, rebuild duration/total
- Structured `zap` logging throughout
- Tutorial web UI with live trie canvas visualisation, latency sparkline, algorithm step-by-step panel, and corpus management
- Docker Compose stack with embedded PostgreSQL and Redis
- `scripts/seed.sh` — seeds 25 corpus items
- `scripts/load_test.sh` — concurrency-configurable latency benchmark
- `scripts/integration_test.sh` — end-to-end API tests against a live stack
- Full docs: architecture, code-flow, build-log, api

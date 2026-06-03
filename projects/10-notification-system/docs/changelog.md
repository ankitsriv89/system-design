# Changelog — Notification System (Project 10)

All notable changes follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] — 2026-06-03

### Added
- Notification acceptance API: `POST /v1/notifications` with template rendering, preference checks, and idempotency keys
- `GET /v1/notifications` (list with pagination) and `GET /v1/notifications/{id}`
- Delivery attempt log: `GET /v1/notifications/{id}/attempts`
- Template CRUD: `POST /v1/templates`, `GET /v1/templates`
- User preference management: `PUT /v1/preferences/{user_id}`, `GET /v1/preferences/{user_id}`; per-channel opt-in/out and quiet hours (wrap-around midnight supported)
- In-process dispatch queue (buffered channel, 1024 cap) with 4-worker pool
- Provider mocks for email, SMS, and push with configurable failure rates
- Exponential-backoff retry (up to 3 attempts, base 200 ms doubling per attempt)
- Dead-letter queue channel with persistent DB status update
- Admin API: `GET /v1/admin/queue/stats`, `PUT /v1/admin/provider/{name}/failure-rate`
- Prometheus metrics: enqueued, delivered, failed, retried, DLQ counters; queue depth gauge; delivery latency histogram; HTTP request/duration counters
- Three-panel tutorial UI with canvas pipeline animation, live stats, failure injection sliders, and event log
- PostgreSQL schema with UUID primary keys, idempotency constraint, and delivery-attempt foreign key
- Docker + Docker Compose for shared-infra deployment (port 8091)
- Integration test script, seed script, and load test script

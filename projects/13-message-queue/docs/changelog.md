# Changelog

All notable changes to this project will be documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] — 2026-06-04

### Added
- Core domain: `Topic`, `Message`, `ConsumerOffset` structs with FNV-1a key-based and round-robin partition selection (`PartitionFor`).
- PostgreSQL store: append-only `messages` table with BIGSERIAL offset, `FOR UPDATE SKIP LOCKED` poll, atomic ack with double-ack guard.
- Redis cache: per-topic partition counter (INCR) and partition-count cache to avoid DB reads on the publish hot path.
- HTTP API:
  - `POST /v1/topics` — create topic
  - `GET  /v1/topics` — list topics
  - `GET  /v1/topics/{topic}` — get topic
  - `POST /v1/topics/{topic}/messages` — publish (key-based or round-robin routing)
  - `POST /v1/topics/{topic}/messages:poll` — consumer poll with visibility timeout
  - `POST /v1/messages/{id}:ack` — acknowledge message
  - `GET  /v1/topics/{topic}/depth` — per-partition queue depth + DLQ count
  - `GET  /v1/topics/{topic}/dlq` — list dead-lettered messages
  - `GET  /v1/stats` — aggregate stats
- Background reaper: promotes poison messages (≥5 attempts) to DLQ, restores expired leases every 5 s.
- Prometheus metrics: publish/poll/ack counters, latency histograms, queue-depth and DLQ-depth gauges.
- Three-panel tutorial web UI: topology canvas, animated message-flow particles, algorithm walk-through, partition depth bars; all driven by live API calls with safe DOM construction (no `innerHTML` with API data).
- Dockerfile (multi-stage, Alpine final image).
- Docker Compose wiring to shared infra network.
- `scripts/migrate.sql` — schema with polling and reaper indexes.
- `scripts/seed.sh` — creates topics and publishes 20 sample messages.
- `scripts/load_test.sh` — concurrent producer/consumer throughput benchmark.
- `scripts/integration_test.sh` — end-to-end test suite covering publish, poll, ack, double-ack, visibility timeout re-delivery.
- Docs: `architecture.md`, `code-flow.md`, `build-log.md`, `api.md`.

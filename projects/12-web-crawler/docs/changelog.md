# Changelog — Web Crawler (Project 12)

All notable changes to this project will be documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)

## [0.1.0] — 2026-06-04

### Added
- `crawler` package: URL normalisation, SHA-256 URL/content hashing, HTML link extraction, robots.txt parser, `HTTPFetcher` with 15s timeout and 2MB body cap
- `store` package: PostgreSQL adapter (`crawl_jobs`, `url_frontier`, `page_fetches`) with `SELECT FOR UPDATE SKIP LOCKED` worker fan-out; Redis adapter for seen-set dedup and robots.txt caching
- `worker` package: async crawl loop — claim, dedupe, robots check, politeness delay, HTTP fetch, content hash, link extraction, re-enqueue
- `api` package: REST handlers for `POST /v1/crawl-jobs`, `GET /v1/crawl-jobs`, `GET /v1/pages`, `GET /v1/frontier/stats`, `POST /v1/frontier/enqueue`
- `metrics` package: Prometheus counters for URLs enqueued/fetched, fetch latency histogram, robots cache hits, dedupe hits, links extracted
- Three-panel tutorial web UI: live Canvas system diagram with animated packets, algorithm step-through, real-time stats polling
- `scripts/migrate.sql`: PostgreSQL schema with partial index on pending frontier entries
- `Dockerfile` + `docker-compose.yml` on port 8093, connected to shared `infra` network
- Unit tests for all domain functions; benchmarks for URL and content hashing

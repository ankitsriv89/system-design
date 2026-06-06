# Changelog

All notable changes to this project are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] — 2026-06-06

### Added
- Initial implementation of the Twitter/X Timeline and Posts service
  (Java 21 / Spring Boot 3.3.4).
- Tweet write path: `POST /v1/posts` and soft-delete `DELETE /v1/posts/{id}`,
  persisting to PostgreSQL and publishing `tweet.created` to Kafka **after
  transaction commit**.
- Follow graph: `POST /v1/follows` with automatic home-timeline backfill.
- Home timeline read: `GET /v1/home` using **hybrid fanout** — materialized
  Redis ZSET timelines (fanout-on-write) merged with read-time celebrity pulls
  (fanout-on-read), re-ranked and de-duplicated, with soft-deletes filtered.
- Timeline operator controls: `POST /v1/home/backfill`, `GET /v1/home/stats`.
- `FanoutService` Kafka consumer (`twitter-fanout`) with a celebrity-threshold
  skip to bound write amplification.
- `SearchIndexer` Kafka consumer (`twitter-search-indexer`) writing tweets to
  OpenSearch as an independent projection.
- Public discovery: full-text `GET /v1/search` (match query) and trending
  hashtags `GET /v1/trends` (terms aggregation over a 24h window).
- Time-decay `RankingService` (`0.5 ^ (ageHours / halfLife)`).
- Demo JWT auth (`POST /api/auth/token`) and a stateless Spring Security chain;
  search/trends are public, everything else requires a token.
- Prometheus metrics (`twitter_*`) for tweets, fanout, celebrity skips,
  read-path pulls, cache hit/miss, indexing, search, trends, plus build/fanout/
  index timers.
- Three-panel tutorial web UI (compose, follow graph, live home timeline,
  search, trends, API log) exercising every endpoint with real fetch calls.
- Flyway migration `V1__init.sql` (tweets + follows) and
  `infra/initdb/19_twitter.sql` DB/user provisioning.
- Bundled single-node OpenSearch in `docker-compose.yml`; shared
  Postgres/Redis/Kafka via the external `infra` network.
- Docs: architecture, code-flow, api, build-log, changelog.
- Integration smoke test `scripts/integration_test.sh`.
- Infra wiring: Caddy `/p19/ → :8100` (with `/p19/actuator/*` blocked at the
  edge), `infra/ports.md` row.

### Notes
- Live integration test not executed in the initial build (no Docker daemon in
  the build environment); build + unit tests pass and the compose config
  validates. Project remains 🔧 in progress until deployed and verified live.

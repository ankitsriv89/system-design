# Changelog — News Feed System

## 0.1.0 — initial implementation

### Added
- Post and follow domain model (PostgreSQL via JPA + Flyway), with soft-delete
  on posts and unique/directional indexes on follows.
- Durable write path: `POST /v1/posts` commits to Postgres, then publishes
  `post.created` to Kafka from an after-commit callback.
- Hybrid fanout: `FanoutService` Kafka worker materializes posts into follower
  Redis timelines (sorted sets), skipping authors above the celebrity threshold.
- Ranked read path: `GET /v1/feed` merges materialized timeline with read-time
  celebrity pulls, filters deleted posts, and ranks by time-decay score.
- `POST /v1/follows` with automatic timeline backfill; `POST /v1/feed/backfill`
  and `GET /v1/feed/stats` operator controls.
- Time-decay ranking (`RankingService`, true half-life formula).
- Prometheus metrics (`newsfeed_` prefix): posts, fanout writes, celebrity skips,
  read-path pulls, feed reads, cache hit/miss, fanout and feed-build timers.
- Single-page demo UI (XSS-safe) and `scripts/integration_test.sh`.
- Unit tests for ranking (`RankingServiceTest`).

### Notes
- Port 8097, Caddy path `/p16/`. Connects to shared `infra` network
  (Postgres/Redis/Kafka); DB provisioned via `infra/initdb/16_newsfeed.sql`.

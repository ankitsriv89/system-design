# Changelog — News Feed System (Project 16)

All notable changes follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] — 2026-06-05

### Added

- **Post write path** — `PostService.create()` commits a post to PostgreSQL inside
  a `@Transactional` method, then publishes a `post.created` event to Kafka via an
  `afterCommit` hook. Guarantees the source of truth leads the async pipeline.
- **Soft-delete** — `DELETE /v1/posts/{id}` sets `deleted=true`; the read path
  filters deleted posts lazily. Only the author may delete (`403` otherwise).
- **Follow graph** — `FollowService` with idempotent `follow()` (duplicate is
  a no-op). Provides `followersOf`, `followeesOf`, and `followerCount` queries
  to serve both the fanout worker and the read path.
- **Fanout worker** — `FanoutService` `@KafkaListener` materializes posts into
  follower home timelines (Redis ZSETs) using per-follower ZADDs. Celebrity
  authors (follower count above configurable threshold, default 1000) are skipped
  — their posts are pulled at read time.
- **Hybrid feed read** — `FeedService.homeFeed()` merges two sources: the
  materialized Redis timeline (fanout-on-write) and a read-time pull for celebrity
  followees (fanout-on-read). De-duplicates, re-scores, sorts descending, pages.
- **Time-decay ranking** — `RankingService` computes `score = 0.5^(age/halfLife)`.
  Half-life is configurable (`newsfeed.ranking.half-life-hours`, default 12 h).
  Score is stored in the ZSET and recomputed at read time for freshness.
- **Timeline store** — `TimelineStore` over Redis sorted sets. Supports `push`,
  `pushMany`, `topN`, `replace` (backfill), and `remove`. Trims to
  `max-cached-items` (default 800) per user.
- **Backfill** — `POST /v1/feed/backfill` and automatic backfill on follow:
  reconstructs a user's timeline from recent non-celebrity followee posts in
  PostgreSQL.
- **Timeline stats** — `GET /v1/feed/stats` returns current Redis ZSET cardinality
  for the authenticated user.
- **Demo auth** — `POST /api/auth/token` mints a JWT for any userId (no password
  verification). Tutorial scope only.
- **Flyway schema** — `V1__init.sql` creates `posts` and `follows` with bi-directional
  follow indexes and an `(author_id, created_at DESC)` composite index on `posts`.
- **Prometheus metrics** — golden signals plus fanout, celebrity-skip, cache
  hit/miss counters and timers, all under the `newsfeed_` prefix.
- **Web UI** — single-page demo: sign in, compose posts, follow users, view ranked
  feed with source/score badges, delete own posts, trigger backfill.
- **Unit tests** — `RankingServiceTest` verifying half-life semantics (4 tests).
- **Integration test script** — `scripts/integration_test.sh` covering the full
  user workflow: auth → follow → post → fanout → feed → delete → 403/401.

### Fixed

- **Ranking half-life formula** — initial formula `exp(-age/τ)` gave a score of
  `0.632` at age = `halfLifeHours` rather than `0.5`. Corrected to
  `exp(-ln2 · age / halfLife)` so the configured half-life is literally accurate.
  Caught by `RankingServiceTest.scoreHalvesAtHalfLife`.

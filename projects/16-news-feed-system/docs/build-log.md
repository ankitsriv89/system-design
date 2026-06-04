# Build Log — News Feed System (Project 16)

## Goal

Personalized, ranked home feed with a hybrid fanout pipeline (push for normal
authors, pull for celebrities), per the plan's learning objectives:
fanout-on-write/read, ranking, timeline materialization, backfill.

## Stack

Java 21 · Spring Boot 3.3 · Gradle · PostgreSQL (source of truth) ·
Redis (materialized timelines, sorted sets) · Kafka (fanout pipeline) ·
Micrometer/Prometheus. Port **8097**, Caddy path `/p16/`.

## What was built

- **Domain + persistence:** `Post`, `Follow` entities; Flyway `V1__init.sql`
  with author/created and follow-direction indexes; soft-delete on posts.
- **Write path:** `PostService` commits to Postgres, then publishes
  `post.created` to Kafka from an `afterCommit` callback (durable-before-fanout).
- **Fanout worker:** `FanoutService` `@KafkaListener` materializes posts into
  follower ZSETs, or skips authors over the celebrity threshold.
- **Read path:** `FeedService` merges materialized timeline + read-time celebrity
  pulls, filters soft-deleted posts, re-scores, ranks.
- **Ranking:** `RankingService` time-decay `0.5^(age/halfLife)`.
- **Timeline store:** `TimelineStore` over Redis sorted sets with size trimming.
- **API:** `/v1/posts`, `/v1/follows`, `/v1/feed`, `/v1/feed/backfill`,
  `/v1/feed/stats`, demo `/api/auth/token`. JWT filter + stateless security.
- **Metrics:** `FeedMetrics` — golden signals + fanout/celebrity/cache counters.
- **Web UI:** single-page demo (sign in, post, follow, ranked feed with
  source/score badges, delete, backfill). XSS-safe (textContent/DOM only).
- **Tests:** `RankingServiceTest` (unit); `scripts/integration_test.sh`
  (auth → follow → post → fanout → feed → delete-disappears → 403 non-author →
  401 unauth).

## Decisions & rationale

- **Hybrid fanout over pure push or pull** — bounds both write amplification
  (celebrity skip) and read cost (single ZREVRANGE for the common case).
- **Score stored as the ZSET score** — timeline stays in ranked order without
  re-sorting on read; recency decay is recomputed on read for freshness.
- **Soft delete + lazy read-time filtering** — the simplest correct answer to
  "deleted post remains in feed"; no need to chase down every materialized copy.
- **after-commit event publish** — the one ordering rule that makes the async
  pipeline safe; a rolled-back post is never fanned out.

## Verification status

- `./gradlew test bootJar` — **passing** (4 unit tests green, jar builds, ~86 MB
  boot jar).
- Initial `RankingServiceTest` caught a real bug: the decay formula used a plain
  time constant (`exp(-age/τ)`, halving at 0.693·τ) rather than a true half-life.
  Fixed to `exp(-ln2·age/halfLife)` so "half-life-hours" is literally accurate.
- Live end-to-end (`integration_test.sh`) runs under the post-build workflow once
  the shared infra stack (Postgres/Redis/Kafka on the `infra` network) is up.

## Follow-ups / stretch

- Pipeline the per-follower ZADDs in `pushMany` for large fan-outs.
- Add an engagement term to the ranking once recency-only is the bottleneck.
- Load test: hotspot (celebrity post) vs steady-state, record numbers in README.

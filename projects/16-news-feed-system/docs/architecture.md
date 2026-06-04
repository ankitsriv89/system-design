# Architecture — News Feed System (Project 16)

## Problem

Generate a personalized home feed for each user from the posts of the people
they follow, ranked rather than purely chronological, at a scale where a naive
"query everyone you follow on every read" does not survive.

## The core tension: fanout-on-write vs fanout-on-read

| | Fanout-on-write (push) | Fanout-on-read (pull) |
|---|---|---|
| When work happens | At post time | At read time |
| Read cost | O(1) — read your own timeline | O(followees) — query all authors |
| Write cost | O(followers) — write to every follower | O(1) — just store the post |
| Pain point | **Celebrity** with millions of followers ⇒ write storm | Active users reading often ⇒ read storm |

This project uses a **hybrid**. Normal authors fan out *on write*; authors above a
follower threshold (`newsfeed.fanout.celebrity-threshold`, default 1000) are
*not* fanned out — their posts are pulled at read time and merged in. This caps
write amplification while keeping the common-case read to a single Redis call.

## Components

```
                 ┌────────────┐   POST /v1/posts
   demo UI ─────▶│  API tier  │──────────────────┐
   (web/)        │ (Spring)   │                  │ 1. commit to Postgres
                 └─────┬──────┘                  ▼
                       │                    ┌──────────┐
                       │ 2. after-commit    │ Postgres │  posts, follows
                       │    publish event   │ (source  │  (durable truth)
                       ▼                    │ of truth)│
                 ┌───────────┐              └────┬─────┘
                 │   Kafka   │ post.created      │ read-path pull
                 │ newsfeed. │                   │ (celebrities, backfill)
                 │  events   │                   │
                 └─────┬─────┘                   │
                       │ 3. consume              │
                       ▼                         │
                 ┌───────────┐  ZADD             │
                 │  Fanout   │──────────┐        │
                 │  worker   │          ▼        ▼
                 └───────────┘     ┌─────────────────┐
                                   │      Redis      │ feed:{user} ZSETs
                 GET /v1/feed ◀────│ (materialized   │ member=postId
                                   │   timelines)    │ score=rank
                                   └─────────────────┘
```

- **API tier** — Spring Boot REST. JWT auth at the boundary (demo-mode token
  minting; no password store). Validates input, writes posts/follows.
- **PostgreSQL** — durable source of truth for `posts` and `follows`. A post is
  committed here *before* any event is published.
- **Kafka** (`newsfeed.events`, 6 partitions, keyed by `authorId`) — carries
  `post.created`, decoupling the synchronous write from async materialization.
- **Fanout worker** (`FanoutService`, `@KafkaListener`) — materializes each post
  into follower timelines, or skips celebrities.
- **Redis** — one sorted set per user (`feed:{userId}`): member = postId,
  score = ranking score. Trimmed to `max-cached-items` (default 800).
- **Ranking** (`RankingService`) — time-decay score `0.5^(age/halfLife)`.

## Consistency model

- **Strongly consistent:** post creation and the follow graph (single Postgres
  transaction). Reading a post you just created back from `/v1/posts` is durable.
- **Eventually consistent:** the home feed. After a post commits, fanout happens
  asynchronously, so a follower's materialized timeline converges within the
  fanout latency (sub-second locally). This is the deliberate tradeoff that keeps
  the write path fast.
- **Durable-before-fanout:** the `post.created` event is published from an
  `afterCommit` transaction callback, so we never fan out a post that rolled back.

## Failure modes addressed

| Failure mode | Handling |
|---|---|
| **Celebrity fanout** | Authors over threshold are skipped on write; pulled on read. Metric: `newsfeed_celebrity_skips_total`. |
| **Stale feed** | Backfill (`POST /v1/feed/backfill`, and automatically on follow) rebuilds a timeline from the source of truth. |
| **Deleted post remains in feed** | Soft-delete + lazy filtering: the feed assembler resolves every candidate post and drops `deleted=true`, so a stale ZSET entry never surfaces a deleted post. |
| **Duplicate events / at-least-once Kafka** | Fanout is idempotent — re-processing re-issues ZADD with the same member/score, a no-op. |
| **Redis loss** | Timelines are a cache; backfill reconstructs them from Postgres. |

## Capacity sketch

- Write amplification per non-celebrity post = follower count (bounded by
  threshold). A post by a 1000-follower user = 1000 ZADDs.
- Redis memory ≈ users × max-cached-items × (postId + score) ≈ users × 800 × ~24B.
- Read path = 1 Redis ZREVRANGE + (for celebrity followees) 1 indexed Postgres
  query, then an in-memory merge/sort of a few hundred candidates.

## Scaling levers (in order)

1. Add fanout-worker instances (Kafka consumer group scales horizontally by
   partition) before touching the API tier.
2. Pipeline/shard the per-follower ZADDs in `TimelineStore.pushMany`.
3. Tune the celebrity threshold from real follower-distribution data.
4. Add engagement signals to `RankingService` once recency-only is the bottleneck.

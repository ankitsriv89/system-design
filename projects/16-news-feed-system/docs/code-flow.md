# Code Flow — News Feed System

Trace of the three core paths through the codebase.

## Write path: creating a post

```
POST /v1/posts  { body }
  └─ PostController.create()                    [controller/PostController.java]
       principal = JWT subject (currentUser)
  └─ PostService.create(authorId, body)         [service/PostService.java]
       1. posts.save(new Post(...))             → INSERT into Postgres (TX)
       2. registerSynchronization(afterCommit → events.publishPostCreated)
       3. metrics.recordPostCreated()
  └─ (TX commits)
  └─ afterCommit fires
       EventPublisher.publishPostCreated(event) [store/EventPublisher.java]
         kafka.send("newsfeed.events", authorId, PostCreatedEvent)
```

The event is keyed by `authorId` so one author's posts keep order on a partition.
The publish happens **only after commit**, guaranteeing durability-before-fanout.

## Async path: fanout worker

```
Kafka "newsfeed.events" → PostCreatedEvent
  └─ FanoutService.onPostCreated()              [service/FanoutService.java]
       timed by metrics.fanoutTimer()
  └─ fanout(event)
       followerCount = follows.followerCount(authorId)   [Postgres COUNT]
       if followerCount > celebrityThreshold:
           metrics.recordCelebritySkip()         → skip, read path handles it
           return
       followers = follows.followersOf(authorId)         [Postgres]
       score     = ranking.score(createdAt, now)         [RankingService]
       timelines.pushMany(followers, postId, score)       [TimelineStore → Redis ZADD + trim]
       metrics.recordFanoutWrites(followers.size())
```

Idempotent: re-delivery re-runs the same ZADDs (same member, same score) = no-op.

## Read path: assembling the home feed

```
GET /v1/feed?limit=20
  └─ FeedController.feed()                       [controller/FeedController.java]
  └─ FeedService.homeFeed(userId, limit)         [service/FeedService.java]
       timed by metrics.feedBuildTimer()
  └─ buildFeed(userId, limit)
       ── Source 1: materialized (push) ──
       materialized = timelines.topN(userId, limit*3)     [Redis ZREVRANGE WITHSCORES]
         (over-fetch so deleted-post filtering still yields a full page)
       cache hit/miss metric
       ── Source 2: read-time pull (celebrities) ──
       celebrities = followees(userId) filtered by followerCount > threshold
       ── resolve + filter + re-score ──
       posts.findAllById(materialized ids)                [Postgres, one query]
         drop deleted, re-score against now, source="materialized"
       posts.findRecentByAuthors(celebrities, ...)        [Postgres, indexed]
         drop deleted/dupes, re-score, source="read-path"
       ── rank ──
       sort by score desc, take top {limit}
```

The merge resolving every candidate to its `Post` row is what makes deletes safe:
a soft-deleted post is filtered here even if a stale ZSET entry still references it.

## Backfill path

```
POST /v1/follows  (triggers backfill) OR POST /v1/feed/backfill
  └─ FeedService.backfill(userId, maxItems)
       followees = non-celebrity authors userId follows
       recent    = posts.findRecentByAuthors(followees, maxItems)   [Postgres]
       scored    = {postId → ranking.score(...)} minus deleted
       timelines.replace(userId, scored)                            [Redis: DEL + ZADD + trim]
```

Used to (a) populate a new follower's timeline with existing history and
(b) reconstruct a timeline after Redis loss — the cache is always rebuildable
from the Postgres source of truth.

## Key files

| File | Role |
|---|---|
| `controller/*` | HTTP boundary, JWT principal extraction |
| `service/PostService` | durable write + after-commit event |
| `service/FanoutService` | Kafka consumer, hybrid fanout decision |
| `service/FeedService` | read-path merge, ranking, delete filtering, backfill |
| `service/RankingService` | time-decay scoring |
| `store/TimelineStore` | Redis ZSET timeline operations |
| `store/EventPublisher` | Kafka producer wrapper |
| `metrics/FeedMetrics` | Prometheus counters/timers |

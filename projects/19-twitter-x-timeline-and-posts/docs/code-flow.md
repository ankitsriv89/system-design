# Code Flow — Twitter/X Timeline and Posts

Traces every significant call from the HTTP edge through services, stores, the
Kafka workers, and back. Three major operations: **post a tweet**, **read the
home timeline**, and **search / trends**.

## 1. Post a tweet (write path + async fanout/index)

```mermaid
flowchart TD
  C[POST /v1/posts] --> TC[TweetController.create]
  TC --> CU[currentUser from JWT principal]
  TC --> TS[TweetService.create]
  TS --> SAVE[TweetRepository.save -> INSERT]
  TS --> REG[registerSynchronization afterCommit]
  REG -->|txn commits| EP[EventPublisher.publishTweetCreated]
  EP --> K[(Kafka twitter.tweets, key=authorId)]
  TS --> MET[TwitterMetrics.recordTweetCreated]
  TC --> RESP[201 TweetResponse]

  K --> FO[FanoutService.onTweetCreated]
  K --> IX[SearchIndexer.onTweetCreated]
```

**Why each call:**
- `currentUser()` reads the authenticated principal set by `JwtAuthFilter` — the
  author id is never trusted from the request body.
- `TweetService.create` runs in a `@Transactional` method so the INSERT is
  atomic. The Kafka publish is deferred to `afterCommit` so we **never fan out
  or index a tweet that wasn't durably stored** — if the txn rolls back, no
  event fires.
- Keying the Kafka record by `authorId` keeps one author's events on a single
  partition, preserving per-author order across both consumers.

## 2. Async fanout (consumer group `twitter-fanout`)

```mermaid
flowchart TD
  EV[tweet.created] --> FO[FanoutService.fanout]
  FO --> FC[FollowService.followerCount]
  FC -->|count > threshold| SKIP[recordCelebritySkip + return]
  FC -->|count <= threshold| FOL[FollowService.followersOf]
  FOL --> SC[RankingService.score createdAt, now]
  SC --> PUSH[TimelineStore.pushMany followers, tweetId, score]
  PUSH --> ZADD[Redis ZADD timeline:follower + trim]
  PUSH --> MW[recordFanoutWrites]
```

**Why:** the celebrity branch is the write-amplification guard. For ordinary
authors we compute one time-decay score and ZADD the tweet id into every
follower's sorted set, trimming each to `max-cached-items`. Re-processing the
same event re-issues identical ZADDs (idempotent), so at-least-once delivery is
safe.

## 3. Async search indexing (consumer group `twitter-search-indexer`)

```mermaid
flowchart TD
  EV[tweet.created] --> IX[SearchIndexer.onTweetCreated]
  IX --> IDX[SearchStore.index]
  IDX --> HT[extractHashtags text]
  IDX --> DOC[build doc tweetId/author/text/hashtags/createdAt]
  DOC --> PUT[OpenSearch index id=tweetId]
  IX --> MI[recordTweetIndexed]
```

**Why:** a second, independent consumer group means indexing failures don't stall
fanout. Doc id = `tweetId` makes indexing idempotent. Hashtags are extracted
once at index time into a keyword field so the trends aggregation counts whole
tags.

## 4. Read the home timeline (hybrid merge)

```mermaid
flowchart TD
  C[GET /v1/home] --> TLC[TimelineController.home]
  TLC --> HT[TimelineService.homeTimeline]
  HT --> TOPN[TimelineStore.topN materialized ids]
  HT --> CEL[FollowService.followeesOf -> filter celebrities]
  TOPN --> ROWS[TweetRepository.findAllById]
  ROWS --> DROP1[drop deleted, re-score]
  CEL --> PULL[TweetRepository.findRecentByAuthors]
  PULL --> DROP2[drop deleted/dupes, re-score]
  DROP1 --> MERGE[merge into LinkedHashMap]
  DROP2 --> MERGE
  MERGE --> SORT[sort by score desc, take page]
  SORT --> RESP[List of TimelineItemResponse]
```

**Why:** source 1 (materialized ZSET) is the cheap common case; source 2 pulls
celebrity tweets the user follows that were skipped on write. Both are resolved
to live `Tweet` rows so soft-deleted tweets are dropped and every item is
re-scored against *now* before the final ranked page is returned. Over-fetching
(`pageLimit * 3`) ensures a full page survives dropping deletes.

## 5. Follow + backfill

```mermaid
flowchart TD
  C[POST /v1/follows] --> FC[FollowController.follow]
  FC --> FS[FollowService.follow]
  FS -->|new edge| INS[FollowRepository.save]
  FC --> BF[TimelineService.backfill]
  BF --> FE[followeesOf -> non-celebrity]
  FE --> REC[findRecentByAuthors]
  REC --> REPL[TimelineStore.replace rewrite ZSET]
```

**Why:** without backfill, a brand-new follow would only surface the followee's
*future* tweets. Backfill reconstructs the materialized timeline from Postgres so
recent history appears immediately. It's also the repair path after Redis loss.

## 6. Search / trends (public read)

```mermaid
flowchart TD
  S[GET /v1/search?q] --> DC[DiscoveryController.search]
  DC --> SS[SearchService.search]
  SS --> SQ[SearchStore.search -> match query]
  T[GET /v1/trends] --> DCT[DiscoveryController.trends]
  DCT --> ST[SearchService.trends]
  ST --> TQ[SearchStore.trends -> range filter + terms agg]
```

## Call-graph summary

```mermaid
graph LR
  TweetController --> TweetService --> EventPublisher --> Kafka
  Kafka --> FanoutService --> TimelineStore --> Redis
  Kafka --> SearchIndexer --> SearchStore --> OpenSearch
  TimelineController --> TimelineService --> TimelineStore
  TimelineService --> TweetRepository --> PostgreSQL
  DiscoveryController --> SearchService --> SearchStore
  FollowController --> FollowService --> FollowRepository --> PostgreSQL
```

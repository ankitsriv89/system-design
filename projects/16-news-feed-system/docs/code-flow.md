# Code Flow — News Feed System (Project 16)

Traces the three core paths through the codebase from the HTTP entry point to
the storage layer and back.

---

## 1. Write path: creating a post

```mermaid
flowchart TD
    A["POST /v1/posts\nPostController.create()"] --> B["@Valid CreatePostRequest\nbody ≤ 1000 chars"]
    B --> C["PostService.create(authorId, body)"]
    C --> D["posts.save(new Post(...))\n@Transactional — INSERT posts"]
    D --> E["TransactionSynchronizationManager\n.registerSynchronization(afterCommit)"]
    E --> F{Transaction commits?}
    F -- yes --> G["EventPublisher.publishPostCreated(event)\nKafkaTemplate.send(topic, authorId, event)"]
    F -- no --> H["No event published\nRow rolled back — pipeline is safe"]
    G --> I["Kafka topic: newsfeed.events\nkey=authorId → per-author ordering"]
    D --> J["FeedMetrics.recordPostCreated()"]
    C --> K["PostResponse.from(post)\n201 Created"]
```

**Why `afterCommit`?** Publishing before commit risks fanning out a post that
never makes it to the source of truth. The `afterCommit` hook ensures Kafka only
receives events for rows that are durable in PostgreSQL.

---

## 2. Fanout path: consuming a post event

```mermaid
flowchart TD
    A["Kafka: newsfeed.events\nFanoutService.onPostCreated()"] --> B["FeedMetrics.fanoutTimer().record()"]
    B --> C["FollowService.followerCount(authorId)"]
    C --> D{followerCount > threshold?}
    D -- yes --> E["FeedMetrics.recordCelebritySkip()\nlog: celebrity skip\nReturn — no write fanout"]
    D -- no --> F["FollowService.followersOf(authorId)\nFollowRepository.findByFolloweeId()"]
    F --> G["RankingService.score(createdAt, now)\n0.5^(ageHours / halfLifeHours)"]
    G --> H["TimelineStore.pushMany(followers, postId, score)"]
    H --> I["For each follower:\nRedis ZADD feed:{userId} score postId\nTrim to maxItems"]
    I --> J["FeedMetrics.recordFanoutWrites(followers.size())"]
```

**Why celebrity skip?** A user with 10M followers would trigger 10M ZADDs on
every post — a write storm. Authors above the threshold are excluded; their posts
are pulled at read time instead.

**Why is fanout idempotent?** Redis ZADD with the same member and score is a
no-op update. At-least-once Kafka delivery is therefore safe — a re-delivered
event re-issues the same ZADDs with no visible effect.

---

## 3. Read path: assembling a home feed

```mermaid
flowchart TD
    A["GET /v1/feed?limit=20\nFeedController.feed()"] --> B["FeedService.homeFeed(userId, limit)"]
    B --> C["FeedMetrics.feedBuildTimer().record()"]
    C --> D["TimelineStore.topN(userId, limit*3)\nZREVRANGEWITHSCORES feed:{userId} 0 N"]
    D --> E{timeline empty?}
    E -- empty --> F["FeedMetrics.recordCacheMiss()"]
    E -- not empty --> G["FeedMetrics.recordCacheHit()"]
    F --> H["FollowService.followeesOf(userId)"]
    G --> H
    H --> I["Filter followees where followerCount > threshold\n= celebrity authors"]
    I --> J["posts.findAllById(materialized.keySet())\nBatch SELECT from PostgreSQL"]
    J --> K["For each post: if deleted → skip\nRankingService.score(post, now)\nAdd to merged map"]
    I --> L["posts.findRecentByAuthors(celebrities, page)\nPull recent celebrity posts"]
    L --> M["For each celeb post: if deleted or dupe → skip\nAdd to merged map with source=read-path"]
    M --> N["FeedMetrics.recordReadPathPulls(count)"]
    K --> O["Sort merged by score DESC\nTruncate to limit\nReturn List<FeedItemResponse>"]
    N --> O
```

**Why over-fetch from Redis?** We request `limit × 3` IDs from the ZSET to
account for soft-deleted posts. After filtering, we still have a full page.

**Why re-score at read time?** The ZSET score was set at fanout time. For
long-lived posts the stored score is stale. Re-scoring with `now` gives an
accurate ranking, and celebrity posts (never materialized) get the same formula.

---

## 4. Follow + backfill path

```mermaid
flowchart TD
    A["POST /v1/follows\nFollowController.follow()"] --> B["FollowService.follow(followerId, followeeId)"]
    B --> C{already following?}
    C -- yes --> D["No-op — idempotent"]
    C -- no --> E["follows.save(new Follow(...))\nINSERT into follows"]
    E --> F["FeedService.backfill(followerId, maxItems)"]
    F --> G["followeesOf(userId) — now includes new followee\nFilter to non-celebrity authors"]
    G --> H["posts.findRecentByAuthors(followees, page)\nPull up to maxItems recent posts"]
    H --> I["RankingService.score each post"]
    I --> J["TimelineStore.replace(userId, scoredMap)\nDEL feed:{userId} then re-ZADD all\nTrim to maxItems"]
    J --> K["Return {follower, followee, backfilledItems}"]
```

**Why backfill on follow?** Without it, following someone only surfaces their
future posts. Backfill brings their recent history into the timeline immediately,
which matches the expected UX.

---

## 5. Soft-delete path

```mermaid
flowchart TD
    A["DELETE /v1/posts/{postId}\nPostController.delete()"] --> B["PostService.delete(requesterId, postId)"]
    B --> C["posts.findById(postId)"]
    C --> D{post exists?}
    D -- no --> E["Return silently — 204 No Content"]
    D -- yes --> F{post.authorId == requesterId?}
    F -- no --> G["Throw SecurityException\n→ GlobalExceptionHandler → 403 Forbidden"]
    F -- yes --> H["post.setDeleted(true)\nposts.save(post) — UPDATE posts SET deleted=true"]
    H --> I["204 No Content"]
    I --> J["Next feed read: FeedService drops this postId\nbecause p.isDeleted() == true"]
```

**Why soft delete?** The post ID may be materialized in dozens or thousands of
follower timelines (Redis ZSETs). Hunting down every ZSET entry would be
expensive and error-prone. Soft delete + lazy read-time filtering is correct,
cheap, and naturally handles the eventual-consistency lag.

---

## Call graph summary

```mermaid
graph LR
    PostCtl --> PostSvc
    PostSvc --> PostRepo
    PostSvc --> EventPub --> Kafka

    FollowCtl --> FollowSvc --> FollowRepo
    FollowCtl --> FeedSvc

    FeedCtl --> FeedSvc
    FeedSvc --> TimelineStore --> Redis
    FeedSvc --> PostRepo --> PG
    FeedSvc --> FollowSvc
    FeedSvc --> RankingSvc

    Kafka --> FanoutSvc
    FanoutSvc --> FollowSvc
    FanoutSvc --> TimelineStore
    FanoutSvc --> RankingSvc
```

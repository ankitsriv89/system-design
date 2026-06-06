# Code Flow — Project 18

Traces each major operation from the HTTP/Kafka entry point through to storage.

## 1. Begin upload (`POST /v1/media/uploads`)

```mermaid
flowchart TD
  A[MediaController.beginUpload] --> B[MediaService.beginUpload]
  B --> C[new Media PENDING]
  C --> D[MediaRepository.save → Postgres]
  B --> E[MediaStore.presignedPutUrl]
  E --> F[MinioClient.getPresignedObjectUrl PUT]
  B --> G[return CreateUploadResponse<br/>mediaId, objectKey, uploadUrl]
```

Why: the metadata row is created up front (PENDING) so the system has a durable
handle before any bytes exist. The presigned URL lets the client upload directly
to object storage, keeping media bytes off the app.

## 2. Complete upload (`POST /v1/media/{id}/complete`)

```mermaid
flowchart TD
  A[MediaController.completeUpload] --> B[MediaService.completeUpload]
  B --> C[MediaRepository.findById]
  C --> D{owner matches<br/>X-User-Id?}
  D -- no --> E[SecurityException → 403]
  D -- yes --> F[MediaStore.objectExists]
  F -- missing --> G[IllegalStateException → 409]
  F -- present --> H[EventPublisher.publishMediaUploaded]
  H --> I[Kafka media.uploaded]
```

Why: completion verifies the client actually uploaded the bytes before announcing
the media to downstream consumers, and enforces ownership.

## 3. Variant generation (Kafka `media.uploaded`)

```mermaid
flowchart TD
  A[MediaProcessingWorker.onMediaUploaded] --> B{isImage?}
  B -- no/video --> C[variants = original only]
  B -- image --> D[MediaStore.getObject original]
  D --> E[VariantGenerator.generate<br/>thumbnail/small/medium]
  E --> F[MediaStore.putObject each variant]
  F --> G[variantKeys map]
  C --> H[MediaService.markProcessed PROCESSED]
  G --> H
  H --> I[CdnInvalidationService.purge keys]
  H --> J[EventPublisher.publishMediaProcessed]
  A -. on error .-> K[markFailed + media.processed success=false]
```

Why: processing is async and idempotent — re-delivery just rewrites the same
variant keys. Reprocessing purges the CDN so stale variants aren't served.

## 4. Create post (`POST /v1/posts`)

```mermaid
flowchart TD
  A[PostController.create] --> B[PostService.createPost]
  B --> C[MediaService.get mediaId]
  C --> D{owner matches?}
  D -- no --> E[SecurityException → 403]
  D -- yes --> F[PostRepository.save → Postgres]
  F --> G[EventPublisher.publishPostCreated]
  G --> H[Kafka post.created]
```

Why: a post can reference a still-PENDING media (write path not blocked on
processing). The event drives fanout asynchronously.

## 5. Fanout (Kafka `post.created`)

```mermaid
flowchart TD
  A[FanoutService.onPostCreated] --> B[FollowService.followerCount]
  B --> C{> celebrity threshold?}
  C -- yes --> D[skip — pulled at read time]
  C -- no --> E[FollowService.followersOf]
  E --> F[RankingService.score]
  F --> G[TimelineStore.pushMany<br/>ZADD feed:follower + trim]
```

Why: hybrid fanout bounds write amplification. Pushing to each follower's ZSET
makes the common-case read O(1) range scan.

## 6. Read feed (`GET /v1/feed`)

```mermaid
flowchart TD
  A[FeedController.feed] --> B[FeedService.feed]
  B --> C[TimelineStore.topN<br/>ZREVRANGE feed:user]
  B --> D[FollowService.followingOf → filter celebrities]
  D --> E[PostRepository.findByUserIdInOrderByCreatedAtDesc]
  E --> F[RankingService.score live]
  C --> G[merge scored map]
  F --> G
  G --> H[sort by score desc, limit]
  H --> I[hydrate: Post + Media + likeCount]
  I --> J[MediaUrlService.urlsFor → CDN URLs]
  J --> K[FeedItemResponse list]
```

Why: the precomputed timeline serves the bulk cheaply; celebrity posts are merged
in live so they're never missed despite being skipped during fanout.

## 7. Like (`POST /v1/posts/{id}/likes`)

```mermaid
flowchart TD
  A[PostController.like] --> B[EngagementService.like]
  B --> C{already liked?}
  C -- yes --> D[return current count idempotent]
  C -- no --> E[EngagementRepository.save → Postgres]
  E --> F[CounterStore.incrementLikes → Redis INCR]
  F --> G[EventPublisher.publishPostLiked]
```

Why: durable row in Postgres (source of truth) + atomic Redis counter (hot read).
Idempotency keeps the counter from drifting under retries/double-clicks.

## Serve media bytes (`GET /media/**`)

```mermaid
flowchart TD
  A[MediaObjectController.serve] --> B[extract objectKey from path]
  B --> C[MediaStore.objectExists]
  C -- no --> D[404]
  C -- yes --> E[MediaStore.getObject]
  E --> F[ResponseEntity + immutable Cache-Control]
```

## Call graph summary

```mermaid
graph LR
  Controllers --> Services
  Services --> Repositories --> Postgres
  Services --> Stores
  Stores --> Redis
  Stores --> MinIO
  Services --> EventPublisher --> Kafka
  Kafka --> Workers --> Services
```

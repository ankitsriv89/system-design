# API — News Feed System

Base URL (local): `http://localhost:8097`

All `/v1/**` endpoints require `Authorization: Bearer <token>`. Obtain a token
from `/api/auth/token` (demo-mode: any userId, no password).

---

## POST /api/auth/token

Mint a demo JWT for a user. **No password verification** — tutorial scope only.

```bash
curl -X POST http://localhost:8097/api/auth/token \
  -H 'Content-Type: application/json' \
  -d '{"userId":"alice"}'
```

```json
{ "token": "eyJhbGc...", "userId": "alice" }
```

---

## POST /v1/posts

Create a post as the authenticated user. Committed durably before fanout.

```bash
curl -X POST http://localhost:8097/v1/posts \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"body":"hello world"}'
```

`201 Created`:
```json
{ "id": 42, "authorId": "alice", "body": "hello world", "createdAt": "2026-06-05T10:00:00Z" }
```

Validation: `body` required, ≤ 1000 chars (`400` otherwise).

---

## DELETE /v1/posts/{postId}

Soft-delete a post. Only the author may delete (`403` otherwise). The post is
filtered out of all feeds lazily at read time. Returns `204 No Content`.
Deleting a non-existent post is a no-op (`204`).

```bash
curl -X DELETE http://localhost:8097/v1/posts/42 -H "Authorization: Bearer $TOKEN"
```

---

## POST /v1/follows

Follow another user. Idempotent (duplicate follow is a no-op). Following
yourself is rejected (`400`). Triggers an automatic timeline backfill so the
followee's recent history appears immediately.

```bash
curl -X POST http://localhost:8097/v1/follows \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"followeeId":"bob"}'
```

```json
{ "follower": "alice", "followee": "bob", "backfilledItems": 7 }
```

---

## GET /v1/feed?limit=20

The authenticated user's ranked home feed. Merges the materialized timeline
(push) with read-time celebrity pulls, drops deleted posts, ranks by score.

```bash
curl "http://localhost:8097/v1/feed?limit=20" -H "Authorization: Bearer $TOKEN"
```

```json
[
  {
    "postId": 42, "authorId": "bob", "body": "hi",
    "createdAt": "2026-06-05T10:00:00Z",
    "score": 0.987, "source": "materialized"
  },
  {
    "postId": 41, "authorId": "celeb", "body": "to my 2M followers",
    "createdAt": "2026-06-05T09:30:00Z",
    "score": 0.961, "source": "read-path"
  }
]
```

`source` is `materialized` (fanned out on write) or `read-path` (pulled at read
time because the author is a celebrity).

---

## POST /v1/feed/backfill?max=800

Operator control: rebuild the caller's materialized timeline from the source of
truth. Used to repair a stale or lost timeline.

```json
{ "user": "alice", "backfilledItems": 120, "timelineSize": 120 }
```

---

## GET /v1/feed/stats

Inspect the caller's materialized timeline size.

```json
{ "user": "alice", "timelineSize": 120 }
```

---

## Observability

- `GET /actuator/health` — liveness.
- `GET /actuator/prometheus` — metrics (prefix `newsfeed_`):
  `newsfeed_posts_created_total`, `newsfeed_fanout_writes_total`,
  `newsfeed_celebrity_skips_total`, `newsfeed_read_path_pulls_total`,
  `newsfeed_feed_reads_total`, `newsfeed_feed_cache_hits_total` /
  `_misses_total`, `newsfeed_feed_build_seconds`, `newsfeed_fanout_seconds`.

## Events

- Topic `newsfeed.events`, keyed by `authorId`:
  - `post.created` → `{ postId, authorId, body, createdAt }`

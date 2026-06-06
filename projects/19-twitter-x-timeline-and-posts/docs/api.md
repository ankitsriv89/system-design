# API — Twitter/X Timeline and Posts

Base URL: `http://localhost:8100` (local) or `https://<host>/p19` (behind Caddy).

All timeline/tweet/follow endpoints require a Bearer JWT. Search and trends are
public. Mint a demo token with `/api/auth/token`.

---

## POST /api/auth/token — mint a demo JWT

Demo-only: accepts any `userId`, no password. (Production would verify a
password hash or delegate to OIDC.)

```bash
curl -s -X POST http://localhost:8100/api/auth/token \
  -H 'Content-Type: application/json' \
  -d '{"userId":"alice"}'
```

```json
{ "token": "eyJhbGciOi...", "userId": "alice" }
```

| Field | Type | Notes |
|---|---|---|
| `userId` | string (required) | The identity the token authenticates as. |

Errors: `400 {"error":"userId required"}` if blank.

---

## POST /v1/posts — create a tweet

```bash
TOKEN=$(curl -s -X POST http://localhost:8100/api/auth/token \
  -H 'Content-Type: application/json' -d '{"userId":"alice"}' | jq -r .token)

curl -s -X POST http://localhost:8100/v1/posts \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"text":"hello from alice about #systemdesign"}'
```

```json
{ "id": 1, "authorId": "alice", "text": "hello from alice about #systemdesign",
  "createdAt": "2026-06-06T09:30:00Z" }
```

| Field | Type | Notes |
|---|---|---|
| `text` | string (required, ≤ 280) | Tweet body. Hashtags feed search/trends. |

Status: `201 Created`. Errors: `400` (blank/too long), `401/403` (no token).

---

## DELETE /v1/posts/{tweetId} — soft-delete a tweet

Only the author may delete. Soft delete: the row remains; reads filter it out.

```bash
curl -s -X DELETE http://localhost:8100/v1/posts/1 \
  -H "Authorization: Bearer $TOKEN"
```

Status: `204 No Content`. Errors: `403 {"error":"only the author can delete this tweet"}`
for a non-author; deleting a missing id is a silent `204`.

---

## POST /v1/follows — follow a user

Adds a follow edge and backfills the caller's home timeline with the followee's
recent history. Idempotent.

```bash
curl -s -X POST http://localhost:8100/v1/follows \
  -H "Authorization: Bearer $BOB_TOKEN" -H 'Content-Type: application/json' \
  -d '{"followeeId":"alice"}'
```

```json
{ "follower": "bob", "followee": "alice", "backfilledItems": 3 }
```

| Field | Type | Notes |
|---|---|---|
| `followeeId` | string (required) | Who to follow. Following yourself → `400`. |

---

## GET /v1/home — home timeline

The hybrid-fanout merged, ranked timeline for the caller.

```bash
curl -s "http://localhost:8100/v1/home?limit=20" \
  -H "Authorization: Bearer $BOB_TOKEN"
```

```json
[
  { "tweetId": 1, "authorId": "alice", "text": "hello…",
    "createdAt": "2026-06-06T09:30:00Z", "score": 0.987, "source": "materialized" },
  { "tweetId": 9, "authorId": "newsbot", "text": "breaking…",
    "createdAt": "2026-06-06T09:25:00Z", "score": 0.901, "source": "read-path" }
]
```

| Field | Type | Notes |
|---|---|---|
| `limit` | query int (default 20) | Page size. |
| `source` | response | `materialized` (fanout-on-write) or `read-path` (celebrity pull). |
| `score` | response | Time-decay ranking score, higher = fresher/more relevant. |

Errors: `401/403` without a token.

---

## POST /v1/home/backfill — rebuild the caller's timeline

Operator/repair control. Rewrites the Redis timeline from PostgreSQL.

```bash
curl -s -X POST "http://localhost:8100/v1/home/backfill?max=800" \
  -H "Authorization: Bearer $BOB_TOKEN"
```

```json
{ "user": "bob", "backfilledItems": 12, "timelineSize": 12 }
```

---

## GET /v1/home/stats — timeline size

```bash
curl -s http://localhost:8100/v1/home/stats -H "Authorization: Bearer $BOB_TOKEN"
```

```json
{ "user": "bob", "timelineSize": 12 }
```

---

## GET /v1/search — full-text tweet search (public)

```bash
curl -s "http://localhost:8100/v1/search?q=systemdesign&limit=10"
```

```json
[
  { "tweetId": 1, "authorId": "alice", "text": "hello from alice about #systemdesign",
    "createdAt": "2026-06-06T09:30:00Z", "relevance": 1.42 }
]
```

| Field | Type | Notes |
|---|---|---|
| `q` | query string (required) | Match query over tweet text. |
| `limit` | query int (default 20) | Max hits. |
| `relevance` | response | OpenSearch BM25 score. |

Errors: `400 {"error":"query parameter 'q' is required"}` if blank.

---

## GET /v1/trends — trending hashtags (public)

Top-N hashtags over the last 24h, by tweet count.

```bash
curl -s http://localhost:8100/v1/trends
```

```json
[
  { "hashtag": "systemdesign", "count": 14 },
  { "hashtag": "kafka", "count": 9 }
]
```

---

## GET /actuator/health, /actuator/prometheus

Spring Boot Actuator. Public on the host; **blocked** at the Caddy edge for
`/p19/actuator/*` (Prometheus scrapes over the internal network).

```bash
curl -s http://localhost:8100/actuator/health
```

```json
{ "status": "UP" }
```

### Metrics (prefix `twitter_`)

`twitter_tweets_created_total`, `twitter_fanout_writes_total`,
`twitter_celebrity_skips_total`, `twitter_read_path_pulls_total`,
`twitter_timeline_reads_total`, `twitter_timeline_cache_hits_total`,
`twitter_timeline_cache_misses_total`, `twitter_tweets_indexed_total`,
`twitter_search_queries_total`, `twitter_trend_queries_total`,
plus timers `twitter_timeline_build_seconds`, `twitter_fanout_seconds`,
`twitter_index_seconds`.

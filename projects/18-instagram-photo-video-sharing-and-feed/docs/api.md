# API — Project 18

Base URL: `http://localhost:8099` (direct) or `http://<host>/p18` (via Caddy).

**Identity**: all write endpoints require an `X-User-Id` header (numeric). This is
a demo-only trusted header — in production an upstream gateway sets it from a
verified credential and never accepts it from the client. Seeded demo users:
`1=alice, 2=bob, 3=carol, 4=dave`.

---

## Media

### Begin upload — `POST /v1/media/uploads`
Creates a PENDING media record and returns a presigned PUT URL.

```bash
curl -X POST http://localhost:8099/v1/media/uploads \
  -H 'X-User-Id: 1' -H 'Content-Type: application/json' \
  -d '{"contentType":"image/png"}'
```
```json
{ "mediaId": 1, "objectKey": "originals/1/3f2c...", "uploadUrl": "http://minio:9000/media/originals/1/3f2c...?X-Amz-..." }
```
| Field | Description |
|---|---|
| `contentType` | MIME type of the bytes to upload (required) |

### Upload the bytes (direct to object storage)
```bash
curl -X PUT "<uploadUrl>" --data-binary @photo.png
```

### Complete upload — `POST /v1/media/{mediaId}/complete`
Verifies the object exists and emits `media.uploaded`.

```bash
curl -X POST http://localhost:8099/v1/media/1/complete -H 'X-User-Id: 1'
```
```json
{ "id": 1, "ownerId": 1, "objectKey": "originals/1/3f2c...", "contentType": "image/png", "status": "PENDING", "variants": {} }
```

### Get media — `GET /v1/media/{mediaId}`
Once the worker runs, `status` is `PROCESSED` and `variants` maps names to **public URLs**.
```bash
curl http://localhost:8099/v1/media/1
```
```json
{ "id": 1, "status": "PROCESSED",
  "variants": { "original": "/p18/media/originals/1/3f2c...",
                "thumbnail": "/p18/media/variants/1/thumbnail.jpg",
                "small": "/p18/media/variants/1/small.jpg",
                "medium": "/p18/media/variants/1/medium.jpg" } }
```

### Serve object bytes — `GET /media/**`
Local CDN-shaped serving (immutable Cache-Control). In prod these terminate at the CDN.
```bash
curl http://localhost:8099/media/variants/1/thumbnail.jpg --output thumb.jpg
```

---

## Posts

### Create post — `POST /v1/posts`
```bash
curl -X POST http://localhost:8099/v1/posts \
  -H 'X-User-Id: 1' -H 'Content-Type: application/json' \
  -d '{"mediaId":1,"caption":"sunset"}'
```
```json
{ "id": 10, "userId": 1, "mediaId": 1, "caption": "sunset", "createdAt": "2026-06-06T..." }
```
| Field | Description |
|---|---|
| `mediaId` | media owned by the caller (required) |
| `caption` | up to 2200 chars (optional) |

### Get post — `GET /v1/posts/{postId}`
```bash
curl http://localhost:8099/v1/posts/10
```

### Like — `POST /v1/posts/{postId}/likes`  ·  Unlike — `DELETE /v1/posts/{postId}/likes`
Idempotent; returns the current like count.
```bash
curl -X POST   http://localhost:8099/v1/posts/10/likes -H 'X-User-Id: 2'
curl -X DELETE http://localhost:8099/v1/posts/10/likes -H 'X-User-Id: 2'
```
```json
{ "likeCount": 1 }
```

---

## Follows

### Follow — `PUT /v1/follows/{followeeId}`  ·  Unfollow — `DELETE /v1/follows/{followeeId}`
```bash
curl -X PUT    http://localhost:8099/v1/follows/2 -H 'X-User-Id: 1'
curl -X DELETE http://localhost:8099/v1/follows/2 -H 'X-User-Id: 1'
```
Returns `204 No Content`.

---

## Feed

### Home feed — `GET /v1/feed?limit=20`
Hybrid read: precomputed Redis timeline + live celebrity pull, ranked by score.
```bash
curl http://localhost:8099/v1/feed?limit=20 -H 'X-User-Id: 1'
```
```json
[ { "postId": 10, "authorId": 2, "caption": "sunset", "mediaId": 5,
    "mediaVariants": { "small": "/p18/media/variants/5/small.jpg" },
    "likeCount": 3, "createdAt": "2026-06-06T...", "score": 0.973 } ]
```

---

## Error responses

| Status | When | Body |
|---|---|---|
| `400 Bad Request` | validation failure (missing `contentType`, `mediaId`) | Spring default validation error |
| `403 Forbidden` | acting on media/post not owned by `X-User-Id` | `{"error":"media 1 not owned by user 99"}` |
| `404 Not Found` | unknown media/post id | `{"error":"media not found: 99"}` |
| `409 Conflict` | completing an upload before the object exists | `{"error":"object not uploaded yet: ..."}` |

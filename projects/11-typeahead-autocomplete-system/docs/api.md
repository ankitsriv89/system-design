# API Reference — Typeahead Autocomplete System

Base URL: `http://localhost:8092`

---

## GET /v1/suggest

Return ranked suggestions for a prefix.

**Query parameters**

| Parameter | Type   | Default | Description |
|-----------|--------|---------|-------------|
| `q`       | string | —       | Prefix to complete (required, max 64 chars) |
| `locale`  | string | `en`    | Locale filter |
| `limit`   | int    | `10`    | Max suggestions (1–20) |

**Response 200**

```json
{
  "prefix": "go",
  "locale": "en",
  "suggestions": [
    { "text": "google search", "category": "product",  "score": 990, "item_id": 2 },
    { "text": "golang",        "category": "language", "score": 950, "item_id": 1 },
    { "text": "goroutine",     "category": "concept",  "score": 800, "item_id": 3 }
  ],
  "latency_ms": 2
}
```

```bash
curl "http://localhost:8092/v1/suggest?q=go&locale=en&limit=5"
```

---

## POST /v1/corpus/items

Add an item to the corpus and index it immediately.

**Request body**

```json
{
  "text":       "elasticsearch",
  "category":   "search",
  "popularity": 880,
  "locale":     "en"
}
```

**Response 201**

```json
{
  "id": 14,
  "text": "elasticsearch",
  "category": "search",
  "popularity": 880,
  "locale": "en",
  "created_at": "2026-06-04T10:00:00Z",
  "updated_at": "2026-06-04T10:00:00Z"
}
```

**Errors**

| Status | Body | Reason |
|--------|------|--------|
| 400 | `{"error":"text is required"}` | Empty text field |
| 400 | `{"error":"invalid JSON"}` | Malformed body |
| 500 | `{"error":"add item failed"}` | Database error |

```bash
curl -X POST http://localhost:8092/v1/corpus/items \
  -H "Content-Type: application/json" \
  -d '{"text":"elasticsearch","category":"search","popularity":880,"locale":"en"}'
```

---

## GET /v1/corpus/items

List corpus items with pagination.

**Query parameters**: `locale`, `limit` (default 50), `offset` (default 0)

**Response 200**

```json
{
  "items": [ { "id": 1, "text": "golang", ... }, ... ],
  "limit": 50,
  "offset": 0
}
```

```bash
curl "http://localhost:8092/v1/corpus/items?limit=10&offset=0"
```

---

## GET /v1/corpus/items/{id}

Fetch a single item by ID.

**Response 200** — item JSON; **404** if not found.

```bash
curl http://localhost:8092/v1/corpus/items/1
```

---

## DELETE /v1/corpus/items/{id}

Remove an item from the corpus. The Redis index is pruned on the next rebuild.

**Response 204** — no body.

```bash
curl -X DELETE http://localhost:8092/v1/corpus/items/14
```

---

## POST /v1/feedback/click

Record a user selecting a suggestion. Increments the item's popularity and updates Redis scores immediately.

**Request body**

```json
{
  "prefix":           "go",
  "selected_item_id": 1,
  "latency_ms":       3,
  "locale":           "en"
}
```

**Response 204** — no body.

```bash
curl -X POST http://localhost:8092/v1/feedback/click \
  -H "Content-Type: application/json" \
  -d '{"prefix":"go","selected_item_id":1,"latency_ms":3,"locale":"en"}'
```

---

## POST /v1/admin/rebuild-index

Trigger a full index rebuild synchronously. Deletes all prefix keys and re-indexes every corpus item.

**Response 200**

```json
{
  "total_items":        25,
  "total_prefixes":     312,
  "last_rebuild_at":    "2026-06-04T10:05:00Z",
  "rebuild_duration_ms": 340
}
```

```bash
curl -X POST http://localhost:8092/v1/admin/rebuild-index
```

---

## GET /v1/admin/stats

Return the most recent index statistics (served from Redis cache).

**Response 200** — same shape as rebuild response.

```bash
curl http://localhost:8092/v1/admin/stats
```

---

## GET /healthz

Liveness check.

**Response 200**: `{"status":"ok"}`

---

## GET /metrics

Prometheus metrics endpoint.

---

## Key Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `typeahead_autocomplete_suggest_requests_total` | Counter | Requests by locale and source (redis/postgres) |
| `typeahead_autocomplete_suggest_latency_seconds` | Histogram | Suggest latency by locale |
| `typeahead_autocomplete_corpus_items_total` | Gauge | Current corpus size |
| `typeahead_autocomplete_rebuild_duration_seconds` | Histogram | Rebuild duration |
| `typeahead_autocomplete_rebuilds_total` | Counter | Rebuilds by result |
| `typeahead_autocomplete_click_feedback_total` | Counter | Click feedback events |
| `typeahead_autocomplete_http_requests_total` | Counter | HTTP requests by method/path/status |
| `typeahead_autocomplete_http_latency_seconds` | Histogram | HTTP handler latency |

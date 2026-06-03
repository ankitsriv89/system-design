# API Reference — Web Crawler (Project 12)

Base URL: `http://localhost:8093` (or via Caddy at `/p12/`)

---

## POST /v1/crawl-jobs

Submit a new crawl job. Seeds the frontier with the given URL.

**Request**
```json
{
  "seed_url": "https://example.com",
  "max_depth": 2
}
```

**Response 201**
```json
{
  "ID": 1,
  "SeedURL": "https://example.com",
  "MaxDepth": 2,
  "Status": "running",
  "CreatedAt": "2026-06-04T12:00:00Z"
}
```

**Errors**
| Status | Body |
|---|---|
| 400 | `{"error":"seed_url required"}` |
| 400 | `{"error":"invalid seed_url"}` |
| 500 | `{"error":"create job failed"}` |

```bash
curl -X POST http://localhost:8093/v1/crawl-jobs \
  -H "Content-Type: application/json" \
  -d '{"seed_url":"https://example.com","max_depth":2}'
```

---

## GET /v1/crawl-jobs

List the 20 most recent crawl jobs.

```bash
curl http://localhost:8093/v1/crawl-jobs
```

**Response 200** — array of job objects (same shape as above).

---

## GET /v1/crawl-jobs/{id}

Get a single crawl job by ID.

```bash
curl http://localhost:8093/v1/crawl-jobs/1
```

**Response 200** — single job object.
**Response 404** — `{"error":"job not found"}`

---

## GET /v1/pages/{url_hash}

Retrieve the fetch record for a URL by its SHA-256 hash.

```bash
HASH=$(echo -n "https://example.com" | sha256sum | cut -d' ' -f1)
curl http://localhost:8093/v1/pages/$HASH
```

**Response 200**
```json
{
  "URLHash": "abc123...",
  "URL": "https://example.com",
  "StatusCode": 200,
  "ContentHash": "def456...",
  "BodySize": 12345,
  "FetchedAt": "2026-06-04T12:00:01Z",
  "Error": ""
}
```

**Response 404** — `{"error":"page not found"}`

---

## GET /v1/pages

List the 50 most recently fetched pages.

```bash
curl http://localhost:8093/v1/pages
```

---

## GET /v1/frontier/stats

Returns frontier counts by status and total dedupe-seen count from Redis.

```bash
curl http://localhost:8093/v1/frontier/stats
```

**Response 200**
```json
{
  "frontier": {
    "pending": 42,
    "fetching": 3,
    "done": 150,
    "failed": 2,
    "skipped": 7
  },
  "seen_count": 162
}
```

---

## POST /v1/frontier/enqueue

Manually add a URL to the frontier with a given priority.

**Request**
```json
{
  "url": "https://example.com/page",
  "priority": 5
}
```

**Response 201**
```json
{"url":"https://example.com/page","status":"enqueued"}
```

```bash
curl -X POST http://localhost:8093/v1/frontier/enqueue \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/about","priority":3}'
```

---

## GET /healthz

Liveness probe.

```bash
curl http://localhost:8093/healthz
# {"status":"ok"}
```

---

## GET /metrics

Prometheus metrics endpoint.

```bash
curl http://localhost:8093/metrics | grep web_crawler
```

Key metrics:
- `web_crawler_urls_enqueued_total` — frontier insertions
- `web_crawler_urls_fetched_total{status="2xx|3xx|4xx|5xx|error"}` — fetch outcomes
- `web_crawler_fetch_duration_seconds` — fetch latency histogram
- `web_crawler_robots_total{result="hit|miss|disallowed"}` — robots.txt cache efficiency
- `web_crawler_dedupe_hits_total` — URLs skipped by Redis dedup
- `web_crawler_links_extracted_total` — outbound links found in HTML pages
- `web_crawler_frontier_pending` — current pending queue depth gauge

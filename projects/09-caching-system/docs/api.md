# API Reference — 09 Caching System

Base URL: `http://localhost:8090` (local) or `https://<host>/p09` (via Caddy).

---

## GET /healthz

Health probe.

```bash
curl http://localhost:8090/healthz
```

```json
{"status":"ok"}
```

---

## PUT /v1/cache/{key}

Insert or update a cache entry.

**Path param**: `key` — cache key (URL-encoded if it contains special characters).

**Body**:

| Field | Type | Description |
|---|---|---|
| `value` | string | Value to cache. Required. |
| `ttl_ms` | int | TTL in milliseconds. 0 = use server default (no expiry by default). Negative = no expiry. |

```bash
# Set with no expiry
curl -X PUT http://localhost:8090/v1/cache/user:1001 \
  -H 'Content-Type: application/json' \
  -d '{"value":"Alice","ttl_ms":0}'
```

```bash
# Set with 10 second TTL
curl -X PUT http://localhost:8090/v1/cache/session:abc \
  -H 'Content-Type: application/json' \
  -d '{"value":"active","ttl_ms":10000}'
```

**Response 201**:
```json
{
  "key": "user:1001",
  "value": "Alice",
  "size_bytes": 15,
  "ttl_ms": 0,
  "expires_at": "0001-01-01T00:00:00Z",
  "last_access": "2026-06-03T10:00:00Z",
  "access_count": 1,
  "created_at": "2026-06-03T10:00:00Z"
}
```

**Response 400** (missing key or value):
```json
{"error":"key and value are required"}
```

---

## GET /v1/cache/{key}

Retrieve a cached value. Updates LRU/LFU order.

```bash
curl http://localhost:8090/v1/cache/user:1001
```

**Response 200 (hit)**:
```json
{
  "key": "user:1001",
  "value": "Alice",
  "expires_at": "0001-01-01T00:00:00Z",
  "access_count": 3
}
```

**Response 404 (miss or expired)**:
```json
{"error":"key not found"}
```

---

## DELETE /v1/cache/{key}

Explicitly remove a key.

```bash
curl -X DELETE http://localhost:8090/v1/cache/user:1001
```

**Response 200**:
```json
{"deleted":true}
```

**Response 404**:
```json
{"error":"key not found"}
```

---

## GET /v1/cache

List all non-expired keys.

```bash
curl http://localhost:8090/v1/cache
```

**Response 200**:
```json
["user:1001","session:abc","config:flags"]
```

---

## DELETE /v1/cache

Flush all entries.

```bash
curl -X DELETE http://localhost:8090/v1/cache
```

**Response 200**:
```json
{"flushed":42}
```

---

## GET /v1/stats

Point-in-time cache statistics.

```bash
curl http://localhost:8090/v1/stats
```

**Response 200**:
```json
{
  "keys": 42,
  "hit_rate": 0.873,
  "hits": 1234,
  "misses": 180,
  "evictions": 11,
  "memory_bytes": 8192,
  "policy": "lru",
  "max_bytes": 67108864
}
```

---

## GET /v1/entries

All non-expired entries with full metadata (used by the UI for eviction-order visualisation).

```bash
curl http://localhost:8090/v1/entries
```

**Response 200**: array of entry objects (same schema as PUT response).

---

## GET /metrics

Prometheus text-format metrics.

```bash
curl http://localhost:8090/metrics | grep caching_system
```

Key metrics:

| Metric | Type | Description |
|---|---|---|
| `caching_system_hits_total` | counter | Total cache hits |
| `caching_system_misses_total` | counter | Total cache misses |
| `caching_system_evictions_total{reason}` | counter | Evictions by reason (ttl/capacity/explicit) |
| `caching_system_sets_total` | counter | Total SET operations |
| `caching_system_deletes_total` | counter | Total DELETE operations |
| `caching_system_memory_bytes` | gauge | Approximate memory used |
| `caching_system_key_count` | gauge | Current key count |
| `caching_system_get_duration_seconds` | histogram | GET latency distribution |
| `caching_system_set_duration_seconds` | histogram | SET latency distribution |
| `caching_system_aof_errors_total` | counter | AOF write errors |

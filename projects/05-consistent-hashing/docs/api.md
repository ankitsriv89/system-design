# API Reference — Consistent Hashing Service

Base URL: `http://localhost:8084`

---

## Health

### `GET /healthz`
```bash
curl http://localhost:8084/healthz
# {"status":"ok"}
```

---

## Rings

### `POST /v1/rings` — Create ring
```bash
curl -X POST http://localhost:8084/v1/rings \
  -H 'Content-Type: application/json' \
  -d '{"id":"my-ring","replicas":150}'
# 201
# {"id":"my-ring","hash_fn":"sha256","replicas":150}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `id` | string | required | Unique ring identifier |
| `hash_fn` | string | `"sha256"` | Hash function label (informational) |
| `replicas` | int | 150 | Virtual nodes per unit weight |

**Errors:** `409 Conflict` — ring already exists.

---

### `GET /v1/rings` — List rings
```bash
curl http://localhost:8084/v1/rings
# [{"id":"my-ring","hash_fn":"sha256","replicas":150,"node_count":3,"vnode_count":450,"version":3}]
```

---

### `DELETE /v1/rings/{ring}` — Delete ring
```bash
curl -X DELETE http://localhost:8084/v1/rings/my-ring
# 204 No Content
```

---

## Nodes

### `POST /v1/rings/{ring}/nodes` — Add node
```bash
curl -X POST http://localhost:8084/v1/rings/my-ring/nodes \
  -H 'Content-Type: application/json' \
  -d '{"id":"node-a","weight":1,"address":"10.0.0.1:6379"}'
```

Response includes rebalance stats:
```json
{
  "Version": 1,
  "NodeCount": 1,
  "VNodeCount": 150,
  "ArcLengths": {"node-a": 1.0},
  "StdDev": 0.0,
  "KeyMovement": {
    "TotalKeys": 10000,
    "MovedKeys": 2497,
    "MovedPct": 24.97
  }
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `id` | string | required | Node identifier |
| `weight` | int | 1 | Capacity weight; vnodes = weight × replicas |
| `address` | string | `""` | Informational; e.g. `"host:port"` |

---

### `DELETE /v1/rings/{ring}/nodes/{node}` — Remove node
```bash
curl -X DELETE http://localhost:8084/v1/rings/my-ring/nodes/node-a
# 200 — same Stats response as AddNode
```

**Errors:** `404` — ring or node not found.

---

## Keys

### `GET /v1/rings/{ring}/keys/{key}/owner` — Single owner lookup
```bash
curl http://localhost:8084/v1/rings/my-ring/keys/user%3A42/owner
```
```json
{"key":"user:42","ring_id":"my-ring","owner":"node-b","version":"3"}
```

**Errors:** `503` — ring is empty.

---

### `GET /v1/rings/{ring}/keys/{key}/replicas?n=3` — Replica set
```bash
curl 'http://localhost:8084/v1/rings/my-ring/keys/user%3A42/replicas?n=3'
```
```json
{"key":"user:42","ring_id":"my-ring","replicas":["node-b","node-c","node-a"],"version":3}
```

The first entry is the primary; subsequent entries are replicas at successive clockwise vnodes belonging to distinct physical nodes.

---

## Inspection

### `GET /v1/rings/{ring}/stats`
```bash
curl http://localhost:8084/v1/rings/my-ring/stats
```
```json
{
  "Version": 3,
  "NodeCount": 3,
  "VNodeCount": 450,
  "ArcLengths": {
    "node-a": 0.338,
    "node-b": 0.331,
    "node-c": 0.331
  },
  "StdDev": 0.0038,
  "KeyMovement": null
}
```

`StdDev` approaches 0 as the ring becomes perfectly balanced.

---

### `GET /v1/rings/{ring}/simulate?keys=10000`
```bash
curl 'http://localhost:8084/v1/rings/my-ring/simulate?keys=10000'
```
```json
{
  "ring_id": "my-ring",
  "total_keys": 10000,
  "version": 3,
  "distribution": {
    "node-a": 3381,
    "node-b": 3306,
    "node-c": 3313
  }
}
```

`keys` max: 1,000,000.

---

### `GET /v1/rings/{ring}/vnodes`
```bash
curl http://localhost:8084/v1/rings/my-ring/vnodes
```
```json
{
  "ring_id": "my-ring",
  "version": 3,
  "vnodes": [
    {"Position": 12345678, "NodeID": "node-b"},
    {"Position": 23456789, "NodeID": "node-a"},
    ...
  ]
}
```

Returns all virtual nodes sorted by `Position`. Used by the frontend ring visualiser.

---

## Metrics

```bash
curl http://localhost:8084/metrics
```

Prometheus text format. Key metrics:

| Metric | Type | Description |
|---|---|---|
| `consistent_hashing_ring_ops_total` | Counter | Mutations by `ring_id` and `op` |
| `consistent_hashing_lookup_duration_seconds` | Histogram | Key lookup latency |
| `consistent_hashing_node_count` | Gauge | Physical nodes per ring |
| `consistent_hashing_vnode_count` | Gauge | Virtual nodes per ring |
| `consistent_hashing_ring_stddev` | Gauge | Arc length std dev (balance quality) |
| `consistent_hashing_key_movement_pct` | Gauge | % keys moved on last topology change |

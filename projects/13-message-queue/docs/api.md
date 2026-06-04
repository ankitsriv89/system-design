# API Reference — Message Queue (Project 13)

Base URL: `http://localhost:8094` (or `/p13/` via Caddy)

All request and response bodies are JSON. All timestamps are RFC 3339 UTC.

---

## Topics

### Create Topic
```
POST /v1/topics
```

**Request**
```json
{ "name": "orders", "partitions": 3, "retention_hours": 24 }
```

| Field | Type | Required | Default | Notes |
|-------|------|----------|---------|-------|
| `name` | string | yes | — | Unique topic name |
| `partitions` | int | no | 1 | 1–16 recommended |
| `retention_hours` | int | no | 24 | Stored for reference; not yet enforced by purge job |

**Response 201**
```json
{
  "Name": "orders",
  "Partitions": 3,
  "RetentionPeriod": 86400000000000,
  "CreatedAt": "2026-06-04T10:00:00Z"
}
```

**Errors**
| Code | Body | Meaning |
|------|------|---------|
| 400 | `{"error":"name is required"}` | Missing name |
| 409 | `{"error":"topic already exists"}` | Duplicate topic |

```bash
curl -X POST http://localhost:8094/v1/topics \
  -H "Content-Type: application/json" \
  -d '{"name":"orders","partitions":3,"retention_hours":24}'
```

---

### List Topics
```
GET /v1/topics
```

**Response 200**
```json
{ "topics": [ {"Name":"orders","Partitions":3,...} ] }
```

```bash
curl http://localhost:8094/v1/topics
```

---

### Get Topic
```
GET /v1/topics/{topic}
```

**Response 200** — same shape as Create response.

**Errors**: 404 if topic not found.

```bash
curl http://localhost:8094/v1/topics/orders
```

---

## Messages

### Publish Message
```
POST /v1/topics/{topic}/messages
```

**Request**
```json
{ "key": "user-42", "payload": "{\"item\":\"book\",\"qty\":1}" }
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `key` | string | no | Deterministic partition routing via FNV-1a hash. Empty = round-robin. |
| `payload` | string | yes | Arbitrary string; stored as bytes. |

**Response 201**
```json
{ "id": "0191f3a2b4c80012", "topic": "orders", "partition": 1, "offset": 42 }
```

**Errors**
| Code | Body | Meaning |
|------|------|---------|
| 400 | `{"error":"payload is required"}` | Missing payload |
| 404 | `{"error":"topic not found"}` | Topic does not exist |

```bash
curl -X POST http://localhost:8094/v1/topics/orders/messages \
  -H "Content-Type: application/json" \
  -d '{"key":"user-42","payload":"{\"item\":\"book\"}"}'
```

---

### Poll Messages
```
POST /v1/topics/{topic}/messages:poll
```

Atomically marks up to `max_messages` messages as in-flight (invisible) for `visibility_timeout_seconds`. Returns the message batch to the caller for processing.

**Request**
```json
{
  "consumer_group": "billing-service",
  "partition": -1,
  "max_messages": 10,
  "visibility_timeout_seconds": 30
}
```

| Field | Type | Required | Default | Notes |
|-------|------|----------|---------|-------|
| `consumer_group` | string | yes | — | Logical consumer identifier |
| `partition` | int | no | -1 | -1 = any partition; 0..N-1 = specific partition |
| `max_messages` | int | no | 10 | Capped at 100 |
| `visibility_timeout_seconds` | int | no | 30 | Seconds before message is re-deliverable |

**Response 200**
```json
{
  "count": 2,
  "messages": [
    {
      "id": "0191f3a2b4c80012",
      "topic": "orders",
      "partition": 1,
      "offset": 42,
      "key": "user-42",
      "payload": "{\"item\":\"book\"}",
      "published_at": "2026-06-04T10:00:00Z",
      "visible_at": "2026-06-04T10:00:30Z",
      "delivery_attempts": 1
    }
  ]
}
```

**Errors**
| Code | Body | Meaning |
|------|------|---------|
| 400 | `{"error":"consumer_group is required"}` | Missing group |

```bash
curl -X POST http://localhost:8094/v1/topics/orders/messages:poll \
  -H "Content-Type: application/json" \
  -d '{"consumer_group":"billing-service","partition":-1,"max_messages":5}'
```

---

### Acknowledge Message
```
POST /v1/messages/{id}:ack
```

Marks a message as successfully processed. Prevents re-delivery.

**Request**
```json
{ "consumer_group": "billing-service" }
```

**Response 200**
```json
{ "status": "acked", "id": "0191f3a2b4c80012" }
```

**Errors**
| Code | Body | Meaning |
|------|------|---------|
| 400 | `{"error":"consumer_group is required"}` | Missing group |
| 404 | `{"error":"message not found or already acked"}` | Already acked or wrong group |

```bash
curl -X POST http://localhost:8094/v1/messages/0191f3a2b4c80012:ack \
  -H "Content-Type: application/json" \
  -d '{"consumer_group":"billing-service"}'
```

---

## Admin / Observability

### Queue Depth
```
GET /v1/topics/{topic}/depth
```

**Response 200**
```json
{ "partitions": {"0": 12, "1": 8, "2": 3}, "dlq": 1 }
```

```bash
curl http://localhost:8094/v1/topics/orders/depth
```

---

### List DLQ Messages
```
GET /v1/topics/{topic}/dlq?limit=50
```

**Response 200**
```json
{
  "count": 1,
  "messages": [
    { "ID": "...", "Topic": "orders", "Partition": 2, "Offset": 99, "DeliveryAttempts": 5, ... }
  ]
}
```

```bash
curl http://localhost:8094/v1/topics/orders/dlq?limit=10
```

---

### Stats
```
GET /v1/stats
```

**Response 200**
```json
{
  "total_messages": 100,
  "acked_messages": 80,
  "dlq_messages": 2,
  "inflight_messages": 3,
  "pending_messages": 15
}
```

```bash
curl http://localhost:8094/v1/stats
```

---

### Health
```
GET /healthz
```

**Response 200**: `{"status":"ok"}`

---

### Metrics
```
GET /metrics
```

Prometheus text exposition. Scrape this endpoint at 15 s intervals.

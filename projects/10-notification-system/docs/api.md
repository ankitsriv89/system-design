# API Reference — Notification System (Project 10)

Base URL: `http://localhost:8091` (local) / `https://<host>/p10` (production via Caddy)

---

## Health

### `GET /healthz`
```
curl http://localhost:8091/healthz
{"status":"ok"}
```

---

## Notifications

### `POST /v1/notifications`
Create and enqueue a notification.

**Request**
```json
{
  "user_id": "alice",
  "channel": "email",
  "template_id": "welcome",
  "params": {"Name": "Alice", "Code": "1234"},
  "priority": 1,
  "idempotency_key": "order-42-welcome"
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `user_id` | string | yes | |
| `channel` | string | yes | `email`, `sms`, or `push` |
| `template_id` | string | no | If set, renders subject/body from template + params |
| `params` | object | no | Template substitution values |
| `subject` | string | no | Used if no template |
| `body` | string | no | Used if no template |
| `priority` | int | no | 0=low, 1=normal (default), 2=high |
| `idempotency_key` | string | no | Duplicate requests coalesced |

**Response 201**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "alice",
  "channel": "email",
  "template_id": "welcome",
  "subject": "Welcome, Alice!",
  "body": "Hi Alice, your code is 1234.",
  "priority": 1,
  "status": "queued",
  "created_at": "2026-06-03T10:00:00Z",
  "updated_at": "2026-06-03T10:00:00Z"
}
```

Status is `queued` (dispatched) or `skipped` (user opted out / quiet hours).

**Errors**
| Status | Body | Cause |
|---|---|---|
| 400 | `{"error":"user_id and channel are required"}` | Missing required fields |
| 400 | `{"error":"channel must be email, sms, or push"}` | Invalid channel |
| 404 | `{"error":"template not found"}` | template_id doesn't exist |
| 503 | `{"error":"queue full, try again"}` | Dispatch queue at capacity |

---

### `GET /v1/notifications`
List notifications, newest first.

**Query params**: `user_id` (filter), `limit` (default 20), `offset` (default 0)

```
curl "http://localhost:8091/v1/notifications?user_id=alice&limit=5"
```

**Response 200**
```json
{
  "notifications": [...],
  "count": 5
}
```

---

### `GET /v1/notifications/{id}`
Get a single notification.

```
curl http://localhost:8091/v1/notifications/550e8400-e29b-41d4-a716-446655440000
```

**Response 200**: notification object (see above). **404** if not found.

---

### `GET /v1/notifications/{id}/attempts`
List all delivery attempts for a notification.

```
curl http://localhost:8091/v1/notifications/550e8400-e29b-41d4-a716-446655440000/attempts
```

**Response 200**
```json
{
  "notification_id": "550e8400-...",
  "attempts": [
    {
      "id": "...",
      "notification_id": "...",
      "provider": "mock-email",
      "attempt_number": 1,
      "status": "failed",
      "error_msg": "mock-email: transient SMTP error",
      "latency_ms": 52,
      "attempted_at": "2026-06-03T10:00:00.123Z"
    },
    {
      "attempt_number": 2,
      "status": "delivered",
      "latency_ms": 48
    }
  ],
  "count": 2
}
```

---

## Preferences

### `PUT /v1/preferences/{user_id}`
Set channel preferences for a user.

```
curl -X PUT http://localhost:8091/v1/preferences/alice \
  -H 'Content-Type: application/json' \
  -d '[
    {"channel":"email","enabled":true,"quiet_start":-1,"quiet_end":-1},
    {"channel":"sms","enabled":false,"quiet_start":-1,"quiet_end":-1},
    {"channel":"push","enabled":true,"quiet_start":22,"quiet_end":8}
  ]'
```

`quiet_start`/`quiet_end` are 0–23 hour values (UTC). `-1` disables quiet hours. Wrap-around midnight is supported (e.g. 22→8 = 10pm to 8am).

**Response 204 No Content**

---

### `GET /v1/preferences/{user_id}`
```
curl http://localhost:8091/v1/preferences/alice
{"user_id":"alice","preferences":[...]}
```

---

## Templates

### `POST /v1/templates`
```
curl -X POST http://localhost:8091/v1/templates \
  -H 'Content-Type: application/json' \
  -d '{"id":"otp","channel":"sms","body":"Your OTP is {{.Code}}."}'
```

**Response 201**: template object.

---

### `GET /v1/templates`
```
curl http://localhost:8091/v1/templates
{"templates":[...]}
```

---

## Admin

### `GET /v1/admin/queue/stats`
```
curl http://localhost:8091/v1/admin/queue/stats
{
  "queue_depth": 3,
  "dlq_depth": 0,
  "by_status": {
    "queued": 12,
    "delivered": 45,
    "failed": 2,
    "dlq": 1,
    "skipped": 3
  }
}
```

---

### `PUT /v1/admin/provider/{name}/failure-rate`
Adjust a provider's failure rate at runtime (demo / failure injection).

`name`: `email`, `sms`, or `push`

```
curl -X PUT http://localhost:8091/v1/admin/provider/email/failure-rate \
  -H 'Content-Type: application/json' \
  -d '{"rate": 0.5}'
{"provider":"email","failure_rate":0.5}
```

Rate must be 0.0–1.0. **404** for unknown provider.

---

## Metrics

`GET /metrics` — Prometheus text format.

Key metrics:
- `notification_system_enqueued_total{channel}` — accepted notifications
- `notification_system_delivered_total{channel}` — successful deliveries
- `notification_system_failed_total{channel}` — delivery failures
- `notification_system_retries_total{channel}` — retry attempts
- `notification_system_dlq_total{channel}` — DLQ entries
- `notification_system_queue_depth` — current queue size
- `notification_system_delivery_latency_ms{channel}` — histogram

# API Reference — 07 API Gateway

All admin endpoints are on the **admin port** (default `:8089`).  
Proxy requests go to the **proxy port** (default `:8088`).

Set `Authorization: Bearer <ADMIN_TOKEN>` on all admin requests when `ADMIN_TOKEN` is configured.

---

## Health

### `GET /healthz`
Available on both ports.

```bash
curl http://localhost:8089/healthz
# {"status":"ok"}
```

---

## Routes

### `PUT /v1/routes/{id}`
Create or update a proxy route. Takes effect immediately (reloads in-process router).

**Request:**
```bash
curl -X PUT http://localhost:8089/v1/routes/users-svc \
  -H "Content-Type: application/json" \
  -d '{
    "path_prefix":    "/api/users",
    "upstream":       "http://user-service:8080",
    "strip_prefix":   false,
    "auth_required":  false,
    "required_scope": "",
    "max_body_bytes": 0,
    "timeout_secs":   0,
    "active":         true
  }'
```

**Response `200 OK`:**
```json
{
  "id": "users-svc",
  "path_prefix": "/api/users",
  "upstream": "http://user-service:8080",
  "strip_prefix": false,
  "auth_required": false,
  "required_scope": "",
  "max_body_bytes": 0,
  "timeout_secs": 0,
  "active": true,
  "updated_at": "2026-06-02T12:00:00Z"
}
```

**Fields:**

| Field | Type | Description |
|---|---|---|
| `path_prefix` | string | URL prefix to match. Longest match wins. |
| `upstream` | string | Full URL of the upstream service. |
| `strip_prefix` | bool | If true, removes `path_prefix` before forwarding. |
| `auth_required` | bool | If true, request must carry a valid API key. |
| `required_scope` | string | Scope the key must have. Empty = any valid key. |
| `max_body_bytes` | int64 | Body size limit in bytes. 0 = gateway default (4 MiB). |
| `timeout_secs` | int | Upstream timeout. 0 = gateway default (30 s). |
| `active` | bool | If false, route is excluded from matching. |

**Errors:**
- `400 Bad Request` — missing `path_prefix`/`upstream`, or invalid upstream URL.
- `500 Internal Server Error` — database failure.

---

### `GET /v1/routes`
List all routes.

```bash
curl http://localhost:8089/v1/routes
# [{"id":"users-svc","path_prefix":"/api/users",...}, ...]
```

---

### `GET /v1/routes/{id}`
Get a single route by ID.

```bash
curl http://localhost:8089/v1/routes/users-svc
```

**Errors:** `404 Not Found` if the route does not exist.

---

### `DELETE /v1/routes/{id}`
Delete a route and immediately reload the router.

```bash
curl -X DELETE http://localhost:8089/v1/routes/users-svc
# {"status":"deleted"}
```

---

## API Keys

### `POST /v1/api-keys`
Create a new API key. The raw key value is hashed with SHA-256 before storage; it is never returned after creation.

**Request:**
```bash
curl -X POST http://localhost:8089/v1/api-keys \
  -H "Content-Type: application/json" \
  -d '{
    "owner":         "alice",
    "key":           "my-secret-token-value",
    "scopes":        ["read", "orders"],
    "quota_per_min": 60
  }'
```

**Response `201 Created`:**
```json
{
  "id":            "a3f1c2d4...",
  "owner":         "alice",
  "scopes":        ["read","orders"],
  "quota_per_min": 60
}
```

| Field | Description |
|---|---|
| `owner` | Human-readable owner identifier. |
| `key` | The raw key value clients will send as `Bearer <key>`. Never stored in plaintext. |
| `scopes` | List of scopes. Use `["*"]` to grant all scopes. |
| `quota_per_min` | Max requests per minute. 0 = unlimited. |

**Errors:** `400 Bad Request` if `owner` or `key` is empty.

---

### `GET /v1/api-keys`
List all API keys (hashed key never returned).

```bash
curl http://localhost:8089/v1/api-keys
```

---

### `GET /v1/api-keys/{id}`
Get a single key by ID.

```bash
curl http://localhost:8089/v1/api-keys/a3f1c2d4
```

---

### `POST /v1/api-keys/{id}/revoke`
Revoke a key (sets `active=false`).

```bash
curl -X POST http://localhost:8089/v1/api-keys/a3f1c2d4/revoke
# {"status":"revoked"}
```

---

## Stats

### `GET /v1/stats/quota/{key_id}`
Return current quota usage for a key.

```bash
curl http://localhost:8089/v1/stats/quota/a3f1c2d4
# {"key_id":"a3f1c2d4","quota_per_min":60,"remaining":47}
```

---

## Proxy

### `ANY /{any-path}`
Proxy request to the matched upstream. Attach the API key if the route requires auth.

```bash
# Public route
curl http://localhost:8088/api/users/42

# Authenticated route
curl -H "Authorization: Bearer alice-secret-token" http://localhost:8088/api/orders/99

# Using X-API-Key header
curl -H "X-API-Key: alice-secret-token" http://localhost:8088/api/orders/99
```

**Error responses:**

| HTTP Status | Condition |
|---|---|
| `401 Unauthorized` | Route requires auth; no token or invalid token. |
| `403 Forbidden` | Token valid but missing required scope. |
| `404 Not Found` | No route matches the request path. |
| `413 Request Entity Too Large` | Body exceeds `max_body_bytes` for the matched route. |
| `429 Too Many Requests` | Key has exceeded `quota_per_min`. |
| `502 Bad Gateway` | Upstream connection failed or timed out. |

**Injected headers (sent to upstream):**
- `X-Request-ID`: unique request identifier for tracing.
- `X-Consumer-ID`: the API key ID (only when auth is required).

---

## Metrics

```bash
curl http://localhost:8089/metrics
```

Key metrics:

| Metric | Type | Labels | Description |
|---|---|---|---|
| `api_gateway_requests_total` | Counter | `route`, `status` | Total requests by route and HTTP status text |
| `api_gateway_request_duration_seconds` | Histogram | `route` | End-to-end latency |
| `api_gateway_upstream_errors_total` | Counter | `route` | Upstream connection/timeout errors |
| `api_gateway_active_routes` | Gauge | — | Routes loaded in the in-process router |
| `api_gateway_active_keys` | Gauge | — | Total active API keys |

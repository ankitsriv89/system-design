# API Reference — 06 Load Balancer

Base URL: `http://localhost:8086`

---

## Control Plane

### Register a Backend

```
POST /v1/backends/{service}
Content-Type: application/json

{"url": "http://backend:8001", "weight": 2}
```

**Response 201**
```json
{"service": "web", "url": "http://backend:8001", "status": "registered"}
```

**Errors**
| Status | Body |
|---|---|
| 400 | `{"error": "url required"}` |
| 400 | `{"error": "invalid JSON"}` |

---

### Remove a Backend

```
DELETE /v1/backends/{service}/{backend}
```

`{backend}` must be URL-encoded. Example:

```sh
curl -X DELETE "http://localhost:8086/v1/backends/web/http%3A%2F%2Fbackend%3A8001"
```

**Response 204** (no body)

**Errors**
| Status | Body |
|---|---|
| 404 | `{"error": "balancer: service \"web\" not found"}` |

---

### Set Routing Algorithm

```
PUT /v1/backends/{service}/algorithm
Content-Type: application/json

{"algorithm": "least_connections"}
```

Valid values: `round_robin`, `least_connections`, `weighted_round_robin`, `random`

**Response 200**
```json
{"algorithm": "least_connections"}
```

---

### List All Backends (flat)

```
GET /v1/backends
```

**Response 200**
```json
[
  {
    "url": "http://echo1:8001",
    "service": "web",
    "status": "healthy",
    "weight": 1,
    "active_conns": 0,
    "total_conns": 42,
    "latency_ewma_ms": 3.2
  }
]
```

---

### Service Stats (structured by service)

```
GET /v1/stats
```

**Response 200**
```json
[
  {
    "service": "web",
    "algorithm": "round_robin",
    "backends": [
      {
        "url": "http://echo1:8001",
        "service": "web",
        "status": "healthy",
        "weight": 1,
        "active_conns": 1,
        "total_conns": 100,
        "latency_ewma_ms": 4.1
      }
    ]
  }
]
```

---

### Health Check History

```
GET /v1/backends/{service}/health?limit=50
```

**Response 200**
```json
[
  {
    "BackendURL": "http://echo1:8001",
    "Status": "healthy",
    "LatencyMs": 3,
    "RecordedAt": "2026-06-02T10:00:00Z"
  }
]
```

---

## Data Plane

### Proxy a Request

Any HTTP method is supported. The path after `/proxy/{service}` is forwarded verbatim to the chosen backend.

```
GET /proxy/{service}/{upstream-path}
```

Example:
```sh
curl http://localhost:8086/proxy/web/get
```

On 5xx from the backend the load balancer retries up to 2 times before returning 502.

**Errors**
| Status | Body |
|---|---|
| 503 | `{"error": "balancer: no healthy backends for \"web\""}` |
| 502 | `{"error": "upstream error after retries"}` |

---

## Observability

### Health Check

```
GET /healthz
→ 200 {"status":"ok"}
```

### Prometheus Metrics

```
GET /metrics
```

Key metrics:

| Metric | Type | Labels |
|---|---|---|
| `load_balancer_requests_total` | counter | service, backend, code |
| `load_balancer_request_duration_seconds` | histogram | service, backend |
| `load_balancer_active_connections` | gauge | service, backend |
| `load_balancer_backend_healthy` | gauge | service, backend |
| `load_balancer_health_checks_total` | counter | service, backend, result |
| `load_balancer_retries_total` | counter | service |

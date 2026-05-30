# Rate Limiter

A distributed rate limiter in Go backed by Redis (hot path) and PostgreSQL (policy store).  
Implements **token bucket** and **sliding window** algorithms with atomic Lua scripts on Redis.

## Stack
| Component | Tech |
|-----------|------|
| API server | Go 1.26, gorilla/mux |
| Counter store | Redis 7 (Lua scripts — no races) |
| Policy store | PostgreSQL 16 |
| Metrics | Prometheus + Grafana |
| Logs | zap (structured JSON) |

## Run locally (Docker Compose)

```bash
# Start all services
docker compose up --build

# In a second terminal — seed policies
./scripts/seed.sh

# Check a request
curl -X POST http://localhost:8080/v1/limits/check \
  -H "Content-Type: application/json" \
  -d '{"subject":"user:42","policy_id":"user-token-bucket"}'
# → {"allowed":true,"remaining":19}

# Run the load test (500 requests, 10 concurrent)
./scripts/load_test.sh 10 500
```

Open dashboards:
- Grafana: http://localhost:3000 (admin / admin) — "Rate Limiter — Golden Signals"
- Prometheus: http://localhost:9090
- Metrics raw: http://localhost:8080/metrics

## APIs

### POST /v1/limits/check
Check and debit one unit from the rate limit for a subject.

```json
// Request
{ "subject": "user:42", "policy_id": "user-token-bucket" }

// 200 — allowed
{ "allowed": true, "remaining": 9 }

// 429 — denied
{ "allowed": false, "remaining": 0, "retry_after_ms": 100, "reason": "rate_limit_exceeded" }
```

### PUT /v1/policies/{policy_id}
Create or update a policy.

```json
// Token bucket — 20 burst, 10 refill/s
{
  "subject_type": "user",
  "algorithm": "token_bucket",
  "capacity": 20,
  "refill_rate": 10,
  "action": "deny"
}

// Sliding window — 100 req per 60 s
{
  "subject_type": "ip",
  "algorithm": "sliding_window",
  "capacity": 100,
  "window_ms": 60000,
  "action": "deny"
}
```

### GET /v1/policies
List all policies.

### GET /v1/usage/{subject}
Usage history for a subject (audit log).

### GET /healthz
Health check — returns `{"status":"ok"}`.

## Data model

```
Policy(id, subject_type, algorithm, capacity, refill_rate, window_ms, action)
AuditDecision(subject, policy_id, allowed, reason, retry_after_ms, decided_at)
```

Redis keys:
- `rl:tb:{policy_id}:{subject}` — token bucket hash (`tokens`, `last_ms`)
- `rl:sw:{policy_id}:{subject}` — sliding window sorted set (score = unix ms)

## Design decisions

**Fail-open on Redis error**: if Redis is unavailable, `/check` allows the request and logs the error. This is the right default for an API gateway — prefer availability over strict enforcement. Change to fail-closed by returning 429 in the error branch.

**Lua scripts for atomicity**: each Redis operation is a single `EVAL` — no WATCH/MULTI/EXEC round-trips, no races between read-compute-write steps.

**Policy cache (10 s TTL)**: policies are read on every `/check` call. A short in-process cache avoids a DB round-trip on the hot path. `PUT /policies/:id` invalidates the entry immediately.

**Audit log is async**: `RecordDecision` runs in a goroutine so it never adds latency to the check path. Rows are never updated, only inserted — natural audit trail.

## Algorithms explained

| Algorithm | Best for | Weakness |
|-----------|----------|----------|
| Token bucket | Burst-tolerant APIs | Slightly complex refill math |
| Sliding window log | Precise per-window limits | Memory: O(requests in window) per subject |

## Milestones
- [x] Single-node token bucket + sliding window (in-memory, unit tested)
- [x] Redis-backed distributed limiter (atomic Lua scripts)
- [x] Policy API + PostgreSQL store + in-process cache
- [x] Docker Compose: Redis, Postgres, Prometheus, Grafana
- [x] Seed script + load test script
- [ ] Load-test report with fairness metrics (run after Docker Compose is up)
- [ ] Failure drill (kill Redis, observe fail-open behavior)

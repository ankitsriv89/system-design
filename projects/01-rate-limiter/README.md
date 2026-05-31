# Rate Limiter

A distributed rate limiter in Go backed by Redis (atomic Lua scripts) and PostgreSQL (policy + audit store).
Implements **token bucket** and **sliding window** algorithms with a live interactive demo — **Interstellar Odyssey**.

## Stack

| Component | Tech |
|-----------|------|
| API server | Go 1.26, gorilla/mux |
| Rate-limit store | Redis 7 (atomic Lua scripts) |
| Policy + audit store | PostgreSQL 16 |
| Question bank | PostgreSQL (110 seed questions + Groq AI generation) |
| Metrics | Prometheus + Grafana |
| Logs | zap (structured JSON) |
| Frontend | Three.js (WebGL), vanilla JS modules |

## Run locally

```bash
# Prerequisites: Redis and PostgreSQL 16 running
redis-server --daemonize yes
sudo systemctl start postgresql@16-main

# Start server
cd projects/01-rate-limiter
SESSION_SECRET="your-32-char-secret" \
DATABASE_URL="postgres://rl:rl@localhost:5432/ratelimiter?sslmode=disable" \
REDIS_ADDR="localhost:6379" \
LISTEN_ADDR=":8080" \
go run ./cmd/server

# In a second terminal — seed rate-limit policies
./scripts/seed.sh
```

Open:
- **Interstellar Odyssey** (main demo): http://localhost:8080/odyssey.html
- **Galaxy simulator** (original demo): http://localhost:8080
- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090
- **Raw metrics**: http://localhost:8080/metrics

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SESSION_SECRET` | random ephemeral | HMAC key for signed session cookies (min 32 chars) |
| `DATABASE_URL` | `postgres://rl:rl@localhost:5432/ratelimiter?sslmode=disable` | PostgreSQL DSN |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `LISTEN_ADDR` | `:8080` | HTTP listen address |
| `GROQ_API_KEY` | — | Groq API key for AI question generation (optional) |
| `TRUSTED_PROXIES` | `127.0.0.1,::1` | Comma-separated CIDRs of trusted reverse proxies |

## Interstellar Odyssey (demo)

`http://localhost:8080/odyssey.html` — a space exploration quiz game where **every answer attempt costs one hop**, enforced by a real per-IP token bucket rate limiter.

**The mechanic:**
- You start at Earth and answer questions to travel to the next destination
- 10 destinations: Earth → Moon → Mars → Asteroid Belt → Jupiter → Saturn → Uranus → Neptune → Pluto → Alpha Centauri → Sagittarius A*
- Each session gets **6 hops per 6 hours**, enforced by IP address via Redis
- Answer correctly → jump to next destination (warp animation)
- Answer wrong → shown correct answer, try again (costs another hop)
- Hint available for each question (costs 1 hop)
- Rate limited → overlay shows exact refill countdown

**Why this demonstrates rate limiting:**
- The limit is enforced server-side by your IP — no client-side bypass possible
- Session cookie (HMAC-signed) tracks journey progress separately from rate limit identity
- Resetting your journey does not restore hops — the IP bucket persists
- The `retry_after_ms` field in the 429 response drives the countdown timer

**Questions:**
- 110 hand-written questions seeded at startup (10 per destination)
- Set `GROQ_API_KEY` to generate additional questions via Llama 3.3 70B on demand
- All Groq-generated questions are saved to PostgreSQL for reuse

## Architecture

See [docs/architecture.md](docs/architecture.md) for full Mermaid diagrams.

### Request flow (check endpoint)

```
Client → POST /v1/limits/check
       → Policy Cache (10s TTL) → PostgreSQL (on miss)
       → Redis Lua script (atomic token bucket / sliding window)
       → 200 allowed | 429 denied
       → async audit log → PostgreSQL
```

### Rate limiting algorithms

| Algorithm | Redis key | Best for |
|-----------|-----------|----------|
| Token bucket | `rl:tb:{policy}:{subject}` | Burst-tolerant APIs, smooth refill |
| Sliding window | `rl:sw:{policy}:{subject}` | Precise per-window request counts |

Both run as atomic Lua scripts — no WATCH/MULTI/EXEC races.

## API

### POST /v1/limits/check
Check and debit one unit from the rate limit for a subject.

```json
// Request
{ "subject": "ip:1.2.3.4", "policy_id": "per-ip" }

// 200 — allowed
{ "allowed": true, "remaining": 9 }

// 429 — denied
{ "allowed": false, "remaining": 0, "retry_after_ms": 5400000, "reason": "rate_limit_exceeded" }
```

### PUT /v1/policies/{policy_id}
Create or update a policy.

```json
// Token bucket — 20 burst, 10 refill/s
{ "subject_type": "ip", "algorithm": "token_bucket", "capacity": 20, "refill_rate": 10 }

// Sliding window — 100 req per 60s
{ "subject_type": "ip", "algorithm": "sliding_window", "capacity": 100, "window_ms": 60000 }
```

### GET /v1/odyssey/state
Returns current player progress and real hop count (reads Redis without debiting).

### GET /v1/odyssey/question
Returns a random question for the current destination (no hop cost).

### POST /v1/odyssey/answer
Submits an answer. Debits 1 hop from the IP bucket regardless of correctness.

```json
// Request
{ "question_id": 42, "choice": 2 }

// 200 — correct
{ "allowed": true, "correct": true, "remaining_hops": 4, "destination_idx": 3 }

// 429 — rate limited
{ "allowed": false, "reason": "hop_limit_exceeded", "retry_after_ms": 21234000 }
```

### GET /v1/odyssey/hint
Returns hint for a question. Debits 1 hop.

### POST /v1/odyssey/reset
Resets journey progress to Earth. **Does not reset hop counter** — rate limit persists.

### GET /v1/policies, GET /v1/policies/{id}
List or get a policy.

### GET /v1/usage/{subject}
Recent audit decisions for a subject (last 30).

### GET /healthz
`{"status":"ok"}`

## Data model

```
Policy(id, subject_type, algorithm, capacity, refill_rate, window_ms, action)
AuditDecision(subject, policy_id, allowed, reason, retry_after_ms, decided_at)
OdysseyQuestion(id, destination_id, question, choices jsonb, answer, hint, source, topic)
OdysseyProgress(ip, destination_idx, hops_used, completed, last_activity)
```

Redis keys:
- `rl:tb:{policy_id}:{subject}` — token bucket hash (`tokens`, `last_ms`)
- `rl:sw:{policy_id}:{subject}` — sliding window sorted set (score = unix ms)

## Design decisions

**Atomic Lua scripts**: each Redis operation is a single `EVAL` — no WATCH/MULTI/EXEC, no race between read-compute-write.

**Fail-open on Redis error**: `/check` allows the request and logs the error. Prefer availability over strict enforcement. Change the error branch to fail-closed if needed.

**Policy cache (10s TTL)**: avoids a DB round-trip on every check. `PUT /policies/:id` invalidates immediately.

**Async audit log**: `RecordDecision` runs in a goroutine — never adds latency to the check path.

**IP extraction security**: `X-Forwarded-For` and `X-Real-IP` are only trusted when the request arrives from a CIDR in `TRUSTED_PROXIES`. Without a trusted proxy, `RemoteAddr` is used directly — clients cannot spoof their IP to bypass rate limits.

**Session cookies**: odyssey progress is keyed by an HMAC-signed session cookie (not IP), so progress cannot be hijacked by IP spoofing. Rate limiting remains IP-keyed — the two identities are intentionally separate.

**Peek without debit**: `GET /v1/odyssey/state` reads the current token count from Redis without consuming a token, so page loads never cost hops.

## Milestones

- [x] Token bucket + sliding window (in-memory, unit tested)
- [x] Redis-backed distributed limiter (atomic Lua scripts)
- [x] Policy API + PostgreSQL store + in-process cache
- [x] Prometheus metrics + Grafana dashboard
- [x] Docker Compose (Redis, PostgreSQL, Prometheus, Grafana)
- [x] Interstellar Odyssey frontend (Three.js, NASA WebP backgrounds)
- [x] IP-based rate limiting with proxy-aware extraction
- [x] HMAC-signed session cookies for tamper-proof progress tracking
- [x] Groq AI question generation with PostgreSQL fallback bank
- [ ] Load-test report with fairness metrics
- [ ] Failure drill (kill Redis, observe fail-open)
- [ ] Oracle Cloud ARM deployment

# Architecture — Rate Limiter

## System overview

```mermaid
graph TB
    Browser["Browser\n(Three.js / odyssey.html)"]
    GoServer["Go HTTP Server\n(:8080)"]
    Redis["Redis 7\nRate-limit state"]
    Postgres["PostgreSQL 16\nPolicies · Audit · Questions · Progress"]
    Prometheus["Prometheus\nMetrics scrape"]
    Grafana["Grafana\nDashboard"]

    Browser -->|"REST API\n/v1/odyssey/*\n/v1/limits/check"| GoServer
    Browser -->|"Static files\nWebP · JS · CSS"| GoServer
    GoServer -->|"Atomic Lua scripts\ntoken bucket / sliding window"| Redis
    GoServer -->|"Policy cache miss\nAudit log (async)\nQuestion bank\nProgress"| Postgres
    Prometheus -->|"Scrape /metrics"| GoServer
    Grafana -->|"Query"| Prometheus
```

## Rate-limit check flow

```mermaid
sequenceDiagram
    participant C as Client
    participant G as Go Server
    participant PC as Policy Cache
    participant PG as PostgreSQL
    participant R as Redis

    C->>G: POST /v1/limits/check<br/>{subject, policy_id}
    G->>PC: Get(policy_id)
    alt cache hit
        PC-->>G: Policy
    else cache miss (TTL expired)
        PC->>PG: SELECT * FROM policies WHERE id=?
        PG-->>PC: Policy row
        PC-->>G: Policy (cached 10s)
    end

    alt algorithm = token_bucket
        G->>R: EVAL tokenBucketScript<br/>capacity, refill_rate, cost
    else algorithm = sliding_window
        G->>R: EVAL slidingWindowScript<br/>limit, window_ms
    end

    alt Redis OK
        R-->>G: {allowed, remaining, retry_ms}
    else Redis error
        R-->>G: error
        note over G: fail-open: allowed=true
    end

    G-->>C: 200 allowed | 429 denied

    G-)PG: INSERT audit_decisions (async goroutine)
```

## Interstellar Odyssey — answer flow

```mermaid
sequenceDiagram
    participant B as Browser
    participant G as Go Server
    participant R as Redis
    participant PG as PostgreSQL

    B->>G: GET /v1/odyssey/state
    note over G: IssueSession() → HMAC-signed cookie
    G->>R: PeekTokenBucket(ip) — read without debit
    G->>PG: GetProgress(session_id)
    G-->>B: {destination, hops_remaining, rate_limited}

    B->>G: GET /v1/odyssey/question
    G->>PG: RandomQuestion(destination_id)
    G-->>B: {question_id, question, choices[4]}

    B->>G: POST /v1/odyssey/answer<br/>{question_id, choice}
    note over G: Debit 1 hop FIRST, then check answer
    G->>R: TokenBucketAllow(ip, capacity=6, refill=0.000278/s)

    alt rate limited
        R-->>G: allowed=false, retry_ms
        G-->>B: 429 {retry_after_ms}
    else hop granted
        R-->>G: allowed=true, remaining
        G->>PG: GetQuestionByID(question_id)
        PG-->>G: question + correct answer

        alt answer correct
            G->>PG: SaveProgress(session_id, dest_idx+1)
            G-->>B: {correct:true, next_destination, remaining_hops}
        else answer wrong
            G-->>B: {correct:false, correct_answer, hint, remaining_hops}
        end
    end
```

## Token bucket algorithm (Redis Lua)

```mermaid
flowchart TD
    Start([EVAL tokenBucketScript]) --> Read["HMGET tokens, last_ms"]
    Read --> Exists{key exists?}
    Exists -->|no| Init["tokens = capacity\nlast_ms = now"]
    Exists -->|yes| Refill["elapsed = now - last_ms\ntokens = min(capacity,\n  tokens + elapsed × refill_rate)"]
    Init --> Check
    Refill --> Check{tokens >= cost?}
    Check -->|yes| Debit["tokens -= cost\nallowed = 1\nretry_ms = 0"]
    Check -->|no| Deny["allowed = 0\nretry_ms = ceil((cost-tokens)/refill_rate × 1000)"]
    Debit --> Save["HSET tokens last_ms\nEXPIRE (2 × refill window)"]
    Deny --> Save
    Save --> Return(["return {allowed, floor(tokens), retry_ms}"])
```

## Sliding window algorithm (Redis Lua)

```mermaid
flowchart TD
    Start([EVAL slidingWindowScript]) --> Evict["ZREMRANGEBYSCORE key\n0 → now_ms - window_ms"]
    Evict --> Count["ZCARD key → current count"]
    Count --> Check{count < limit?}
    Check -->|yes| Add["ZADD key now_ms member\nPEXPIRE key window_ms\nallowed = 1"]
    Check -->|no| Deny["allowed = 0"]
    Add --> Return(["return {1, count+1}"])
    Deny --> Return2(["return {0, count}"])
```

## Security model

```mermaid
graph LR
    subgraph "Rate limit identity (IP)"
        IP["Client IP\nextracted from RemoteAddr\nor XFF if from trusted proxy CIDR"]
        RLKey["Redis key\nrl:tb:odyssey-hops:ip:{ip}"]
        IP --> RLKey
    end

    subgraph "Progress identity (Session)"
        Cookie["HMAC-signed cookie\nodyssey_sid=<id>.<sha256>"]
        PGRow["PostgreSQL row\nodyssey_progress WHERE ip=session_id"]
        Cookie --> PGRow
    end

    note1["Rate limit and progress\nare intentionally separate.\nSpoofing one cannot\nbypass the other."]
```

## Data model

```mermaid
erDiagram
    policies {
        text id PK
        text subject_type
        text algorithm
        float8 capacity
        float8 refill_rate
        int8 window_ms
        text action
    }

    audit_decisions {
        bigserial id PK
        text subject
        text policy_id FK
        bool allowed
        text reason
        int8 retry_after_ms
        timestamptz decided_at
    }

    odyssey_questions {
        bigserial id PK
        text destination_id
        text question
        jsonb choices
        smallint answer
        text hint
        text source
        text topic
        timestamptz created_at
    }

    odyssey_progress {
        text ip PK
        int destination_idx
        int hops_used
        bool completed
        timestamptz last_activity
    }

    policies ||--o{ audit_decisions : "policy_id"
```

## Component layout

```mermaid
graph TD
    subgraph "projects/01-rate-limiter"
        subgraph "cmd/server"
            main["main.go\nwire deps, HTTP server,\ngraceful shutdown"]
        end

        subgraph "internal/api"
            handler["handler.go\ncheck, policies,\nusage, health"]
            odyssey_h["odyssey.go\nstate, question,\nanswer, hint, reset"]
        end

        subgraph "internal/algorithm"
            tb["token_bucket.go\nin-memory impl + tests"]
            sw["sliding_window.go\nin-memory impl + tests"]
        end

        subgraph "internal/store"
            redis_l["redis_limiter.go\nLua scripts,\nPeekTokenBucket"]
        end

        subgraph "internal/policy"
            pol["policy.go\nStore (PG), Cache (RWMutex)"]
        end

        subgraph "internal/odyssey"
            journey["journey.go\ndestinations, ClientIP,\nParseProxies"]
            groq["groq.go\nGroq API client"]
            store_o["store.go\nquestions + progress PG\n110 seed questions"]
            session["session.go\nHMAC cookie signing"]
        end

        subgraph "internal/metrics"
            met["metrics.go\nPrometheus counters/histograms"]
        end

        subgraph "web/"
            odyssey_html["odyssey.html"]
            odyssey_js["odyssey-scene.js\nThree.js, star field,\nspacecraft, warp FX"]
            odyssey_ui["odyssey-ui.js\nAPI calls, question\nrendering, overlays"]
            assets["assets/bg/*.webp\n11 NASA images 1280×720"]
        end
    end
```

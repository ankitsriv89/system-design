# Architecture — Notification System (Project 10)

## System Diagram

```mermaid
graph TD
    Client["Client / UI"]
    API["API Layer\n(gorilla/mux)"]
    DB["PostgreSQL\n(notifications, templates,\npreferences, delivery_attempts)"]
    Queue["In-Process Channel\n(buffered, size 1024)"]
    Workers["Worker Pool\n(4 goroutines)"]
    EmailProv["Mock Email Provider"]
    SMSProv["Mock SMS Provider"]
    PushProv["Mock Push Provider"]
    DLQ["DLQ Channel\n(buffered, size 256)"]
    Metrics["Prometheus Metrics"]
    Prom["Prometheus Scraper"]

    Client -->|POST /v1/notifications| API
    API -->|check preferences| DB
    API -->|render template| DB
    API -->|persist notification| DB
    API -->|enqueue job| Queue
    Queue --> Workers
    Workers -->|attempt delivery| EmailProv
    Workers -->|attempt delivery| SMSProv
    Workers -->|attempt delivery| PushProv
    Workers -->|record attempt| DB
    Workers -->|retry after backoff| Queue
    Workers -->|exhausted retries| DLQ
    DLQ -->|drain goroutine| DB
    API -->|expose /metrics| Metrics
    Prom -->|scrape| Metrics
```

## Primary Request Sequence

```mermaid
sequenceDiagram
    participant C as Client
    participant A as API Handler
    participant DB as PostgreSQL
    participant Q as Dispatch Queue
    participant W as Worker
    participant P as Provider Mock

    C->>A: POST /v1/notifications
    A->>DB: GetTemplate (if template_id set)
    DB-->>A: template row
    A->>A: RenderTemplate ({{.Key}} substitution)
    A->>DB: GetPreference (user_id, channel)
    DB-->>A: preference row
    A->>A: check enabled + quiet hours
    A->>DB: CreateNotification (status=queued)
    DB-->>A: notification with UUID
    A->>Q: Enqueue(job)
    A-->>C: 201 Created {id, status:"queued"}

    W->>Q: receive job
    W->>P: Provider.Send(ctx, notification)
    P-->>W: nil | error
    alt success
        W->>DB: InsertAttempt (status=delivered)
        W->>DB: UpdateStatus (delivered)
    else failure, attempt < 3
        W->>DB: InsertAttempt (status=failed)
        W->>W: time.AfterFunc(backoff, re-enqueue)
    else failure, attempt == 3
        W->>DB: InsertAttempt (status=failed)
        W->>Q: DLQ channel
        W->>DB: UpdateStatus (dlq)
    end
```

## Components

### API Layer (`api/`)
Thin HTTP transport. Validates inputs, enforces channel enum, resolves templates, checks preferences, persists to DB, and enqueues the job. Does not block on delivery. Returns 201 with the persisted notification.

### Notification Domain (`notification/`)
Pure domain types with no external imports. Contains `RenderTemplate` (string substitution), `IsQuietHour` (wrap-around time window logic), and all status/channel/priority constants.

### Store (`store/`)
`*sql.DB` wrapper. All persistence operations for notifications, preferences, templates, and delivery attempts. Uses `pgcrypto` UUIDs on the DB side. Implements idempotency via `ON CONFLICT (idempotency_key) DO UPDATE`.

### Worker (`worker/`)
**Dispatcher**: owns a `chan Job` (queue) and `chan Job` (DLQ), both buffered. Starts 4 worker goroutines and 1 DLQ drain goroutine. Workers call `Provider.Send` and apply exponential-backoff retry (`200ms → 400ms → 800ms`) via `time.AfterFunc`. After 3 failures the job goes to the DLQ.

**Provider mocks**: `MockEmailProvider`, `MockSMSProvider`, `MockPushProvider`. Each has a configurable `FailureRate` (0.0–1.0) exposed via the admin API so failure injection works live.

### Metrics (`metrics/`)
Prometheus counters and histograms: enqueued, delivered, failed, retried, DLQ, queue depth, delivery latency, HTTP requests/duration.

## Data Model

| Table | Key Columns | Purpose |
|---|---|---|
| `notifications` | id (UUID), user_id, channel, status, idempotency_key | Canonical notification record |
| `templates` | id (TEXT PK), channel, subject, body | Named reusable templates |
| `preferences` | (user_id, channel) PK, enabled, quiet_start, quiet_end | Per-user channel opt-in and quiet hours |
| `delivery_attempts` | id (UUID), notification_id FK, attempt_number, status, error_msg, latency_ms | Audit trail of every send attempt |

## Capacity Estimates

| Dimension | Estimate |
|---|---|
| Notification throughput | ~500 req/s (single instance, PostgreSQL bound) |
| Worker throughput | ~200 deliveries/s (4 workers × mock latency ~20–50ms) |
| Queue saturation point | 1024 jobs (backpressure returns 503 beyond this) |
| DB storage growth | ~1 KB/notification + ~2 KB/attempt → 3 KB/notification average |
| At 10k notifications/day | ~30 MB/day, ~10 GB/year |

## External Dependencies
- PostgreSQL (shared infra container)
- Prometheus (shared infra container)

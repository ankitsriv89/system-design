# Architecture — Unique ID Generator

## System diagram

```mermaid
graph TD
    Client["Client (service / CLI)"]

    subgraph "04-unique-id-generator"
        HTTP["HTTP API\n:8083\ngorilla/mux"]
        GEN["Generator\ngenerator.Generator\nSnowflake engine"]
        METRICS["Prometheus metrics\n/metrics"]
    end

    subgraph "Shared Infra (infra/)"
        PG[("PostgreSQL\nworker_leases\nclock_incidents")]
        PROM["Prometheus"]
        GRAF["Grafana"]
    end

    Client -->|"POST /v1/ids/next\nPOST /v1/ids/batch"| HTTP
    HTTP -->|"gen.Next() / gen.Batch()"| GEN
    GEN -->|"purely in-memory\nno DB on hot path"| GEN

    HTTP -->|"GET /metrics"| METRICS
    METRICS --> PROM --> GRAF

    subgraph "Lease subsystem (background)"
        LEASE["lease.Manager\nacquire / renew / release"]
    end

    LEASE -->|"SELECT FOR UPDATE SKIP LOCKED\nUPDATE worker_leases"| PG
    GEN -.->|"worker_id assigned at startup\nno runtime coupling"| LEASE
    LEASE -.->|"INSERT clock_incidents\non rollback"| PG
```

## Sequence diagram — generate a single ID

```mermaid
sequenceDiagram
    participant C as Client
    participant H as api.Handler
    participant G as generator.Generator
    participant M as metrics

    C->>H: POST /v1/ids/next
    H->>M: start timer
    H->>G: gen.Next()
    G->>G: lock mutex
    G->>G: read wall clock (nowMs)
    alt same millisecond as last call
        G->>G: sequence++
        alt sequence wrapped to 0
            G->>G: spin until next ms
        end
    else new millisecond
        G->>G: sequence = 0
    end
    G->>G: build ID: ts<<22 | workerID<<12 | seq
    G->>G: unlock mutex
    G-->>H: int64 ID
    H->>M: observe generation_duration
    H->>M: ids_generated++
    H-->>C: 200 {id, id_string, worker_id, region}
```

## Sequence diagram — startup lease acquisition

```mermaid
sequenceDiagram
    participant P as main()
    participant L as lease.Manager
    participant DB as PostgreSQL

    P->>L: lease.New(dsn, region)
    P->>L: lm.Acquire(ctx)
    L->>DB: UPDATE worker_leases SET expires_at=past WHERE expired
    L->>DB: BEGIN
    L->>DB: SELECT worker_id FOR UPDATE SKIP LOCKED (first free row)
    DB-->>L: worker_id = N
    L->>DB: UPDATE worker_leases SET holder=..., expires_at=now+30s WHERE worker_id=N
    L->>DB: COMMIT
    L-->>P: workerID = N
    P->>L: lm.StartRenewer(ctx)
    Note over L: background goroutine renews every 10s
    P->>G: generator.New(workerID, incidentHook)
```

## Components

### `generator` package
Pure in-memory Snowflake ID engine. Holds a mutex, the last-used millisecond, and a sequence counter. No network or disk I/O on the hot path — a single `Next()` call takes nanoseconds.

### `lease` package
Manages the PostgreSQL-backed worker ID lease. On startup it claims one of the 1024 pre-seeded `worker_leases` rows using `SELECT FOR UPDATE SKIP LOCKED` to prevent two instances claiming the same ID. A background goroutine renews the lease every 10 s; if renewal fails the TTL (30 s) provides a safe runway before the slot is reclaimed.

### `api` package
Thin HTTP transport. Validates inputs, calls the generator, records metrics, and serialises responses. Includes `id_string` alongside `id` in every response because JavaScript loses precision when parsing 64-bit integers from JSON.

### `metrics` package
Registers all Prometheus metrics with `promauto` (auto-registered at init time). HTTP middleware records request count and latency for every route.

### `main`
Wires the above together: wait for PostgreSQL → acquire lease → build generator → start renewer → serve HTTP. On SIGINT/SIGTERM: stop renewer → HTTP graceful shutdown → release lease.

## Data model

```
worker_leases (1024 rows, pre-seeded)
├── worker_id   SMALLINT  PK  0–1023
├── holder      TEXT          empty = unclaimed
├── region      TEXT
└── expires_at  TIMESTAMPTZ   < NOW() = claimable

clock_incidents (append-only audit)
├── id          BIGSERIAL  PK
├── worker_id   SMALLINT   FK → worker_leases
├── drift_ms    BIGINT     magnitude of rollback
└── detected_at TIMESTAMPTZ
```

## Capacity

| Parameter | Value |
|---|---|
| Max workers | 1024 |
| IDs per ms per worker | 4096 |
| Global max throughput | ~4.2 billion IDs/ms |
| Timestamp range | 2020–2089 (69 years) |
| ID size | 8 bytes (int64) |

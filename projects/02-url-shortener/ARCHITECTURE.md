# Architecture — URL Shortener (Project 02)

## Overview

A production-pattern URL shortener built in Go, designed as a study in **read-heavy system design**.
Short codes are 7-character Base62 strings. Redirects are served from an in-memory cache with
PostgreSQL as the durable backing store.  Click events are recorded asynchronously so the redirect
hot-path stays under 1 ms.

---

## Stack

| Layer | Technology | Notes |
|---|---|---|
| HTTP server | Go + gorilla/mux | Single binary, no framework |
| Cache | In-process `sync.Map` + TTL | Replaces Redis; resets on restart |
| Database | PostgreSQL 16 | Shared instance across all projects on the VM |
| Metrics | Prometheus + Grafana | Golden-signals dashboard |
| Reverse proxy | Caddy (host-level) | TLS termination, routes by subdomain |
| Container runtime | Docker + Docker Compose | Multi-stage ARM64 build |

---

## Request Flow — Redirect (hot path)

```mermaid
sequenceDiagram
    participant Browser
    participant Server
    participant MemCache
    participant Postgres

    Browser->>Server: GET /{code}
    Server->>MemCache: GetURL(code)

    alt Cache hit
        MemCache-->>Server: longURL
        Server->>Browser: 302 Found → longURL
        Server-)Postgres: INSERT click_event (async)
    else Negative cache hit
        MemCache-->>Server: __missing__ sentinel
        Server->>Browser: 404 Not Found
    else Cache miss
        MemCache-->>Server: (not found)
        Server->>Postgres: SELECT * FROM links WHERE code=?
        alt Link found and active
            Postgres-->>Server: link row
            Server->>MemCache: SetURL(code, longURL, 10m)
            Server->>Browser: 302 Found → longURL
            Server-)Postgres: INSERT click_event (async)
        else Not found
            Server->>MemCache: SetMissing(code, 30s)
            Server->>Browser: 404 Not Found
        else Expired / disabled
            Server->>Browser: 410 Gone
        end
    end
```

---

## Request Flow — Create Link

```mermaid
sequenceDiagram
    participant Client
    participant Server
    participant Postgres

    Client->>Server: POST /v1/links\n{long_url, expires_at}\nX-Owner-ID: demo

    Server->>Postgres: SELECT owner WHERE id=demo
    Postgres-->>Server: owner{quota:100}

    Server->>Postgres: COUNT active links for owner
    Postgres-->>Server: count=3

    loop up to 8 retries
        Server->>Server: Generate 7-char Base62 code (crypto/rand)
        Server->>Postgres: INSERT INTO links(code, long_url, ...)
        alt Unique constraint violation
            Postgres-->>Server: ErrCollision → retry
        else Success
            Postgres-->>Server: ok
        end
    end

    Server->>Client: 201 Created\n{code, short_url, long_url, ...}
```

---

## Component Diagram

```mermaid
graph TB
    subgraph "Oracle Cloud ARM VM"
        Caddy["Caddy\n(reverse proxy + TLS)"]

        subgraph "infra network (Docker)"
            PG["PostgreSQL 16\n(shared across projects)"]
        end

        subgraph "02-url-shortener (Docker Compose)"
            Server["Go Server :8081\n- HTTP handlers\n- Service layer\n- In-memory cache\n- Prometheus metrics"]
            Prom["Prometheus :9091"]
            Grafana["Grafana :3001"]
        end

        Caddy -->|proxy :8081| Server
        Server -->|SQL| PG
        Prom -->|scrape /metrics| Server
        Grafana -->|query| Prom
    end

    Browser["Browser / Client"] -->|HTTPS| Caddy
```

---

## Database Schema

```mermaid
erDiagram
    owners {
        text id PK
        int  quota
        text plan
        datetime created_at
    }

    links {
        text     code       PK
        text     long_url
        text     owner_id   FK
        datetime expires_at
        datetime created_at
        datetime disabled_at
    }

    click_events {
        bigint   id         PK
        text     code       FK
        datetime clicked_at
        text     referrer
        text     user_agent
        text     ip_hash
    }

    owners ||--o{ links : "owns"
    links  ||--o{ click_events : "records"
```

---

## Caching Strategy

```mermaid
flowchart LR
    A[Redirect request] --> B{In-memory cache?}
    B -- hit --> C[Return cached URL\n~0.01 ms]
    B -- negative hit --> D[Return 404\nno DB hit]
    B -- miss --> E[Query Postgres]
    E -- found & active --> F[Cache 10 min\nReturn 302]
    E -- not found --> G[Negative cache 30 s\nReturn 404]
    E -- expired/disabled --> H[Return 410 Gone]
```

**Why negative caching?**  
A thundering herd of requests for a non-existent code would hammer Postgres.
Caching the 404 for 30 seconds absorbs the burst with zero DB load.

---

## Deployment Topology

```mermaid
graph LR
    subgraph "GitHub"
        Repo["system-design repo"]
    end

    subgraph "Oracle Cloud ARM VM (free tier)"
        Caddy
        subgraph "infra/"
            SharedPG["PostgreSQL\n(shared)"]
        end
        subgraph "projects/02-url-shortener/"
            App["Go server\n:8081"]
            Metrics["Prometheus + Grafana\n:9091 / :3001"]
        end
    end

    Repo -- "git pull + docker compose up" --> App
    App --- SharedPG
```

---

## Key Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Cache backend | In-process `sync.Map` | Eliminates Redis as an external dependency; acceptable for a single-instance demo |
| Redirect status | `302 Found` | Intentional — prevents browsers from caching the redirect, ensuring click events are always recorded |
| Click recording | Async goroutine | Keeps redirect p99 latency unaffected by DB write latency |
| Code generation | `crypto/rand` Base62 | Cryptographically unpredictable — prevents enumeration attacks |
| Negative caching | 30-second TTL sentinel | Absorbs thundering herd on invalid codes without DB fan-out |
| Quota enforcement | Count active links at create time | Simple and consistent; avoids complex distributed counters |
| Shared Postgres | One container, per-project DB | Efficient on a free-tier VM with limited RAM; each project owns its schema |

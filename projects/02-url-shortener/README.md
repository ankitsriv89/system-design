# Project 02 — URL Shortener

A production-pattern URL shortener built in Go, designed as a hands-on study of **read-heavy system design** concepts: cache-aside, negative caching, async analytics, quota enforcement, and graceful shutdown.

---

## System Design Concepts Demonstrated

| Concept | Where |
|---|---|
| Cache-aside pattern | `internal/api/handler.go` — Redirect handler |
| Negative caching | `store.MemCache.SetMissing` — 30 s TTL sentinel |
| Async write path | `handler.recordClick` — goroutine per click |
| Quota enforcement | `api.Service.CreateLink` — count before insert |
| Collision retry | `api.Service.CreateLink` — up to 8 retries |
| Graceful shutdown | `cmd/server/main.go` — SIGTERM handler |
| Golden-signal metrics | `internal/metrics/metrics.go` + Grafana dashboard |

---

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for full mermaid diagrams covering:
- Redirect hot-path sequence diagram
- Create-link sequence diagram
- Component and deployment topology
- Database schema (ER diagram)
- Caching strategy flowchart

---

## Stack

| Component | Technology |
|---|---|
| Language | Go 1.26 |
| HTTP router | gorilla/mux |
| Database | PostgreSQL 16 (shared VM instance) |
| Cache | In-process memory (no Redis required) |
| Metrics | Prometheus + Grafana |
| Logging | Uber zap (structured JSON) |
| Container | Docker (multi-stage, static ARM64 binary) |

---

## Project Structure

```
02-url-shortener/
├── cmd/server/main.go          # Entry point — wiring, server, graceful shutdown
├── internal/
│   ├── api/
│   │   ├── handler.go          # HTTP handlers (create, redirect, stats, health)
│   │   ├── service.go          # Business logic (quota, collision retry)
│   │   └── service_test.go     # Unit tests (fake store, no DB required)
│   ├── link/
│   │   ├── link.go             # Domain model + URL validation
│   │   ├── code.go             # Base62 code generator (crypto/rand)
│   │   └── link_test.go        # Unit tests
│   ├── store/
│   │   ├── cache.go            # Cache interface
│   │   ├── mem_cache.go        # In-process TTL cache (replaces Redis)
│   │   └── postgres.go         # PostgreSQL data layer
│   ├── analytics/analytics.go  # ClickEvent + Stats types
│   ├── metrics/metrics.go      # Prometheus metric definitions
│   └── owner/owner.go          # Owner domain model
├── web/
│   ├── index.html              # Demo UI
│   ├── app.js                  # Frontend logic
│   └── styles.css              # Styles
├── deploy/
│   ├── prometheus/prometheus.yml
│   └── grafana/                # Dashboard + datasource provisioning
├── scripts/
│   ├── seed.sh                 # Create a single demo link
│   └── load_test.sh            # Parallel load test (curl)
├── ARCHITECTURE.md             # Mermaid diagrams + design decisions
├── Dockerfile                  # Multi-stage build (CGO_ENABLED=0, ARM64-safe)
└── docker-compose.yml          # Server + Prometheus + Grafana
```

---

## Running Locally

### Prerequisites

- Docker + Docker Compose
- The shared infra stack running (provides PostgreSQL)

### 1 — Start shared infrastructure (once per VM)

```bash
# From the repo root
docker compose -f infra/docker-compose.yml up -d
```

This creates a `postgres` container on the `infra` Docker network and runs
`infra/initdb/01_urlshortener.sql` on first start to provision the `urlshortener`
database and `url` user.

### 2 — Start this project

```bash
cd projects/02-url-shortener
docker compose up -d
```

Open the demo UI at **http://localhost:8081**

### 3 — Override the public base URL (production)

```bash
BASE_URL=https://url.yourdomain.com docker compose up -d
```

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `LISTEN_ADDR` | `:8081` | TCP address for the HTTP server |
| `BASE_URL` | `http://localhost:8081` | Public root used to build short URLs |
| `DATABASE_URL` | `postgres://url:url@postgres:5432/urlshortener?sslmode=disable` | PostgreSQL DSN |

---

## API

### POST /v1/links — Create a short link

```bash
curl -s -X POST http://localhost:8081/v1/links \
  -H "Content-Type: application/json" \
  -H "X-Owner-ID: demo" \
  -d '{"long_url": "https://example.com/very/long/path"}'
```

Response:

```json
{
  "code": "aB3kZ9w",
  "short_url": "http://localhost:8081/aB3kZ9w",
  "long_url": "https://example.com/very/long/path",
  "owner_id": "demo",
  "created_at": "2025-01-01T12:00:00Z"
}
```

### GET /{code} — Redirect

```bash
curl -v http://localhost:8081/aB3kZ9w
# → 302 Found, Location: https://example.com/very/long/path
```

### GET /v1/links/{code}/stats — Click analytics

```bash
curl -s http://localhost:8081/v1/links/aB3kZ9w/stats \
  -H "X-Owner-ID: demo"
```

### GET /healthz — Health check

```bash
curl http://localhost:8081/healthz
# → {"status":"ok"}
```

---

## Pre-seeded Owners

| Owner ID | Quota | Plan |
|---|---|---|
| `demo` | 100 | demo |
| `marketing` | 1000 | team |
| `creator` | 250 | creator |

---

## Running Tests

```bash
cd projects/02-url-shortener
/usr/local/go/bin/go test ./...
```

All tests use fake in-memory stores — no database required.

---

## Load Testing

```bash
# Default: 10 concurrent workers, 500 requests
./scripts/load_test.sh

# Custom: 20 workers, 1000 requests
./scripts/load_test.sh 20 1000
```

---

## Monitoring

| Service | URL | Credentials |
|---|---|---|
| Prometheus | http://localhost:9091 | — |
| Grafana | http://localhost:3001 | admin / admin |

The Grafana dashboard auto-provisions with golden signals: request rate, cache hit ratio,
redirect p95 latency, and links created per owner.

---

## Deployment on Oracle Cloud ARM

This project is part of a 50-project system design series, all deployed on a single
Oracle Cloud free-tier ARM (aarch64) VM using a shared PostgreSQL instance.

```
VM
├── Caddy                  ← TLS termination, subdomain routing
├── infra/                 ← Shared postgres (docker compose, one-time)
└── projects/
    └── 02-url-shortener/  ← This project (docker compose)
```

The Docker image is built with `CGO_ENABLED=0 GOOS=linux` — runs on both x86-64
and ARM64 without modification.

# 03 — Pastebin

Store and share text snippets with optional expiration, visibility controls, and burn-after-read semantics. Content lives in object storage (MinIO); metadata in PostgreSQL; rate limiting via Redis.

## Stack

| Component | Technology |
|-----------|-----------|
| API server | Go 1.26, gorilla/mux |
| Metadata | PostgreSQL 16 (shared infra) |
| Content | MinIO (S3-compatible) |
| Rate limiting | Redis 8.8 |
| Observability | Prometheus + Grafana (shared infra) |

## Quick start

**1. Start shared infra (once, from repo root):**

```bash
docker compose -f infra/docker-compose.yml up -d
```

**2. Provision the database (one-time):**

```bash
docker exec -it infra-postgres-1 psql -U admin \
  -c "CREATE DATABASE pastebin;" \
  -c "CREATE USER paste WITH PASSWORD 'paste';" \
  -c "GRANT ALL PRIVILEGES ON DATABASE pastebin TO paste;"
```

**3. Start the pastebin stack:**

```bash
cd projects/03-pastebin
docker compose up -d --build
```

**4. Run migrations:**

```bash
docker compose exec server wget -qO- http://localhost:8082/healthz   # confirm it's up
psql postgres://paste:paste@localhost:5432/pastebin -f scripts/migrate.sql
```

**5. Seed sample data:**

```bash
./scripts/seed.sh
```

**6. Open dashboards:**

- API: http://localhost:8082
- MinIO console: http://localhost:9003 (user: `minioadmin`, pass: `minioadmin`)
- Grafana: http://localhost:3000 → "Pastebin — Golden Signals"

## API

### Create a paste

```bash
curl -X POST http://localhost:8082/v1/pastes \
  -H "Content-Type: application/json" \
  -d '{
    "title": "My snippet",
    "language": "go",
    "visibility": "public",
    "content": "package main\n\nfunc main() {}",
    "ttl_seconds": 3600
  }'
```

### Get a paste (JSON)

```bash
curl http://localhost:8082/v1/pastes/{id}
```

### Get raw content

```bash
curl http://localhost:8082/v1/pastes/{id}/raw
```

### Delete a paste

```bash
curl -X DELETE http://localhost:8082/v1/pastes/{id} \
  -H "X-Owner-ID: your-user-id"
```

### Visibility modes

| Value | Behaviour |
|-------|-----------|
| `public` | Listed and cached; anyone can read |
| `unlisted` | Not listed; readable by anyone with the link |
| `private` | **Not available yet** — requires authentication (returns 501) |

> **Auth note:** Private pastes and owner-scoped deletes are intentionally disabled until a verified identity mechanism (signed JWT or session) is implemented. Accepting a client-supplied `X-Owner-ID` header would allow any caller to impersonate any user.

### Burn-after-read

Set `"burn_after_read": true` to have the paste deleted immediately after the first successful read.

### TTL

Set `"ttl_seconds"` to an integer number of seconds. The background sweeper removes expired pastes every minute.

## Design decisions

**Why object storage for content?**  
Paste bodies vary from bytes to megabytes. Storing blobs in PostgreSQL creates table bloat and slows vacuuming. Object storage scales cheaply, streams efficiently, and keeps the database focused on indexed metadata.

**Why Redis only for rate limiting?**  
In-process caching breaks with multiple API replicas. Redis rate limit counters are intentionally ephemeral (a restart just resets windows — acceptable). Paste metadata caching uses a short in-memory path inside the service to avoid the network hop on every read, with cache promotion after the first DB hit.

**Compensating delete on partial write failure**  
`Create` uploads the blob first, then writes metadata. If the metadata write fails, it issues a best-effort blob delete. This keeps the object store free of orphaned objects without needing a distributed transaction.

**Expiry sweep vs. lazy deletion**  
Lazy deletion (check on read) is cheap but leaves data on disk indefinitely. The sweeper runs every minute and deletes up to 500 expired pastes per tick, bounding storage growth without hammering the DB.

## Load test

```bash
# Install hey: go install github.com/rakyll/hey@latest
./scripts/load_test.sh
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8082` | HTTP listen address |
| `DATABASE_URL` | `postgres://paste:paste@postgres:5432/pastebin?sslmode=disable` | PostgreSQL DSN |
| `REDIS_ADDR` | `redis:6379` | Redis address |
| `MINIO_ENDPOINT` | `minio:9000` | MinIO endpoint |
| `MINIO_ACCESS_KEY` | `minioadmin` | MinIO access key |
| `MINIO_SECRET_KEY` | `minioadmin` | MinIO secret key |
| `MINIO_USE_SSL` | `false` | Enable TLS for MinIO |
| `SWEEP_INTERVAL` | `1m` | How often the expiry sweeper runs |

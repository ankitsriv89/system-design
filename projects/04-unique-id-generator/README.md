# 04 — Unique ID Generator

A Snowflake-style 64-bit unique ID generator with PostgreSQL-backed worker leases, Prometheus metrics, and a REST HTTP API.

## Bit layout

```
 0          1                          41  42          51  52         63
 ┌──────────┬───────────────────────────┬──┬────────────┬──┬───────────┐
 │  unused  │  timestamp (41 bits)      │  │ worker_id  │  │ sequence  │
 │  1 bit   │  ms since 2020-01-01      │  │  10 bits   │  │  12 bits  │
 └──────────┴───────────────────────────┴──┴────────────┴──┴───────────┘
```

- **timestamp** — 41 bits → ~69 years from 2020-01-01
- **worker_id** — 10 bits → 1024 workers max
- **sequence** — 12 bits → 4096 IDs per millisecond per worker
- **max throughput** — 4096 × 1024 workers = ~4.2 million IDs/ms globally

## Quick start

```bash
# 1. Start shared infra (Postgres, Prometheus, Grafana)
docker compose -f ../../infra/docker-compose.yml up -d

# 2. Create the database and user
docker exec -it infra-postgres-1 psql -U admin -c "CREATE DATABASE uniqueid;"
docker exec -it infra-postgres-1 psql -U admin -c "CREATE USER uniqueid WITH PASSWORD 'uniqueid';"
docker exec -it infra-postgres-1 psql -U admin -c "GRANT ALL PRIVILEGES ON DATABASE uniqueid TO uniqueid;"

# 3. Run migrations
cat scripts/migrate.sql | docker exec -i infra-postgres-1 psql -U uniqueid -d uniqueid

# 4. Start the service
docker compose up -d

# 5. Verify
./scripts/seed.sh
```

## API

### Generate one ID
```
POST /v1/ids/next
```
```json
{
  "id": 7277946587078656,
  "id_string": "7277946587078656",
  "worker_id": 0,
  "region": "local"
}
```
> `id_string` is provided because JavaScript `JSON.parse` loses precision on integers > 2^53.

### Generate a batch
```
POST /v1/ids/batch
Content-Type: application/json

{"count": 100}
```
```json
{
  "ids": [7277946587078656, 7277946587078657, ...],
  "id_strings": ["7277946587078656", ...],
  "count": 100,
  "worker_id": 0
}
```
Max `count` is 1000 per request.

### Inspect / decompose an ID
```
GET /v1/ids/7277946587078656/inspect
```
```json
{
  "id": "7277946587078656",
  "timestamp_ms": 1748800000000,
  "time": "2025-06-01T12:00:00Z",
  "worker_id": 0,
  "sequence": 0
}
```

### Worker health
```
GET /v1/workers/health
```

### Liveness
```
GET /healthz
```

## Running unit tests

```bash
go test ./generator/...
```

## Running integration tests (requires live service)

```bash
docker compose up -d
./scripts/integration_test.sh
```

## Load test

```bash
# concurrency=20, 2000 requests
./scripts/load_test.sh http://localhost:8083 20 2000
```

## Design decisions

### Why not UUIDs?
UUIDs (v4) are random — they cause B-tree index fragmentation on insert and carry no timestamp, making time-range queries impossible. Snowflake IDs are roughly time-ordered and insert in append order.

### Why not a database sequence?
A Postgres `SERIAL` or `BIGSERIAL` column requires a round-trip to the database for every ID. At high throughput this becomes the bottleneck. Snowflake IDs are generated entirely in memory with no database round-trip on the hot path.

### Clock rollback handling
If the system clock moves backward, `generator.Next()` spins (sleeps 1 ms at a time) until wall time catches back up. This prevents duplicate IDs at the cost of a brief stall. The incident is recorded in `clock_incidents` and surfaced as a Prometheus counter (`uniqueid_clock_rollback_total`).

### Worker lease design
- 1024 rows pre-seeded in `worker_leases` — no inserts on the hot path.
- `SELECT ... FOR UPDATE SKIP LOCKED` prevents two instances racing to claim the same ID.
- Leases auto-expire after 30 s; a crashed process loses its slot without manual intervention.
- The background renewer fires every 10 s (TTL/3), giving 2 missed heartbeats before expiry.

### Scaling beyond one worker
Run more instances — each claims a distinct `worker_id`. Up to 1024 workers can run simultaneously without coordination. Beyond 1024, the bit layout needs widening (worker_id or timestamp bits).

## Failure modes

| Failure | Behaviour |
|---|---|
| Clock moves backward | Spins ≤ drift ms, records incident |
| Sequence exhausted (>4096 IDs/ms) | Spins ≤ 1 ms to next millisecond |
| Lease renewal fails | Logs error; TTL gives 20 s of runway before expiry |
| Lease expires before renewal | Log error; process should restart and re-acquire |
| All 1024 worker slots occupied | Startup fails with a clear error message |
| PostgreSQL unavailable at startup | Retries for 60 s, then fatal |

## Metrics

| Metric | Meaning |
|---|---|
| `uniqueid_ids_generated_total` | Throughput counter |
| `uniqueid_generation_duration_seconds` | Latency histogram (p99 should be < 1 ms) |
| `uniqueid_clock_rollback_total` | Alert if non-zero |
| `uniqueid_sequence_exhaustions_total` | Alert if rising (approaching throughput ceiling) |
| `uniqueid_lease_renewals_total` | Should increment every 10 s |
| `uniqueid_lease_failures_total` | Alert if non-zero |

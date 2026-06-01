# Changelog — Unique ID Generator

All notable changes to this project are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [0.2.0] — 2026-06-01

### Changed — memory and performance optimisations for shared-host deployment

**`generator` package**
- `Batch(n)` now acquires the mutex **once** for the entire batch instead of once per ID. For a batch of 1000, this eliminates 999 unnecessary lock/unlock round-trips through the OS scheduler.
- Extracted lock-free `next()` inner method; public `Next()` acquires the mutex and delegates to it; `Batch()` does the same across the whole loop.
- `Generator` struct fields reordered and padded to 64 bytes (one CPU cache line). Immutable fields (`workerID`, `onIncident`) are grouped before a `[32]byte` pad; hot mutable fields (`lastMs`, `sequence`) land on their own cache line. This prevents false sharing when multiple services allocate generators adjacently in memory on the same ARM instance.

**`api` package**
- Added a `sync.Pool` of `*bytes.Buffer` (`bufPool`) reused across all JSON serialisation calls. Each request borrows a buffer, encodes into it, writes to the `ResponseWriter`, then returns it — eliminating the per-request heap allocation previously caused by `json.NewEncoder(w)`.
- `GET /healthz` now writes a pre-built `[]byte` (`healthzBody`) with a direct `w.Write` call. The previous implementation allocated a `map[string]string{"status":"ok"}` on every Docker health-check probe (every 10 s × N co-hosted services).
- `Handler.workerIDStr` is pre-rendered once in `New()` instead of calling `strconv.FormatInt` on every `/v1/ids/next` response.

**`lease` package**
- `MaxOpenConns` reduced from 4 → 2 (only two concurrent DB calls are ever possible: renew ticker + one ad-hoc call).
- `MaxIdleConns` raised to match `MaxOpenConns` (2) so the pool never destroys and recreates connections between the 10-second renew ticks — avoids reconnect overhead and saves file descriptors on a shared host.
- Added `SetConnMaxIdleTime(5m)` to reclaim idle connections if the service is quiet for an extended period.

---

## [0.1.0] — 2026-06-01

### Added

**Core generator**
- Snowflake-style 64-bit ID generation: 41-bit timestamp + 10-bit worker_id + 12-bit sequence.
- Custom epoch anchored at 2020-01-01T00:00:00Z for ~69 years of range.
- Mutex-based concurrent safety — multiple goroutines share one generator safely.
- Clock rollback detection: spins until wall time recovers; fires configurable incident hook.
- Sequence exhaustion handling: spins < 1 ms to the next millisecond when all 4096 slots are used.
- `Decompose()` function to reverse-engineer any Snowflake ID into its fields.

**Worker lease (PostgreSQL)**
- `worker_leases` table with 1024 pre-seeded rows (one per valid worker_id 0–1023).
- `SELECT FOR UPDATE SKIP LOCKED` claim — prevents two instances taking the same worker_id.
- 30-second TTL with automatic background renewal every 10 seconds.
- Graceful `Release()` on shutdown to free the slot immediately.
- `clock_incidents` audit table — records every backward clock drift event with magnitude.

**HTTP API**
- `POST /v1/ids/next` — generate one ID; returns `id` (int64) and `id_string` (string) for JS safety.
- `POST /v1/ids/batch` — generate up to 1000 IDs in one call.
- `GET /v1/ids/{id}/inspect` — decompose any Snowflake ID into timestamp, worker_id, sequence.
- `GET /v1/workers/health` — reports current worker_id, region, and status.
- `GET /healthz` — liveness probe.
- `GET /metrics` — Prometheus exposition.

**Observability**
- `uniqueid_ids_generated_total` — throughput counter.
- `uniqueid_generation_duration_seconds` — latency histogram (nanosecond-resolution buckets).
- `uniqueid_clock_rollback_total` — alert trigger for NTP issues.
- `uniqueid_clock_drift_ms` — magnitude histogram for clock incidents.
- `uniqueid_sequence_exhaustions_total` — throughput ceiling early-warning.
- `uniqueid_lease_renewals_total` / `uniqueid_lease_failures_total` — lease health.
- Per-route HTTP request count and latency metrics.

**Infrastructure**
- Multi-stage `Dockerfile` (golang:1.23-alpine builder → alpine:3.21 runtime).
- `docker-compose.yml` connected to shared `infra` Docker network.
- `scripts/migrate.sql` — idempotent schema creation and 1024-row seed.
- `scripts/seed.sh` — quick smoke-test of all API endpoints.
- `scripts/integration_test.sh` — 10 correctness assertions against a live service.
- `scripts/load_test.sh` — throughput baseline using GNU parallel.

**Documentation**
- `README.md` — quick start, API reference, design decisions, failure mode table, metrics guide.
- `docs/architecture.md` — Mermaid system and sequence diagrams.
- `docs/code-flow.md` — function call flowcharts for every major operation.
- `docs/build-log.md` — pinned dependency versions and build output.
- `docs/changelog.md` — this file.
- `CLAUDE.md` at repo root — standing workflow instructions for all 50 projects.

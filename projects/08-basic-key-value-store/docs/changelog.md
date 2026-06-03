# Changelog — Basic Key-Value Store (08)

All notable changes follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] — 2026-06-03

### Added
- WAL (Write-Ahead Log) with CRC32 per entry and fsync-per-write durability.
- WAL atomic truncation after memtable flush (rename via temp file).
- Memtable: in-memory sorted map with RWMutex, tombstone support, and byte-size tracking.
- SSTable writer: immutable sorted file with CRC32 per entry.
- SSTable reader: linear scan with CRC verification.
- Engine: orchestrates WAL + memtable + SSTables; memtable flush at 4 MiB threshold.
- Background compaction loop: triggers when ≥ 4 L0 SSTables accumulate.
- Compaction: merges L0 files into a single L1 file, newest-wins, drops tombstones.
- WAL crash recovery: replays all valid entries on engine open.
- SSTable recovery: discovers existing SSTable files on disk on restart.
- HTTP API: `PUT /v1/kv/{key}`, `GET /v1/kv/{key}`, `DELETE /v1/kv/{key}`.
- Admin API: `POST /v1/admin/compact`, `GET /v1/admin/stats`.
- Value size limit: 1 MiB per key.
- Prometheus metrics: operation duration histogram and counter, labelled by op + result.
- Structured logging with `go.uber.org/zap` throughout.
- Three-panel tutorial web UI with live Canvas LSM-tree visualisation.
- Write particle animations triggered by SET, DELETE, flush, and compaction events.
- Algorithm walkthrough panel explaining each operation step-by-step.
- Load test (200 keys) and failure demo (50 tombstones) controls in the UI.
- `Dockerfile` (multi-stage, alpine runtime, arm64).
- `docker-compose.yml` wired to the shared `infra` external network.
- `scripts/integration_test.sh` covering all API endpoints.
- `scripts/load_test.sh` with sequential write and read benchmarks.
- Unit tests: `TestSetGet`, `TestDelete`, `TestMissingKey`, `TestWALRecovery`,
  `TestFlushAndReadFromSST`, `TestCompaction`, `TestWAL_AppendAndReplay`.
- Benchmarks: `BenchmarkSet`, `BenchmarkGet`.
- Full documentation: `architecture.md`, `code-flow.md`, `build-log.md`, `api.md`.

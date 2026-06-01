# Changelog

All notable changes to this project will be documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)

## [0.1.0] — 2026-06-02

### Added
- Core consistent hash ring with virtual nodes and weighted placement (`ring/ring.go`)
- Binary search key lookup — O(log V) — returning primary owner
- `LookupN` for replication: returns first R distinct physical nodes clockwise
- `Stats()` — arc length fractions and standard deviation per node
- `SimulateKeys(n)` — distributes n synthetic keys for distribution visualisation
- Ring versioning via `atomic.Uint64` — monotonically incremented on every topology change
- In-memory multi-ring store (`store/store.go`)
- HTTP API on `:8084`:
  - `POST /v1/rings` — create ring
  - `DELETE /v1/rings/{ring}` — delete ring
  - `GET /v1/rings` — list all rings
  - `POST /v1/rings/{ring}/nodes` — add node (with weight)
  - `DELETE /v1/rings/{ring}/nodes/{node}` — remove node
  - `GET /v1/rings/{ring}/keys/{key}/owner` — single-owner lookup
  - `GET /v1/rings/{ring}/keys/{key}/replicas?n=3` — replica set lookup
  - `GET /v1/rings/{ring}/stats` — distribution stats
  - `GET /v1/rings/{ring}/simulate?keys=N` — key distribution simulation
  - `GET /v1/rings/{ring}/vnodes` — raw vnode list for visualisation
  - `GET /metrics` — Prometheus metrics
  - `GET /healthz` — liveness probe
- Prometheus metrics: `ring_ops_total`, `lookup_duration_seconds`, `node_count`, `vnode_count`, `ring_stddev`, `key_movement_pct`
- Structured logging with `go.uber.org/zap`
- React 18 + Vite animated tutorial frontend:
  - SVG ring with coloured arc segments, vnode dots, key position indicator
  - 6 tutorial steps: Problem → Ring → Vnodes → Weights → Lookup → Rebalance
  - Live bar charts for key distribution and rebalance stats
  - Presets: empty, small (3 nodes), weighted, large (5 nodes)
- Docker multi-stage build + `docker-compose.yml` on external `infra` network
- Integration test script (`scripts/integration_test.sh`)
- Load test script (`scripts/load_test.sh`)
- Full documentation: architecture, code-flow, build-log, api

### Performance
- Lookup: 1.04 µs / 1M req/s per core (BenchmarkLookup, 10 nodes × 150 vnodes)
- Key movement on 3→4 node scale-out: 21% (theoretical 25%)
- Ring std dev with 200 vnodes/node, 4 nodes: 0.0138

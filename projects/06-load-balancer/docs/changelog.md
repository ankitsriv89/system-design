# Changelog — 06 Load Balancer

All notable changes to this project will be documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] — 2026-06-02

### Added
- Round-robin, least-connections, and weighted-round-robin routing algorithms.
- Active HTTP health checks (10 s interval, 3 s timeout) with per-backend EWMA latency tracking.
- Reverse-proxy data plane with up to 2 retries on 5xx responses.
- Control plane REST API: register/remove backends, change algorithm, query stats and health history.
- PostgreSQL persistence for backend configuration and health event time-series.
- Prometheus metrics: request rate, latency histogram, active connections, health check results, retry counter.
- Three-panel interactive tutorial UI: controls, live Canvas animation of request flow, API log and stats table.
- Canvas animation shows client → LB → backend → LB → client packet flow in real time, reflecting actual API state.
- Failure-injection controls: kill a random backend, revive all, flood with 10 concurrent requests.
- Docker Compose with three hashicorp/http-echo backends for local demo.
- `scripts/seed.sh`, `scripts/load_test.sh`, `scripts/integration_test.sh`.

### Performance
- `backend.ActiveConns` and `backend.TotalConns` updated with `sync/atomic` — no mutex on the hot data-plane path.
- Health event channel is buffered (512) and drop-on-full — health checks never block the data plane.
- Static `healthzBody` pre-allocated as a package-level `[]byte`.

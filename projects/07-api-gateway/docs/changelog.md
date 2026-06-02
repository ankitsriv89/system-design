# Changelog — 07 API Gateway

All notable changes to this project follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] — 2026-06-02

### Added
- **Reverse proxy data plane** (`:8088`): longest-prefix route matching, `httputil.ReverseProxy` forwarding, `X-Request-ID` and `X-Consumer-ID` header injection.
- **Admin control plane** (`:8089`): REST endpoints for CRUD on routes and API keys, quota stats, optional Bearer-token auth middleware.
- **API key authentication**: SHA-256 hashed keys stored in PostgreSQL; `Authorization: Bearer` and `X-API-Key` header support.
- **Scope-based authorization**: per-route `required_scope` checked against key scopes; wildcard `*` scope grants all access.
- **Sliding-window rate limiter**: Redis sorted-set implementation; 4-command pipeline per request (ZREMRANGEBYSCORE, ZCARD, ZADD, EXPIRE); fail-open on Redis error.
- **In-process route table** (`gateway.Router`): copy-on-write atomic reload; immediate consistency on admin upsert; 30-second background periodic refresh.
- **Decision log**: every gateway decision (allowed, blocked_auth, blocked_rate, blocked_scope) written to `gateway_decisions` PostgreSQL table with request ID, route, key, outcome, status code, and latency.
- **Prometheus metrics**: `api_gateway_requests_total`, `api_gateway_request_duration_seconds`, `api_gateway_upstream_errors_total`, `api_gateway_active_routes`, `api_gateway_active_keys`.
- **Web UI** (tutorial + live visualization): three-panel Canvas animation showing request particles traversing client → gateway → Redis → upstream; live route table; load test runner; API output log.
- **Docker Compose**: gateway + three echo backends; attaches to shared `infra` network.
- **Scripts**: `migrate.sql`, `seed.sh`, `load_test.sh`, `integration_test.sh`.
- **Docs**: architecture diagram, sequence diagram, code-flow diagrams, build log, API reference.
- 8 unit tests covering: longest-prefix routing, auth required/missing/valid, scope enforcement, rate limiting, strip-prefix URL building.
- Benchmark: `BenchmarkRouterMatch` for the hot-path route lookup.

### Performance
- `sync.Pool`-backed 32 KiB copy buffer injected into `httputil.ReverseProxy` via `BufferPool` interface — eliminates per-request allocation.
- Static error response bodies pre-built as package-level `[]byte` vars — never allocated in handlers.
- Route match is a read-lock linear scan; no network hop; table fits in CPU cache for typical route counts (<500).

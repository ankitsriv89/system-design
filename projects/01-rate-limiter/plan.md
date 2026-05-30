# 01. Design a Rate Limiter

## Project Brief
Protect APIs from abusive or accidental traffic spikes while preserving fair access for legitimate clients.

Primary users: API platform teams, gateway owners, and product teams exposing public or partner APIs.

Phase: Core Building Blocks (01-13)

Recommended stack: Go, Redis, PostgreSQL, Docker, REST/gRPC, Prometheus

## Why This Stack
- Go keeps the request-path limiter small, fast, and easy to deploy beside gateways or services.
- Redis is the right fit for atomic counters, TTL-backed buckets, and sub-millisecond policy checks.
- PostgreSQL stores durable policies and audit summaries where relational constraints matter more than raw speed.

## Learning Objectives
- token bucket and sliding-window algorithms
- hot-key mitigation
- distributed counters
- policy evaluation at the edge

## Requirements
Functional requirements:
- Provide the core user workflow end to end.
- Include admin or operator controls needed to demonstrate the system.
- Persist enough state to recover from restarts.
- Publish domain events for asynchronous work and observability.

Non-functional requirements:
- Define target p50, p95, and p99 latency for the read and write paths.
- Define availability and durability expectations for the MVP and production design.
- Include rate limits, quotas, or backpressure for overload protection.
- Make data retention, privacy, and audit behavior explicit.

## Scope
MVP:
- Implement the narrow happy path with local Docker Compose dependencies.
- Expose the primary APIs and a minimal CLI or UI only where it proves the system behavior.
- Add metrics, structured logs, and a repeatable seed/load script.

Production version:
- Add multi-node or multi-worker behavior, backpressure, retries, and idempotency.
- Document capacity estimates, SLOs, data ownership, and operational runbooks.
- Harden security, authorization, quotas, and failure recovery where relevant.

Stretch goals:
- Add a realistic dashboard or simulator for demos.
- Run load, soak, and failure-injection tests and record results in the project README.
- Write a design narrative that explains the system from MVP to scale.

## Architecture
Diagram to draw:
- Client or demo UI calls the public API layer.
- API layer validates requests, checks auth or tenant context, and writes to the primary store.
- Async events flow through the message bus to workers, projectors, or processors.
- Cache and read models serve hot read paths.
- Observability pipeline collects logs, metrics, traces, and alerts.

Core components:
- API service for synchronous user and admin workflows.
- Storage layer: Redis for low-latency counters and PostgreSQL for durable policy configuration and audit summaries.
- Worker or stream processor for asynchronous processing and retries.
- Cache/read path: Cache active policies in process with short TTL and version invalidation.
- Monitoring stack with Prometheus metrics, Grafana dashboards, structured logs, and OpenTelemetry traces.

## APIs, Events, and Data Model
Core APIs:
- POST /v1/limits/check
- PUT /v1/policies/{policy_id}
- GET /v1/usage/{subject}

Core data model:
- Policy(id, subject_type, algorithm, capacity, refill_rate, window, action)
- UsageCounter(subject, policy_id, bucket, count, expires_at)
- AuditDecision(subject, allowed, reason, retry_after_ms)

Events:
- rate_limit.exceeded
- policy.updated
- counter.compacted

## Design Decisions
Storage: Redis for low-latency counters and PostgreSQL for durable policy configuration and audit summaries.

Consistency: Favor low-latency approximate enforcement with bounded drift; use Lua scripts or Redis transactions per key.

Caching: Cache active policies in process with short TTL and version invalidation.

Scaling strategy:
- Partition by the natural high-cardinality key for this project.
- Separate control-plane writes from high-throughput data-plane reads where applicable.
- Add horizontal workers for async processing before scaling the synchronous API tier.
- Use load tests to identify the first bottleneck before introducing more infrastructure.

Failure modes to design for:
- Redis unavailable
- policy cache stale
- clock skew between nodes
- single tenant hot key

## Build Milestones
1. Single-node token bucket library: acceptance is a demo, test, or metric proving the behavior works.
2. Redis-backed distributed limiter: acceptance is a demo, test, or metric proving the behavior works.
3. policy API and admin CLI: acceptance is a demo, test, or metric proving the behavior works.
4. load-test report with fairness metrics: acceptance is a demo, test, or metric proving the behavior works.

## Testing and Verification
Unit tests:
- Validate domain rules, state transitions, serialization, and edge cases.

Integration tests:
- Run API plus storage plus cache/message bus in Docker Compose.
- Verify idempotency, retries, and recovery after process restart.

Load tests:
- Establish baseline throughput and latency.
- Include one hotspot scenario and one steady-state scenario.

Failure tests:
- Kill a worker during processing.
- Restart the cache or database dependency.
- Inject duplicate events or requests.
- Verify alerts fire for error rate, latency, saturation, and lag.

## Observability
Logs:
- Emit structured logs with request ID, actor, resource ID, and decision reason.

Metrics:
- Request rate, error rate, latency, saturation, queue lag, cache hit rate, and storage latency.

Traces:
- Trace the synchronous API path and at least one async workflow end to end.

Dashboards and alerts:
- Golden signals dashboard.
- Dependency health dashboard.
- Alert on SLO burn, queue lag, failed retries, and elevated p99 latency.

## Design Talking Points
- Explain fixed window vs sliding window vs token bucket.
- Defend fail-open vs fail-closed behavior.
- Discuss sharding counters and handling celebrity tenants.
- Start from the MVP, then explain which bottleneck forces each production design change.
- Call out what is strongly consistent, what is eventually consistent, and why.
- Estimate rough scale: users, requests per second, storage growth, and hot-key behavior.

## Done Criteria
- A new engineer can run the MVP locally from the README.
- APIs and data model are documented with examples.
- Load-test numbers and known bottlenecks are recorded.
- At least one failure drill is documented with expected and actual behavior.
- The design narrative covers the system end to end with tradeoffs clearly stated.

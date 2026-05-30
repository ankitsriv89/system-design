# 09. Design a Caching System

## Project Brief
Implement a cache service with eviction, TTL, metrics, and predictable behavior under memory pressure.

Primary users: Backend services needing low-latency reads and reduced database load.

Phase: Core Building Blocks (01-13)

Recommended stack: Go, Redis-compatible protocol subset, Docker, Prometheus

## Why This Stack
- Go works well for an in-memory server that needs predictable latency and explicit memory accounting.
- A Redis-compatible subset makes the project practical to test with familiar clients and benchmarks.
- Prometheus metrics are central because hit rate, eviction rate, and memory pressure define cache quality.

## Learning Objectives
- LRU and LFU eviction
- TTL expiration
- cache-aside patterns
- stampede protection

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
- Storage layer: In-memory indexed map plus eviction list; optional AOF snapshot for warm restart.
- Worker or stream processor for asynchronous processing and retries.
- Cache/read path: This is the cache; include singleflight request coalescing for missed hot keys.
- Monitoring stack with Prometheus metrics, Grafana dashboards, structured logs, and OpenTelemetry traces.

## APIs, Events, and Data Model
Core APIs:
- PUT /v1/cache/{key}
- GET /v1/cache/{key}
- DELETE /v1/cache/{key}
- GET /v1/cache/stats

Core data model:
- CacheEntry(key, value, size_bytes, ttl, last_accessed)
- EvictionRecord(key, reason, size_bytes)
- CacheStats(hit_rate, evictions, memory_used)

Events:
- cache.hit
- cache.miss
- cache.evicted

## Design Decisions
Storage: In-memory indexed map plus eviction list; optional AOF snapshot for warm restart.

Consistency: Cache is explicitly non-authoritative; correctness comes from TTL, invalidation, and source-of-truth fallback.

Caching: This is the cache; include singleflight request coalescing for missed hot keys.

Scaling strategy:
- Partition by the natural high-cardinality key for this project.
- Separate control-plane writes from high-throughput data-plane reads where applicable.
- Add horizontal workers for async processing before scaling the synchronous API tier.
- Use load tests to identify the first bottleneck before introducing more infrastructure.

Failure modes to design for:
- cache stampede
- memory exhaustion
- stale data after source update
- large object evicts many small hot keys

## Build Milestones
1. LRU cache library: acceptance is a demo, test, or metric proving the behavior works.
2. HTTP cache service: acceptance is a demo, test, or metric proving the behavior works.
3. TTL and eviction metrics: acceptance is a demo, test, or metric proving the behavior works.
4. stampede demo with load test: acceptance is a demo, test, or metric proving the behavior works.

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
- Explain cache-aside vs write-through vs write-back.
- Discuss TTL and invalidation tradeoffs.
- Estimate hit-rate impact and memory sizing.
- Start from the MVP, then explain which bottleneck forces each production design change.
- Call out what is strongly consistent, what is eventually consistent, and why.
- Estimate rough scale: users, requests per second, storage growth, and hot-key behavior.

## Done Criteria
- A new engineer can run the MVP locally from the README.
- APIs and data model are documented with examples.
- Load-test numbers and known bottlenecks are recorded.
- At least one failure drill is documented with expected and actual behavior.
- The design narrative covers the system end to end with tradeoffs clearly stated.

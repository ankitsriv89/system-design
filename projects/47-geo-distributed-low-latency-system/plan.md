# 47. Design a Geo-Distributed Low-Latency System

## Project Brief
Serve users from multiple regions while minimizing latency and surviving regional failure.

Primary users: Global SaaS, social, commerce, and collaboration platforms.

Phase: Senior/Staff-Level Scale (47-50)

Recommended stack: Go/Java, Kubernetes, Terraform, Redis, PostgreSQL/CockroachDB notes, service mesh

## Why This Stack
- Go or Java services make the architecture realistic while keeping the focus on regional routing and resilience.
- Kubernetes, Terraform, and service mesh concepts are essential because deployment topology is part of the system design.
- CockroachDB/PostgreSQL notes help compare global SQL, home-region writes, and async replication tradeoffs.

## Learning Objectives
- regional routing
- data locality
- active-active tradeoffs
- failover

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
- Storage layer: Regional caches and services, globally replicated metadata store, asynchronous replication for non-critical data.
- Worker or stream processor for asynchronous processing and retries.
- Cache/read path: Edge and regional caches with locality-aware invalidation.
- Monitoring stack with Prometheus metrics, Grafana dashboards, structured logs, and OpenTelemetry traces.

## APIs, Events, and Data Model
Core APIs:
- GET /v1/edge/profile
- POST /v1/write
- GET /v1/health/regions

Core data model:
- Region(id, status, latency)
- UserHomeRegion(user_id, region)
- ReplicationLag(source, target, ms)

Events:
- region.degraded
- write.replicated
- traffic.shifted

## Design Decisions
Storage: Regional caches and services, globally replicated metadata store, asynchronous replication for non-critical data.

Consistency: Classify data by consistency need: local reads, home-region writes, or global consensus for critical records.

Caching: Edge and regional caches with locality-aware invalidation.

Scaling strategy:
- Partition by the natural high-cardinality key for this project.
- Separate control-plane writes from high-throughput data-plane reads where applicable.
- Add horizontal workers for async processing before scaling the synchronous API tier.
- Use load tests to identify the first bottleneck before introducing more infrastructure.

Failure modes to design for:
- region outage
- replication lag
- misrouted write
- data residency conflict

## Build Milestones
1. Single-region app: acceptance is a demo, test, or metric proving the behavior works.
2. multi-region read routing: acceptance is a demo, test, or metric proving the behavior works.
3. home-region writes: acceptance is a demo, test, or metric proving the behavior works.
4. failover drill plan: acceptance is a demo, test, or metric proving the behavior works.

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
- Explain active-active vs active-passive.
- Discuss data locality and residency.
- Handle failover without data loss.
- Start from the MVP, then explain which bottleneck forces each production design change.
- Call out what is strongly consistent, what is eventually consistent, and why.
- Estimate rough scale: users, requests per second, storage growth, and hot-key behavior.

## Done Criteria
- A new engineer can run the MVP locally from the README.
- APIs and data model are documented with examples.
- Load-test numbers and known bottlenecks are recorded.
- At least one failure drill is documented with expected and actual behavior.
- The design narrative covers the system end to end with tradeoffs clearly stated.

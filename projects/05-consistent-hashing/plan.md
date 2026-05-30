# 05. Design a Consistent Hashing

## Project Brief
Distribute keys across changing node sets while minimizing key movement during scale events.

Primary users: Infrastructure engineers designing caches, storage rings, and shard routers.

Phase: Core Building Blocks (01-13)

Recommended stack: Go, CLI visualizer, Docker, optional React ring explorer

## Why This Stack
- Go is ideal for a reusable hashing library and CLI simulator because the core algorithm is small and performance-sensitive.
- A visualizer helps show key movement, virtual nodes, and weighting, which are the main learning goals.
- Persistence is intentionally light because the project is about placement behavior, not durable storage.

## Learning Objectives
- hash rings
- virtual nodes
- weighted placement
- rebalance analysis

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
- Storage layer: In-memory ring for MVP; persisted ring snapshots in PostgreSQL or local files for replay.
- Worker or stream processor for asynchronous processing and retries.
- Cache/read path: Clients cache ring snapshots with version checks and graceful rollover.
- Monitoring stack with Prometheus metrics, Grafana dashboards, structured logs, and OpenTelemetry traces.

## APIs, Events, and Data Model
Core APIs:
- POST /v1/rings
- POST /v1/rings/{ring}/nodes
- GET /v1/rings/{ring}/keys/{key}/owner

Core data model:
- Ring(id, hash_fn, replicas)
- Node(id, weight, status)
- Assignment(key_hash, primary_node, replica_nodes)

Events:
- node.added
- node.removed
- ring.rebalanced

## Design Decisions
Storage: In-memory ring for MVP; persisted ring snapshots in PostgreSQL or local files for replay.

Consistency: Ring versioning ensures clients route with a coherent snapshot during changes.

Caching: Clients cache ring snapshots with version checks and graceful rollover.

Scaling strategy:
- Partition by the natural high-cardinality key for this project.
- Separate control-plane writes from high-throughput data-plane reads where applicable.
- Add horizontal workers for async processing before scaling the synchronous API tier.
- Use load tests to identify the first bottleneck before introducing more infrastructure.

Failure modes to design for:
- uneven key distribution
- node churn
- ring version mismatch
- hot shard despite balanced hashing

## Build Milestones
1. Hash-ring library: acceptance is a demo, test, or metric proving the behavior works.
2. virtual node weighting: acceptance is a demo, test, or metric proving the behavior works.
3. rebalance simulator: acceptance is a demo, test, or metric proving the behavior works.
4. visual report of key movement: acceptance is a demo, test, or metric proving the behavior works.

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
- Explain why modulo hashing causes massive movement.
- Discuss virtual nodes and weighted capacity.
- Relate the ring to distributed caches and databases.
- Start from the MVP, then explain which bottleneck forces each production design change.
- Call out what is strongly consistent, what is eventually consistent, and why.
- Estimate rough scale: users, requests per second, storage growth, and hot-key behavior.

## Done Criteria
- A new engineer can run the MVP locally from the README.
- APIs and data model are documented with examples.
- Load-test numbers and known bottlenecks are recorded.
- At least one failure drill is documented with expected and actual behavior.
- The design narrative covers the system end to end with tradeoffs clearly stated.

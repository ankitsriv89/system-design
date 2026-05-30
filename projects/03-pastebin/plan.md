# 03. Design a Pastebin

## Project Brief
Store text snippets with optional expiration, privacy controls, syntax metadata, and high-read sharing links.

Primary users: Developers, support engineers, and teams sharing logs or code snippets.

Phase: Core Building Blocks (01-13)

Recommended stack: Go, PostgreSQL, S3-compatible object storage, Redis, REST

## Why This Stack
- Object storage is best for paste bodies because content size varies and blobs should not pressure relational tables.
- PostgreSQL fits metadata, visibility, expiration, and abuse workflows that need indexes and transactions.
- Go gives a compact API and background cleanup service with straightforward streaming upload/download support.

## Learning Objectives
- blob storage boundaries
- metadata indexing
- expiration jobs
- abuse controls

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
- Storage layer: Metadata in PostgreSQL, content in object storage, Redis for hot metadata and rate limits.
- Worker or stream processor for asynchronous processing and retries.
- Cache/read path: Cache public paste metadata and content for short TTL, bypass for private or recently deleted items.
- Monitoring stack with Prometheus metrics, Grafana dashboards, structured logs, and OpenTelemetry traces.

## APIs, Events, and Data Model
Core APIs:
- POST /v1/pastes
- GET /v1/pastes/{id}
- DELETE /v1/pastes/{id}

Core data model:
- Paste(id, owner_id, visibility, language, size_bytes, object_key, expires_at)
- PasteVersion(paste_id, version, object_key)
- AbuseReport(paste_id, reason, status)

Events:
- paste.created
- paste.expired
- paste.reported

## Design Decisions
Storage: Metadata in PostgreSQL, content in object storage, Redis for hot metadata and rate limits.

Consistency: Metadata and object writes need a compensating cleanup path when one succeeds and the other fails.

Caching: Cache public paste metadata and content for short TTL, bypass for private or recently deleted items.

Scaling strategy:
- Partition by the natural high-cardinality key for this project.
- Separate control-plane writes from high-throughput data-plane reads where applicable.
- Add horizontal workers for async processing before scaling the synchronous API tier.
- Use load tests to identify the first bottleneck before introducing more infrastructure.

Failure modes to design for:
- object write succeeds but metadata write fails
- large paste abuse
- expired paste still cached
- read surge on shared paste

## Build Milestones
1. CRUD for public pastes: acceptance is a demo, test, or metric proving the behavior works.
2. private and expiring pastes: acceptance is a demo, test, or metric proving the behavior works.
3. object storage integration: acceptance is a demo, test, or metric proving the behavior works.
4. moderation and abuse workflow: acceptance is a demo, test, or metric proving the behavior works.

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
- Explain why content should not live directly in SQL for large pastes.
- Discuss retention and deletion semantics.
- Handle read-after-delete and cache invalidation.
- Start from the MVP, then explain which bottleneck forces each production design change.
- Call out what is strongly consistent, what is eventually consistent, and why.
- Estimate rough scale: users, requests per second, storage growth, and hot-key behavior.

## Done Criteria
- A new engineer can run the MVP locally from the README.
- APIs and data model are documented with examples.
- Load-test numbers and known bottlenecks are recorded.
- At least one failure drill is documented with expected and actual behavior.
- The design narrative covers the system end to end with tradeoffs clearly stated.

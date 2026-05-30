# 22. Design a Ticket Booking System

## Project Brief
Sell limited inventory without overselling while supporting holds, payment windows, and seat maps.

Primary users: Events, cinema, travel, and reservation platforms.

Phase: Real-Time and Product Systems (14-24)

Recommended stack: Java/Spring Boot, PostgreSQL, Redis, Kafka, payment mock

## Why This Stack
- Java/Spring Boot fits transactional booking workflows, validations, and payment integration patterns.
- PostgreSQL provides the strongest local fit for seat inventory constraints and booking records.
- Redis handles short-lived holds efficiently, while Kafka drives expiry and payment saga events.

## Learning Objectives
- inventory locking
- payment saga
- seat map reads
- idempotent checkout

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
- Storage layer: PostgreSQL for inventory and bookings, Redis for short-lived holds, Kafka for expiration/payment workflow.
- Worker or stream processor for asynchronous processing and retries.
- Cache/read path: Cache seat maps but bypass or refresh for selected seats.
- Monitoring stack with Prometheus metrics, Grafana dashboards, structured logs, and OpenTelemetry traces.

## APIs, Events, and Data Model
Core APIs:
- GET /v1/events/{id}/seats
- POST /v1/holds
- POST /v1/bookings

Core data model:
- Seat(event_id, seat_id, status)
- Hold(id, seat_id, user_id, expires_at)
- Booking(id, hold_id, payment_status)

Events:
- hold.created
- hold.expired
- booking.confirmed
- payment.failed

## Design Decisions
Storage: PostgreSQL for inventory and bookings, Redis for short-lived holds, Kafka for expiration/payment workflow.

Consistency: Prevent oversell with transactional seat state changes or atomic Redis reservation backed by reconciliation.

Caching: Cache seat maps but bypass or refresh for selected seats.

Scaling strategy:
- Partition by the natural high-cardinality key for this project.
- Separate control-plane writes from high-throughput data-plane reads where applicable.
- Add horizontal workers for async processing before scaling the synchronous API tier.
- Use load tests to identify the first bottleneck before introducing more infrastructure.

Failure modes to design for:
- payment timeout
- hold expiry race
- oversell
- flash-sale traffic

## Build Milestones
1. Event and seat APIs: acceptance is a demo, test, or metric proving the behavior works.
2. hold and expiry worker: acceptance is a demo, test, or metric proving the behavior works.
3. booking/payment saga: acceptance is a demo, test, or metric proving the behavior works.
4. flash-sale load test: acceptance is a demo, test, or metric proving the behavior works.

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
- Explain locking choices.
- Discuss sagas and compensation.
- Handle high concurrency on popular events.
- Start from the MVP, then explain which bottleneck forces each production design change.
- Call out what is strongly consistent, what is eventually consistent, and why.
- Estimate rough scale: users, requests per second, storage growth, and hot-key behavior.

## Done Criteria
- A new engineer can run the MVP locally from the README.
- APIs and data model are documented with examples.
- Load-test numbers and known bottlenecks are recorded.
- At least one failure drill is documented with expected and actual behavior.
- The design narrative covers the system end to end with tradeoffs clearly stated.

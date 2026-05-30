# 37. Design a Ad Serving and Tracking System

## Project Brief
Select ads under targeting, pacing, budget, and latency constraints while tracking impressions and clicks.

Primary users: Ad tech platforms and marketplace monetization systems.

Phase: Advanced Data and Platform Systems (36-46)

Recommended stack: Java/Spring Boot, Kafka, Redis, PostgreSQL, ClickHouse

## Why This Stack
- Java/Spring Boot works well for campaign management and low-latency serving APIs with mature operational patterns.
- Redis keeps targeting and budget state hot enough for ad-decision latency constraints.
- ClickHouse is suited for impression/click analytics, while Kafka absorbs tracking volume.

## Learning Objectives
- auction/ranking
- budget pacing
- tracking attribution
- low-latency serving

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
- Storage layer: Redis for hot campaign/budget state, PostgreSQL for campaign config, ClickHouse for event analytics.
- Worker or stream processor for asynchronous processing and retries.
- Cache/read path: Cache eligible campaigns by segment and placement.
- Monitoring stack with Prometheus metrics, Grafana dashboards, structured logs, and OpenTelemetry traces.

## APIs, Events, and Data Model
Core APIs:
- POST /v1/ad-request
- POST /v1/track/impression
- POST /v1/track/click

Core data model:
- Campaign(id, budget, bid, targeting)
- AdRequest(user_context, placement)
- Impression(id, campaign_id, price)

Events:
- ad.served
- impression.logged
- click.logged
- budget.updated

## Design Decisions
Storage: Redis for hot campaign/budget state, PostgreSQL for campaign config, ClickHouse for event analytics.

Consistency: Budget deductions need strong-enough atomicity; tracking is at-least-once with dedupe.

Caching: Cache eligible campaigns by segment and placement.

Scaling strategy:
- Partition by the natural high-cardinality key for this project.
- Separate control-plane writes from high-throughput data-plane reads where applicable.
- Add horizontal workers for async processing before scaling the synchronous API tier.
- Use load tests to identify the first bottleneck before introducing more infrastructure.

Failure modes to design for:
- overspend
- tracking fraud
- latency SLA breach
- campaign config lag

## Build Milestones
1. Campaign API: acceptance is a demo, test, or metric proving the behavior works.
2. ad selection: acceptance is a demo, test, or metric proving the behavior works.
3. tracking pipeline: acceptance is a demo, test, or metric proving the behavior works.
4. pacing and analytics: acceptance is a demo, test, or metric proving the behavior works.

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
- Explain serving latency budget.
- Discuss pacing and budget correctness.
- Handle click fraud and attribution.
- Start from the MVP, then explain which bottleneck forces each production design change.
- Call out what is strongly consistent, what is eventually consistent, and why.
- Estimate rough scale: users, requests per second, storage growth, and hot-key behavior.

## Done Criteria
- A new engineer can run the MVP locally from the README.
- APIs and data model are documented with examples.
- Load-test numbers and known bottlenecks are recorded.
- At least one failure drill is documented with expected and actual behavior.
- The design narrative covers the system end to end with tradeoffs clearly stated.

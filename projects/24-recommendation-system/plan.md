# 24. Design a Recommendation System

## Project Brief
Generate personalized recommendations using behavior events, ranking features, and online serving.

Primary users: Media, commerce, feed, and learning platforms.

Phase: Real-Time and Product Systems (14-24)

Recommended stack: Python/FastAPI, Kafka, PostgreSQL, Redis, feature store, vector DB optional

## Why This Stack
- Python/FastAPI fits recommendation work because model code, feature jobs, and experimentation usually live in Python.
- Kafka captures behavior events and supports streaming feature updates.
- Redis or a feature store keeps online ranking fast, while PostgreSQL stores item/user metadata.

## Learning Objectives
- candidate generation
- feature pipelines
- online ranking
- feedback loops

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
- Storage layer: PostgreSQL for metadata, Kafka for events, Redis/feature store for online features, object storage for model artifacts.
- Worker or stream processor for asynchronous processing and retries.
- Cache/read path: Cache recommendation lists with short TTL and diversity constraints.
- Monitoring stack with Prometheus metrics, Grafana dashboards, structured logs, and OpenTelemetry traces.

## APIs, Events, and Data Model
Core APIs:
- POST /v1/events
- GET /v1/recommendations/{user_id}
- POST /v1/models/promote

Core data model:
- UserEvent(user_id, item_id, action, ts)
- FeatureVector(entity_id, values, version)
- Recommendation(user_id, item_id, score, reason)

Events:
- user.event_logged
- features.updated
- model.promoted

## Design Decisions
Storage: PostgreSQL for metadata, Kafka for events, Redis/feature store for online features, object storage for model artifacts.

Consistency: Recommendations can be eventually consistent; model promotion must be controlled and reversible.

Caching: Cache recommendation lists with short TTL and diversity constraints.

Scaling strategy:
- Partition by the natural high-cardinality key for this project.
- Separate control-plane writes from high-throughput data-plane reads where applicable.
- Add horizontal workers for async processing before scaling the synchronous API tier.
- Use load tests to identify the first bottleneck before introducing more infrastructure.

Failure modes to design for:
- cold start
- feedback loop
- model regression
- feature skew

## Build Milestones
1. Event ingestion: acceptance is a demo, test, or metric proving the behavior works.
2. baseline popularity recommender: acceptance is a demo, test, or metric proving the behavior works.
3. feature pipeline: acceptance is a demo, test, or metric proving the behavior works.
4. online ranking service: acceptance is a demo, test, or metric proving the behavior works.

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
- Explain candidate generation vs ranking.
- Discuss offline/online feature skew.
- Handle cold start and evaluation.
- Start from the MVP, then explain which bottleneck forces each production design change.
- Call out what is strongly consistent, what is eventually consistent, and why.
- Estimate rough scale: users, requests per second, storage growth, and hot-key behavior.

## Done Criteria
- A new engineer can run the MVP locally from the README.
- APIs and data model are documented with examples.
- Load-test numbers and known bottlenecks are recorded.
- At least one failure drill is documented with expected and actual behavior.
- The design narrative covers the system end to end with tradeoffs clearly stated.

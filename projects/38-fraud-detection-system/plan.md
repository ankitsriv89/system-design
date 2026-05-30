# 38. Design a Fraud Detection System

## Project Brief
Score transactions or actions for fraud using rules, features, and ML models with human review.

Primary users: Payments, marketplaces, fintech, and trust/safety teams.

Phase: Advanced Data and Platform Systems (36-46)

Recommended stack: Python/FastAPI, Kafka, Redis, feature store, PostgreSQL, model service

## Why This Stack
- Python/FastAPI fits rules, model inference, and feature engineering in the same ecosystem.
- Kafka captures risk events for both realtime scoring and offline model improvement.
- Redis or a feature store keeps online fraud features low-latency, while PostgreSQL preserves audit and review cases.

## Learning Objectives
- streaming features
- rules engine
- model inference
- case management

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
- Storage layer: Kafka for events, Redis/feature store for online features, PostgreSQL for cases and decisions.
- Worker or stream processor for asynchronous processing and retries.
- Cache/read path: Cache hot entity features with TTL and version metadata.
- Monitoring stack with Prometheus metrics, Grafana dashboards, structured logs, and OpenTelemetry traces.

## APIs, Events, and Data Model
Core APIs:
- POST /v1/score
- POST /v1/events
- GET /v1/cases/{id}

Core data model:
- RiskEvent(id, actor_id, type, amount)
- Feature(entity_id, name, value)
- Case(id, score, status, reviewer)

Events:
- risk.event_received
- score.generated
- case.opened

## Design Decisions
Storage: Kafka for events, Redis/feature store for online features, PostgreSQL for cases and decisions.

Consistency: Scoring must be synchronous for high-risk actions; feature updates are eventually consistent.

Caching: Cache hot entity features with TTL and version metadata.

Scaling strategy:
- Partition by the natural high-cardinality key for this project.
- Separate control-plane writes from high-throughput data-plane reads where applicable.
- Add horizontal workers for async processing before scaling the synchronous API tier.
- Use load tests to identify the first bottleneck before introducing more infrastructure.

Failure modes to design for:
- feature missing
- model outage
- false positive spike
- fraud pattern shift

## Build Milestones
1. Rule-based scorer: acceptance is a demo, test, or metric proving the behavior works.
2. feature pipeline: acceptance is a demo, test, or metric proving the behavior works.
3. ML model endpoint: acceptance is a demo, test, or metric proving the behavior works.
4. review queue and feedback: acceptance is a demo, test, or metric proving the behavior works.

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
- Explain real-time vs batch fraud detection.
- Discuss precision/recall tradeoffs.
- Handle model fallback and auditability.
- Start from the MVP, then explain which bottleneck forces each production design change.
- Call out what is strongly consistent, what is eventually consistent, and why.
- Estimate rough scale: users, requests per second, storage growth, and hot-key behavior.

## Done Criteria
- A new engineer can run the MVP locally from the README.
- APIs and data model are documented with examples.
- Load-test numbers and known bottlenecks are recorded.
- At least one failure drill is documented with expected and actual behavior.
- The design narrative covers the system end to end with tradeoffs clearly stated.

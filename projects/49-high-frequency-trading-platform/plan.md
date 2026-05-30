# 49. Design a High-Frequency Trading Platform

## Project Brief
Design an ultra-low-latency trading platform with deterministic matching, risk checks, and market data.

Primary users: Capital markets infrastructure and latency-sensitive systems.

Phase: Senior/Staff-Level Scale (47-50)

Recommended stack: Rust or Java, low-latency networking, in-memory matching, append-only journal, market data feed

## Why This Stack
- Rust or carefully tuned Java is suited for latency-sensitive matching where allocation, pauses, and predictability matter.
- An in-memory matching engine and append-only journal are the realistic architecture for deterministic replay.
- A binary protocol and separate market-data stream keep the hot path lean and measurable.

## Learning Objectives
- low-latency path
- deterministic matching
- pre-trade risk
- market data fanout

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
- Storage layer: In-memory hot path with append-only journal; separate read/audit stores fed asynchronously.
- Worker or stream processor for asynchronous processing and retries.
- Cache/read path: Keep hot books and risk limits in memory; avoid request-path distributed cache calls.
- Monitoring stack with Prometheus metrics, Grafana dashboards, structured logs, and OpenTelemetry traces.

## APIs, Events, and Data Model
Core APIs:
- binary submit order protocol
- GET /v1/audit/orders/{id}
- market data stream

Core data model:
- Order(id, account, symbol, side, price, qty)
- RiskLimit(account, max_position)
- Trade(id, symbol, price, qty)

Events:
- order.accepted
- risk.rejected
- trade.executed
- market_data.tick

## Design Decisions
Storage: In-memory hot path with append-only journal; separate read/audit stores fed asynchronously.

Consistency: Single-threaded deterministic matching per symbol; every accepted command is journaled before effects are published.

Caching: Keep hot books and risk limits in memory; avoid request-path distributed cache calls.

Scaling strategy:
- Partition by the natural high-cardinality key for this project.
- Separate control-plane writes from high-throughput data-plane reads where applicable.
- Add horizontal workers for async processing before scaling the synchronous API tier.
- Use load tests to identify the first bottleneck before introducing more infrastructure.

Failure modes to design for:
- GC pause or tail latency spike
- journal disk stall
- bad risk limit
- market data sequencing gap

## Build Milestones
1. Order protocol simulation: acceptance is a demo, test, or metric proving the behavior works.
2. risk checks: acceptance is a demo, test, or metric proving the behavior works.
3. matching engine: acceptance is a demo, test, or metric proving the behavior works.
4. latency benchmark and replay: acceptance is a demo, test, or metric proving the behavior works.

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
- Explain mechanical sympathy and hot path design.
- Discuss deterministic replay.
- Separate HFT from general stock exchange design.
- Start from the MVP, then explain which bottleneck forces each production design change.
- Call out what is strongly consistent, what is eventually consistent, and why.
- Estimate rough scale: users, requests per second, storage growth, and hot-key behavior.

## Done Criteria
- A new engineer can run the MVP locally from the README.
- APIs and data model are documented with examples.
- Load-test numbers and known bottlenecks are recorded.
- At least one failure drill is documented with expected and actual behavior.
- The design narrative covers the system end to end with tradeoffs clearly stated.

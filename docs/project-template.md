# Project Brief Template

Use this when expanding a plan into implementation docs.

## Problem
- What user or business problem does this system solve?
- What scale and workload are assumed?
- What is explicitly out of scope?

## Requirements
Functional:
- Core workflows.
- Admin/operator workflows.
- Events and async workflows.

Non-functional:
- Latency, availability, durability, and consistency targets.
- Security, privacy, compliance, and abuse constraints.
- Cost and operational constraints.

## Architecture
- Public APIs and clients.
- Synchronous services.
- Storage systems.
- Caches and read models.
- Message bus and workers.
- Observability pipeline.

## Why This Stack
- Why the primary language fits the latency, concurrency, and ecosystem requirements.
- Why each data store fits the access pattern and consistency requirement.
- Why each async, cache, search, analytics, or infrastructure tool is worth its operational cost.

## APIs, Events, and Data Model
- REST/gRPC/WebSocket endpoints.
- Event names and payload ownership.
- Tables, indexes, keys, partitions, and retention.
- Idempotency and versioning strategy.

## Design Decisions
- Storage choice and access patterns.
- Caching strategy and invalidation.
- Consistency model.
- Scaling and partitioning strategy.
- Backpressure and overload behavior.
- Failure handling and recovery.

## Build Plan
1. MVP happy path.
2. Durable storage and recovery.
3. Async workflow.
4. Scaling path.
5. Observability and operations.
6. Load and failure testing.

## Testing
- Unit tests for domain rules.
- Integration tests with local dependencies.
- Contract tests for APIs/events.
- Load tests for normal and hotspot traffic.
- Failure-injection tests for dependencies and workers.

## Interview Story
- Start with requirements and assumptions.
- Draw the MVP.
- Identify bottlenecks.
- Add scale changes one by one.
- Explain consistency and failure tradeoffs.
- Close with monitoring, rollout, and future work.

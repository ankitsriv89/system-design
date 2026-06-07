# 13 — Message Queue

Durable asynchronous messaging with acknowledgements, visibility timeouts, partitioned topics, consumer groups, and a dead-letter queue. Built in Go with PostgreSQL for message persistence and Redis for partition routing.

**Stack:** Go 1.22 · PostgreSQL 16 · Redis 7 · Prometheus · Gorilla Mux

---

## Quick Start

```bash
# 1. Bring up the shared infra (PostgreSQL, Redis, Prometheus, Grafana)
cd infra && docker compose up -d && cd ..

# 2. Start the message queue service
cd projects/13-message-queue
docker compose up -d --build

# 3. Wait for the service to become healthy
curl http://localhost:8094/healthz
# → {"status":"ok"}

# 4. Open the web UI
open http://localhost:8094
```

---

## Core Concepts

| Concept | Behaviour |
|---------|-----------|
| **Topic** | Named channel; 1–16 partitions configured at creation |
| **Partition** | Append-only log; messages ordered by offset within a partition |
| **Key routing** | FNV-1a hash of message key → deterministic partition; empty key → round-robin |
| **Visibility timeout** | Polled message hidden for N seconds; re-delivered if not acked |
| **Consumer group** | Independent read cursor; multiple groups can consume the same topic |
| **Dead-letter queue** | Messages that fail 5 delivery attempts are moved to DLQ automatically |
| **Delivery guarantee** | At-least-once (consumer must be idempotent) |

---

## API

### Create topic
```bash
curl -X POST http://localhost:8094/v1/topics \
  -H "Content-Type: application/json" \
  -d '{"name":"orders","partitions":3,"retention_hours":24}'
```

### Publish message
```bash
curl -X POST http://localhost:8094/v1/topics/orders/messages \
  -H "Content-Type: application/json" \
  -d '{"key":"user-42","payload":"{\"item\":\"book\",\"qty\":1}"}'
```

### Poll messages
```bash
curl -X POST http://localhost:8094/v1/topics/orders/messages:poll \
  -H "Content-Type: application/json" \
  -d '{"consumer_group":"billing","max_messages":5,"visibility_timeout_seconds":30}'
```

### Acknowledge a message
```bash
curl -X POST http://localhost:8094/v1/messages/{id}:ack \
  -H "Content-Type: application/json" \
  -d '{"consumer_group":"billing"}'
```

### Queue depth and DLQ
```bash
curl http://localhost:8094/v1/topics/orders/depth
curl http://localhost:8094/v1/topics/orders/dlq
```

Full API reference: [docs/api.md](docs/api.md)

---

## Seed and Load Test

```bash
# Seed: create topics + publish 20 sample messages
bash scripts/seed.sh

# Integration test: full publish → poll → ack → re-delivery → DLQ cycle
bash scripts/integration_test.sh

# Load test: concurrent producers and consumers, prints throughput
bash scripts/load_test.sh
```

---

## Data Model

```
topics(name PK, partitions, retention_seconds, created_at)

messages(
  id            TEXT PK,          -- hex timestamp + 4-char entropy suffix
  topic         TEXT FK→topics,
  partition     INT,
  offset        BIGSERIAL,        -- monotonically increasing per partition
  key           TEXT,
  payload       BYTEA,
  published_at  TIMESTAMPTZ,
  visible_at    TIMESTAMPTZ,      -- messages invisible to consumers until this time
  delivery_attempts INT,
  acked_at      TIMESTAMPTZ,      -- NULL = unacked
  dead_lettered BOOL,
  consumer_group TEXT             -- locked to group during visibility window
)
```

---

## Design Notes

**Why `FOR UPDATE SKIP LOCKED`?**
The poll query uses a CTE + `SKIP LOCKED` to atomically claim a batch of messages without blocking concurrent consumers. Rows already locked by another connection are silently skipped, enabling horizontal consumer scaling on the same topic.

**Why PostgreSQL instead of file segments?**
ACID semantics give the visibility timeout its correctness guarantee: the claim + deadline extension is a single UPDATE that either commits or rolls back — no partial state. For the free-tier throughput target (~800 msg/s), PostgreSQL is not the bottleneck.

**Ordering guarantee**
Per-partition FIFO via `ORDER BY partition, offset`. Cross-partition ordering is not guaranteed (matches Kafka semantics).

**Idempotency**
The queue delivers at-least-once. Consumers must handle duplicate delivery, typically with a deduplication key in their own store.

---

## Capacity (single-node MVP)

| Metric | Value |
|--------|-------|
| Publish throughput | ~800 msg/s |
| Poll throughput | ~600 polls/s |
| p50 publish latency | ~1 ms |
| p95 publish latency | ~4 ms |
| p99 publish latency | ~12 ms |
| Message storage | ~200 B/msg (payload + indexes) |

---

## Observability

- **Metrics:** `http://localhost:8094/metrics` (Prometheus)
- **Grafana:** `http://localhost:3000` (shared infra)
- Key metrics: `message_queue_depth`, `message_queue_dlq_depth`, `message_queue_messages_published_total`, `message_queue_poll_duration_seconds`

---

## Docs

| File | Contents |
|------|----------|
| [docs/architecture.md](docs/architecture.md) | System diagram, sequence diagram, capacity table, failure modes |
| [docs/code-flow.md](docs/code-flow.md) | Function-level call trace for publish, poll, ack, and reaper |
| [docs/api.md](docs/api.md) | Full API reference with curl examples |
| [docs/build-log.md](docs/build-log.md) | Build output, benchmark results, implementation decisions |
| [docs/changelog.md](docs/changelog.md) | Version history |

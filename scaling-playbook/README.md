# Scaling to a Million — A Playbook

How a system grows from **100 → 1,000 → 10,000 → 100,000 → 1,000,000 users**, expressed as
**tech-agnostic patterns** rather than vendor products. Games are the primary lens; the same spine
is applied to four other archetypes so the patterns transfer.

> These are **design documents**, not code. They are the spec a future numbered project
> (`projects/07+`) will be built against. Read them top to bottom like a book.

---

## How to read this

1. **[00-scaling-spine.md](00-scaling-spine.md)** — the backbone. Five tiers, ten cross-cutting
   dimensions, one matrix. *Read this first.* Every archetype is a specialization of it.
2. **[01-games/](01-games/)** — the primary track. The real-time plane (game servers, netcode,
   matchmaking, fleets) is where games stop looking like any other web service.
3. **02–05 archetypes** — [SaaS](02-saas/architecture.md), [social/feed](03-social-feed/architecture.md),
   [real-time chat/collab](04-realtime-chat/architecture.md), [marketplace](05-marketplace/architecture.md).
   Each is the same spine bent around a different dominant workload.
4. **[99-patterns/](99-patterns/)** — pattern cards (caching, sharding, queues, LB, SLOs, cells,
   resilience). Cross-linked from everywhere so a concept is explained once.

---

## Philosophy — seven rules that survive every tier

1. **Stateless compute.** Any request can hit any node. State lives in stores, not in app memory.
   (The one principled exception is a live game match — see games.)
2. **Idempotent writes.** Every mutating operation carries a key so a retry is a no-op, not a
   double-charge. Retries are inevitable at scale; design for them.
3. **Async the slow path.** If the user doesn't need the result in the response, put it on a queue.
4. **Partition before you must — but not before.** Sharding buys capacity and costs you joins,
   transactions, and operational pain. Earn it.
5. **Observe before you scale.** You cannot fix what you cannot see. Metrics/traces/SLOs precede
   the next architecture move, not follow it.
6. **Bulkhead the blast radius.** Past ~100k users, the question is not "will it fail" but "how
   much fails when it does." Cells, AZs, and circuit breakers cap the damage.
7. **Size on the metric that actually drives load — not the headline user count.** For games that
   metric is **peak concurrent users (CCU)**, not registrations. For feeds it's **fan-out
   amplification**. For marketplaces it's **hot-item write contention**. Find yours.

---

## The spine in one screen

| Tier | Users | What just broke | The move |
|---|---|---|---|
| **T0** | ~100 | nothing yet | one box, one DB, ship it, measure |
| **T1** | ~1k | app and DB fight for the box | split app from DB; add a shared cache + backups |
| **T2** | ~10k | one app node saturates | N stateless replicas behind an LB; read replicas; a queue |
| **T3** | ~100k | the single primary DB is the wall | shard the data; multi-AZ; event bus; SLOs + tracing |
| **T4** | ~1M | one region / one blast radius is too big | multi-region **cells**; geo-partitioned data; DR drills |

Full detail, dimension by dimension, in **[00-scaling-spine.md](00-scaling-spine.md)**.

---

## A note on numbers

The tier labels (100 / 1k / 10k / 100k / 1M) are **orders of magnitude, not thresholds**. A
write-heavy system shards at 10k; a cache-friendly read-heavy one coasts to 1M on replicas. The
point of the tiers is the *sequence of bottlenecks*, which is remarkably consistent, not the exact
user count at which each appears.

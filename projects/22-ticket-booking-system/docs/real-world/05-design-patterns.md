# Universal Patterns in Ticket Booking Systems

All production booking systems — regardless of scale, stack, or domain — converge on the
same set of patterns. This file documents the patterns, explains the reasoning behind each,
and maps them to the IRCTC/BookMyShow/Ticketmaster implementations described in the other files.

---

## The five-layer model

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 1: Traffic control (virtual waiting room / queue)    │
│  Goal: prevent the booking system from seeing more load     │
│        than it can handle                                    │
├─────────────────────────────────────────────────────────────┤
│  Layer 2: Fast pre-check (Redis atomic operation)           │
│  Goal: reject impossible requests before touching the DB    │
├─────────────────────────────────────────────────────────────┤
│  Layer 3: Inventory lock (DB row lock / optimistic lock)    │
│  Goal: exactly one winner per seat — correctness guarantee  │
├─────────────────────────────────────────────────────────────┤
│  Layer 4: Async event bus (Kafka)                           │
│  Goal: decouple notifications, fraud, analytics from the    │
│        booking hot path                                      │
├─────────────────────────────────────────────────────────────┤
│  Layer 5: Idempotent checkout (idempotency key)             │
│  Goal: safe retries on payment timeout or network failure   │
└─────────────────────────────────────────────────────────────┘
```

| Layer | IRCTC | BookMyShow | Ticketmaster | This project |
|-------|-------|-----------|--------------|--------------|
| 1 Traffic control | CAPTCHA + jitter | Virtual queue (post-Coldplay) | Virtual waiting room | — (Milestone 5) |
| 2 Redis pre-check | DECR counter | SETNX per seat | Redis Lua CAS | HoldStore SETNX |
| 3 DB lock | Oracle SELECT FOR UPDATE | MySQL UNIQUE constraint | MySQL UNIQUE + Cassandra LWT | @Version optimistic lock |
| 4 Event bus | — (limited) | Kafka | Kafka | Kafka (hold/booking events) |
| 5 Idempotency | — (limited) | idempotency key | idempotency key | idempotencyKey field |

---

## Layer 1 — Traffic control

### The core problem

A flash sale creates a demand spike that is orders of magnitude above normal load.
No booking system can be cost-effectively sized to handle the absolute peak — the hardware
would sit idle 99.9% of the time.

The solution is not to scale the booking system to peak demand.
The solution is to prevent the peak from reaching the booking system.

### Virtual waiting room

```
All incoming traffic → Queue → Controlled release at N users/second → Booking system
```

Key properties:
- The queue must handle far more connections than the booking system
  (waiting room can be stateless; booking is stateful)
- Each user gets a **position** and a **fair ordering** (usually first-in-first-out)
- Released users get a **time-limited token** (JWT with short expiry)
  so they don't idle in the booking flow indefinitely
- The booking system only sees `N * TTL` concurrent active sessions maximum,
  regardless of queue depth

### When a queue is NOT enough (the Taylor Swift lesson)

A queue only works if:
1. The queue itself can absorb all incoming connections
2. Bot traffic can be distinguished from human traffic before entering the queue
3. The queue release rate is calibrated to the booking system's actual capacity

Ticketmaster's failure was that even the queue front-door was overwhelmed.
The fix: edge-layer bot detection (Cloudflare / AWS WAF) must run before the queue,
not after.

### IRCTC's approach (no formal queue)

IRCTC doesn't have a formal waiting room. Instead:
- CAPTCHA slows bots and human-typed-entries
- Random jitter (±2 seconds) on form submission spreads the spike slightly
- Per-user rate limit (5 booking attempts/minute) prevents hammering
- Redis DECR as Layer 2 absorbs the overflow

This works for IRCTC because Tatkal demand, while intense, is spread across thousands
of trains. The most popular train might get 50,000 attempts for 50 seats — manageable.
A Taylor Swift concert gets 14 million attempts for 70,000 seats — qualitatively different.

---

## Layer 2 — Redis fast pre-check

### The pattern

Before any database operation, perform a cheap atomic Redis operation that:
- Proves the request cannot possibly succeed (and rejects it immediately), OR
- Decrements a counter / sets a flag that reserves a "slot" for the DB operation

Two common implementations:

**Counter DECR (IRCTC style):**
```
counter = DECR seat_count:{train}:{date}:{class}
if counter < 0:
    INCR seat_count:{train}:{date}:{class}  # restore
    return SOLD_OUT
# proceed to DB
```
Pros: simple, extremely fast. Cons: counter can drift from DB truth (needs periodic sync).

**SETNX per seat (BookMyShow style):**
```
result = SET seat:{eventId}:{seatId} {userId} NX EX {ttl}
if result is nil:
    return SEAT_HELD
# proceed to DB
```
Pros: per-seat granularity, no drift. Cons: uses more Redis memory for large venues.

### Why Redis and not the database?

A Redis `SET NX` or `DECR` is:
- **In-memory:** no I/O — sub-millisecond response
- **Single-threaded command execution:** no locking overhead, atomicity is free
- **Horizontally scalable:** Redis Cluster shards by key, linear throughput scaling

A MySQL `SELECT FOR UPDATE` is:
- Disk I/O (even with buffer pool, there are log writes)
- Lock manager overhead
- Each waiting transaction consumes a thread / connection
- At 50,000 concurrent requests, the connection pool is the first bottleneck

The Redis layer exists to protect the database from requests that cannot succeed.
It does not replace the database — it is a filter in front of it.

---

## Layer 3 — Inventory lock (the correctness guarantee)

This is the only layer that actually prevents overselling.
Layers 1 and 2 are optimisations. Layer 3 is correctness.

### Three locking approaches

**Pessimistic locking (SELECT FOR UPDATE):**
```sql
BEGIN;
SELECT available FROM inventory WHERE id = ? FOR UPDATE;
-- if available > 0:
UPDATE inventory SET available = available - 1 WHERE id = ?;
INSERT INTO bookings ...;
COMMIT;
```
- Serialises all concurrent writers for this row
- Safe against all races
- Degrades under high concurrency (queue of waiters at the DB)
- IRCTC / GDS use this pattern

**Optimistic locking (@Version / compare-and-swap):**
```sql
UPDATE seats SET status = 'HELD', version = version + 1
WHERE id = ? AND status = 'AVAILABLE' AND version = ?;
-- if rowsAffected == 0: someone else won, retry or fail
```
- No waiting — each attempt either wins or fails instantly
- Higher throughput under moderate contention
- Under very high contention, most attempts fail → wasted work → retry storms
- This project uses this pattern (JPA @Version)
- Suitable when contention is moderate; less suitable for IRCTC-scale Tatkal

**UNIQUE constraint (last-writer-rejected):**
```sql
INSERT INTO bookings (event_id, seat_id, user_id)
VALUES (?, ?, ?);
-- unique(event_id, seat_id) — DB rejects duplicate automatically
```
- Simplest possible approach
- Works for confirmed bookings (one booking per seat ever)
- Combined with Redis SETNX, this is BookMyShow's pattern
- Not suitable for holds (seats need to be releasable)

### When to use which

| Scenario | Recommended approach |
|----------|---------------------|
| Low concurrency (< 100 req/s per seat) | Optimistic lock (@Version) |
| High concurrency, holds needed | Pessimistic lock (SELECT FOR UPDATE) |
| High concurrency, no holds | Redis SETNX + UNIQUE constraint |
| Distributed / multi-node writes | Redis Lua CAS + DB UNIQUE |

---

## Layer 4 — Async event bus

### What goes on Kafka

The booking hot path must be as short as possible. Every millisecond of latency on the hot
path is felt by every user. The event bus allows non-critical work to happen asynchronously:

**Always async (never on the hot path):**
- Email / SMS / push notification
- Analytics and reporting
- Fraud score update
- Recommendation model update ("users who booked X also booked Y")
- Revenue reporting
- Audit log write

**Sometimes async (saga pattern):**
- Payment processing (initiate sync, confirm async via webhook)
- PNR generation (IRCTC: confirm booking sync, generate PNR async)
- Refund processing

**Never async (must be sync):**
- The seat lock acquisition
- The booking confirmation write
- The payment initiation

### The saga pattern for payment

Payment is the most common async workflow in booking systems:

```
1. User submits checkout
2. Booking service creates booking record (status: PENDING)
3. Booking service calls payment gateway (sync — get a payment URL or token)
4. User completes payment on gateway UI
5. Gateway sends webhook to booking service (async)
6. Booking service receives webhook, updates booking (status: CONFIRMED)
7. Booking service publishes booking.confirmed to Kafka
8. Notification service sends email (consumes from Kafka)
```

**Failure modes and compensating transactions:**

| Failure | Compensation |
|---------|-------------|
| Payment gateway timeout | Booking stays PENDING; webhook arrives later; idempotency key prevents double-booking |
| Gateway webhook never arrives | Scheduled reconciliation job polls gateway API for PENDING bookings older than N minutes |
| User closes browser after payment | Webhook still arrives; booking confirmed without user's browser |
| Double payment (gateway retries webhook) | Idempotency key on booking record prevents double-processing |

---

## Layer 5 — Idempotency

### Why it's needed

Payment networks are unreliable. A payment request may:
- Time out without a response (did it succeed or fail?)
- Receive a network error on the response (the charge went through but we never got the 200)
- Be retried by the client after a timeout

Without idempotency, a retry = a double charge.

### Implementation

```sql
CREATE TABLE bookings (
    id               UUID PRIMARY KEY,
    idempotency_key  VARCHAR(128) UNIQUE,   -- provided by client
    hold_id          UUID NOT NULL UNIQUE,
    ...
);
```

```
checkout(holdId, userId, amount, idempotencyKey):
    existing = SELECT * FROM bookings WHERE idempotency_key = ?
    if existing:
        return existing   # same response as original — safe to return to client
    
    # proceed with new booking
    INSERT INTO bookings (idempotency_key = ?)
```

**Key properties:**
- The idempotency key must be client-generated (UUID or hash of the hold + user + amount)
- It must be stored with the booking record before the payment call
- The UNIQUE constraint on `idempotency_key` handles concurrent retries atomically
- The key should expire after a reasonable window (e.g., 24 hours) to prevent table bloat

---

## The consistency spectrum

Not all data in a booking system needs the same consistency level:

```
←──────────────── consistency ────────────────→
Eventually consistent          Strongly consistent

Seat map display        Seat holds         Confirmed bookings
(30s cache ok)          (Redis + DB)       (DB ACID transaction)

Analytics              Hold expiry         Payment records
(minutes delay ok)     (10s sweep ok)      (must not lose)

Recommendations        Queue position      PNR / booking ID
(hours delay ok)       (approximate ok)    (must be exact)
```

Getting the consistency level wrong in either direction is expensive:
- Too strong (serialise everything through DB): kills throughput
- Too weak (allow reads from stale cache everywhere): oversell, user confusion

The standard approach is to map each piece of data to the weakest consistency level
that is still correct for that data's use case — and no weaker.

---

## Flash sale playbook (operational checklist)

For a known high-demand event, the pre-sale checklist should include:

**24 hours before:**
- [ ] Load-test the entire booking flow at 2× expected peak TPS
- [ ] Pre-warm Redis with seat inventory
- [ ] Pre-warm DB connection pools
- [ ] Verify payment gateway capacity with the gateway provider
- [ ] Enable virtual waiting room for the event

**1 hour before:**
- [ ] Enable enhanced bot detection (stricter thresholds)
- [ ] Scale out stateless API servers to expected peak
- [ ] Verify CDN caching for the event landing page
- [ ] Alert on-call team — manual escalation path ready

**During:**
- [ ] Monitor Redis memory (holds can spike Redis memory)
- [ ] Monitor DB replication lag (reads from replica — lag = stale availability display)
- [ ] Monitor payment gateway error rate
- [ ] Monitor queue depth (is the release rate right?)

**After:**
- [ ] Run reconciliation: Redis counts match DB booking counts
- [ ] Verify all PENDING payments resolved
- [ ] Hold expiry worker swept all expired holds
- [ ] Post-mortem if any SLO was breached

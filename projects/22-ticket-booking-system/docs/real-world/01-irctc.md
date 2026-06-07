# IRCTC — Indian Railway Catering and Tourism Corporation

## Scale

| Metric | Number |
|--------|--------|
| Registered users | ~30 million |
| Daily ticket bookings | 8–10 lakh (800,000–1,000,000) |
| Concurrent users on Tatkal opening (10:00 AM) | 7–8 lakh burst |
| Tatkal booking window | 10:00–10:30 AM, 30-minute sprint |
| Total trains in India | ~13,000 daily |

IRCTC is probably the hardest ticket booking problem in the world by one metric: the demand
spike is completely predictable (10:00 AM every day), enormous, and extremely concentrated
on a tiny inventory (popular trains on popular routes sell out in under 2 minutes).

---

## History and architecture evolution

### 2002 — Original monolith (J2EE + Oracle)

The original system launched in 2002 as a monolithic J2EE application on Oracle.
Every seat-availability check was a `SELECT COUNT(*)` against Oracle.
Every booking was a `SELECT ... FOR UPDATE` row lock.

This worked fine at 100 concurrent users. At 10,000 it started struggling.
At 100,000 it fell over completely.

The root cause was that every train's "available seats" was a single Oracle row.
All concurrent booking attempts for that train serialised through one row lock.
At Tatkal opening, this became a single-threaded queue in the database.

### 2012 — First major crisis

Three consecutive days of Tatkal opening brought the site down.

Post-mortem findings:
- Connection pool exhaustion on JBoss app servers — threads piled up waiting for Oracle locks
- Oracle RAC failover triggered during peak load, causing a 4-minute blackout
- No request queuing — all 7 lakh users hit the app tier simultaneously

**Fixes applied:**
- Added Redis as a session store (removed Oracle session load)
- Implemented a Redis counter pre-check as a fast-reject gate
- Shortened Oracle lock timeout from 30s to 5s — fail fast rather than pile up
- Added Apache request queuing at the web tier

### 2015–2019 — CDN and read-replica era

- Moved static assets (booking form, images) to Akamai CDN — reduced origin load ~40%
- Oracle standby configured as a read replica for seat-availability reads
- Only confirmed-booking writes hit the RAC primary
- PNR generation moved to async background job — user gets booking confirmation immediately,
  PNR arrives within 2 minutes

### 2020–present — Partial microservices migration

- Availability service split out to a dedicated Oracle instance
- Can be independently scaled and has its own connection pool
- Payments delegated to a separate service (PayU / Razorpay integration)
- Mobile API layer rewritten in Java Spring Boot wrapping legacy core

---

## Current architecture

```
User (browser / mobile)
    │
    ▼
F5 BIG-IP Load Balancer (hardware, ~200 Gbps)
    │
    ▼
Apache HTTPD (web tier, ~200 nodes)
    │
    ▼
JBoss WildFly app servers
    │
    ├──► Redis Cluster ──── session state, seat counter pre-check
    │
    ├──► Oracle RAC (2-node) ─── primary booking transactions
    │        └─► Oracle Standby ── availability reads (eventually consistent)
    │
    ├──► Payment gateway (PayU / Razorpay) ─── async callback
    │
    └──► Akamai CDN ─── static assets, some booking-form caching
```

**Deployed on:** NIC (National Informatics Centre) owned hardware in government data centres
(Delhi and Mumbai). Not on public cloud — a policy decision driven by data sovereignty.

---

## The core locking mechanism

IRCTC uses `SELECT ... FOR UPDATE SKIP LOCKED` on Oracle. Each train+class+date is a row
in a seat-inventory table:

```sql
-- Simplified schema
CREATE TABLE seat_inventory (
    train_number    VARCHAR2(10),
    journey_date    DATE,
    class_code      VARCHAR2(4),     -- SL, 3A, 2A, 1A
    available_seats NUMBER,
    waitlist_count  NUMBER,
    version         NUMBER,
    PRIMARY KEY (train_number, journey_date, class_code)
);
```

A booking transaction:

```sql
BEGIN
    -- 1. Lock the inventory row
    SELECT available_seats
    FROM seat_inventory
    WHERE train_number = :train AND journey_date = :date AND class_code = :class
    FOR UPDATE WAIT 5;  -- fail after 5 seconds if locked

    -- 2. Reject if no seats
    IF available_seats <= 0 THEN
        RAISE_APPLICATION_ERROR(-20001, 'No seats available');
    END IF;

    -- 3. Decrement
    UPDATE seat_inventory
    SET available_seats = available_seats - 1
    WHERE train_number = :train AND journey_date = :date AND class_code = :class;

    -- 4. Insert passenger
    INSERT INTO bookings (...) VALUES (...);

    COMMIT;
END;
```

**The bottleneck:** everything serialises through that one row lock per train+class+date.
For a popular train on Tatkal day, this is effectively a single-threaded queue.
They sustain ~1,200 booking transactions/second across the entire Oracle cluster —
which sounds low, but each transaction touches multiple tables and triggers.

---

## Redis as a pre-check gate

Before any Oracle interaction, the app server does:

```
DECR seat_counter:{train}:{date}:{class}
```

If the result is `< 0`, the request is rejected immediately without touching Oracle.

The Redis counter is initialised from Oracle at midnight and refreshed every few minutes.
It is deliberately slightly pessimistic (may reject valid requests) but never over-optimistic
(never lets through more bookings than seats exist).

This single change was the most impactful optimisation in IRCTC's history.
It cuts Oracle load by ~85% on Tatkal day — only the ~15% of requests that pass the Redis
gate ever reach Oracle.

```
7,00,000 concurrent users
    │
    ▼
Redis DECR check ──── ~85% rejected here (fast, microseconds)
    │
    ▼ ~1,05,000 proceed
Oracle booking ──── ~97% of these succeed (seats available)
```

---

## The Tatkal problem in detail

Tatkal (emergency quota) opens at exactly 10:00:00 AM.
The booking window for Tatkal closes 1 hour before train departure.

**Why it's hard:**
- The demand spike hits a precise second — not spread over minutes
- All users have been refreshing since 9:59 AM
- The most popular trains (Mumbai–Delhi Rajdhani, etc.) have ~50 Tatkal seats
- ~5,00,000 users are trying to book those 50 seats simultaneously

**Current mitigations:**
1. CAPTCHA mandatory on Tatkal — slows automated bots
2. Redis DECR gate — 85% rejected before Oracle
3. Random jitter on the booking form submission — spreads the spike by ±2 seconds
4. Per-user rate limit — max 2 Tatkal bookings per user per day via Redis sorted set

**What they haven't solved (as of 2024):**
- Sophisticated bots that solve CAPTCHAs using ML services
- Tout networks with thousands of real human accounts booking simultaneously
- The fundamental fairness problem: a user on a 100ms connection wins over a user on 500ms

---

## Known failure modes and post-mortems

### 2012 Tatkal crash (3 days)
**Root cause:** JBoss connection pool exhausted; Oracle RAC failover during peak.
**Fix:** Redis session, shorter lock timeout, request queuing.

### 2015 PNR generation lag
**Root cause:** PNR generation was synchronous — blocked booking confirmation during peak.
**Fix:** Async PNR generation; booking confirmation decoupled from PNR.

### 2019 Payment gateway timeout storm
**Root cause:** PayU gateway latency spike caused booking threads to pile up waiting for
payment callbacks, exhausting the thread pool.
**Fix:** Payment timeout reduced to 8 seconds; failed payments go to a retry queue;
booking state machine explicitly handles PAYMENT_PENDING → PAYMENT_TIMEOUT.

### 2023 Mobile app API crash
**Root cause:** A new mobile API release sent malformed requests that bypassed Redis counter
and hit Oracle directly, causing a connection storm.
**Fix:** Input validation added at the API gateway before Redis pre-check.

---

## Useful references

- NIC procurement tender documents (search "IRCTC high availability NIC tender") — contain
  detailed infrastructure specs including Oracle RAC configuration and capacity planning numbers
- "Indian Railways Ticketing System Architecture" — occasional coverage in Indian engineering
  conference proceedings (IEEE ICACCI, etc.)
- IRCTC annual reports (Ministry of Railways) — contain uptime SLA commitments and incident counts

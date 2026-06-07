# Ticketmaster / Live Nation

## Scale

| Metric | Number |
|--------|--------|
| Tickets sold per year | ~500 million |
| Countries | 30+ |
| Peak: Taylor Swift Eras Tour pre-sale (Nov 2022) | 3.5 billion system requests in one day |
| Concurrent verified fan sessions at peak | ~14 million |
| Normal peak TPS | ~5,000–10,000 |
| Taylor Swift peak TPS | estimated 400,000+ |

Ticketmaster is the largest ticketing platform in the world by volume. The Taylor Swift
Eras Tour pre-sale in November 2022 is the most studied ticketing failure in history and
directly triggered US Senate hearings about Live Nation's market dominance.

---

## Architecture overview

```
User (browser / mobile app)
    │
    ▼
AWS CloudFront (CDN, DDoS, edge caching)
    │
    ▼
Virtual Waiting Room (queue service — custom-built, runs on AWS)
    │
    ▼
API Gateway (AWS API Gateway + custom routing)
    │
    ├──► Inventory Service (Java microservice)
    │        ├──► Cassandra Cluster ─── seat availability (eventual consistency)
    │        └──► MySQL (Aurora) ─── confirmed bookings (strong consistency)
    │
    ├──► Hold Service
    │        └──► Redis Cluster ─── TTL-based holds, Lua atomic scripts
    │
    ├──► Payment Service
    │        └──► Braintree / Stripe / PayPal integrations
    │
    ├──► Fraud Detection Service (real-time ML scoring)
    │        └──► Kafka ─── event stream for fraud signals
    │
    └──► Notification Service
             └──► Kafka ─── email, SMS, push via AWS SNS/SES
```

**Infra:** Primarily AWS (multi-region: us-east-1 primary, us-west-2 failover).
Ticketmaster runs one of the largest Cassandra deployments outside of Netflix/Apple.

---

## The Cassandra + MySQL split — why and how

This is the most architecturally interesting decision in Ticketmaster's stack.

### The problem with using only MySQL

For a regular event (10,000 seats, moderate demand), MySQL handles reads and writes fine.
For a stadium concert release (50,000 seats, 2 million people trying to buy simultaneously):
- Every user refreshing the seat map = a read on the same rows
- MySQL read replicas help, but replication lag means stale reads
- The write path (booking) creates lock contention on seat rows
- Vertical scaling MySQL hits a ceiling quickly

### The problem with using only Cassandra

Cassandra is eventually consistent by default. Two users can both read "Row A = AVAILABLE"
from different Cassandra nodes, both proceed to book, and both succeed at the Cassandra level
before the conflict is detected. This is the oversell problem.

### The split solution

```
Read path  (seat availability, seat map display)
    └──► Cassandra
          - Wide row per event+section: { seat_id → status }
          - Reads from any replica — fast, scalable, but eventually consistent
          - Stale reads are acceptable: "seat shows available but is taken by checkout"
            is a known UX trade-off

Write path (confirmed bookings — the truth)
    └──► MySQL (Aurora)
          - INSERT with UNIQUE(event_id, seat_id) constraint
          - The unique constraint is the final oversell guard
          - After a successful INSERT, Cassandra is updated asynchronously
```

**The user experience consequence of this split:**
A user can see a seat as available on the seat map, click it, hold it, proceed to payment,
and then be told at checkout that the seat was already taken. This is not a bug — it is an
explicit trade-off of availability over consistency on the read path.

Ticketmaster's checkout page says "your seats are not confirmed until payment completes"
precisely because of this architecture.

### Cassandra schema (simplified)

```
-- Keyspace: ticketing, replication_factor: 3
CREATE TABLE seat_availability (
    event_id    uuid,
    section     text,
    seat_id     uuid,
    status      text,     -- AVAILABLE, HELD, SOLD
    version     bigint,   -- for lightweight transaction (LWT) on hold
    PRIMARY KEY ((event_id, section), seat_id)
);
```

Partition key is `(event_id, section)` — a section's worth of seats fit on one partition,
making seat-map reads a single Cassandra partition read (fast).

### Cassandra LWT for holds

For hold creation, Ticketmaster uses Cassandra Lightweight Transactions (Paxos-based CAS):

```cql
UPDATE seat_availability
SET status = 'HELD', version = 2
WHERE event_id = ? AND section = ? AND seat_id = ?
IF status = 'AVAILABLE' AND version = 1;
```

This is linearisable (though slow — ~3× latency of a normal write) and prevents two users
from simultaneously acquiring the same hold. For the booking confirmation, MySQL's UNIQUE
constraint serves as the backup guarantee.

---

## Virtual Waiting Room

The virtual waiting room is Ticketmaster's most important resilience mechanism and the one
they most visibly failed to scale for Taylor Swift.

### How it works (normal operation)

1. When a high-demand sale opens, **all traffic enters a queue** rather than hitting the
   booking system directly.
2. Users receive a **position in queue** and a **visual progress indicator**.
3. The queue releases tokens at a controlled rate — say, 2,000 users/second.
4. Each released user receives a **JWT with a 10-minute expiry** that permits them to
   enter the booking flow.
5. The booking system behind the queue only ever sees ~2,000 concurrent active sessions,
   regardless of how many millions are waiting.

```
2,000,000 users waiting
    │
    ▼
Queue service (token bucket: 2,000 tokens/second released)
    │
    ▼
~2,000 active booking sessions at any time
    │
    ▼
Inventory / hold / payment services (manageable load)
```

### The Taylor Swift failure (November 2022)

The Eras Tour pre-sale was limited to "Verified Fans" — users who had pre-registered.
Ticketmaster issued 1.5 million Verified Fan codes. But:

- ~14 million people tried to access the sale simultaneously
- Millions of non-verified users bypassed the intended flow by directly hitting booking URLs
- Bot traffic was estimated at hundreds of millions of requests
- The queue service itself became a bottleneck — it was not designed for 14 million
  simultaneous queue entrants
- Queue position numbers were inaccurate; many users waited hours and never got through

**Technical root causes (from subsequent reporting and Ticketmaster's own testimony):**

1. **Queue service capacity:** The waiting room was not horizontally scaled to handle
   14 million concurrent WebSocket/polling connections. Connection limits were hit.
2. **Verified Fan bypass:** The Verified Fan code validation was bypassable — bots did not
   need codes to enter the queue, they just needed to be in the queue.
3. **Bot traffic volume:** 3.5 billion requests in one day. Ticketmaster's bot detection
   (VerifiedFan + CAPTCHA) was overwhelmed by sophisticated bots using residential proxies
   and ML-based CAPTCHA solving.
4. **No graceful degradation:** When the queue service degraded, it did not cleanly shed load —
   it became partially available in a confusing way, some users getting through and others
   stuck indefinitely.

**Post-mortem fixes:**
- Queue service rewritten to use horizontally-scalable serverless architecture (AWS Lambda
  + SQS) rather than stateful WebSocket servers
- Hard cap on simultaneous Verified Fan codes issued per sale
- Improved bot detection: device fingerprinting + behavioural analysis pre-queue
- Senate hearing led to DOJ antitrust investigation (separate from engineering decisions)

---

## Hold mechanism (Redis + Lua)

```lua
-- Atomic hold creation: check availability and set hold in one round-trip
local seat_key = "seat:" .. KEYS[1]          -- seat:{seatId}
local hold_key = "hold:" .. KEYS[2]          -- hold:{holdId}
local user_id  = ARGV[1]
local ttl      = tonumber(ARGV[2])           -- 600 seconds

local current = redis.call("GET", seat_key)
if current ~= false then
    return {0, "seat_already_held"}          -- seat taken
end

redis.call("SET", seat_key, user_id, "EX", ttl)
redis.call("SET", hold_key, seat_key, "EX", ttl)
return {1, "ok"}
```

Hold TTLs are aggressive: 5–10 minutes for general sale, 3 minutes during extremely high
demand. Short TTLs reduce the number of "dead" holds during a rush and make seats
available faster for the next person in queue.

---

## Fraud detection

Ticketmaster runs real-time fraud scoring on every booking attempt:

**Signals used:**
- Purchase velocity: how many tickets has this account bought in the last 24 hours?
- Device fingerprint: is this a known bot device?
- Payment method: new card + VPN + fast checkout = high fraud score
- Behavioural: did the user visit the event page before clicking checkout?
- Network: is the IP a datacenter, VPN, or residential proxy?

**Architecture:**
- Every booking action produces a Kafka event
- Fraud detection service consumes the stream, scores within 200ms
- Score above threshold: challenge (CAPTCHA) or block
- Scores are cached in Redis so repeat offenders are blocked faster

---

## Observability and SLOs

**SLO commitments (public):**
- 99.9% availability for the booking flow
- p99 end-to-end booking latency < 3 seconds (excluding payment gateway)
- Hold creation p99 < 200ms

**Monitoring stack:**
- Datadog (primary APM, metrics, logs)
- Kafka for event streaming to analytics
- PagerDuty for on-call alerting
- Synthetic monitoring: Datadog Synthetics runs a full booking flow every 60 seconds

---

## Useful references

- US Senate Judiciary Committee hearing on Ticketmaster, January 2023
  — transcript available on senate.gov, contains technical details about bot volumes
- "How Ticketmaster's Systems Work" — various tech journalists covering the Taylor Swift
  outage have published good technical reconstructions (Ars Technica, The Verge)
- Cassandra Summit talks from Live Nation Engineering (2016–2019) — YouTube
  — "Ticketmaster's Journey with Cassandra" covers the Cassandra+MySQL split in detail
- Ticketmaster Engineering Blog: engineering.ticketmaster.com (archived)

# BookMyShow

## Scale

| Metric | Number |
|--------|--------|
| Monthly active users | ~50 million |
| Monthly ticket sales | ~5 million |
| Peak: Coldplay India tour (Jan 2025) | 8 lakh bookings attempted in 30 minutes |
| Cities served | 650+ |
| Events listed | ~5,000 active at any time |

BookMyShow is India's dominant entertainment ticketing platform — movies, concerts, sports,
comedy shows, plays. The concert business is where the hard engineering problems live.

---

## Architecture

```
User (browser / mobile)
    │
    ▼
Cloudflare (DDoS, CDN, bot mitigation layer 1)
    │
    ▼
Nginx (web tier)
    │
    ▼
API Gateway (custom Go service — rate limiting, auth token validation, routing)
    │
    ├──► Java Spring Boot microservices (booking, payment, notification, user)
    │        │
    │        ├──► MySQL Cluster (sharded by city) ─── transactional bookings
    │        ├──► Redis Cluster ─── seat locks, session, rate limits, cache
    │        ├──► Kafka ─── booking events, notifications, analytics
    │        └──► Elasticsearch ─── event search, discovery, recommendations
    │
    ├──► Payment gateways (Razorpay, Paytm, UPI aggregators)
    │
    └──► AWS S3 + CloudFront ─── images, static assets
```

**Infra:** Primarily AWS (Mumbai region) + some on-prem for latency-sensitive paths.

---

## Seat locking mechanism

Two-phase locking: Redis for speed, MySQL for correctness.

### Phase 1 — Redis SETNX (atomic fast-path)

```
SET seat:{eventId}:{seatId} {userId} NX EX 600
```

- `NX` — only set if key does not exist (atomic check-and-set)
- `EX 600` — 10-minute TTL
- Returns `OK` if lock acquired, `nil` if seat already held

If Redis returns `nil`, the user gets "seat unavailable" immediately, without any DB round-trip.

The Redis lock is the speed layer. It handles the overwhelming majority of contention
during a popular event release.

### Phase 2 — MySQL UNIQUE constraint (correctness layer)

Once the Redis lock is acquired, the booking service does:

```sql
INSERT INTO bookings (event_id, seat_id, user_id, status, created_at)
VALUES (?, ?, ?, 'CONFIRMED', NOW())
-- seat_id has a UNIQUE constraint — DB rejects if another booking slipped through
```

If this INSERT fails with a duplicate-key error, the Redis lock is released and the user
is told the seat is unavailable. This case is rare (Redis SETNX is atomic) but handles
Redis failures, clock skew, or bugs in the lock logic.

### Why Redis SETNX instead of database SELECT FOR UPDATE?

`SELECT FOR UPDATE` on MySQL serialises all concurrent attempts through a single row lock,
degrading linearly as concurrency increases. At 50,000 concurrent users on a single event,
this becomes a bottleneck.

Redis SETNX is an O(1) atomic operation handled entirely in memory.
The Redis cluster can serve ~1,000,000 SET operations/second.
The contention check is done in microseconds rather than milliseconds.

---

## MySQL sharding strategy

The booking database is sharded by city (or region):

```
booking_db_mumbai  ──► MySQL primary + 2 read replicas
booking_db_delhi   ──► MySQL primary + 2 read replicas
booking_db_south   ──► MySQL primary + 2 read replicas (Bangalore, Chennai, Hyderabad)
...
```

**Why by city?** Concerts and movies are local events. A user in Mumbai booking a Mumbai
concert never needs to talk to the Delhi shard. Cross-shard queries are rare.

**Downside:** popular events can hot-spot a single shard. A national concert (Coldplay touring
multiple cities) spreads load naturally, but a single massive Mumbai event hammers only the
Mumbai shard. They handle this with read replicas and by routing seat-map reads to replicas.

---

## Kafka usage

Every significant booking action produces an event:

```
topic: bms.booking.events
  - booking.initiated   { userId, eventId, seats, timestamp }
  - booking.confirmed   { bookingId, userId, amount, paymentMethod }
  - booking.cancelled   { bookingId, refundAmount }
  - seat.held           { eventId, seatId, userId, expiresAt }
  - seat.released       { eventId, seatId, reason }

topic: bms.notifications
  - email, SMS, push notifications consumed by notification service

topic: bms.analytics
  - raw event stream consumed by ClickHouse for real-time dashboards
```

**Hold expiry via Kafka + scheduled consumer:**
BookMyShow uses a Kafka consumer with a delayed-delivery pattern for hold expiry.
When a hold is created, an event with `expiresAt` timestamp is published.
A consumer polls and processes events whose `expiresAt` has passed, releasing the Redis lock
and updating MySQL. This is more reliable than a cron job because Kafka guarantees delivery
even if the consumer was down during the expiry window.

---

## The Coldplay incident (January 2025)

Coldplay's India tour went on sale in December 2024. Within 30 minutes:
- 8 lakh booking attempts
- Website effectively unusable for most users
- Social media outrage about bots securing hundreds of tickets

**What went wrong:**

1. **Bot traffic:** Rate limiting was per-IP. Bots used residential proxies (each request from
   a different IP). Cloudflare blocked obvious datacenter IPs but missed residential proxy farms.

2. **Seat-map reads hammered the cache:** 50,000+ concurrent users refreshing the seat map
   every second. Even with Redis caching, cache-miss thundering herd on cache expiry caused
   MySQL spikes.

3. **Payment gateway bottleneck:** Razorpay/Paytm couldn't handle the simultaneous payment
   initiation volume, causing timeouts. Users with held seats couldn't complete payment,
   seats expired and re-released, creating a confusing experience.

**Post-Coldplay fixes (reported):**

- Behavioural bot detection: time-on-page, scroll patterns, mouse movement analysis
  before allowing seat selection
- Virtual waiting room: pre-registration queue for high-demand events
  (similar to Ticketmaster's approach)
- Payment gateway pre-warming: load-test the payment path at concert-scale before sale opens
- Seat-map cache: extended TTL + background refresh pattern (stale-while-revalidate)
  to prevent thundering herd

---

## Rate limiting architecture

BookMyShow uses a multi-layer rate limiting approach:

```
Layer 1 — Cloudflare (edge)
  - IP-based rate limits for known bot patterns
  - Challenge (JS challenge / CAPTCHA) for suspicious IPs

Layer 2 — API Gateway (application)
  - Per-user token bucket (Redis): 100 req/min for browsing, 5 req/min for booking actions
  - Per-event rate limit: max N booking attempts per event per user per hour
  - Implemented with Redis sliding window counters using Lua scripts

Layer 3 — Booking service
  - Per-user hold limit: max 2 active holds simultaneously
  - Per-event hold limit: max 6 seats per booking
```

The Redis Lua script for per-user rate limiting:

```lua
local key = KEYS[1]              -- "ratelimit:user:{userId}:booking"
local limit = tonumber(ARGV[1])  -- 5
local window = tonumber(ARGV[2]) -- 60 (seconds)
local now = tonumber(ARGV[3])

redis.call('ZREMRANGEBYSCORE', key, 0, now - window * 1000)
local count = redis.call('ZCARD', key)
if count < limit then
    redis.call('ZADD', key, now, now)
    redis.call('EXPIRE', key, window)
    return 1  -- allowed
end
return 0  -- rate limited
```

---

## Observability

- **Metrics:** Prometheus + Grafana. Key dashboards: booking funnel conversion, seat-lock
  hit rate, Redis memory, MySQL replication lag, payment gateway success rate.
- **Tracing:** Jaeger (OpenTelemetry). Every booking request gets a trace ID that flows
  through API gateway → booking service → MySQL → Redis → payment gateway.
- **Alerting:** PagerDuty. SLO: 99.9% availability, p99 booking latency < 2s.
  Alerts fire at 10% SLO burn rate (within 1 hour) and 2% burn rate (within 6 hours).
- **Logs:** ELK stack. Structured JSON logs with request ID, user ID, event ID, seat ID.

---

## Useful references

- BookMyShow Engineering Blog: medium.com/bookmyshow-engineering
  - "How we handle 3 million concurrent users" (2019)
  - "Redis at BookMyShow" (covers cluster topology and Lua scripting)
- Ashish Hemrajani (founder) interviews — business context for scale decisions
- "BookMyShow: Scaling to 50 Million Users" — talks at various Indian engineering conferences

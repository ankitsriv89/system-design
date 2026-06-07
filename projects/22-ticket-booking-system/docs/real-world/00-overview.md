# Real-World Ticket Booking Systems — Deep Dive

A reference collection covering how production-grade booking systems are actually built,
what goes wrong at scale, and the engineering decisions behind the patterns this project implements.

## Files in this series

| File | System | Key topics |
|------|--------|------------|
| [01-irctc.md](01-irctc.md) | IRCTC (Indian Railways) | Oracle RAC, Redis pre-check, Tatkal flash sales, crash post-mortems |
| [02-bookmysshow.md](02-bookmysshow.md) | BookMyShow | MySQL sharding, Redis SETNX, Kafka, bot problems, Coldplay incident |
| [03-ticketmaster.md](03-ticketmaster.md) | Ticketmaster / Live Nation | Cassandra + MySQL split, virtual waiting room, Taylor Swift failure |
| [04-amadeus-sabre.md](04-amadeus-sabre.md) | Amadeus / Sabre (GDS) | TPF mainframes, flight segment locking, why mainframes persist |
| [05-design-patterns.md](05-design-patterns.md) | Universal patterns | The five-layer model, consistency spectrum, flash-sale playbook |
| [06-papers-and-reading.md](06-papers-and-reading.md) | Papers & books | Academic papers, engineering blogs, recommended reading order |

## The universal pattern (spoiler)

Every system at scale converges on the same five-layer defence:

```
1. Virtual waiting room / queue     — throttle inbound before touching inventory
2. Redis atomic pre-check           — fast rejection without DB round-trip
3. DB row lock / optimistic lock    — final correctness guarantee, one winner per seat
4. Async event bus (Kafka)          — notifications, fraud, analytics, saga steps
5. Idempotency key on checkout      — safe retries on payment timeout or network failure
```

This project (22-ticket-booking-system) implements layers 2–5.
Layer 1 (virtual waiting room) is the natural Milestone 5 extension.

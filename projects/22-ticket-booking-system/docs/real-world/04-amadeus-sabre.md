# Amadeus & Sabre — Global Distribution Systems (GDS)

## What is a GDS?

A Global Distribution System is the middleware layer between airlines/hotels and every
travel agent, OTA (Online Travel Agency), and airline website in the world.

When you book on MakeMyTrip, Expedia, Cleartrip, or even directly on Air India's website,
your booking almost certainly flows through a GDS. The GDS holds the real-time inventory
of every airline seat, hotel room, and car rental.

The two dominant GDS platforms are:

| GDS | Owner | Founded | Scale |
|-----|-------|---------|-------|
| **Amadeus** | Amadeus IT Group (Spain) | 1987 | ~1 billion API calls/day, 500M+ bookings/year |
| **Sabre** | Sabre Corporation (US) | 1964 | ~250M travel bookings/year, 400+ airlines |

There is also **Travelport** (owns Galileo and Worldspan), now a distant third.

---

## Why mainframes? (The answer most engineers are surprised by)

Both Amadeus and Sabre run their core reservation engines on **IBM z-Series mainframes**
running **TPF (Transaction Processing Facility)** — an operating system designed
specifically for airline reservation workloads.

This surprises most software engineers. Why hasn't it been replaced with modern distributed
systems? The answer is both technical and economic.

### Technical reasons

**TPF was purpose-built for this workload:**

- Designed in the 1960s for American Airlines' SABRE system (yes, Sabre the GDS is named
  after the original AA system)
- Handles **45,000–100,000 transactions per second** on a single logical system
- Sub-**100ms** response time guaranteed for 99.99% of transactions
- **99.999% availability** (< 5 minutes downtime per year) — hardware-enforced redundancy
  with two CPUs running in lockstep ("dual-copy" mode)
- Record-level locking in **microseconds** — faster than any distributed lock
- Designed for exactly the flight-segment locking problem: one record = one flight segment,
  locked atomically, no distributed coordination needed

**The IBM Z hardware guarantees:**
- Error-correcting memory that can correct multi-bit errors in flight
- Hot-swap CPUs, memory, and I/O without downtime
- Cryptographic co-processors on-chip (important for PCI compliance)
- A single IBM z16 can run at 200+ billion instructions/second

### Economic reasons

- Airlines have been running on TPF since the 1960s. The accumulated business logic is
  enormous — pricing rules, frequent flyer calculations, codeshare agreements.
  Rewriting it is a 10-year, multi-billion dollar project with high failure risk.
  (See: United Airlines' failed migration attempt in the 1990s, which cost ~$1 billion
  and was abandoned)
- TPF operators are highly specialised and expensive, but there are enough of them
  because airlines have trained them for 60 years
- The mainframe total cost of ownership, when calculated per transaction, is competitive
  with cloud alternatives at these volumes — IBM Z pricing is per-MIPS and
  mainframes are extremely MIPS-efficient at this workload

### The modern wrapper pattern

Neither Amadeus nor Sabre expose raw TPF interfaces externally. The modern architecture is:

```
External call (REST / SOAP API)
    │
    ▼
Modern API Layer (Java Spring Boot / Node.js)
    │  ─── Authentication, rate limiting, protocol translation
    ▼
TPF Bridge (proprietary middleware)
    │  ─── Translates REST to TPF message format
    ▼
IBM z-Series / TPF (core reservation engine)
    │  ─── The actual inventory lock and booking
    ▼
TPF Bridge (response)
    │
    ▼
Modern API Layer (response transformation)
    │
    ▼
JSON/XML response to caller
```

---

## Amadeus in depth

### Scale and reach

- 500+ airlines connected
- 150,000+ hotels
- 900+ rail operators
- 190+ countries
- Data centres in Erding (Germany) and Bangalore (India)

### Architecture layers

**Core (TPF on IBM Z):**
The Amadeus Central System (ACS) runs on IBM z-Series. It holds the real-time availability
and booking records for every connected airline. When an airline updates its inventory
(a seat is sold, a flight is cancelled), the update propagates to the ACS within seconds.

**Amadeus Altéa (airline operations suite):**
Built on top of the TPF core, Altéa handles:
- Inventory management (how many seats of each fare class to release)
- Departure control (check-in, boarding, bag reconciliation)
- Reservations (the booking record, passenger details, payment)

Altéa itself is a Java/J2EE application that reads and writes the TPF layer via the bridge.

**Distribution layer:**
The modern REST/SOAP APIs that OTAs and travel agents use. Run on commodity hardware
(Dell / HP servers) in Java and .NET. These are stateless API servers that translate
external calls into TPF messages.

### Flight segment locking

A "booking" in airline terms is a Passenger Name Record (PNR). A PNR contains one or
more booking segments (each flight leg).

Locking a segment:

```
1. API server sends "book segment" message to TPF bridge
2. TPF bridge translates to a TPF record read with lock
3. TPF acquires exclusive lock on the flight+date+class record (microseconds)
4. Available seat count checked and decremented
5. Booking record created
6. Lock released
7. Response returned
```

Total time: typically 50–150ms end-to-end (mostly network, not lock time).

The lock time itself is measured in **microseconds** on TPF — this is the key reason
TPF survives: a distributed lock via ZooKeeper or etcd takes 5–20ms round-trip.
At 100,000 TPS, that difference is the system being feasible vs. not.

### Amadeus's hybrid cloud evolution

Amadeus has been migrating non-core workloads to AWS since ~2018:

- Search / shopping (fare calculation, availability queries): moved to AWS
  — these are read-heavy, can tolerate eventual consistency, benefit from elasticity
- Ancillary services (hotels, cars, activities): on AWS
- Core reservations (flight booking): still on TPF/IBM Z

The strategy is "cloud around the core" — surround the mainframe with cloud-native services
for everything that doesn't need TPF's guarantees, while keeping the transactional core
where it performs best.

---

## Sabre in depth

### History

Sabre is the oldest surviving GDS. It began in 1960 as a joint project between American
Airlines and IBM to automate AA's reservation system (previously done by hand by 35+ agents
who maintained a paper drum system).

The original SABRE (Semi-Automated Business Research Environment) ran on two IBM 7090 mainframes.
Modern Sabre still descends from this lineage.

### Current architecture

Similar dual-layer approach to Amadeus:

**SynXis (hospitality core):** Hotel reservation system, cloud-native, on AWS.

**Sabre Travel AI:** Modern cloud-native product suite on AWS/GCP — pricing, retailing,
recommendations. Built in Java and Python, uses Kafka and Spark.

**Sabre Central Reservations:** Still TPF on IBM Z for airline core.

**Sabre Red 360:** The agent desktop — a web application wrapping TPF queries in a modern UI.

### The GDS debit memo problem (an interesting failure mode)

Airlines issue "debit memos" to travel agents when a booking is made incorrectly —
wrong fare class, wrong route, missed deadlines. With hundreds of thousands of agents
globally, this creates a massive reconciliation problem.

Sabre handles this with a nightly batch job that compares every issued ticket against the
fare rules at the time of booking. This batch job runs on the mainframe and processes
~200 million records overnight. Cloud-native systems haven't displaced this because the
data is already on the mainframe and the batch processing efficiency of TPF is hard to match.

---

## Comparing GDS to consumer ticketing

| Dimension | GDS (Amadeus/Sabre) | Consumer (IRCTC/BookMyShow) |
|-----------|--------------------|-----------------------------|
| Consistency model | Strong (TPF row lock) | Mixed (Redis pre-check + DB) |
| Latency target | 50–150ms | 200ms–2s |
| Peak TPS | 100,000+ | 1,000–50,000 |
| User base | Travel agents (professional) | General public |
| Failure tolerance | Near-zero (flight operations) | Short outages acceptable |
| Primary hardware | IBM Z mainframe | Commodity x86 / cloud VMs |
| Lock granularity | Per flight segment | Per seat |
| Data age | 60 years of accumulated rules | 5–15 years |

---

## Why airlines don't self-host their inventory

A question worth addressing: why do airlines pay Amadeus/Sabre instead of building their own
reservation systems?

1. **Network effect:** Every OTA and travel agent already connects to Amadeus/Sabre.
   An airline that builds its own system only reaches customers who connect to it directly.
   Low-cost carriers (Ryanair, IndiGo) have done this — direct booking only — but full-service
   carriers need GDS distribution.

2. **Interline agreements:** A passenger flying Delhi → London → New York on three airlines
   needs a single PNR that all three airlines can access. GDS provides this neutral shared record.

3. **Switching cost:** Migrating off a GDS requires re-training every agent and rebuilding
   every OTA integration. Estimated cost: $500M–$2B for a large carrier.

---

## Useful references

- **IBM TPF documentation** — IBM Knowledge Center, search "TPF programming guide"
- **"The History of the SABRE System"** — Robert Sobel (1983) — the original AA/IBM project
- **Amadeus Annual Report** — contains technology investment and architecture roadmap details
- **"Legacy Systems: Their Continued Use in the Travel Industry"** — IEEE Software, various years
- **"Why Banks and Airlines Still Use Mainframes"** — Martin Thompson, LMAX blog (applies to GDS too)
- **Cassandra Summit 2018** — "Amadeus's Journey to Cloud" (YouTube) — covers the hybrid strategy

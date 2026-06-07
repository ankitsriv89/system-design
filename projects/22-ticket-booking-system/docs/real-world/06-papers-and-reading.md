# Papers, Books, and Reading List

A curated reading list organised by topic. Start with the "recommended order" section
if you want a guided path through the material.

---

## Recommended reading order

If you are new to distributed systems and want to understand booking systems from first principles:

1. **Kleppmann — Designing Data-Intensive Applications** (book) — read chapters 7, 8, 9
2. **Hellerstein et al. — Architecture of a Database System** (paper) — understand how
   the lock manager and transaction isolation work
3. **DeCandia et al. — Dynamo** (paper) — understand eventual consistency and why it exists
4. **Gray & Reuter — Transaction Processing** (book) — chapters on 2PL, sagas, compensation
5. **The Chubby Lock Service** (paper) — how distributed locking works at scale
6. **BookMyShow and Ticketmaster engineering blogs** — how the theory lands in practice

---

## Foundational papers

### Transactions and isolation

**"A Critique of ANSI SQL Isolation Levels"**
— Berenson, Bernstein, Gray, Melton, O'Neil, O'Neil (1995)
- The definitive paper on what serialisability, read committed, repeatable read, and
  snapshot isolation actually mean and how they differ
- Relevant because booking systems need to choose the right isolation level for each operation
- URL: `research.microsoft.com/en-us/um/people/gray/papers/IsolationLevels.pdf`

**"Concurrency Control and Recovery in Database Systems"**
— Bernstein, Hadzilacos, Goodman (1987)
- The textbook on 2PL (two-phase locking), optimistic concurrency control, and timestamp ordering
- Chapter 3 (2PL) and Chapter 5 (optimistic CC) are directly relevant to seat locking
- Free PDF available at research.microsoft.com

**"ARIES: A Transaction Recovery Method Supporting Fine-Granularity Locking and Partial Rollbacks"**
— Mohan, Haderle, Lindsay, Pirahesh, Schwarz (1992), ACM Transactions on Database Systems
- Describes the WAL (write-ahead log) mechanism used by PostgreSQL, MySQL, Oracle
- Understanding WAL explains why booking writes are durable even if the server crashes
  mid-transaction

### Distributed systems

**"Dynamo: Amazon's Highly Available Key-Value Store"**
— DeCandia, Hastorun, Jampani, Kakulapati, Lakshman, Pilchin, Sivasubramanian,
  Vosshall, Vogels (SOSP 2007)
- Describes Amazon's eventual consistency model — the exact trade-off Ticketmaster made
  with Cassandra for seat availability reads
- Introduces vector clocks, consistent hashing, and the availability-vs-consistency choice
- Search: "Amazon Dynamo paper" — widely mirrored

**"Cassandra: A Decentralized Structured Storage System"**
— Lakshman, Malik (SIGOPS 2010)
- The original Cassandra paper from Facebook (pre-Apache)
- Explains the wide-column data model that Ticketmaster uses for seat availability
- Search: "Cassandra Facebook SIGOPS 2010"

**"ZooKeeper: Wait-free Coordination for Internet-Scale Systems"**
— Hunt, Konar, Junqueira, Reed (USENIX ATC 2010)
- Describes the distributed coordination primitive underlying Kafka's consumer group
  coordination and many distributed lock implementations
- Search: "ZooKeeper USENIX 2010"

**"In Search of an Understandable Consensus Algorithm (Raft)"**
— Ongaro, Ousterhout (USENIX ATC 2014)
- Raft is the consensus algorithm used by etcd (Kubernetes) and CockroachDB
- Relevant if you want to understand how distributed systems agree on a single value —
  the core problem that seat locking is solving at the application level
- Full paper + interactive visualisation: raft.github.io

### Database internals

**"Architecture of a Database System"**
— Hellerstein, Stonebraker, Hamilton (Foundations and Trends in Databases, 2007)
- The best single overview of how a relational database actually works:
  query processing, lock manager, buffer pool, recovery
- Free PDF: db.cs.berkeley.edu/papers/fntdb07-architecture.pdf
- Read sections 6 (storage) and 8 (lock manager) for booking relevance

**"Spanner: Google's Globally-Distributed Database"**
— Corbett et al. (OSDI 2012)
- Describes TrueTime and how Google achieves external consistency across data centres
- Relevant for the "how would you build IRCTC at global scale with strong consistency?" question
- Search: "Google Spanner OSDI 2012"

### Sagas and compensation

**"Sagas"**
— Garcia-Molina, Salem (SIGMOD 1987)
- The original paper that coined the term "saga" for long-lived transactions
  that can be compensated rather than rolled back atomically
- Directly applicable to the payment saga in booking systems:
  hold → payment → confirm, with compensation (release hold) on failure
- Search: "Garcia-Molina Sagas SIGMOD 1987"

**"Microservices Patterns"**
— Richardson (2018, Manning) — Chapter 4: Managing transactions with sagas
- The best practical treatment of sagas in a microservices context
- Covers choreography-based sagas (Kafka events) vs orchestration-based sagas (saga orchestrator)
- The BookMyShow payment flow uses choreography; a more complex system would use orchestration

---

## Books

**"Designing Data-Intensive Applications"**
— Martin Kleppmann (O'Reilly, 2017)
- The single best book for understanding modern data systems engineering
- Chapter 7 (Transactions) — isolation levels, serialisability, 2PL, SSI
- Chapter 8 (Trouble with Distributed Systems) — partial failures, clocks, network
- Chapter 9 (Consistency and Consensus) — linearisability, CAP, Raft
- Every engineer building booking systems should read chapters 7–9

**"Transaction Processing: Concepts and Techniques"**
— Jim Gray, Andreas Reuter (Morgan Kaufmann, 1992)
- The definitive reference on transaction processing — this is the TPF/mainframe world's bible
- Chapter 7: ACID and isolation
- Chapter 8: Locking — everything about lock granularity, deadlock detection, timeouts
- Chapter 9: Log-based recovery
- Heavy reading but extraordinarily precise

**"Database Internals"**
— Alex Petrov (O'Reilly, 2019)
- More modern than Gray & Reuter, covers B-trees, LSM trees, distributed consensus
- Chapter 5 (Transaction processing) and Chapter 9 (Failure detection) are most relevant

**"Release It! Design and Deploy Production-Ready Software"**
— Michael Nygard (Pragmatic Programmers, 2nd ed. 2018)
- Chapter on bulkheads, timeouts, circuit breakers — exactly the resilience patterns
  that IRCTC learned the hard way in 2012
- The "stability patterns" in Part I are directly applicable to booking system operations

---

## Engineering blog posts

**BookMyShow Engineering**
- medium.com/bookmyshow-engineering
- Key posts: "Redis at BookMyShow", "Kafka at BookMyShow", "How we handle peak load"
- Note: posts are from 2017–2020; architecture has evolved since

**Ticketmaster Engineering** (archived)
- engineering.ticketmaster.com (mostly archived; some posts on Wayback Machine)
- Key talks: Cassandra Summit 2016–2019 talks from Live Nation Engineering on YouTube
  — search "Live Nation Cassandra Summit" for the data architecture talks

**Netflix Tech Blog**
- netflixtechblog.com
- Not ticketing but the Hystrix (circuit breaker), Eureka (service discovery), and
  Chaos Engineering posts are directly applicable to resilience at scale

**Shopify Engineering Blog**
- shopify.engineering
- Flash sale patterns from e-commerce are identical to concert ticket releases
- "Surviving Black Friday" posts describe the same Redis + DB pattern

**High Scalability** (aggregator)
- highscalability.com
- Aggregates architecture posts from many companies; search "ticketing" or "booking"

---

## Conference talks (YouTube)

**"How We Scaled BookMyShow to Handle 8 Million Concurrent Users"**
— BookMyShow team, various Indian engineering conferences (2018–2022)
- Search YouTube: "BookMyShow scaling architecture"

**"Cassandra at Ticketmaster"**
— Cassandra Summit (Apache Con) 2016, 2017
- Search YouTube: "Ticketmaster Cassandra Summit"
- Covers the Cassandra + MySQL split in detail with actual numbers

**"LMAX Architecture"**
— Martin Thompson, QCon 2011
- The Disruptor pattern for ultra-low-latency event processing
- Relevant for understanding why GDS systems (which need < 1ms locks) stay on mainframes

**"Event Sourcing and CQRS"**
— Greg Young, various (2011–2016)
- The booking saga is a natural fit for event sourcing
- Understanding CQRS explains why seat-map reads and booking writes can use different stores
  (Cassandra for reads, MySQL for writes)

---

## IRCTC-specific resources

Official technical documents are rare (government system). The best sources are:

- **NIC (National Informatics Centre) procurement tenders** — when IRCTC upgrades
  infrastructure, the tender document contains detailed specs. Search:
  "IRCTC site:eprocure.gov.in" or "NIC IRCTC high availability"
- **RFP documents** — Oracle, IBM, and HP have published case studies on IRCTC
  infrastructure (though redacted). Search: "IRCTC Oracle RAC case study"
- **IRCTC Annual Reports** — Ministry of Railways; contain uptime SLA commitments
  and transaction volume statistics. Available at irctc.co.in/nget/
- **IEEE ICACCI / ICDCS proceedings** — Indian CS conferences occasionally include papers
  from NIC engineers on IRCTC architecture. Search Google Scholar: "IRCTC architecture"

---

## Topics to explore further (not covered in this series)

**Content delivery and seat map caching:**
How to serve a seat map to 50,000 concurrent users without hammering the DB.
See: Fastly / Varnish stale-while-revalidate patterns.

**Database sharding for booking:**
When a single DB node is not enough. How to shard by event_id without cross-shard
transactions. See: Vitess (YouTube's MySQL sharding layer) documentation.

**Distributed tracing:**
How to trace a booking request across 10 microservices. See: OpenTelemetry specification,
Jaeger / Zipkin documentation.

**Seat pricing algorithms:**
Dynamic pricing (why the same seat costs different amounts at different times).
See: airline revenue management literature — "The Theory and Practice of Revenue Management"
by Talluri & van Ryzin.

**Fraud detection at booking scale:**
Real-time ML scoring of booking attempts. See: "Ad Click Prediction: A View from the
Trenches" (Google, KDD 2013) — same problem class as booking fraud.

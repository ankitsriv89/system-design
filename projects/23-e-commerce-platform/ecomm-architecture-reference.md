# E-Commerce Platform Architecture Reference

## Table of Contents

1. [Service Decomposition](#1-service-decomposition)
2. [Data Stores](#2-data-stores)
3. [Event-Driven Patterns](#3-event-driven-patterns)
4. [Inventory Management](#4-inventory-management)
5. [Cart Architecture](#5-cart-architecture)
6. [Search and Catalog](#6-search-and-catalog)
7. [Checkout Saga](#7-checkout-saga)
8. [Caching Strategy](#8-caching-strategy)
9. [Scaling Decisions](#9-scaling-decisions)
10. [Failure Modes](#10-failure-modes)

---

## 1. Service Decomposition

### Amazon

Amazon's decomposition story is one of the most influential in software history. Starting around 2001, Jeff Bezos issued his API mandate: every team must expose its capabilities through a service interface, teams must communicate only through those interfaces, and no other form of inter-process communication is permitted. Violators would be fired.

The monolith ("Obidos") had become a deployment bottleneck. The initial decomposition split into three domains — customers, catalog, orders — but these proved too coarse. Each was re-decomposed: the customers service alone became ~10 smaller services (Login, Address Book, Preferences, Payment Methods, etc.).

Today, the Amazon retail homepage assembles its page from **a few hundred independent microservices**. Teams are "two-pizza teams" (fewer than 10 people) with single-threaded ownership of a service — "you build it, you run it." Core retail services include:

- **Catalog / Product Detail** — product metadata, images, descriptions
- **Inventory** — stock levels per fulfillment center, reservation management
- **Cart** — persistent per-user, originally built on Dynamo-style key-value store
- **Order Management (OMS)** — order state machine (Pending → Confirmed → Fulfilled → Delivered)
- **Payment Orchestration** — authorization, capture, fraud scoring, tokenization/vault
- **Fulfillment Router** — selects which warehouse fills the order
- **Search (A9/A10)** — query understanding, retrieval, ranking
- **Recommendations** — collaborative filtering, item-to-item similarity
- **Notifications** — email/SMS/push via SNS fan-out to SQS queues
- **Pricing / Promotions** — dynamic pricing, coupon validation
- **Seller / Marketplace** — 3P seller listings, FBA logic

### Shopify

Shopify made a different architectural choice: a **modular monolith** rather than a distributed microservices mesh. The core application is a single Ruby on Rails codebase with 2.8 million lines of code and 500,000+ commits, modularized through Rails Engines (mini-applications with isolated routes, models, and controllers). The monolith is never rewritten, but modules are extracted into standalone services when they need independent scaling.

**Current service topology:**
- Core monolith (Ruby on Rails) — storefront, checkout, order management, products, customers
- **Shipping** — standalone Rails app
- **Identity** — standalone auth service
- **App Store** — Rails app managing partner apps
- **Payments infrastructure** — CardSink (encrypts card data at point of entry, never touches the monolith) → CardServer (decrypts, routes to payment processors) — PCI compliance by isolation
- 100+ internal Rails apps running alongside the monolith
- Shop app (consumer-facing) horizontally scaled via Vitess

Kafka (~66 million messages/second peak) decouples domain events from downstream consumers (search indexing, ML pipelines, notifications, analytics).

### Flipkart

Flipkart operates fully decomposed microservices, predominantly Java/Spring Boot, orchestrated by Kubernetes. Core services:

- **Catalog Service** — read-heavy, 13M+ products, static product attributes
- **Inventory Service** — write-heavy, NRT (Near Real-Time) store with leader-follower replication, Redis pre-warming for flash sales
- **Cart Service** — Redis-backed with TTL, merged on login
- **Order Capture** — write-heavy OLTP, customized HBase serving 500K QPS at peak
- **Payment Service** — integration with payment gateways, fraud detection
- **Search** — Solr/Elasticsearch, ~1,000 QPS baseline, 75ms target latency, ~5ms autosuggest
- **Recommendation Engine** — ALS on Spark, Kafka event-driven
- **Logistics / Fulfillment** — warehousing, last-mile routing
- **Notifications** — Kafka-backed email/SMS/push
- **Analytics** — HBase + Cassandra + Spark for historical data

During Big Billion Days (BBD), the platform handles **6–7× normal traffic**, with 300M+ visitors expected and burst capacity via hybrid cloud (Flipkart Cloud Platform + Google Cloud Platform).

### eBay

eBay ran one of the first large-scale SOA decompositions (~2001), replacing a C++ ISAPI monolith with J2EE services. Today the platform runs **1,000+ microservices**, with front-end experiences (web, iOS, Android) calling orchestration services that fan out to backend services. Core services:

- **Listing / Item** — seller-created listings with item specifics, images
- **Search / Browse** — Voyager (real-time, in-memory, horizontally sharded) + Pronto (Elasticsearch-as-a-service)
- **Buyer / Bidding** — auction and fixed-price buying flows
- **Checkout / Order** — multi-item checkout, combined payments
- **Payments** — PayPal legacy integration, eBay Managed Payments (current, proprietary)
- **Seller Hub** — inventory management, analytics, Terapeak pricing research
- **Fraud / Trust** — ML-based fraud scoring
- **Notifications** — order/shipping events
- **Recommendations** — purchase and listing recommendations

Organization: each service has an independent team. New services are provisioned via an internal cloud portal (dev/test/staging/prod environments automatically created).

### Etsy

Etsy historically maintained a **PHP monolith** (the core e-commerce flow) deployed dozens of times per day. They pursued a pragmatic hybrid: microservices for high-independence concerns (search, payments, recommendations) while the core buying/selling funnel stayed in the monolith. This reflected their "microservices monoliths and laser nail guns" philosophy: don't extract a service until the cost of coupling exceeds the cost of distribution.

**Service topology:**
- PHP monolith — listings, orders, checkout, user accounts
- **Search** — Solr (legacy) + Elasticsearch (~50/50 split)
- **Recommendations** — ML platform, standalone enablement service giving product teams tools to build modules
- **Payments** — standalone, PCI-isolated
- **API Gateway** — API-first transformation to expose monolith capabilities as versioned REST APIs
- **Analytics** — Kafka pipelines, BigQuery/GCP

---

## 2. Data Stores

### Amazon

| Domain | Data Store | Rationale |
|--------|-----------|-----------|
| Shopping Cart | DynamoDB (Dynamo-inspired) | Always-write availability, eventual consistency intentionally chosen — a cart that loses items is worse than showing a stale count |
| Product Catalog | DynamoDB + ElastiCache | Key-value access by ASIN, cache absorbs read spikes; 70% of catalog operations are simple key-value |
| Orders | RDS (Aurora) + DynamoDB | Strong consistency for order records; DynamoDB for high-throughput order state writes |
| Sessions | ElastiCache (Redis) | Sub-millisecond latency, TTL-based expiry |
| Search | OpenSearch (A9/A10 proprietary inverted index) | Full-text search with ML re-ranking |
| Analytics | Redshift + S3 data lake | Columnar analytics, petabyte-scale |
| Inventory | DynamoDB | High write throughput for stock adjustments across thousands of fulfillment centers |

**Key insight:** The 2007 Dynamo paper codified Amazon's architectural decision for the cart: they chose AP (availability + partition tolerance) over CP for the cart specifically because a customer who cannot add to cart is worse than a customer who adds to a cart that has a slight consistency lag. The system uses vector clocks for conflict resolution and sloppy quorum (R+W > N) for consistency.

### Shopify

| Domain | Data Store | Rationale |
|--------|-----------|-----------|
| All transactional data | MySQL (sharded by shop_id) | Relational integrity, ACID; shop-level sharding provides natural tenant isolation |
| Background jobs | Redis (one per pod) | Sidekiq queues; pod-local to prevent Redismageddon-style shared-resource failures |
| Page/query caching | Memcached (one per pod) | Query result caching, single-digit millisecond latency |
| Inventory reservations | MySQL 8 with SKIP LOCKED | Replaced Redis in 2025; row-per-unit model with ACID atomicity; capped pool of 1,000 rows/item/location |
| ML/analytics | BigQuery (GCP) | 216M embeddings/day stored for large-scale querying |
| Event streaming | Kafka | 66M messages/second peak; topics for order events, product updates, search indexing |

**Key insight:** Shopify's MySQL is petabyte-scale, with 45M queries/second read peak and 7.6M writes/second. Ghostferry (Go-based, open-source) handles zero-downtime shard migrations via batch copy + binlog tailing. ProxySQL with SQL comment tags attributes connections to business processes for observability.

### Flipkart

| Domain | Data Store | Rationale |
|--------|-----------|-----------|
| Transactional (orders, payments) | MySQL + TiDB (1M QPS) | ACID guarantees; TiDB for horizontal scale on write-heavy OLTP |
| Order Capture | Custom HBase (modified for strong consistency) | 500K QPS, multi-tenant, transactional change propagation; source-modified HBase for write-heavy workloads |
| Inventory (hot path) | Redis/Aerospike | Sub-millisecond reads; pre-warmed before BBD for flash-sale readiness |
| Inventory (NRT store) | Leader-follower MySQL | Write consistency on leader, read distribution to followers |
| Catalog (flexible schema) | MongoDB | Flexible attributes across 900+ product categories |
| Search | Elasticsearch + Solr | Full-text search with ML-ranked relevance; autosuggest at 5ms |
| Analytics (historical) | HBase + Cassandra | Time-series and archival analytics |
| Stream processing | Kafka + Spark Streaming | Real-time inventory updates, recommendation signals |

### eBay

| Domain | Data Store | Rationale |
|--------|-----------|-----------|
| Core transactional (items, transactions) | Oracle (primary) | Legacy relational, ~340B operations/day across 400B total daily DB calls |
| Distributed/flexible | Apache Cassandra | 250TB, 6B+ writes/day, 5B+ reads/day; multi-datacenter DataStax Enterprise |
| Document store | MongoDB | 15B operations/day (per 2015 data) |
| Key-value | CouchBase | 12B operations/day (per 2015 data) |
| Search index | Elasticsearch via Pronto | 60+ clusters, 2,000+ nodes, 18B documents/day ingested, 3.5B search requests/day |
| Caching | Redis + Memcached | Hot path caching, session management |
| Analytics | Apache Spark + HDFS | Big data analytics |
| Database counts | 600+ Oracle instances across 100+ server clusters | 70+ segmented databases by function (user, item, account, feedback, transaction) |

**Key insight:** eBay deliberately shards at the function level: separate database pools for users, items, accounts, feedback, and transactions. Each pool is further sharded by modulo (Z-axis). Minimum 3 online replicas per database, with some replicas running 15 minutes to 4 hours behind for read distribution. No stored procedures; no client-side distributed transactions.

### Etsy

| Domain | Data Store | Rationale |
|--------|-----------|-----------|
| Core relational | MySQL (sharded via Vitess) | ~1,000 tables across ~1,000 shards, 425TB, 1.7M requests/second |
| Caching | Redis + Memcached | Query caching, session storage, cart TTLs |
| API caching | Varnish | 80% cache hit rate, 10–15 minute staleness window accepted |
| Search | Elasticsearch + Solr (~50/50) | Full-text with ML ranking tuned to handcrafted listings |
| Analytics | GCP / BigQuery | ML model training, batch analytics |

**Key insight:** Etsy migrated from custom application-level sharding to Vitess in 2018. Vitess provides a MySQL-compatible query routing layer, schema migration safety (online DDL), and connection pooling that makes 1,000 shards manageable without application changes. User_id is the sharding key for user-owned tables; global tables are unsharded.

---

## 3. Event-Driven Patterns

### Amazon — SNS/SQS Fan-Out

Amazon's async workflows are built on **SNS → SQS fan-out**. A single SNS topic (e.g., `order-placed`) fans out to multiple SQS queues, each consumed by an independent service:

```
OrderService → SNS("order.placed") → SQS(FulfillmentService)
                                   → SQS(NotificationService)
                                   → SQS(InventoryService)
                                   → SQS(AnalyticsService)
```

SQS provides at-least-once delivery, durable message retention, and independent consumer scaling. SNS FIFO is used where ordering matters. EventBridge extends this for scheduled events and cross-account routing.

For high-throughput internal streams (clickstream, inventory movements), Amazon uses Kinesis Data Streams — partitioned by shard key (e.g., customer ID or product ID), with 24-hour default retention.

### Shopify — Kafka at 66M msg/sec

Shopify's Kafka cluster processes ~**66 million messages/second at peak**. Domain events flow through Kafka topics by business entity:

- `order.created`, `order.updated`, `order.cancelled`
- `product.updated`, `inventory.adjusted`
- `checkout.completed`

Downstream consumers include: search index updater, ML inference pipeline, notification dispatcher, analytics sink, fraud scoring service. Consumer groups allow multiple independent services to read the same topic at their own pace. Patterns used include event sourcing (Kafka as the event log), CQRS (write model via Rails, read model materialized by consumers), and at-least-once delivery with idempotent consumer logic.

Sidekiq (Redis-backed) handles job queues for lower-throughput async work: webhook delivery, email sends, payment retries, inventory sync with external channels.

### Flipkart — Kafka + RabbitMQ

Flipkart uses **Apache Kafka** as the primary event bus for high-throughput streams:
- Inventory updates from warehouse databases to the NRT Inventory Store
- Order events to downstream fulfillment, notification, and analytics services
- Real-time signals (clicks, purchases) to the recommendation engine (ALS on Spark)

**RabbitMQ** handles lower-latency point-to-point messaging for service-to-service commands where response is needed.

Pattern: catalog writes (low frequency, high fan-out) go to Kafka; inventory writes (very high frequency, targeted) go directly to the NRT store with Kafka as the audit log.

### eBay — Kafka + Event-Driven Listing Pipeline

eBay uses Kafka extensively in its classifieds and marketplace services. The Motor Vertical (and broader classifieds) uses Kafka as the primary inter-service communication bus:
- Seller service publishes listing events to Kafka
- Listing service subscribes and updates buyer search indexes
- Topic compaction keeps the latest state per key (behaves as a database changelog)

**Real-time indexing pipeline:** eBay uses a "fault-tolerant ETL" pipeline with two modes — real-time (Kafka-driven) and backfill — feeding Pronto's Elasticsearch clusters. Events trigger index updates within seconds of a listing change.

### Etsy — Kafka + GCP Pub/Sub

Etsy publishes domain events (listing updates, purchase events) to Kafka, consumed by:
- Search indexers (Elasticsearch/Solr updates)
- Recommendation signal collectors
- Analytics pipelines (GCP)

Less event-heavy than Shopify or Amazon; Etsy's approach is pragmatic — async where it reduces coupling, synchronous where consistency is critical.

---

## 4. Inventory Management

### The Core Problem

At scale, inventory is a **read-modify-write** race condition: multiple concurrent transactions read stock level N, all compute N-1, and all write N-1, resulting in over-selling. The solution space involves: atomic operations, row-level locking, optimistic locking, soft reservations, and bounded pools.

### Amazon — Soft Reservations with TTL

Amazon implements a **soft reservation** (also called a "hold") when a customer begins checkout:

1. Customer adds item to cart → no reservation yet (cart is eventually consistent)
2. Customer proceeds to checkout → inventory service creates a **soft reservation** with a **15-minute TTL**
3. If payment succeeds → reservation is converted to a confirmed deduction
4. If payment fails or TTL expires → reservation is released automatically

Strong consistency is required at reservation creation (Step 2) to prevent overselling. Amazon uses conditional writes in DynamoDB (`ConditionExpression: stock > 0`) to atomically check and decrement. The TTL-based expiry handles abandonment without requiring saga compensation.

### Shopify — MySQL SKIP LOCKED (Replaced Redis in 2025)

**Previous architecture (Redis):** One key per item with a quantity value. Reserve = `DECR`, release = `INCR`. Problem: reservations lived in Redis, the ledger lived in MySQL — two systems with no atomic cross-boundary guarantee. Two failure modes:
- Oversell: payment succeeds but Redis DECR didn't complete
- Undersell: stock deducted in MySQL but Redis reservation never cleaned up

**Current architecture (MySQL 8, SKIP LOCKED):**
- One row per sellable unit (a 10-unit item has 10 rows)
- Reserve 3 units = `SELECT ... FOR UPDATE SKIP LOCKED LIMIT 3` (atomically locks exactly 3 unlocked rows)
- SKIP LOCKED means concurrent reservations skip already-locked rows rather than waiting — eliminates hot-row contention
- Reserve + ledger update wrapped in a single ACID transaction
- Bounded pool: max 1,000 rows per item/location; a background replenishment process refills from the ledger
- Composite primary key (`shop_id, inventory_item_id, inventory_group_id, id`) reduces lock count from 2 to 1 per reservation
- `READ COMMITTED` isolation (not REPEATABLE READ) to avoid gap locks blocking replenishment inserts

Results: Writer CPU under 50%, reader CPU under 16%, $5.1M/minute peak on Black Friday 2025.

### Flipkart — NRT Inventory Store with Leader-Follower

Flipkart separates catalog (static, read-heavy: product name, images, specs) from inventory (dynamic, write-heavy: stock counts, price). The **NRT Inventory Store** uses leader-follower MySQL replication:

- All writes go to the leader (strong consistency for stock adjustments)
- Reads distributed across followers (eventual consistency acceptable for "in stock" display)
- Redis/Aerospike cache sits in front of followers for flash-sale pre-warming

Pre-BBD preparation: inventory caches are pre-loaded with expected hot SKUs before the sale begins, turning cold-cache misses into sub-millisecond hits.

For the order capture path, Flipkart's custom HBase (source-modified for strong consistency + basic index support) handles 500K QPS with multi-tenant isolation.

### eBay — Optimistic Locking + Modulo Sharding

eBay's inventory (listing quantity) uses **optimistic locking** at the database level:

```sql
UPDATE item SET quantity = quantity - 1, version = version + 1
WHERE item_id = ? AND version = ? AND quantity > 0
```

If the version check fails (concurrent modification), the transaction retries. This works at eBay's scale because listing-level contention is relatively low (unlike flash sales). Item databases are sharded by `item_id % N`, distributing hot items across shards.

For auction bidding (a distinct contention pattern), eBay uses a serialized bidding queue per item to prevent concurrent bids from conflicting.

---

## 5. Cart Architecture

### Amazon — DynamoDB, AP Semantics, Cookie-Based Guest Cart

**Data model:**
```
PK: cart_id (uuid, stored as cookie for guests)
SK: item_id
Attributes: quantity, price_at_add, seller_id, added_at
TTL: 7 days (authenticated), 1 day (anonymous)
```

**Design choices:**
- Cart is intentionally **eventually consistent** (AP, not CP)
- Adding to cart must always succeed, even during partial failures — losing an item is worse than showing a slightly stale count
- Guest carts keyed by a randomly generated UUID stored in a browser cookie
- On login: server-side merge — items from guest cart are unioned into the authenticated cart; conflicts (same item in both) take the higher quantity

**Dynamo properties applied:** sloppy quorum, hinted handoff for temporary failures, Merkle trees for background reconciliation. The "add to cart" was literally the motivating use case for the 2007 Dynamo paper.

### Shopify — MySQL per Pod (Shop-Level Isolation)

Shopify's cart lives inside the MySQL shard for the shop being purchased from. Every cart operation is scoped to a `shop_id`, making it trivial to route to the correct pod.

- **Session data:** Redis (per-pod), used for ephemeral checkout state
- **Persistent cart:** MySQL (ACID guarantees for multi-item carts)
- Checkout state (discounts, shipping, taxes) assembled at checkout time — not pre-computed in the cart — to reflect current prices

Cart-to-checkout transition involves assembling: line items + active discounts + tax calculations + available shipping options. Payment data never touches this layer — it is immediately encrypted by CardSink.

### Flipkart — Redis with Merger on Login

Cart architecture:
- **Anonymous cart:** Redis hash, keyed by a session cookie, TTL = session lifetime
- **Authenticated cart:** Redis hash, keyed by user ID, TTL = 30 days
- **Merge on login:** union strategy — for overlapping items, take max quantity; trigger "item no longer available" signals for OOS items in the merged cart
- **Persistence layer:** MySQL as the source of truth; Redis is a write-through cache

Flash sale carts get special treatment: item reservation happens at cart-add time (not checkout initiation) to prevent the "cart full but can't checkout" experience. This requires the inventory service to issue a soft reservation immediately on add.

### eBay — Session-Based with Guest Persistence

eBay's "cart" (called Buy It Now flow) is largely session-based for the checkout flow itself:
- Active bids and BIN orders are tracked per session
- Saved items (Watchlist) are persistent, stored in the user profile database (sharded Oracle)
- Multi-item checkout merges orders from different sellers into a single payment session
- Guest users can complete purchases without an account; cart state held in a signed, encrypted cookie

### Etsy — MySQL-Backed Persistent Cart

Etsy's cart is stored in MySQL (the monolith's database), consistent with their philosophy of not over-engineering:
- Carts are rows in a MySQL table, sharded by `user_id` via Vitess
- Anonymous carts use a UUID cookie; merged into user cart on login (last-write-wins for quantity conflicts)
- Redis/Memcached cache the cart display for active sessions

---

## 6. Search and Catalog

### Amazon — A9/A10 Proprietary Search

Amazon's search engine (A9, later evolved to A10) is entirely proprietary. Key characteristics:

**Catalog indexing:**
- Product data is denormalized into search documents at indexing time — all attributes (title, brand, category, ASIN, reviews count, Prime eligibility, price) stored in a flat document
- Near-real-time indexing: product updates propagate to search indexes in seconds via Kinesis streams
- Separate index shards for different categories (electronics, books, apparel) allow category-specific ranking models

**Ranking signals:**
- Keyword relevance (BM25)
- Sales velocity, CTR, conversion rate
- Seller performance metrics
- Price competitiveness
- Prime eligibility
- Review score and count

**Personalization:** Collaborative filtering overlaid on top of relevance ranking; browsing history and purchase history re-rank results per user.

**Scale:** The A9/A10 system handles hundreds of millions of product pages, serving results for hundreds of millions of active users. Results served via OpenSearch Service internally with custom ranking layers.

### Shopify — Kafka-Driven Index Updates

Shopify's search is merchant-scoped (each storefront searches only its own catalog):
- Kafka events (`product.updated`, `product.created`) trigger near-real-time updates to Elasticsearch indexes
- Each merchant's product catalog is an isolated Elasticsearch index
- ML embeddings (~2,500/second, 216M/day) power visual search and semantic similarity
- Images deduplicated before embedding to reduce storage and compute

### Flipkart — Solr + Elasticsearch Hybrid

**Scale:** 13M+ products across 900+ categories at the time of the 2013 Solr architecture (substantially larger today).

**Performance targets:**
- Search latency: ~75ms (p99)
- Autosuggest latency: ~5ms
- Availability: 99.99%
- Throughput: ~1,000 QPS baseline (spikes to 6–7× during BBD)

**Index design:**
- Documents denormalize catalog + inventory attributes (price, availability) into a single search document
- Problem with naive approach: 500+ dynamic fields × 10M products = ~17GB heap for field cache — solved by using Solr's external fields for ephemeral data (price, availability) loaded from files, not stored in-memory per document
- **Multi-layer caching:** Complete response objects cached keyed on all request parameters. Cache hit is 10–50× faster than a full query

**Relevance:**
- Handcrafted boosts for retail signals (margin, inventory, seller quality)
- User-feedback-based ranking (click-through, purchase signals)
- Query classification routes to category-specific ranking models
- Learning-to-Rank model in Solr using XGBoost

**Indexing pipeline:** Kafka events from the catalog service trigger near-real-time index updates (seconds latency); full re-index runs in batch nightly.

### eBay — Voyager + Pronto Elasticsearch Platform

**Voyager** (real-time search): eBay's own in-house search system using reliable multicast for real-time listing feeds, in-memory inverted index, horizontal segmentation across N slices load-balanced over M instances. Handles the core item search with sub-second latency across 2 billion+ active and completed listings.

**Pronto** (Elasticsearch-as-a-Service):
- **Scale:** 60+ clusters, 2,000+ nodes
- **Ingestion:** 18 billion documents/day
- **Queries:** 3.5 billion search requests/day
- Serves full-text search, log analytics, monitoring, seller analytics (Terapeak)
- Pronto team handles cluster lifecycle: provisioning, performance testing, tuning, monitoring

**Indexing pipeline:** Fault-tolerant ETL with real-time path (Kafka-driven, seconds latency) and backfill path (batch, for re-indexing). Events captured from marketplace changes trigger index updates via a reliable event stream.

**Query construction (250B search queries/day in peak years):** Z-axis sharding — search indexes partitioned by listing age, category, or item ID range; queries fan out across shards and results are merged.

### Etsy — Solr + Elasticsearch + Handcrafted Relevance

Etsy's search challenge is unique: listings are handmade, one-of-a-kind items with natural-language titles ("vintage mid-century brass candelabra") rather than structured product names.

**Technology:** ~50% Solr, ~50% Elasticsearch (in transition). ML ranking models trained specifically on buyer behavior signals from Etsy's marketplace.

**Index design:**
- Listing documents denormalize: title, tags, materials, seller location, shop name, price, shipping time, review count
- Personalization re-ranks based on buyer location (shipping time), past purchase categories, price range preferences
- "About half" of search traffic includes personalization signals

**Search quality investment:** Etsy treats search relevance as a core product — significant ML investment in learning-to-rank models that understand the semantic intent behind searches for handcrafted items.

---

## 7. Checkout Saga

The checkout flow is the most critical distributed transaction in e-commerce. It must coordinate: inventory reservation → payment authorization → order creation → fulfillment dispatch — across multiple independent services with no global transaction coordinator.

### Saga Pattern: Choreography vs Orchestration

**Choreography:** Each service publishes events that trigger the next step. No central coordinator.
```
CheckoutService → publishes OrderInitiated
InventoryService ← consumes OrderInitiated → reserves stock → publishes StockReserved
PaymentService ← consumes StockReserved → charges card → publishes PaymentCaptured
OrderService ← consumes PaymentCaptured → creates order → publishes OrderConfirmed
FulfillmentService ← consumes OrderConfirmed → creates shipment
```

**Orchestration:** A central Checkout Saga Orchestrator drives the workflow, calling each service and handling failures.

**Compensation on failure:**
```
FulfillmentService fails →
  OrderService.cancelOrder() →
  PaymentService.refund() →
  InventoryService.releaseReservation() →
  NotificationService.sendCancellationEmail()
```

### Amazon — Choreography with SQS/SNS

Amazon uses choreography-style saga via SNS fan-out → SQS queues:

1. **Checkout initiation:** Cart service reads cart items; calls Inventory service (strongly consistent) to create reservation (15-minute TTL)
2. **Payment:** Payment Orchestrator service calls tokenization vault → processor (synchronous, with idempotency key)
3. **Order creation:** On payment success, SNS event → Order service creates order record; inventory reservation is committed
4. **Fulfillment dispatch:** SNS event → Fulfillment Router selects warehouse → Warehouse Management System picks/packs/ships
5. **Notifications:** SNS → SQS → Notification service sends confirmation email/SMS

**Idempotency:** Every payment call carries a unique idempotency key (ULID or UUID). Amazon Pay's API: `x-amz-pay-idempotency-key` header. Retry of same key returns the saved response without re-charging.

**Failure rate benchmarks (industry):** inventory reservation fails ~1%, payment authorization fails ~3%, shipment creation fails ~0.5%.

### Shopify — Rails-Native Saga with Circuit Breakers

Shopify's checkout is largely handled within the Rails monolith (for atomicity) with PCI-isolated payment handling:

1. **Price lock:** At checkout start, current prices are locked into the checkout object (not cart)
2. **Reservation:** Flash sales use Redis/MySQL SKIP LOCKED reservation; normal flows use MySQL row-level locks
3. **Payment collection:** Checkout form posts to CardSink (isolates card data from monolith) → CardServer routes to payment processor
4. **Payment capture:** On authorization success, MySQL ACID transaction: reserve → capture → decrement inventory ledger (all in one transaction)
5. **Order creation:** MySQL insert with the shop_id shard key
6. **Async fulfillment:** Kafka event dispatched; Sidekiq job for notification delivery

**Circuit breakers:** Semian library protects Redis and MySQL from cascading failures. If MySQL becomes unresponsive, circuit opens and checkout fails fast rather than hanging.

**ULID for idempotency keys:** ULIDs (48-bit timestamp + 80-bit random) are used instead of random UUIDs for idempotency keys. Lexicographic sort order matches insertion order, reducing B-tree insert fragmentation. Shopify reports **50% reduction in INSERT statement duration** from switching to ULIDs.

### Flipkart — Kafka-Orchestrated Saga

Flipkart's checkout flow uses Kafka as the event bus with orchestration logic in an Order Capture service:

1. **Cart validation:** Verify all items still available at quoted price
2. **Inventory reservation:** Synchronous call to NRT Inventory Store (leader, strong consistency); if any item OOS, checkout blocked
3. **Payment processing:** Async via Kafka — `payment.initiated` event → Payment service → `payment.captured` or `payment.failed`
4. **Order creation:** On `payment.captured`, Order service creates the order in HBase
5. **Fulfillment dispatch:** `order.confirmed` event → Logistics service schedules pickup from seller/warehouse
6. **Notifications:** `order.confirmed` → Notification service (email/SMS)

**Compensation on failure:** If payment fails, `payment.failed` event triggers `inventory.release` event. The reservation TTL (a fallback) also eventually releases unreserved stock.

### eBay — Multi-Seller Checkout Aggregation

eBay's checkout is more complex because a buyer may purchase from multiple sellers in one session:

1. Items from different sellers are grouped by seller in the checkout
2. Each seller's inventory is checked independently (modulo-sharded Oracle)
3. A single payment is taken; eBay Managed Payments splits disbursements to sellers
4. Order records created per seller
5. Fulfillment is seller-managed (or eBay fulfillment for FBA-equivalent programs)

**Auction bidding** (distinct from BIN): Bids are serialized per item via a queue to prevent concurrent bid conflicts. The highest valid bid wins; auto-bidding increments to maximum.

---

## 8. Caching Strategy

### Layer Model (Applies Broadly)

```
Browser Cache (304 Not Modified, ETag)
    ↓
CDN Edge (CloudFront / Fastly / Cloudflare)
    ↓
Application Cache (Varnish / Nginx)
    ↓
In-Process Cache (local LRU in service)
    ↓
Distributed Cache (Redis / Memcached)
    ↓
Database Read Replica
    ↓
Primary Database
```

### Amazon

| Layer | What is Cached | TTL | Invalidation |
|-------|---------------|-----|--------------|
| CloudFront | Static assets (images, CSS, JS) | 30 days (versioned URLs) | URL versioning (content-addressed) |
| CloudFront | Product detail pages | 5–60 minutes | Tag-based purge on product update |
| ElastiCache (Redis) | Cart contents | 7 days (authenticated), 1 day (anonymous) | Write-through; explicit eviction on cart update |
| ElastiCache (Redis) | Session tokens | Session lifetime | TTL-based |
| ElastiCache (Memcached) | Product catalog data | Minutes to hours | Conditional invalidation on catalog event |

Product page caching is aggressive because 80%+ of product page views are repeat views. A cache hit for a popular product page avoids database queries, search index lookups, and recommendation service calls.

### Shopify

| Layer | What is Cached | TTL | Invalidation |
|-------|---------------|-----|--------------|
| CDN (Fastly) | Storefront pages (logged-out) | Short (varies) | Purge on product/theme change |
| Memcached (per pod) | Database query results, product listings | Minutes | Write-invalidate on mutation |
| Redis (per pod) | Sidekiq job queues, session data | Job-specific | Consumed on processing |

**Isolation principle:** Every cache (Redis and Memcached instance) is local to a pod. The "Redismageddon" incident — where a shared Redis cluster failed and took down all of Shopify — led to this architectural decision. Failure is now bounded to a single pod/shop group.

**Genghis load testing:** Shopify runs weekly realistic end-to-end load tests using Genghis (internal load generator) to validate cache hit rates and cache warming strategies before Black Friday.

### Flipkart

| Layer | What is Cached | TTL | Strategy |
|-------|---------------|-----|---------|
| CDN (Akamai/custom) | Product images, static assets | Long (versioned) | Purge on product image change |
| Edge cache | Product listing pages | Short | Purge on inventory/price change |
| Redis/Aerospike (pre-warmed) | Inventory counts for hot SKUs | Minutes | Pre-loaded before BBD flash sales |
| Redis | Session, cart | Session/30 days | Write-through |
| Solr in-memory cache | Complete search response objects | Request-scoped | Cache hit = 10–50× faster |

**BBD pre-warming:** Before Big Billion Days, Flipkart pre-loads the expected hot SKUs (known from pre-sale interest) into Redis. This converts what would be cold cache misses during the flash sale into cache hits, preventing a "thundering herd" on inventory databases.

### eBay

| Layer | What is Cached | Notes |
|-------|---------------|-------|
| Memcached | Session state, transient data | Never stored in app servers |
| Oracle read replicas (15 min – 4 hr lag) | Historical/analytical reads | Explicitly designed staleness accepted |
| Search (Pronto/Elasticsearch) | Recent search results | Short TTL; near-real-time index |

eBay's rule: transient state stored in cookies or scratch databases, **never in application servers**. This makes app servers fully stateless, enabling horizontal scaling with no session affinity.

### Etsy

| Layer | What is Cached | TTL | Notes |
|-------|---------------|-----|-------|
| Varnish | API responses | 10–15 min staleness accepted | 80% cache hit rate |
| Memcached | DB query results | Minutes | Standard read-through |
| Redis | Session, cart, rate limiting | Session-scoped | Write-through |

Etsy's API-first transformation includes Varnish as a reverse proxy cache for public API endpoints, accepting 10–15 minutes of staleness for non-personalized content in exchange for 80% cache efficiency.

---

## 9. Scaling Decisions

### Amazon

- **CQRS everywhere:** Write models (DynamoDB, RDS) separate from read models (ElastiCache, OpenSearch, Redshift). Product detail pages are pre-rendered from read models; catalog updates propagate asynchronously.
- **Functional decomposition first, then horizontal scale:** Services are small enough that each can be independently scaled. A spike in checkout traffic doesn't affect the search service.
- **Auto Scaling Groups:** EC2 ASGs with target tracking (CPU, request rate). RDS read replicas added dynamically during peaks.
- **Cell-based architecture:** AWS uses availability zones; retail uses regional isolation. A regional failure doesn't cascade globally.
- **Two-pizza team = bounded scope:** No single team owns a service large enough to be a bottleneck to decomposition.

### Shopify — Pod Model

**Core scaling primitive: the pod.** A pod is a fully isolated Shopify instance (MySQL shard + Redis + Memcached) deployed on GKE. To scale:
- Add more pods (horizontal scale)
- Migrate hot shops to under-loaded pods (Ghostferry, zero-downtime)
- Each pod is independently failsafe: a MySQL failure in one pod doesn't affect others

**Kubernetes on GKE:** Container-based workloads, 400,000+ unit tests, 15–20 minute build cycle. Canary deployments with feature flags.

**Black Friday 2024 stats:**
- 173 billion requests in 24 hours
- 284 million requests/minute at peak
- 12 TB egress/minute across edge network
- $5 billion GMV processed

**Black Friday 2025:**
- $5.1 million in sales per minute at peak
- SKIP LOCKED inventory system: writer CPU under 50%

### Flipkart

- **Kubernetes auto-scaling:** Microservices scale independently; during BBD, pods for inventory, cart, and checkout scale to 6–7× normal
- **Hybrid cloud burst:** Flipkart Cloud Platform (on-prem baseline) + GCP (burst capacity for BBD) — "millions of cores" of burst
- **HBase for order capture:** Custom HBase modifications for strong consistency + horizontal scale at 500K QPS
- **TiDB** for transactional workloads requiring 1M QPS with horizontal scaling
- **Circuit breakers (Phantom):** Inspired by Hystrix, Phantom proxies protect services from cascading failures; built on Netty + Unix Domain Sockets

### eBay

- **X-axis, Y-axis, Z-axis scaling:** X-axis = horizontal replication (stateless app servers); Y-axis = functional decomposition (buy vs. sell vs. search as separate tiers); Z-axis = data sharding (modulo by user ID, item ID, or hash)
- **15,000 J2EE application servers** (peak historical, per High Scalability)
- **600+ Oracle database instances** across 100+ clusters
- **No CPU work in the database:** All logic in the application tier; databases are pure storage
- **Asynchronous integration preferred:** Minimizes availability coupling between services
- **1,000+ microservices**, each with dedicated team and isolated deployment

### Etsy

- **Vitess for MySQL horizontal scale:** 1,000 shards, 1.7M requests/second, 425TB, managed as a single logical database
- **Single monolith + extracted services:** Reduces operational overhead; 95%+ of traffic handled by the monolith, which is deployed dozens of times per day
- **GKE-based container deployment**
- **Varnish for API cache hits:** 80% hit rate reduces load on backend services substantially

---

## 10. Failure Modes

### Oversell Prevention

**The problem:** Two concurrent requests read `qty = 1`, both proceed, both decrement → `qty = -1`.

| Platform | Mechanism |
|----------|-----------|
| Amazon | DynamoDB conditional write (`qty > 0`); soft reservation with TTL at checkout |
| Shopify | MySQL SKIP LOCKED (2025+); bounded row pool prevents hot-row contention |
| Flipkart | NRT Inventory Store on MySQL leader; flash-sale pre-warming + soft reservation at cart-add |
| eBay | Optimistic locking with version column; serialized auction bid queue |
| Etsy | MySQL row-level locks + Vitess routing |

**Flash sale oversell mitigation (Flipkart/Amazon):** For limited-quantity flash sales, inventory reservations are issued at **cart-add time** (not checkout), and the reservation pool is limited (queue-it style). This means only N customers can hold reservations simultaneously.

### Payment Failures and Idempotency

Every payment call is wrapped in an **idempotency key** — a unique token per checkout attempt:

```
POST /payments
  Idempotency-Key: ulid_01HV8X...   (or UUID)
  Body: { amount: 99.99, currency: "USD", token: "tok_xxx" }
```

If the network times out and the request is retried, the payment processor (or internal payment service) returns the same result for the same idempotency key without re-charging. The key is stored server-side with a TTL of 24–72 hours.

**Shopify's ULID advantage:** ULIDs contain a 48-bit millisecond timestamp, so they sort chronologically. B-tree indexes on idempotency keys benefit from sequential insertion → **50% reduction in INSERT duration**.

### At-Least-Once Delivery and Idempotent Consumers

SQS/Kafka deliver messages **at least once** — the same message may arrive multiple times (due to retries after consumer crash). Every consumer must be idempotent:
- **Fulfillment service:** check if shipment already created before creating a new one
- **Notification service:** check if email/SMS already sent before sending
- **Inventory service:** check if reservation already exists before creating a duplicate

Common pattern: store a `processed_message_ids` set in Redis with a TTL; reject duplicate message IDs.

### Checkout Saga Failure Recovery

**Compensation matrix:**

| Failed Step | Compensations Required |
|------------|----------------------|
| Payment fails | Release inventory reservation; notify customer; optionally retry with different payment method |
| Fulfillment fails | Refund payment; cancel order; release inventory; notify customer |
| Order creation fails | Refund payment; release inventory |
| Inventory reservation fails | Return to cart; show "out of stock" message |

**Timeout policy:** Reservations have TTLs (15 minutes at Amazon, comparable at others). If checkout is never completed, reservations expire automatically — no manual saga compensation needed for abandoned checkouts.

### Cascading Failure Prevention

| Platform | Mechanism |
|----------|-----------|
| Shopify | Semian circuit breaker (Redis + MySQL); Toxiproxy for chaos testing |
| Flipkart | Phantom proxy with Hystrix-style circuit breakers; fallback to degraded mode |
| Amazon | Service-level bulkheads; SQS dead-letter queues for failed messages |
| eBay | Circuit breaker pattern across 1,000+ services; asynchronous integration to minimize coupling |

**Graceful degradation examples:**
- Amazon: if recommendation service is down, show static "customers also bought" fallback
- Shopify: if Redis is unreachable, serve degraded mode (no background jobs; core checkout still works via MySQL)
- Flipkart: if NRT inventory store is lagging, serve cached inventory counts (accept stale reads temporarily)

### The Oversell Oracle — Shopify's Insight

The most important insight from Shopify's 2025 inventory migration: **the real failure mode was not the algorithm — it was connection pool exhaustion.** Reservations weren't slow; other checkout components were holding database connections longer than necessary, starving the reservation path. Adding SQL comment tags (`/* conn_tag:checkout_completion */`) at the ProxySQL layer to attribute connections to business processes identified the true bottleneck. Optimizing unrelated code removed 50% of reads and 33% of transactions from the primary, solving inventory reservation latency without touching the reservation code itself.

---

## Summary Comparison Table

| Dimension | Amazon | Shopify | Flipkart | eBay | Etsy |
|-----------|--------|---------|----------|------|------|
| Architecture style | Microservices (hundreds) | Modular monolith + pods | Microservices | Microservices (1,000+) | Hybrid (monolith core + services) |
| Primary language | Java, C++, Go | Ruby, Rust, Go | Java (Spring Boot) | Java (Spring Boot) | PHP, Scala, Java |
| Transactional DB | DynamoDB, RDS Aurora | MySQL (sharded, petabyte) | MySQL, TiDB, HBase | Oracle (primary), Cassandra | MySQL via Vitess |
| Cart store | DynamoDB (eventual consistency) | MySQL per pod | Redis + MySQL | Session cookie + Oracle watchlist | MySQL (Vitess) |
| Inventory mechanism | DynamoDB conditional writes + 15-min reservation | MySQL SKIP LOCKED (row-per-unit, 2025) | NRT Store + Redis pre-warm | Optimistic locking + version column | MySQL row locks |
| Search | A9/A10 proprietary + OpenSearch | Kafka-driven Elasticsearch | Solr + Elasticsearch (75ms target) | Voyager + Pronto (60+ ES clusters, 3.5B queries/day) | Solr + Elasticsearch |
| Event bus | SQS/SNS/Kinesis | Kafka (66M msg/sec peak) | Kafka + RabbitMQ | Kafka | Kafka + GCP Pub/Sub |
| Caching | CloudFront + ElastiCache | Fastly CDN + Memcached/Redis per pod | CDN + Redis/Aerospike (pre-warmed) | Memcached + Oracle read replicas | Varnish (80% hit rate) + Redis |
| Checkout saga style | Choreography (SNS/SQS) | Orchestration (Rails + Kafka async) | Kafka orchestration | Choreography + serialized auction queue | Rails monolith + async Kafka |
| Scale (peak) | N/A publicly disclosed | $5.1M/min, 284M RPM (BFCM 2025) | 6–7× spike, 300M visitors (BBD) | 130M active buyers, $74B/year GMV | Not publicly disclosed |
| Idempotency | `x-amz-pay-idempotency-key` | ULID keys (50% faster inserts) | UUID-based | UUID-based | UUID-based |

---

## Key Architectural Principles Across All Five

1. **Separate reads from writes (CQRS):** High-read catalog/product data serves from caches and search indexes, never from the transactional database.

2. **Bound the blast radius:** Pod architecture (Shopify), functional sharding (eBay), microservice isolation (Amazon/Flipkart) — failures should affect the smallest possible surface area.

3. **Accept staleness where correctness is not critical:** Cart display (Amazon), API responses (Etsy/Varnish), inventory counts (Flipkart NRT followers) — eventual consistency is acceptable when the cost of inconsistency is low.

4. **Strong consistency only where it matters:** Inventory reservation at checkout, payment capture, order creation — these require ACID semantics or conditional atomic writes.

5. **Async everything non-critical:** Email, notifications, search index updates, analytics — all handled asynchronously via Kafka/SQS to keep the critical path thin and fast.

6. **Idempotency keys are mandatory for payments:** The retry problem in distributed systems makes deduplication at the payment layer non-negotiable.

7. **Pre-warm, not heal:** Pre-loading caches before flash sales (Flipkart, Amazon) is safer than cache stampede recovery during a sale.

8. **Test at scale continuously:** Shopify runs weekly full-scale load tests via Genghis; Amazon's chaos engineering practice (origin of Netflix's Chaos Monkey concept) validates failure recovery continuously.

---

Sources:
- [How Amazon Scaled E-commerce Shopping Cart Data Infrastructure](https://newsletter.systemdesign.one/p/amazon-dynamo-architecture)
- [Shopify's Architecture to Handle the World's Biggest Flash Sales - InfoQ](https://www.infoq.com/presentations/shopify-architecture-flash-sale/)
- [We replaced Redis with MySQL for inventory reservations—and it scaled (2026) - Shopify](https://shopify.engineering/scaling-inventory-reservations)
- [Shopify Tech Stack - ByteByteGo](https://blog.bytebytego.com/p/shopify-tech-stack)
- [How Shopify Manages its Petabyte Scale MySQL Database - ByteByteGo](https://blog.bytebytego.com/p/how-shopify-manages-its-petabyte)
- [Horizontally scaling the Rails backend of Shop app with Vitess - Shopify](https://shopify.engineering/horizontally-scaling-the-rails-backend-of-shop-app-with-vitess)
- [Shard Balancing: Moving Shops Confidently with Zero-Downtime - Shopify](https://shopify.engineering/mysql-database-shard-balancing-terabyte-scale)
- [A Pods Architecture To Allow Shopify To Scale - Shopify](https://shopify.engineering/a-pods-architecture-to-allow-shopify-to-scale)
- [eBay Architecture - High Scalability](https://highscalability.com/ebay-architecture/)
- [Inside EBay Tech Stack And Infrastructure - Appscrip](https://appscrip.com/blog/ebay-tech-stack-and-infrastructure/)
- [Elasticsearch Cluster Lifecycle at eBay](https://innovation.ebayinc.com/stories/elasticsearch-cluster-lifecycle-at-ebay/)
- [Elasticsearch Performance Tuning Practice at eBay](https://innovation.ebayinc.com/stories/elasticsearch-performance-tuning-practice-at-ebay/)
- [Cassandra at eBay - Cassandra Summit 2013](https://www.slideshare.net/jaykumarpatel/cassandra-at-ebay-cassandra-summit-2013)
- [Flipkart Big Billion Days TechStack - DEV Community](https://dev.to/zeeshanali0704/flipkart-big-billion-days-techblog-1b4j)
- [Search@Flipkart - SlideShare](https://www.slideshare.net/umeshprasad/searchflipkart)
- [Scaling write-heavy OLTP systems with strong data guarantees - Flipkart/HasGeek](https://hasgeek.com/fifthelephant/2018/sub/scaling-write-heavy-oltp-systems-with-strong-data-HtFnXZaYhVYxoJmQrh6DMR)
- [Etsy Engineering - Migrating Etsy's database sharding to Vitess](https://www.etsy.com/codeascraft/migrating-etsyas-database-sharding-to-vitess)
- [Etsy Engineering - Code as Craft blog](https://www.etsy.com/codeascraft)
- [Inside Etsy Tech Stack And Infrastructure - Appscrip](https://appscrip.com/blog/inside-etsy-tech-stack-and-infrastructure/)
- [Amazon Two-Pizza Teams - AWS Executive Insights](https://aws.amazon.com/executive-insights/content/amazon-two-pizza-team/)
- [Werner Vogels on 21st Century Cloud Architectures - InfoQ](https://www.infoq.com/news/2017/12/vogels-21st-century-architecture/)
- [Dynamo: Amazon's Highly Available Key-value Store (2007 Paper)](https://www.allthingsdistributed.com/files/amazon-dynamo-sosp2007.pdf)
- [How Shopify's Event-Driven & Streaming Architecture Powers 66M Kafka Msg/sec](https://medium.com/@shaikhjavedmail/how-shopifys-event-driven-streaming-architecture-powers-66m-kafka-msg-sec-32e6f25534af)
- [Microservices and Kafka - eBay Classifieds](https://medium.com/berlin-tech-blog/microservices-and-kafka-part-1-614767d27b20)
- [Building a Serverless Event-Driven Retail Order Management System - AWS](https://aws.amazon.com/blogs/industries/building-a-serverless-event-driven-retail-order-management-system/)
- [Idempotency - Amazon Pay](https://developer.amazon.com/docs/amazon-pay-api-v2/idempotency.html)
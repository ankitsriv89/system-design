# 23 — E-Commerce Platform

Catalog, cart, checkout, inventory reservation, and event-driven order fulfillment. The checkout flow atomically reserves inventory, creates an order, and publishes a Kafka event that drives a payment saga to completion — all without a distributed transaction.

**Stack:** Java 21 · Spring Boot 3.3.4 · PostgreSQL 16 · Redis · Kafka · Prometheus

---

## Quick Start

```bash
# 1. Bring up the shared infra (PostgreSQL, Redis, Kafka, Prometheus, Grafana)
cd infra && docker compose up -d && cd ..

# 2. Start the e-commerce service
cd projects/23-e-commerce-platform
docker compose up -d --build

# 3. Wait for health
curl http://localhost:8104/actuator/health
# → {"status":"UP"}

# 4. Seed sample products
bash scripts/seed.sh

# 5. Open the web UI
open http://localhost:8104
```

---

## Key Design Decisions

### Inventory reservation (oversell prevention)
```sql
UPDATE products SET stock = stock - :qty WHERE id = :id AND stock >= :qty
```
A single atomic UPDATE — zero rows affected means out of stock — no SELECT-then-UPDATE race. All items in an order are reserved in one `@Transactional` block; any failure rolls back the entire set.

### Idempotent checkout
The `orders` table has a `UNIQUE` constraint on `idempotency_key`. The same checkout request (same key) returns the existing order without re-executing side effects.

### Order saga (choreography)
Checkout publishes `OrderEvent` to Kafka → `PaymentSagaConsumer` picks it up and approves payment (mock) → order transitions to `CONFIRMED`. On failure, `InventoryService.release()` is called as a compensation step to restore stock.

### Cart in Redis
Cart is a JSON blob with a 7-day TTL — high write frequency, loss-tolerant. If Redis is unavailable the service degrades gracefully (cart starts empty; no crash).

---

## API Cheat Sheet

```bash
# Create a product
curl -X POST http://localhost:8104/v1/products \
  -H "Content-Type: application/json" \
  -d '{"sku":"BOOK-001","name":"The Pragmatic Programmer","price":42.99,"stock":50,"category":"books"}'

# Add to cart
curl -X POST http://localhost:8104/v1/cart/user-42/items \
  -H "Content-Type: application/json" \
  -d '{"productId":"<id>","quantity":2}'

# Checkout (idempotent)
curl -X POST http://localhost:8104/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"userId":"user-42","idempotencyKey":"order-001"}'

# Get order (poll for saga completion)
curl http://localhost:8104/v1/orders/<order-id>

# Admin: ship an order
curl -X POST http://localhost:8104/v1/admin/orders/<order-id>/ship
```

Full reference: [docs/api.md](docs/api.md)

---

## Order Status Flow

```
PENDING → INVENTORY_RESERVED → PAYMENT_AUTHORIZED → CONFIRMED → SHIPPED → DELIVERED
                ↓ (failure)
        INVENTORY_FAILED / PAYMENT_FAILED (stock released)
```

---

## Scripts

```bash
bash scripts/seed.sh              # create 6 sample products
bash scripts/integration_test.sh  # 8 scenarios: CRUD, saga, oversell, idempotency
bash scripts/load_test.sh         # concurrent throughput benchmark
```

---

## Observability

- **Health:** `http://localhost:8104/actuator/health`
- **Metrics:** `http://localhost:8104/actuator/prometheus`
- **Grafana:** `http://localhost:3000`

---

## Docs

| File | Contents |
|------|----------|
| [docs/architecture.md](docs/architecture.md) | System diagram, sequence diagrams, capacity estimates, failure modes |
| [docs/code-flow.md](docs/code-flow.md) | Function-level flowcharts for checkout, saga, cart, inventory |
| [docs/api.md](docs/api.md) | Full API reference with curl examples |
| [docs/build-log.md](docs/build-log.md) | Build output, dependencies, implementation decisions |
| [docs/changelog.md](docs/changelog.md) | Version history |
| [ecomm-architecture-reference.md](ecomm-architecture-reference.md) | Real-world architecture deep-dive: Amazon, Shopify, Flipkart, eBay, Etsy |

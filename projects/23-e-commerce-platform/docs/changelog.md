# Changelog

## [0.1.0] — 2026-06-07

### Added
- Domain: `Product`, `Cart`, `CartItem`, `Order`, `OrderItem`, `InventoryReservation`, `OrderStatus` enum.
- Flyway migration `V1__init.sql`: products, orders, order_items, inventory_reservations tables with indexes.
- Repositories: `ProductRepository` (atomic `decrementStock`/`incrementStock`), `OrderRepository` (idempotency key lookup), `InventoryReservationRepository`.
- Services: `CatalogService` (product CRUD + text search + Redis cache), `CartService` (Redis-backed cart), `InventoryService` (atomic reservation + compensation), `OrderService` (state machine), `CheckoutService` (saga orchestrator with idempotency).
- Kafka: `OrderEvent` DTO, `EventPublisher`, `PaymentSagaConsumer` (mock auto-approve).
- Store adapters: `CartStore` (Redis JSON), `EventPublisher` (Kafka).
- REST API: `ProductController`, `CartController`, `OrderController`, `AdminController`.
- Spring configs: `KafkaConfig` (idempotent producer, typed consumer), `RedisConfig` (cache manager + TTL), `SecurityConfig` (open for demo), `WebConfig` (static file serving).
- Three-panel web UI: catalog search, cart management, checkout button, order saga visualiser with live status polling, admin ship/deliver controls; all DOM writes use `esc()` helper to prevent XSS.
- Scripts: `seed.sh`, `integration_test.sh` (8 scenarios including oversell + saga), `load_test.sh`.
- Docs: `architecture.md`, `code-flow.md`, `api.md`, `build-log.md`.
- Dockerfile (multi-stage Gradle + Temurin JRE 21 Alpine).
- Docker Compose wired to shared `infra` network on port 8104.
- `infra/initdb/23_ecommerce.sql` — user + database provisioning.

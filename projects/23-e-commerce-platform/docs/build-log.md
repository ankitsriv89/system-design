# Build Log — E-Commerce Platform (Project 23)

## Environment

| Item | Value |
|------|-------|
| Java version | 21 (Eclipse Temurin) |
| Build tool | Gradle 8 |
| Spring Boot | 3.3.4 |
| Module path | `com.ankitsriv89.ecommerce` |

## Dependencies

| Artifact | Version | Role |
|----------|---------|------|
| `spring-boot-starter-web` | 3.3.4 | REST API, Jackson, Tomcat |
| `spring-boot-starter-data-jpa` | 3.3.4 | JPA / Hibernate ORM |
| `spring-boot-starter-data-redis` | 3.3.4 | Redis client (Lettuce) |
| `spring-boot-starter-actuator` | 3.3.4 | Health + Prometheus endpoint |
| `spring-boot-starter-validation` | 3.3.4 | Bean validation on domain objects |
| `spring-boot-starter-security` | 3.3.4 | Security filter chain (open for demo) |
| `spring-kafka` | 3.x | Kafka producer + listener |
| `micrometer-registry-prometheus` | latest | Metrics exposition |
| `postgresql` | latest | JDBC driver |
| `flyway-core` + `flyway-database-postgresql` | latest | Schema migrations |
| `jackson-datatype-jsr310` | latest | Java time serialization |
| `opensearch-rest-high-level-client` | 2.11.0 | Search (wired, not yet used in MVP) |
| `spring-boot-starter-test` | 3.3.4 | JUnit 5, Mockito |
| `spring-kafka-test` | 3.x | Kafka test utilities |

## Build Output

```
$ ./gradlew test --info 2>&1 | tail -30

> Task :test
CheckoutServiceTest > checkout_emptyCart_throwsIllegalState() PASSED
CheckoutServiceTest > checkout_idempotent_returnsSameOrder() PASSED
CheckoutServiceTest > inventoryService_insufficientStock_throws() PASSED
CheckoutServiceTest > inventoryService_sufficientStock_reserves() PASSED
CheckoutServiceTest > partitionFor_keyBased_isDeterministic() PASSED

BUILD SUCCESSFUL in 18s
5 tests, 0 failures
```

## Build Decisions

- **Atomic stock decrement via JPQL UPDATE**: `UPDATE Product p SET p.stock = p.stock - :qty WHERE p.id = :id AND p.stock >= :qty` runs as a single SQL statement checked-and-decremented atomically under PostgreSQL row locking. Zero rows affected signals out-of-stock and triggers a full transaction rollback — no separate SELECT needed, no TOCTOU race.
- **Idempotency on `idempotency_key` unique constraint**: the database enforces uniqueness; the service layer simply catches the constraint violation. Retrying with the same key is free.
- **Cart in Redis, not PostgreSQL**: Cart mutation is high-frequency and low-durability (stale carts are acceptable). Redis TTL naturally handles abandonment. If Redis is unavailable, `CartStore.get()` returns `Optional.empty()` so the service degrades gracefully without crashing.
- **`@EnableScheduling`** on the application class: reserved for future scheduled tasks (cart expiry cleanup, search re-index).
- **Mock payment saga**: `PaymentSagaConsumer` auto-approves all orders so the full saga is demonstrable without a payment gateway. The transaction boundaries are real — payment failure path triggers `InventoryService.release()` compensation.
- **`open-in-view: false`**: prevents the anti-pattern of lazy-loading JPA associations during HTTP response serialization. All associations that need to be serialized are fetched eagerly or in the service layer.
- **No `@Transactional` on controllers**: transactions are scoped to the service layer only, keeping HTTP concerns separate from database concerns.

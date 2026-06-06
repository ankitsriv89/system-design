# Build Log — WhatsApp Real-Time Messaging (Project 20)

## Environment

| | |
|---|---|
| Java | 21 (eclipse-temurin:21-jre-alpine in container) |
| Gradle | 8.10 (wrapper) |
| Spring Boot | 3.3.4 |
| Build tool | `./gradlew test bootJar --no-daemon` |
| Module / image | `com.ankitsriv89.whatsapp` / `20-whatsapp-real-time-messaging:latest` |

## Direct dependencies

| Dependency | Version | Role |
|---|---|---|
| `spring-boot-starter-web` | 3.3.4 | REST controllers, Jackson, embedded Tomcat |
| `spring-boot-starter-websocket` | 3.3.4 | Spring `TextWebSocketHandler`, SockJS support |
| `spring-boot-starter-data-jpa` | 3.3.4 | Hibernate ORM + Spring Data repositories |
| `spring-boot-starter-data-redis` | 3.3.4 | `StringRedisTemplate`, Lettuce client |
| `spring-boot-starter-security` | 3.3.4 | JWT filter, BCrypt, stateless CSRF-disabled config |
| `spring-boot-starter-actuator` | 3.3.4 | `/actuator/health`, `/actuator/prometheus` |
| `spring-boot-starter-validation` | 3.3.4 | `@Valid`, `@NotBlank`, `@Size` on DTOs |
| `spring-kafka` | 3.2.4 | `KafkaTemplate<String,String>`, `@KafkaListener`, topic auto-create |
| `micrometer-registry-prometheus` | 1.13.4 | Prometheus metrics export |
| `postgresql` | 42.7.4 | JDBC driver |
| `flyway-core` | 10.10.0 | Schema migration runner |
| `flyway-database-postgresql` | 10.10.0 | Flyway Postgres dialect |
| `jjwt-api` | 0.12.6 | JWT builder / parser API |
| `jjwt-impl` | 0.12.6 | HS256 implementation |
| `jjwt-jackson` | 0.12.6 | Jackson-based JWT serialisation |
| `jackson-databind` | 2.17.2 | JSON serialisation (via Spring Boot BOM) |

## Test output

```
> Task :compileJava
> Task :processResources
> Task :classes
> Task :resolveMainClassName
> Task :bootJar
> Task :compileTestJava
> Task :processTestResources NO-SOURCE
> Task :testClasses
> Task :test

BUILD SUCCESSFUL in ~54s
6 actionable tasks: 5 executed, 1 up-to-date
```

### Tests

| Test class | Tests | Result |
|---|---|---|
| `ReceiptStateTest` | 6 | PASSED |
| `MessageServiceResolveRecipientsTest` | 5 | PASSED |
| `WhatsappApplicationTests` | 1 | PASSED (context load skipped — requires Postgres/Redis/Kafka) |

## Build decisions

**String-based Kafka serialisation**: Producer and consumer both use `StringSerializer`/`StringDeserializer` with manual Jackson `ObjectMapper` in `MessageRouter`. This avoids Spring Kafka's type-header magic (`__TypeId__`) which breaks across consumer group rebalances when event types evolve. The trade-off is explicit `mapper.readValue` calls.

**One-time WS ticket in Redis**: JWT is excluded from the WebSocket URL by issuing a 30 s `GETDEL` ticket. Alternatives considered: (a) Sec-WebSocket-Protocol header — rejected because Spring's `WebSocketConfigurer` doesn't parse subprotocols by default without STOMP; (b) cookie — rejected for simplicity and CORS concerns in the demo.

**In-memory password map in `AuthService`**: The demo stores password hashes in a `ConcurrentHashMap` rather than a `passwords` Postgres table. This avoids a schema migration and a dedicated `app_user_credential` entity, acceptable for a portfolio demo. Production would use a proper credential store.

**`AccessDeniedException` for 403**: Spring Security maps `AccessDeniedException` to HTTP 403 automatically when the request is authenticated. This keeps authorization logic out of the HTTP layer.

## Security fixes applied

| Finding | Fix |
|---|---|
| JWT in WS query string | One-time Redis ticket via `POST /v1/ws-ticket` |
| IDOR on `/v1/messages/sync` | `assertParticipant()` verifies caller is in DM pair or group |
| IDOR on `/v1/receipts` | `device.getUser().getUsername().equals(callerUsername)` check |
| Missing sender-is-participant in `send()` | `assertParticipant()` called at top of `MessageService.send()` |
| Receipt broadcast to all sessions | `KafkaReceiptEvent` carries `participantUserIds`; router pushes only to participant devices |
| Weak default JWT secret | `JwtService` constructor enforces ≥ 32 bytes; logs warning if dev placeholder used |

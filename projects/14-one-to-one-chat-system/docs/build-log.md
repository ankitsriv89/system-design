# Build Log — 14: One-to-One Chat System

## Environment

| Item | Value |
|---|---|
| Java | OpenJDK 21.0.10 (Ubuntu 24.04 ARM64) |
| Gradle | 8.10 (via wrapper) |
| Spring Boot | 3.3.4 |
| OS | Linux 6.8.0 aarch64 |

## Module

`com.ankitsriv89:14-one-to-one-chat-system:0.1.0`

## Direct dependencies

| Dependency | Version | Role |
|---|---|---|
| `spring-boot-starter-web` | 3.3.4 | HTTP server, REST controllers, Jackson |
| `spring-boot-starter-websocket` | 3.3.4 | STOMP over SockJS, SimpMessagingTemplate |
| `spring-boot-starter-data-jpa` | 3.3.4 | Hibernate ORM, JPA repositories |
| `spring-boot-starter-data-redis` | 3.3.4 | StringRedisTemplate, Redis connection pool |
| `spring-boot-starter-security` | 3.3.4 | JWT filter chain, CSRF disabled, stateless sessions |
| `spring-boot-starter-actuator` | 3.3.4 | `/actuator/health`, `/actuator/prometheus` |
| `spring-kafka` | 3.2.x | KafkaTemplate producer, @KafkaListener consumer |
| `micrometer-registry-prometheus` | 1.13.x | Prometheus metrics export |
| `postgresql` | 42.7.x | JDBC driver |
| `flyway-core` | 10.x | Schema migrations (V1__init.sql) |
| `flyway-database-postgresql` | 10.x | Flyway PostgreSQL dialect support |
| `jjwt-api` | 0.12.6 | JWT token interface |
| `jjwt-impl` | 0.12.6 | JWT implementation (runtime) |
| `jjwt-jackson` | 0.12.6 | JWT Jackson JSON serialiser (runtime) |
| `spring-boot-starter-test` | 3.3.4 | JUnit 5, Mockito, AssertJ |
| `spring-kafka-test` | 3.2.x | Embedded Kafka for integration tests |
| `spring-security-test` | 6.3.x | SecurityMockMvcRequestPostProcessors |

## Build output

```
> Task :compileJava
> Task :processResources
> Task :classes
> Task :compileTestJava
> Task :processTestResources NO-SOURCE
> Task :testClasses
> Task :test

BUILD SUCCESSFUL in 20m 48s
4 actionable tasks: 4 executed
```

(First build time dominated by Gradle 8.10 wrapper download ~18 min; subsequent builds ~30 s)

## Test results

| Test class | Tests | Result |
|---|---|---|
| `ConversationServiceTest` | 6 | ✅ PASS |
| `MessageDomainTest` | 5 | ✅ PASS |
| `JwtUtilTest` | 3 | ✅ PASS |
| **Total** | **14** | **✅ all green** |

## Build decisions

- **Gradle 8.10** chosen over Maven for cleaner Spring Boot DSL and faster incremental builds.
- **Flyway** (not `ddl-auto=create`) so schema is explicit, auditable, and safe on restarts.
- **`jjwt 0.12.6`** — latest stable; uses the builder/parser fluent API (older `0.9.x` API deprecated).
- **`eclipse-temurin:21-jre-alpine`** for the Docker final stage — smallest JRE image for ARM64 with security patches.
- **`-Xmx512m -Xms256m`** JVM flags in Dockerfile — conservative for t4g.large shared with other Java projects.
- Deprecation warning about Gradle 9.0 incompatibility is from Spring Boot 3.3.x plugin — no impact on build correctness.

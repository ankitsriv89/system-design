# Build Log — News Feed System (Project 16)

## Stack

| Item | Value |
|---|---|
| Java | OpenJDK 21.0.10 (Ubuntu 24.04) |
| Spring Boot | 3.3.4 |
| Gradle | 8.10 |
| Group / version | `com.ankitsriv89` / `0.1.0` |
| Base package | `com.ankitsriv89.newsfeed` |
| Server port | 8097 |

## Direct dependencies

| Artifact | Resolved version | Role |
|---|---|---|
| `spring-boot-starter-web` | 3.3.4 | REST controllers, embedded Tomcat |
| `spring-boot-starter-data-jpa` | 3.3.4 | JPA / Hibernate ORM, Hikari pool |
| `spring-boot-starter-data-redis` | 3.3.4 | Spring Data Redis / Lettuce client |
| `spring-boot-starter-security` | 3.3.4 | JWT filter, stateless security config |
| `spring-boot-starter-actuator` | 3.3.4 | `/actuator/health`, `/actuator/prometheus` |
| `spring-boot-starter-validation` | 3.3.4 | `@Valid` / `@NotBlank` on request DTOs |
| `spring-kafka` | 3.2.4 | Kafka producer (EventPublisher) + `@KafkaListener` (FanoutService) |
| `micrometer-registry-prometheus` | 1.13.4 | Prometheus metrics export |
| `postgresql` | 42.7.4 | JDBC driver |
| `flyway-core` | 10.10.0 | Schema migrations (`V1__init.sql`) |
| `flyway-database-postgresql` | 10.10.0 | Flyway PostgreSQL dialect support |
| `jjwt-api` | 0.12.6 | JWT API (compile) |
| `jackson-databind` | 2.17.2 | JSON serialization |
| `jjwt-impl` | 0.12.6 | JWT runtime implementation |
| `jjwt-jackson` | 0.12.6 | JWT Jackson integration (runtime) |

## Build commands

```
./gradlew test
```

## Build output

```
> Task :compileJava UP-TO-DATE
> Task :processResources UP-TO-DATE
> Task :classes UP-TO-DATE
> Task :compileTestJava UP-TO-DATE
> Task :processTestResources NO-SOURCE
> Task :testClasses UP-TO-DATE
> Task :test UP-TO-DATE

BUILD SUCCESSFUL in ~15s
4 actionable tasks: 4 up-to-date
```

## Test results

4 unit tests in `RankingServiceTest` — all green.

| Test | Assertion |
|---|---|
| `freshPostScoresNearOne` | A post aged 0 s scores ≥ 0.999 |
| `scoreHalvesAtHalfLife` | Post aged exactly `halfLifeHours` scores 0.5 ± 0.001 |
| `oldPostScoresLow` | Post aged 10 × half-life scores < 0.01 |
| `laterPostScoresHigher` | Newer post always outscores older post |

## Notable build decisions

**`afterCommit` event publish** — The `post.created` Kafka event is published
inside a `TransactionSynchronization.afterCommit()` hook rather than
inline in the `@Transactional` method. This ensures the row is durable in
PostgreSQL before any consumer processes it, preventing fanout of rolled-back
posts.

**Ranking bug caught by tests** — The initial implementation used
`exp(-age/τ)` (which halves at `0.693·τ`). The unit test `scoreHalvesAtHalfLife`
caught this: the score at exactly `halfLifeHours` was 0.632, not 0.5. Fixed to
`exp(-ln2 · age / halfLife)` so the half-life semantics are literally accurate.

**Gradle 8 deprecation warning** — Gradle reports compatibility warnings for
Gradle 9.0 (`--warning-mode all`). No action required for this project; the
warnings are in Spring Boot plugin internals, not project code.

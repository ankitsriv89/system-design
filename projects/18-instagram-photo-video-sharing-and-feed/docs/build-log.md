# Build Log — Project 18

## Toolchain

- **Language**: Java 21 (Gradle toolchain)
- **Framework**: Spring Boot 3.3.0
- **Build**: Gradle (wrapper), `io.spring.dependency-management` 1.1.5
- **Base package**: `com.ankitsriv89.instagram`

## Direct dependencies

| Dependency | Role |
|---|---|
| `spring-boot-starter-web` | REST controllers, embedded Tomcat |
| `spring-boot-starter-data-jpa` | Postgres entities & repositories |
| `spring-boot-starter-data-redis` | Timeline ZSETs + like counters |
| `spring-boot-starter-actuator` | Health / Prometheus endpoints (blocked at public proxy) |
| `spring-boot-starter-validation` | Request DTO validation |
| `spring-kafka` | Event producer + `@KafkaListener` workers |
| `flyway-core` + `flyway-database-postgresql` | Schema migration (`V1__init.sql`) |
| `micrometer-registry-prometheus` | Metrics export |
| `io.minio:minio:8.5.10` | S3-compatible object storage client |
| `net.coobird:thumbnailator:0.4.20` | Image variant generation (pure Java) |
| `postgresql` (runtime) | JDBC driver |
| `lombok` (compile-only) | Boilerplate reduction |

### Test dependencies

`spring-boot-starter-test` (JUnit 5, Mockito, AssertJ), `spring-kafka-test`,
`testcontainers` (junit-jupiter, postgresql, kafka, minio).

## Build output

```
$ ./gradlew clean build --no-daemon

> Task :compileJava
> Task :bootJar
> Task :compileTestJava
> Task :test

BUILD SUCCESSFUL in 1m 25s
```

## Test results

15 tests, 0 failures, 0 errors across 5 classes:

| Test class | Tests | Covers |
|---|---|---|
| `InstagramApplicationTests` (MediaService) | 4 | begin/complete upload, ownership, missing-object |
| `VariantGeneratorTest` | 3 | real image resize, content-type discrimination, bad input |
| `EngagementServiceTest` | 3 | like/unlike idempotency, counter behavior |
| `FanoutServiceTest` | 2 | push for normal author, skip for celebrity |
| `MediaUrlServiceTest` | 3 | local URL, CDN URL, variant-map mapping |

Artifacts: `build/libs/instagram-0.0.1-SNAPSHOT.jar` (executable, ~95 MB).

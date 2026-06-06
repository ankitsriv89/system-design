# Build Log — Twitter/X Timeline and Posts

## Toolchain

- **Java**: 21 (Temurin / OpenJDK `21.0.10`)
- **Build**: Gradle 8.10 wrapper, Spring Boot Gradle plugin `3.3.4`
- **Base package**: `com.ankitsriv89.twitter`
- **Image tag**: `19-twitter-x-timeline-and-posts:latest`
- **Host port**: `8100` · Caddy path `/p19/`
- **Prometheus prefix**: `twitter_`

## Direct dependencies

| Dependency | Version | Role |
|---|---|---|
| `spring-boot-starter-web` | 3.3.4 | REST controllers |
| `spring-boot-starter-data-jpa` | 3.3.4 | Tweets + follow graph (PostgreSQL) |
| `spring-boot-starter-data-redis` | 3.3.4 | Home-timeline sorted sets |
| `spring-boot-starter-security` | 3.3.4 | JWT auth filter chain |
| `spring-boot-starter-actuator` | 3.3.4 | Health + Prometheus endpoint |
| `spring-boot-starter-validation` | 3.3.4 | Request DTO validation |
| `spring-kafka` | (managed) | `tweet.created` producer + 2 consumer groups |
| `micrometer-registry-prometheus` | (managed) | Metrics export |
| `postgresql` | (managed) | JDBC driver |
| `flyway-core` + `flyway-database-postgresql` | (managed) | Schema migration `V1__init.sql` |
| `jjwt-api/impl/jackson` | 0.12.6 | HS256 demo tokens |
| `opensearch-rest-high-level-client` | 2.11.1 | Search + trends index/query |

## Architecture decisions

- **Hybrid fanout** mirrors project 16 (news-feed): push into Redis ZSETs on
  write, skip celebrities (followers > `celebrity-threshold`, default 1000),
  pull them on read. Keeps both paths bounded.
- **Two Kafka consumer groups** on one topic (`twitter-fanout`,
  `twitter-search-indexer`) isolate timeline delivery from search indexing, so
  search lag never blocks the feed. Both are idempotent under at-least-once
  delivery (ZADD by member; OpenSearch doc id = tweetId).
- **Publish after commit** (`TransactionSynchronization.afterCommit`) so an
  event is never emitted for a tweet that didn't persist.
- **OpenSearch bundled per-project** (single-node, security disabled) like the
  MinIO origin in project 18 — tweet text is project-specific and kept off the
  shared infra boxes. Search + trends both read the same `tweets` index;
  hashtags are stored as a keyword array so the terms aggregation counts whole
  tags.
- **Soft delete** keeps the audit row and is honored lazily at read time.

## Build & test output

```
$ ./gradlew test bootJar --no-daemon
> Task :compileJava
> Task :bootJar
> Task :compileTestJava
> Task :test
BUILD SUCCESSFUL
```

- `RankingServiceTest` (5 tests) — time-decay scoring (fresh ≈ 1.0, halves every
  half-life, monotonic decay) and hashtag extraction (lower-casing, dedupe
  count, empty case). Pure logic, no Spring context / no external services.
- Produced fat jar: `build/libs/19-twitter-x-timeline-and-posts-0.1.0.jar`.

## Notes / known limitations

- The **live integration test** (`scripts/integration_test.sh`) exercises the
  full happy path (auth → follow → tweet → fanout → home → search → trends →
  delete → auth boundaries) against a running stack. It was **not** run in this
  build because no Docker daemon was available; the compose config validates and
  the project compiles + unit-tests clean. Run it after `docker compose up` once
  the shared `infra` network (postgres/redis/kafka) and the bundled OpenSearch
  are healthy.
- `TimelineStore.pushMany` issues per-key ZADDs; at scale this is where Redis
  pipelining/sharding would go.
- Demo auth has no user store — tokens are unverified identities for the tutorial.

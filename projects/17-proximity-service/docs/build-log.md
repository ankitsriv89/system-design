# Build Log — Proximity Service (Project 17)

## Stack

| Component | Version |
|---|---|
| Java | 21 (Eclipse Temurin) |
| Spring Boot | 3.3.0 |
| Gradle Wrapper | 8.10 |
| Spring Data Redis | 3.3.0 (via BOM) |
| Spring Kafka | 3.1.4 (via BOM) |
| Flyway | 10.x (via BOM) |
| PostgreSQL driver | 42.x (via BOM) |
| postgis-jdbc | 2023.1.0 |
| Lombok | 1.18.x (via BOM) |
| Micrometer Prometheus | 1.13.x (via BOM) |

## Build command

```bash
./gradlew bootJar --no-daemon
```

## Compile fixes applied

**GeoStore — `StringRedisTemplate` generic mismatch**

`StringRedisTemplate.opsForGeo()` returns `GeoOperations<String, String>`.
The `radius(key, Point, Distance, args)` overload takes a `String` key and a
`Point` (the centre), not a `String` for the centre member. Fixed by switching
to the `radius(key, Circle, args)` overload which wraps the centre point and
radius in a `Circle` object, making the types unambiguous.

## Design decisions

**No net.postgis geometry column in JPA entity**

PostGIS `GEOMETRY(Point,4326)` is defined in the Flyway migration and indexed
via GIST for raw SQL / future PostGIS queries. The JPA `Location` entity
does not map it — Hibernate's dialect does not ship with a PostGIS geometry
type out of the box and adding the full `hibernate-spatial` dependency pulls in
a large tree. For MVP the GIST index is available for DBA / raw JDBC queries;
spatial queries are served by Redis GEO on the hot path.

**Pure-Java geohash encoder**

Chose to implement `GeohashUtil` in-process rather than pulling `ch.hsr.geohash`
or `com.github.davidmoten:geo` to keep the dependency footprint minimal. The
Base32 bit-interleave algorithm is ~40 lines and fully testable without I/O.

**Testcontainers removed from unit tests**

The unit tests (`GeohashUtilTest`, `LocationServiceTest`) use no infrastructure.
Testcontainers was declared in `build.gradle` for potential integration tests
but removed from the default test run because Maven Central is not reachable
in the build environment. Integration testing is covered by
`scripts/integration_test.sh` against the running Docker Compose stack.

## Test results

Unit tests (GeohashUtilTest, LocationServiceTest): **3 passed, 0 failed**

Integration tests: run manually with:
```bash
docker compose up -d
BASE_URL=http://localhost:8098 ./scripts/integration_test.sh
```

# Code Flow — Proximity Service (Project 17)

## Entry point

`ProximityServiceApplication.main()` boots Spring Boot, which:
1. Applies Flyway migrations (PostGIS extension + DDL)
2. Registers Kafka topics via `KafkaConfig` `@Bean` methods
3. Starts `LocationExpiryWorker` scheduler
4. Listens on port 8098

---

## Operation: Update Location (`POST /v1/locations`)

```mermaid
flowchart TD
    A[HTTP POST /v1/locations] --> B[LocationController.updateLocation]
    B --> B1[Set MDC requestId]
    B --> C[LocationService.updateLocation]
    C --> D[GeohashUtil.encode lat lng]
    D --> D1[Base32 bit-interleave encode\nprecision=9 ~4.8m accuracy]
    C --> E{findByUserId in PG}
    E -->|exists| F[update lat/lng/geohash/updatedAt]
    E -->|new| G[new Location entity]
    F --> H[locationRepo.save — SQL UPSERT]
    G --> H
    H --> I[geoStore.upsert\nGEOADD geo:locations lon lat userId]
    I --> J[kafkaTemplate.send location.updated]
    J --> K[return Location to controller]
    K --> L[200 OK + JSON]
```

**Why both stores?**
Redis GEO is ephemeral — data is TTL-evicted. PostgreSQL is the system of
record. Writing both on every update keeps them consistent until the TTL worker
runs. The JPA save is inside a `@Transactional` boundary; Redis and Kafka writes
happen after the transaction commits (post-commit effect not explicitly isolated
here — acceptable for MVP, a `TransactionSynchronization.afterCommit` hook
would harden it for production).

---

## Operation: Nearby Query (`GET /v1/nearby`)

```mermaid
flowchart TD
    A[HTTP GET /v1/nearby?userId&lat&lng&radiusMeters] --> B[LocationController.nearby]
    B --> C[LocationService.findNearby]
    C --> D[Timer.record start]
    C --> E[GeoStore.nearby lat lon radiusMeters]
    E --> F[GEORADIUS geo:locations circle 200 results ASC]
    F --> G[List of userId strings]
    G --> H{for each candidate}
    H --> I[filter userId == requesterId — skip self]
    H --> J[visibilityRepo.findById userId]
    J -->|HIDDEN| K[exclude]
    J -->|PUBLIC or FRIENDS or absent| L[include]
    L --> M[collect visible list]
    M --> N[Timer.record stop — prometheus]
    N --> O[kafkaTemplate.send nearby.queried]
    O --> P[return visible list]
    P --> Q[200 OK JSON]
```

**Why filter in process, not in Redis?**
Redis GEO has no concept of visibility. The per-user mode is a low-cardinality
join (one row per user in PG), so filtering in the application after a Redis
radius query is cheaper than adding a secondary index or Lua script. For
production at scale, visibility could be cached in Redis hashes to avoid PG
round-trips per candidate.

---

## Operation: Delete Location (`DELETE /v1/locations/{userId}`)

```mermaid
flowchart TD
    A[HTTP DELETE /v1/locations/userId] --> B[LocationController.deleteLocation]
    B --> C[LocationService.deleteLocation]
    C --> D[locationRepo.deleteByUserId — @Modifying JPQL]
    D --> E[geoStore.remove\nGEOREM geo:locations userId]
    E --> F[kafkaTemplate.send location.expired reason=manual]
    F --> G[204 No Content]
```

---

## Operation: Expiry Worker (scheduled)

```mermaid
flowchart TD
    A[Spring @Scheduled fixed-delay = TTL seconds] --> B[LocationExpiryWorker.evictStale]
    B --> C[locationRepo.findAll]
    C --> D{filter updatedAt < now - TTL}
    D -->|stale| E[locationRepo.delete]
    E --> F[geoStore.remove]
    F --> G[kafkaTemplate.send location.expired reason=ttl_expired]
    G --> H[log INFO evicted userId]
    D -->|fresh| I[skip]
```

**Trade-off note:** `findAll()` is O(N) — fine for MVP but must be replaced
with a range query `WHERE updated_at < :cutoff` for production. A Redis sorted
set keyed by timestamp could drive expiry with zero PostgreSQL scans.

---

## Operation: Update Visibility (`PUT /v1/visibility/{userId}`)

```mermaid
flowchart TD
    A[HTTP PUT /v1/visibility/userId] --> B[LocationController.updateVisibility]
    B --> C[parse VisibilityMode from body.mode]
    C --> D[LocationService.updateVisibility]
    D --> E{visibilityRepo.findById}
    E -->|exists| F[update mode + updatedAt]
    E -->|new| G[new VisibilitySetting]
    F --> H[visibilityRepo.save]
    G --> H
    H --> I[200 OK + VisibilitySetting JSON]
```

---

## Call graph summary

```mermaid
graph LR
    LC[LocationController] --> LS[LocationService]
    LS --> GH[GeohashUtil]
    LS --> GS[GeoStore]
    LS --> LR[LocationRepository]
    LS --> VR[VisibilityRepository]
    LS --> KT[KafkaTemplate]
    GS --> RD[(Redis)]
    LR --> PG[(PostgreSQL)]
    VR --> PG
    EW[ExpiryWorker] --> LR
    EW --> GS
    EW --> KT
```

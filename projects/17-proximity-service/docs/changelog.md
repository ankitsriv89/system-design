# Changelog — Proximity Service (Project 17)

All notable changes to this project will be documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] — 2026-06-06

### Added
- `POST /v1/locations` — upsert current location (Redis GEO + PostgreSQL)
- `GET /v1/nearby` — find users within radius, privacy-filtered
- `GET /v1/locations/{userId}` — fetch last known location
- `DELETE /v1/locations/{userId}` — remove location from both stores
- `PUT /v1/visibility/{userId}` — set PUBLIC / FRIENDS / HIDDEN mode
- Pure-Java geohash encoder (`GeohashUtil`) — Base32, precision 9 (~4.8m)
- PostGIS Flyway migration — `locations` table with GIST spatial index
- Redis GEO adapter (`GeoStore`) — GEOADD / GEORADIUS
- Location TTL expiry worker — scheduled eviction with Kafka event
- Kafka topics: `location.updated`, `nearby.queried`, `location.expired`
- Prometheus metrics: location update counter, nearby query counter + latency timer
- Three-panel demo UI — canvas map with user dots and query radius circle
- Integration test script (`scripts/integration_test.sh`)
- Unit tests for `GeohashUtil` and `LocationService` visibility filtering

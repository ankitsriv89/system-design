# API — Proximity Service (Project 17)

Base URL (local): `http://localhost:8098`

All endpoints accept and return `application/json`.

---

## POST /v1/locations

Upsert the current location for a user. Creates the entry if it does not exist;
updates lat/lng/geohash and `updated_at` otherwise. Writes to both Redis GEO
and PostgreSQL and publishes a `location.updated` Kafka event.

```bash
curl -X POST http://localhost:8098/v1/locations \
  -H 'Content-Type: application/json' \
  -d '{"userId":"alice","lat":37.7749,"lng":-122.4194}'
```

`200 OK`:
```json
{
  "id": 1,
  "userId": "alice",
  "lat": 37.7749,
  "lng": -122.4194,
  "geohash": "9q8yy9mzu",
  "updatedAt": "2026-06-06T10:00:00Z",
  "createdAt": "2026-06-06T09:00:00Z"
}
```

**Errors:**
- `400` — missing or malformed body

---

## GET /v1/nearby

Find users within `radiusMeters` of the given coordinates. Results are sorted
nearest-first. Hidden users and the requester themselves are excluded.

```bash
curl "http://localhost:8098/v1/nearby?userId=alice&lat=37.7749&lng=-122.4194&radiusMeters=1000"
```

`200 OK`:
```json
{
  "nearby": ["bob", "carol"],
  "count": 2
}
```

**Query parameters:**

| Param | Type | Required | Default | Notes |
|---|---|---|---|---|
| `userId` | string | yes | — | requester; excluded from results |
| `lat` | double | yes | — | WGS84 latitude |
| `lng` | double | yes | — | WGS84 longitude |
| `radiusMeters` | double | no | 1000 | max 50000 |

**Errors:**
- `400` — missing required params

---

## GET /v1/locations/{userId}

Fetch the last known location for a specific user.

```bash
curl http://localhost:8098/v1/locations/alice
```

`200 OK`:
```json
{
  "id": 1,
  "userId": "alice",
  "lat": 37.7749,
  "lng": -122.4194,
  "geohash": "9q8yy9mzu",
  "updatedAt": "2026-06-06T10:00:00Z",
  "createdAt": "2026-06-06T09:00:00Z"
}
```

**Errors:**
- `404` — user has no location record

---

## DELETE /v1/locations/{userId}

Remove a user's location from both Redis GEO and PostgreSQL. Publishes a
`location.expired` event with `reason=manual`. Returns `204 No Content`
even if the user had no location entry.

```bash
curl -X DELETE http://localhost:8098/v1/locations/alice
```

`204 No Content`

---

## PUT /v1/visibility/{userId}

Set privacy mode for a user. `HIDDEN` users never appear in `/v1/nearby` results.
`PUBLIC` and `FRIENDS` are treated equivalently in the MVP (full friend-graph
enforcement is a production extension).

```bash
curl -X PUT http://localhost:8098/v1/visibility/alice \
  -H 'Content-Type: application/json' \
  -d '{"mode":"HIDDEN"}'
```

`200 OK`:
```json
{
  "userId": "alice",
  "mode": "HIDDEN",
  "updatedAt": "2026-06-06T10:05:00Z"
}
```

**Valid modes:** `PUBLIC`, `FRIENDS`, `HIDDEN`

**Errors:**
- `400` — unknown mode value

---

## Health & metrics

```bash
# Liveness
curl http://localhost:8098/actuator/health

# Prometheus metrics
curl http://localhost:8098/actuator/prometheus | grep proximity_
```

Key metrics:
- `proximity_location_updates_total` — cumulative location writes
- `proximity_nearby_queries_total` — cumulative nearby queries
- `proximity_nearby_query_latency_seconds_{count,sum,max}` — latency distribution

---

## End-to-end demo sequence

```bash
BASE=http://localhost:8098

# 1. Seed users around San Francisco
curl -sX POST $BASE/v1/locations -H 'Content-Type: application/json' \
  -d '{"userId":"alice","lat":37.7749,"lng":-122.4194}'
curl -sX POST $BASE/v1/locations -H 'Content-Type: application/json' \
  -d '{"userId":"bob","lat":37.7755,"lng":-122.4185}'
curl -sX POST $BASE/v1/locations -H 'Content-Type: application/json' \
  -d '{"userId":"carol","lat":37.9000,"lng":-122.5000}'

# 2. Query nearby from alice's position, 1km radius → should return bob only
curl -s "$BASE/v1/nearby?userId=alice&lat=37.7749&lng=-122.4194&radiusMeters=1000"

# 3. Hide bob
curl -sX PUT $BASE/v1/visibility/bob \
  -H 'Content-Type: application/json' -d '{"mode":"HIDDEN"}'

# 4. Query again → should return empty (bob hidden, carol too far)
curl -s "$BASE/v1/nearby?userId=alice&lat=37.7749&lng=-122.4194&radiusMeters=1000"

# 5. Delete alice's location
curl -sX DELETE $BASE/v1/locations/alice
```

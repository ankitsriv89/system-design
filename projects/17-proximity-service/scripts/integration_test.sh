#!/usr/bin/env bash
# Integration tests for proximity-service
# Requires: curl, jq
# Usage: BASE_URL=http://localhost:8098 ./scripts/integration_test.sh

set -euo pipefail

BASE="${BASE_URL:-http://localhost:8098}"
PASS=0
FAIL=0

ok()   { echo "  ✓ $1"; ((PASS++)); }
fail() { echo "  ✗ $1"; ((FAIL++)); }

check_status() {
  local got=$1 want=$2 label=$3
  [ "$got" -eq "$want" ] && ok "$label (HTTP $got)" || fail "$label (want $want got $got)"
}

echo "=== Proximity Service Integration Tests ==="
echo "  Base: $BASE"
echo ""

# Health
echo "── Health ──────────────────────────────────────"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/actuator/health")
check_status "$STATUS" 200 "health endpoint"

# Update locations
echo "── Location update ─────────────────────────────"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/locations" \
  -H 'Content-Type: application/json' \
  -d '{"userId":"u1","lat":37.7749,"lng":-122.4194}')
check_status "$STATUS" 200 "POST /v1/locations (u1)"

STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/locations" \
  -H 'Content-Type: application/json' \
  -d '{"userId":"u2","lat":37.7755,"lng":-122.4185}')
check_status "$STATUS" 200 "POST /v1/locations (u2)"

STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/locations" \
  -H 'Content-Type: application/json' \
  -d '{"userId":"u3","lat":37.9000,"lng":-122.5000}')
check_status "$STATUS" 200 "POST /v1/locations (u3 — far away)"

# Get location
echo "── Get location ────────────────────────────────"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/locations/u1")
check_status "$STATUS" 200 "GET /v1/locations/u1"

STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/locations/nonexistent")
check_status "$STATUS" 404 "GET /v1/locations/nonexistent → 404"

# Nearby query
echo "── Nearby query ────────────────────────────────"
RESP=$(curl -s "$BASE/v1/nearby?userId=u1&lat=37.7749&lng=-122.4194&radiusMeters=1000")
COUNT=$(echo "$RESP" | jq '.count')
[ "$COUNT" -ge 1 ] && ok "nearby finds u2 within 1km (count=$COUNT)" || fail "nearby returned count=$COUNT, expected >= 1"

# u3 should NOT appear within 1km
echo "$RESP" | jq -r '.nearby[]' | grep -q u3 \
  && fail "u3 should not appear within 1km" \
  || ok "u3 correctly excluded (too far)"

# Visibility — hide u2
echo "── Visibility ──────────────────────────────────"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE/v1/visibility/u2" \
  -H 'Content-Type: application/json' \
  -d '{"mode":"HIDDEN"}')
check_status "$STATUS" 200 "PUT /v1/visibility/u2 HIDDEN"

RESP=$(curl -s "$BASE/v1/nearby?userId=u1&lat=37.7749&lng=-122.4194&radiusMeters=1000")
echo "$RESP" | jq -r '.nearby[]' | grep -q u2 \
  && fail "hidden u2 should not appear in nearby" \
  || ok "hidden u2 correctly excluded from nearby"

# Delete location
echo "── Delete location ─────────────────────────────"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE/v1/locations/u1")
check_status "$STATUS" 204 "DELETE /v1/locations/u1"

STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/locations/u1")
check_status "$STATUS" 404 "GET /v1/locations/u1 after delete → 404"

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1

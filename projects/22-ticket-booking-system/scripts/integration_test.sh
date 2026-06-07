#!/usr/bin/env bash
# Integration test for 22-ticket-booking-system
# Requires: curl, jq, docker compose up (service healthy on port 8103)
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8103}"
PASS=0; FAIL=0

ok()   { echo "  PASS: $*"; ((PASS++)); }
fail() { echo "  FAIL: $*"; ((FAIL++)); }

assert_eq() {
  local label="$1" expected="$2" actual="$3"
  if [ "$actual" = "$expected" ]; then ok "$label"; else fail "$label (expected=$expected actual=$actual)"; fi
}

# ── Health ──────────────────────────────────────────────────────────────────
echo "=== Health ==="
STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "$BASE/actuator/health")
assert_eq "health check returns 200" "200" "$STATUS"

# ── Create event ────────────────────────────────────────────────────────────
echo "=== Create Event ==="
EVENT=$(curl -sf -X POST "$BASE/v1/events" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Test Concert","venue":"Arena","eventTime":"2027-01-01T20:00:00Z","totalSeats":30}')
EVENT_ID=$(echo "$EVENT" | jq -r '.id')
assert_eq "event id present" "false" "$([ -z "$EVENT_ID" ] && echo true || echo false)"

# ── Get seats ───────────────────────────────────────────────────────────────
echo "=== Get Seats ==="
SEATS=$(curl -sf "$BASE/v1/events/$EVENT_ID/seats")
AVAIL_COUNT=$(echo "$SEATS" | jq '[.[] | select(.status=="AVAILABLE")] | length')
assert_eq "30 seats available" "30" "$AVAIL_COUNT"

FIRST_SEAT=$(echo "$SEATS" | jq -r '.[0].id')

# ── Place hold ──────────────────────────────────────────────────────────────
echo "=== Place Hold ==="
HOLD=$(curl -sf -X POST "$BASE/v1/holds" \
  -H 'Content-Type: application/json' \
  -d "{\"seatId\":\"$FIRST_SEAT\",\"userId\":\"test-user\"}")
HOLD_ID=$(echo "$HOLD" | jq -r '.id')
HOLD_STATUS=$(echo "$HOLD" | jq -r '.status')
assert_eq "hold status ACTIVE" "ACTIVE" "$HOLD_STATUS"

# Seat should now show as HELD
STATS=$(curl -sf "$BASE/v1/events/$EVENT_ID/seats/stats")
HELD_COUNT=$(echo "$STATS" | jq -r '.held')
assert_eq "1 seat held" "1" "$HELD_COUNT"

# ── Duplicate hold rejected ──────────────────────────────────────────────────
echo "=== Duplicate Hold Rejection ==="
DUP_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/holds" \
  -H 'Content-Type: application/json' \
  -d "{\"seatId\":\"$FIRST_SEAT\",\"userId\":\"test-user-2\"}" || true)
assert_eq "duplicate hold returns 409" "409" "$DUP_STATUS"

# ── Checkout (idempotent) ───────────────────────────────────────────────────
echo "=== Checkout ==="
IDEM_KEY="test-idem-$(date +%s)"
BOOKING=$(curl -sf -X POST "$BASE/v1/bookings" \
  -H 'Content-Type: application/json' \
  -d "{\"holdId\":\"$HOLD_ID\",\"userId\":\"test-user\",\"amount\":\"99.99\",\"idempotencyKey\":\"$IDEM_KEY\"}")
BOOKING_ID=$(echo "$BOOKING" | jq -r '.id')
PAYMENT_STATUS=$(echo "$BOOKING" | jq -r '.paymentStatus')
assert_eq "booking payment COMPLETED" "COMPLETED" "$PAYMENT_STATUS"

# Retry with same idempotency key — must return same booking
BOOKING2=$(curl -sf -X POST "$BASE/v1/bookings" \
  -H 'Content-Type: application/json' \
  -d "{\"holdId\":\"$HOLD_ID\",\"userId\":\"test-user\",\"amount\":\"99.99\",\"idempotencyKey\":\"$IDEM_KEY\"}")
BOOKING2_ID=$(echo "$BOOKING2" | jq -r '.id')
assert_eq "idempotent checkout same id" "$BOOKING_ID" "$BOOKING2_ID"

# Seat should now be BOOKED
STATS2=$(curl -sf "$BASE/v1/events/$EVENT_ID/seats/stats")
BOOKED_COUNT=$(echo "$STATS2" | jq -r '.booked')
assert_eq "1 seat booked" "1" "$BOOKED_COUNT"

# ── Payment failure (amount=0) ──────────────────────────────────────────────
echo "=== Payment Failure ==="
SECOND_SEAT=$(curl -sf "$BASE/v1/events/$EVENT_ID/seats" | jq -r '[.[] | select(.status=="AVAILABLE")][0].id')
HOLD2=$(curl -sf -X POST "$BASE/v1/holds" \
  -H 'Content-Type: application/json' \
  -d "{\"seatId\":\"$SECOND_SEAT\",\"userId\":\"test-user\"}")
HOLD2_ID=$(echo "$HOLD2" | jq -r '.id')
FAIL_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/bookings" \
  -H 'Content-Type: application/json' \
  -d "{\"holdId\":\"$HOLD2_ID\",\"userId\":\"test-user\",\"amount\":\"0\"}" || true)
assert_eq "zero-amount payment returns 409" "409" "$FAIL_STATUS"

# ── Summary ─────────────────────────────────────────────────────────────────
echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1

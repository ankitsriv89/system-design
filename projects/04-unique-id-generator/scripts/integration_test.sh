#!/usr/bin/env bash
# integration_test.sh — end-to-end correctness checks against a live service.
# Requires: curl, jq, bc
# Usage: ./scripts/integration_test.sh [base_url]
set -euo pipefail

BASE="${1:-http://localhost:8083}"
PASS=0
FAIL=0

pass() { echo "  PASS: $1"; ((PASS++)); }
fail() { echo "  FAIL: $1"; ((FAIL++)); }

echo "=== integration tests against $BASE ==="

# 1. Healthz returns ok
echo "--- healthz ---"
STATUS=$(curl -sf "$BASE/healthz" | jq -r '.status')
[ "$STATUS" = "ok" ] && pass "healthz returns ok" || fail "healthz returned: $STATUS"

# 2. Worker health returns a valid worker_id
echo "--- worker health ---"
WID=$(curl -sf "$BASE/v1/workers/health" | jq -r '.worker_id')
[ "$WID" -ge 0 ] && [ "$WID" -le 1023 ] && pass "worker_id in range [0,1023]: $WID" || fail "invalid worker_id: $WID"

# 3. Single ID is a positive integer
echo "--- single ID ---"
ID=$(curl -sf -X POST "$BASE/v1/ids/next" | jq -r '.id')
[ "$ID" -gt 0 ] && pass "single ID is positive: $ID" || fail "bad ID: $ID"

# 4. IDs are monotonically increasing across 10 calls
echo "--- monotonicity ---"
PREV=0
MONOTONIC=true
for i in $(seq 1 10); do
  CUR=$(curl -sf -X POST "$BASE/v1/ids/next" | jq -r '.id')
  if [ "$CUR" -le "$PREV" ]; then
    MONOTONIC=false
    fail "non-monotonic: $CUR <= $PREV"
    break
  fi
  PREV="$CUR"
done
[ "$MONOTONIC" = true ] && pass "10 sequential IDs are strictly increasing"

# 5. Batch returns the requested count with no duplicates
echo "--- batch uniqueness ---"
BATCH=$(curl -sf -X POST "$BASE/v1/ids/batch" \
  -H "Content-Type: application/json" \
  -d '{"count": 100}')
COUNT=$(echo "$BATCH" | jq '.count')
UNIQUE=$(echo "$BATCH" | jq '[.ids[]] | unique | length')
[ "$COUNT" -eq 100 ] && pass "batch count=100" || fail "batch count: $COUNT"
[ "$UNIQUE" -eq 100 ] && pass "batch 100 unique IDs" || fail "duplicates found: unique=$UNIQUE"

# 6. Inspect decodes a valid ID
echo "--- inspect ---"
ID_STR=$(curl -sf -X POST "$BASE/v1/ids/next" | jq -r '.id_string')
INSP=$(curl -sf "$BASE/v1/ids/$ID_STR/inspect")
TS=$(echo "$INSP" | jq -r '.timestamp_ms')
WID=$(echo "$INSP" | jq -r '.worker_id')
SEQ=$(echo "$INSP" | jq -r '.sequence')
NOW_MS=$(date +%s%3N)
AGE_MS=$(echo "$NOW_MS - $TS" | bc)
[ "$AGE_MS" -lt 5000 ] && pass "inspect timestamp within 5s: age=${AGE_MS}ms" || fail "inspect timestamp too old: age=${AGE_MS}ms"
[ "$WID" -ge 0 ] && [ "$WID" -le 1023 ] && pass "inspect worker_id valid: $WID" || fail "inspect worker_id invalid: $WID"
[ "$SEQ" -ge 0 ] && [ "$SEQ" -le 4095 ] && pass "inspect sequence valid: $SEQ" || fail "inspect sequence invalid: $SEQ"

# 7. Batch rejects count=0 and count=1001
echo "--- batch validation ---"
HTTP400=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/ids/batch" \
  -H "Content-Type: application/json" -d '{"count": 0}')
[ "$HTTP400" = "400" ] && pass "batch count=0 returns 400" || fail "batch count=0 returned: $HTTP400"

HTTP400B=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/ids/batch" \
  -H "Content-Type: application/json" -d '{"count": 1001}')
[ "$HTTP400B" = "400" ] && pass "batch count=1001 returns 400" || fail "batch count=1001 returned: $HTTP400B"

# 8. Metrics endpoint is reachable
echo "--- metrics ---"
HTTP200=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/metrics")
[ "$HTTP200" = "200" ] && pass "metrics endpoint reachable" || fail "metrics returned: $HTTP200"

echo ""
echo "Results: $PASS passed, $FAIL failed."
[ "$FAIL" -eq 0 ] || exit 1

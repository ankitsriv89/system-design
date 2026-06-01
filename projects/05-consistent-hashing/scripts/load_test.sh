#!/usr/bin/env bash
set -euo pipefail

BASE="${API_URL:-http://localhost:8084}"
RING="load-test-$$"
CONCURRENCY="${CONCURRENCY:-20}"
REQUESTS="${REQUESTS:-5000}"

echo "=== Consistent Hashing Load Test ==="
echo "  Base: $BASE | Concurrency: $CONCURRENCY | Requests: $REQUESTS"

# Setup
curl -sf -X POST "$BASE/v1/rings" \
  -H 'Content-Type: application/json' \
  -d "{\"id\":\"$RING\",\"replicas\":150}" >/dev/null

for n in alpha beta gamma delta epsilon; do
  curl -sf -X POST "$BASE/v1/rings/$RING/nodes" \
    -H 'Content-Type: application/json' \
    -d "{\"id\":\"$n\",\"weight\":1}" >/dev/null
done

echo "--- Lookup throughput (ab)"
if command -v ab &>/dev/null; then
  ab -n "$REQUESTS" -c "$CONCURRENCY" -q \
    "$BASE/v1/rings/$RING/keys/bench-key/owner" 2>&1 | grep -E 'Requests per|Time per|Transfer rate|Failed'
else
  echo "  ab not found; using sequential curl timing"
  START=$(date +%s%N)
  for i in $(seq 1 200); do
    curl -sf "$BASE/v1/rings/$RING/keys/key-$i/owner" >/dev/null
  done
  END=$(date +%s%N)
  ELAPSED=$(( (END - START) / 1000000 ))
  echo "  200 sequential lookups in ${ELAPSED}ms = $(( 200 * 1000 / ELAPSED )) req/s"
fi

# Rebalance timing
echo "--- Rebalance timing"
TIME_START=$(date +%s%N)
curl -sf -X POST "$BASE/v1/rings/$RING/nodes" \
  -H 'Content-Type: application/json' \
  -d '{"id":"new-node","weight":1}' | grep -o '"MovedPct":[0-9.]*' || true
TIME_END=$(date +%s%N)
echo "  Add node took $(( (TIME_END - TIME_START) / 1000000 ))ms"

# Cleanup
curl -sf -X DELETE "$BASE/v1/rings/$RING" >/dev/null

echo "=== Load test complete ==="

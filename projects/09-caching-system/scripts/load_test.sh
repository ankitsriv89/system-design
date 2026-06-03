#!/usr/bin/env bash
# Load test — measures throughput and latency for GET and SET.
# Requires: hey (https://github.com/rakyll/hey) or falls back to curl loop.
# Usage: bash scripts/load_test.sh
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8090}"
REQUESTS="${REQUESTS:-2000}"
CONCURRENCY="${CONCURRENCY:-50}"

echo "=== Load Test (${BASE}) ==="
echo "  Requests: ${REQUESTS}, Concurrency: ${CONCURRENCY}"
echo ""

# Pre-seed a key for GET test
curl -s -X PUT "$BASE/v1/cache/load-test-key" \
  -H 'Content-Type: application/json' \
  -d '{"value":"load-test-value","ttl_ms":0}' > /dev/null

if command -v hey &>/dev/null; then
  echo "--- GET (cache hit) ---"
  hey -n "$REQUESTS" -c "$CONCURRENCY" "$BASE/v1/cache/load-test-key"

  echo ""
  echo "--- PUT (write) ---"
  hey -n "$REQUESTS" -c "$CONCURRENCY" -m PUT \
    -H 'Content-Type: application/json' \
    -d '{"value":"lv","ttl_ms":0}' \
    "$BASE/v1/cache/load-test-key"

  echo ""
  echo "--- GET (mixed hit/miss) ---"
  hey -n "$REQUESTS" -c "$CONCURRENCY" "$BASE/v1/cache/miss-$(shuf -i 1-1000 -n 1)"

else
  echo "hey not found — running curl loop (slower)"
  START=$(date +%s%N)
  for i in $(seq 1 200); do
    curl -s "$BASE/v1/cache/load-test-key" > /dev/null
  done
  END=$(date +%s%N)
  ELAPSED=$(( (END - START) / 1000000 ))
  echo "200 sequential GETs completed in ${ELAPSED}ms"
fi

echo ""
echo "Final stats:"
curl -s "$BASE/v1/stats" | python3 -c '
import json,sys
d=json.load(sys.stdin)
print(f"  keys={d[\"keys\"]}  hits={d[\"hits\"]}  misses={d[\"misses\"]}  evictions={d[\"evictions\"]}")
print(f"  hit_rate={d[\"hit_rate\"]*100:.1f}%  memory={d[\"memory_bytes\"]} bytes")
'

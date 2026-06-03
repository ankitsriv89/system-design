#!/usr/bin/env bash
# Integration tests — requires a running Docker Compose stack.
# Usage: BASE_URL=http://localhost:8090 bash scripts/integration_test.sh
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8090}"
PASS=0; FAIL=0

check() {
  local name="$1"; local got="$2"; local want="$3"
  if [ "$got" = "$want" ]; then
    echo "  ✓ $name"
    PASS=$((PASS+1))
  else
    echo "  ✗ $name: got=$got want=$want"
    FAIL=$((FAIL+1))
  fi
}

echo "=== Integration Tests (${BASE}) ==="

# health
SC=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/healthz")
check "healthz returns 200" "$SC" "200"

# SET
SC=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE/v1/cache/testkey" \
  -H 'Content-Type: application/json' -d '{"value":"testval","ttl_ms":0}')
check "SET returns 201" "$SC" "201"

# GET hit
VAL=$(curl -s "$BASE/v1/cache/testkey" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("value",""))')
check "GET returns correct value" "$VAL" "testval"

# GET miss
SC=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/cache/nonexistent-key-xyz")
check "GET miss returns 404" "$SC" "404"

# stats has hits and misses
HITS=$(curl -s "$BASE/v1/stats" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("hits",0))')
check "stats.hits >= 1" "$([ "$HITS" -ge 1 ] && echo ok || echo fail)" "ok"

# TTL expiry
curl -s -X PUT "$BASE/v1/cache/ttl-test" \
  -H 'Content-Type: application/json' -d '{"value":"expires","ttl_ms":300}' > /dev/null
sleep 0.4
SC=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/cache/ttl-test")
check "TTL-expired key returns 404" "$SC" "404"

# DELETE
curl -s -X PUT "$BASE/v1/cache/del-test" \
  -H 'Content-Type: application/json' -d '{"value":"to-delete","ttl_ms":0}' > /dev/null
SC=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE/v1/cache/del-test")
check "DELETE returns 200" "$SC" "200"
SC=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/cache/del-test")
check "GET after DELETE returns 404" "$SC" "404"

# list keys
KEYS=$(curl -s "$BASE/v1/cache" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "list /v1/cache returns array with >= 1 keys" "$([ "$KEYS" -ge 1 ] && echo ok || echo fail)" "ok"

# flush
curl -s -X DELETE "$BASE/v1/cache" > /dev/null
KEYS=$(curl -s "$BASE/v1/cache" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "flush clears all keys" "$KEYS" "0"

echo ""
echo "Results: ${PASS} passed, ${FAIL} failed"
[ "$FAIL" -eq 0 ]

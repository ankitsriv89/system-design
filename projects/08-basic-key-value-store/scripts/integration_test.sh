#!/usr/bin/env bash
# Integration tests for 08-basic-key-value-store.
# Requires a running stack: docker compose up -d
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8088}"
PASS=0; FAIL=0

ok()   { echo "  ✓ $1"; PASS=$((PASS+1)); }
fail() { echo "  ✗ $1"; FAIL=$((FAIL+1)); }

assert_eq() {
  local desc="$1" got="$2" want="$3"
  [ "$got" = "$want" ] && ok "$desc" || fail "$desc (got='$got' want='$want')"
}

echo "=== Integration Tests: Basic Key-Value Store ==="

# Health
status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/healthz")
assert_eq "GET /healthz → 200" "$status" "200"

# SET and GET
curl -s -X PUT "$BASE/v1/kv/integ-key-1" --data "hello-world" > /dev/null
val=$(curl -s "$BASE/v1/kv/integ-key-1")
assert_eq "GET after SET returns value" "$val" "hello-world"

# Overwrite
curl -s -X PUT "$BASE/v1/kv/integ-key-1" --data "updated" > /dev/null
val=$(curl -s "$BASE/v1/kv/integ-key-1")
assert_eq "GET after overwrite" "$val" "updated"

# DELETE
curl -s -X DELETE "$BASE/v1/kv/integ-key-1" > /dev/null
status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/kv/integ-key-1")
assert_eq "GET after DELETE → 404" "$status" "404"

# Multiple keys
for i in $(seq 1 20); do
  curl -s -X PUT "$BASE/v1/kv/batch-key-$i" --data "batch-val-$i" > /dev/null
done
val=$(curl -s "$BASE/v1/kv/batch-key-15")
assert_eq "Batch key-15 readable" "$val" "batch-val-15"

# Stats endpoint
stats=$(curl -s "$BASE/v1/admin/stats")
writes=$(echo "$stats" | grep -o '"writes":[0-9]*' | cut -d: -f2)
[ "$writes" -gt 0 ] && ok "Stats: writes > 0" || fail "Stats: writes == 0"

# Compact
status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/admin/compact")
assert_eq "POST /v1/admin/compact → 200" "$status" "200"

echo ""
echo "Results: ${PASS} passed, ${FAIL} failed"
[ "$FAIL" -eq 0 ] || exit 1

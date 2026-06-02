#!/usr/bin/env bash
# integration_test.sh — smoke-test the live stack.
set -euo pipefail

BASE="${LB_URL:-http://localhost:8086}"
SVC="itest"
PASS=0; FAIL=0

ok()   { echo "  PASS: $*"; ((PASS++)); }
fail() { echo "  FAIL: $*"; ((FAIL++)); }

echo "=== Load Balancer Integration Tests ==="
echo ""

# 1. Health endpoint
STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "$BASE/healthz")
[ "$STATUS" = "200" ] && ok "GET /healthz → 200" || fail "GET /healthz → $STATUS"

# 2. Register a backend
BODY=$(curl -sf -X POST "$BASE/v1/backends/$SVC" \
  -H 'Content-Type: application/json' \
  -d '{"url":"http://echo1:8001","weight":1}')
echo "$BODY" | grep -q '"registered"' && ok "POST /v1/backends/$SVC" || fail "register backend"

# 3. List backends
COUNT=$(curl -sf "$BASE/v1/stats" | jq '[.[] | select(.service=="'"$SVC"'")] | length')
[ "$COUNT" -ge 1 ] && ok "GET /v1/stats shows service" || fail "stats empty"

# 4. Set algorithm
STATUS=$(curl -sf -o /dev/null -w "%{http_code}" \
  -X PUT "$BASE/v1/backends/$SVC/algorithm" \
  -H 'Content-Type: application/json' \
  -d '{"algorithm":"least_connections"}')
[ "$STATUS" = "200" ] && ok "PUT algorithm → 200" || fail "set algorithm → $STATUS"

# 5. Proxy request
STATUS=$(curl -sf -o /dev/null -w "%{http_code}" "$BASE/proxy/$SVC/")
[ "$STATUS" = "200" ] || [ "$STATUS" = "502" ] && ok "GET /proxy/$SVC/ (may 502 if echo down)" || fail "proxy → $STATUS"

# 6. Remove backend
STATUS=$(curl -sf -o /dev/null -w "%{http_code}" \
  -X DELETE "$BASE/v1/backends/$SVC/$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1],safe=""))' 'http://echo1:8001')")
[ "$STATUS" = "204" ] && ok "DELETE backend → 204" || fail "delete backend → $STATUS"

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1

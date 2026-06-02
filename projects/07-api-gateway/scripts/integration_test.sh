#!/usr/bin/env bash
# integration_test.sh — End-to-end integration tests for the API gateway.
# Requires a live Docker Compose stack: docker compose up -d
# Usage: bash scripts/integration_test.sh
set -euo pipefail

ADMIN="${ADMIN_URL:-http://localhost:8089}"
PROXY="${PROXY_URL:-http://localhost:8088}"
TOKEN="${ADMIN_TOKEN:-}"

PASS=0; FAIL=0

check() {
  local desc="$1" expected="$2" actual="$3"
  if [ "$actual" = "$expected" ]; then
    echo "  ✓ $desc"
    PASS=$((PASS+1))
  else
    echo "  ✗ $desc (expected=$expected, got=$actual)"
    FAIL=$((FAIL+1))
  fi
}

ah() { ${TOKEN:+-H "Authorization: Bearer $TOKEN"} -H "Content-Type: application/json"; }

echo "=== Integration Tests: API Gateway ==="

# ---- Health ----
echo ""
echo "--- Health ---"
CODE=$(curl -so /dev/null -w "%{http_code}" "$ADMIN/healthz")
check "admin healthz returns 200" "200" "$CODE"
CODE=$(curl -so /dev/null -w "%{http_code}" "$PROXY/healthz")
check "proxy healthz returns 200" "200" "$CODE"

# ---- Seed test data ----
echo ""
echo "--- Seed ---"
curl -sf -X PUT "$ADMIN/v1/routes/test-pub" \
  ${TOKEN:+-H "Authorization: Bearer $TOKEN"} \
  -H "Content-Type: application/json" \
  -d '{"path_prefix":"/test/pub","upstream":"http://echo-a:9001","active":true}' > /dev/null
check "upsert public route" "0" "$?"

curl -sf -X PUT "$ADMIN/v1/routes/test-auth" \
  ${TOKEN:+-H "Authorization: Bearer $TOKEN"} \
  -H "Content-Type: application/json" \
  -d '{"path_prefix":"/test/auth","upstream":"http://echo-b:9002","auth_required":true,"required_scope":"read","active":true}' > /dev/null
check "upsert auth route" "0" "$?"

curl -sf -X POST "$ADMIN/v1/api-keys" \
  ${TOKEN:+-H "Authorization: Bearer $TOKEN"} \
  -H "Content-Type: application/json" \
  -d '{"owner":"tester","key":"test-key-valid","scopes":["read"],"quota_per_min":50}' > /dev/null
check "create api key (read scope)" "0" "$?"

curl -sf -X POST "$ADMIN/v1/api-keys" \
  ${TOKEN:+-H "Authorization: Bearer $TOKEN"} \
  -H "Content-Type: application/json" \
  -d '{"owner":"narrowkey","key":"test-key-write-only","scopes":["write"],"quota_per_min":50}' > /dev/null
check "create api key (write-only scope)" "0" "$?"

curl -sf -X POST "$ADMIN/v1/api-keys" \
  ${TOKEN:+-H "Authorization: Bearer $TOKEN"} \
  -H "Content-Type: application/json" \
  -d '{"owner":"throttled","key":"test-key-throttle","scopes":["read"],"quota_per_min":3}' > /dev/null
check "create api key (quota=3/min)" "0" "$?"

# Allow route reload (periodic reload is 30s; admin upsert triggers immediate reload).
sleep 1

# ---- Proxy: public route ----
echo ""
echo "--- Proxy: public route ---"
CODE=$(curl -so /dev/null -w "%{http_code}" "$PROXY/test/pub/resource")
check "public route proxies without auth" "200" "$CODE"

# ---- Proxy: auth route – missing token ----
echo ""
echo "--- Proxy: auth route ---"
CODE=$(curl -so /dev/null -w "%{http_code}" "$PROXY/test/auth/resource")
check "auth route rejects missing token with 401" "401" "$CODE"

# ---- Proxy: auth route – valid token ----
CODE=$(curl -so /dev/null -w "%{http_code}" -H "Authorization: Bearer test-key-valid" "$PROXY/test/auth/resource")
check "auth route allows valid token" "200" "$CODE"

# ---- Proxy: scope mismatch ----
CODE=$(curl -so /dev/null -w "%{http_code}" -H "Authorization: Bearer test-key-write-only" "$PROXY/test/auth/resource")
check "auth route rejects wrong scope with 403" "403" "$CODE"

# ---- Proxy: unknown route ----
CODE=$(curl -so /dev/null -w "%{http_code}" "$PROXY/does-not-exist/123")
check "unknown path returns 404" "404" "$CODE"

# ---- Rate limiting ----
echo ""
echo "--- Rate limiting (quota=3/min) ---"
for i in 1 2 3; do
  CODE=$(curl -so /dev/null -w "%{http_code}" -H "Authorization: Bearer test-key-throttle" "$PROXY/test/pub/r$i")
  check "request $i within quota (200)" "200" "$CODE"
done
CODE=$(curl -so /dev/null -w "%{http_code}" -H "Authorization: Bearer test-key-throttle" "$PROXY/test/pub/r4")
check "request 4 exceeds quota (429)" "429" "$CODE"

# ---- Admin: list endpoints ----
echo ""
echo "--- Admin list endpoints ---"
CODE=$(curl -so /dev/null -w "%{http_code}" ${TOKEN:+-H "Authorization: Bearer $TOKEN"} "$ADMIN/v1/routes")
check "GET /v1/routes returns 200" "200" "$CODE"
CODE=$(curl -so /dev/null -w "%{http_code}" ${TOKEN:+-H "Authorization: Bearer $TOKEN"} "$ADMIN/v1/api-keys")
check "GET /v1/api-keys returns 200" "200" "$CODE"

# ---- Cleanup ----
echo ""
echo "--- Cleanup ---"
curl -sf -X DELETE "$ADMIN/v1/routes/test-pub"  ${TOKEN:+-H "Authorization: Bearer $TOKEN"} > /dev/null
curl -sf -X DELETE "$ADMIN/v1/routes/test-auth" ${TOKEN:+-H "Authorization: Bearer $TOKEN"} > /dev/null
check "cleanup routes" "0" "$?"

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[ "$FAIL" -eq 0 ]

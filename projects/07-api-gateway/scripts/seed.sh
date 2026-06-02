#!/usr/bin/env bash
# seed.sh — Seed the API gateway with demo routes and API keys.
# Usage: ADMIN_URL=http://localhost:8089 ADMIN_TOKEN=secret bash scripts/seed.sh
set -euo pipefail

ADMIN="${ADMIN_URL:-http://localhost:8089}"
TOKEN="${ADMIN_TOKEN:-}"

auth_header() {
  if [ -n "$TOKEN" ]; then
    echo "-H \"Authorization: Bearer $TOKEN\""
  else
    echo ""
  fi
}

H="Content-Type: application/json"
A="${TOKEN:+-H \"Authorization: Bearer $TOKEN\"}"

call() {
  local method="$1" path="$2" body="${3:-}"
  if [ -n "$body" ]; then
    eval curl -sf -X "$method" -H "\"$H\"" ${TOKEN:+-H "\"Authorization: Bearer $TOKEN\""} \
      -d "'$body'" "$ADMIN$path"
  else
    eval curl -sf -X "$method" -H "\"$H\"" ${TOKEN:+-H "\"Authorization: Bearer $TOKEN\""} \
      "$ADMIN$path"
  fi
}

echo "=== Seeding routes ==="

curl -sf -X PUT "$ADMIN/v1/routes/users-svc" \
  ${TOKEN:+-H "Authorization: Bearer $TOKEN"} \
  -H "Content-Type: application/json" \
  -d '{"path_prefix":"/api/users","upstream":"http://echo-a:9001","strip_prefix":false,"auth_required":false,"required_scope":"","active":true}'
echo " ✓ route: users-svc → http://echo-a:9001"

curl -sf -X PUT "$ADMIN/v1/routes/orders-svc" \
  ${TOKEN:+-H "Authorization: Bearer $TOKEN"} \
  -H "Content-Type: application/json" \
  -d '{"path_prefix":"/api/orders","upstream":"http://echo-b:9002","strip_prefix":false,"auth_required":true,"required_scope":"orders","active":true}'
echo " ✓ route: orders-svc → http://echo-b:9002 (auth required, scope=orders)"

curl -sf -X PUT "$ADMIN/v1/routes/admin-svc" \
  ${TOKEN:+-H "Authorization: Bearer $TOKEN"} \
  -H "Content-Type: application/json" \
  -d '{"path_prefix":"/api/admin","upstream":"http://echo-c:9003","strip_prefix":true,"auth_required":true,"required_scope":"admin","active":true}'
echo " ✓ route: admin-svc → http://echo-c:9003 (strip prefix, scope=admin)"

echo ""
echo "=== Seeding API keys ==="

curl -sf -X POST "$ADMIN/v1/api-keys" \
  ${TOKEN:+-H "Authorization: Bearer $TOKEN"} \
  -H "Content-Type: application/json" \
  -d '{"owner":"alice","key":"alice-secret-token","scopes":["read","orders"],"quota_per_min":30}'
echo " ✓ key: alice (scopes: read,orders, quota: 30/min)"

curl -sf -X POST "$ADMIN/v1/api-keys" \
  ${TOKEN:+-H "Authorization: Bearer $TOKEN"} \
  -H "Content-Type: application/json" \
  -d '{"owner":"admin-bot","key":"admin-bot-token","scopes":["*"],"quota_per_min":100}'
echo " ✓ key: admin-bot (scopes: *, quota: 100/min)"

curl -sf -X POST "$ADMIN/v1/api-keys" \
  ${TOKEN:+-H "Authorization: Bearer $TOKEN"} \
  -H "Content-Type: application/json" \
  -d '{"owner":"rate-test","key":"rate-test-token","scopes":["read"],"quota_per_min":5}'
echo " ✓ key: rate-test (scopes: read, quota: 5/min — triggers rate limits easily)"

echo ""
echo "=== Verify ==="
echo "Routes:"
curl -sf "$ADMIN/v1/routes" ${TOKEN:+-H "Authorization: Bearer $TOKEN"} | python3 -m json.tool 2>/dev/null || true
echo ""
echo "Keys:"
curl -sf "$ADMIN/v1/api-keys" ${TOKEN:+-H "Authorization: Bearer $TOKEN"} | python3 -m json.tool 2>/dev/null || true
echo ""
echo "Done. Try:"
echo "  curl http://localhost:8088/api/users/1"
echo "  curl -H 'Authorization: Bearer alice-secret-token' http://localhost:8088/api/orders/1"
echo "  curl -H 'Authorization: Bearer rate-test-token' http://localhost:8088/api/users/1  # 6x to hit limit"

#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8096}"

echo "=== Group Chat System — Integration Test ==="

# 1. Get tokens for two users
echo "[1] Authenticating users..."
TOKEN_A=$(curl -sf -X POST "$BASE/api/auth/token" \
  -H "Content-Type: application/json" \
  -d '{"userId":"alice"}' | jq -r .token)
TOKEN_B=$(curl -sf -X POST "$BASE/api/auth/token" \
  -H "Content-Type: application/json" \
  -d '{"userId":"bob"}' | jq -r .token)
echo "  alice token: ${TOKEN_A:0:20}..."
echo "  bob token:   ${TOKEN_B:0:20}..."

# 2. Alice creates a group
echo "[2] Alice creates group..."
GROUP=$(curl -sf -X POST "$BASE/api/groups" \
  -H "Authorization: Bearer $TOKEN_A" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-group"}')
GROUP_ID=$(echo "$GROUP" | jq -r .id)
echo "  group_id=$GROUP_ID"

# 3. Alice adds Bob
echo "[3] Alice adds Bob..."
curl -sf -X POST "$BASE/api/groups/$GROUP_ID/members" \
  -H "Authorization: Bearer $TOKEN_A" \
  -H "Content-Type: application/json" \
  -d '{"userId":"bob"}' > /dev/null
echo "  Bob added"

# 4. Alice sends a message
echo "[4] Alice sends message..."
MSG=$(curl -sf -X POST "$BASE/api/groups/$GROUP_ID/messages" \
  -H "Authorization: Bearer $TOKEN_A" \
  -H "Content-Type: application/json" \
  -d '{"body":"hello group!"}')
echo "  msg_id=$(echo "$MSG" | jq -r .id)"

# 5. Bob reads history
echo "[5] Bob reads history..."
HISTORY=$(curl -sf "$BASE/api/groups/$GROUP_ID/messages" \
  -H "Authorization: Bearer $TOKEN_B")
COUNT=$(echo "$HISTORY" | jq length)
echo "  message count=$COUNT"
[ "$COUNT" -ge 1 ] || { echo "FAIL: expected messages"; exit 1; }

# 6. Non-member is rejected
echo "[6] Non-member access rejected..."
TOKEN_C=$(curl -sf -X POST "$BASE/api/auth/token" \
  -H "Content-Type: application/json" \
  -d '{"userId":"charlie"}' | jq -r .token)
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  "$BASE/api/groups/$GROUP_ID/messages" \
  -H "Authorization: Bearer $TOKEN_C")
[ "$STATUS" = "403" ] || { echo "FAIL: expected 403 for non-member, got $STATUS"; exit 1; }
echo "  correctly rejected (403)"

echo ""
echo "=== All tests passed ==="

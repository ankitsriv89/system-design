#!/usr/bin/env bash
# integration_test.sh — Requires a running Docker Compose stack.
set -euo pipefail

BASE="http://localhost:8095"

echo "=== Integration Test: 14-one-to-one-chat-system ==="

# 1. Health
echo "--- Health check ---"
curl -sf "$BASE/actuator/health" | grep -q '"status":"UP"' && echo "PASS: health"

# 2. Get tokens for alice and bob
echo "--- Auth ---"
ALICE_TOKEN=$(curl -sf -X POST "$BASE/api/v1/auth/token?userId=alice" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
BOB_TOKEN=$(curl -sf -X POST "$BASE/api/v1/auth/token?userId=bob" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
echo "PASS: tokens issued for alice and bob"

# 3. Create conversation
echo "--- Create conversation ---"
CONV=$(curl -sf -X POST "$BASE/api/v1/conversations?recipientId=bob" -H "Authorization: Bearer $ALICE_TOKEN")
CONV_ID=$(echo "$CONV" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "PASS: conversation id=$CONV_ID"

# 4. Send message
echo "--- Send message ---"
MSG=$(curl -sf -X POST "$BASE/api/v1/conversations/$CONV_ID/messages" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"body":"Hello Bob!"}' | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['id'], d['status'])")
echo "PASS: message sent — $MSG"

# 5. History
echo "--- Message history ---"
HIST=$(curl -sf "$BASE/api/v1/conversations/$CONV_ID/messages" -H "Authorization: Bearer $ALICE_TOKEN")
echo "$HIST" | python3 -c "import sys,json; msgs=json.load(sys.stdin); assert len(msgs)>=1, 'no messages'"
echo "PASS: history returned"

# 6. Presence
echo "--- Presence ---"
curl -sf "$BASE/api/v1/presence/alice" -H "Authorization: Bearer $ALICE_TOKEN" | python3 -c "import sys,json; print(json.load(sys.stdin))"
echo "PASS: presence endpoint"

echo "=== All integration tests passed ==="

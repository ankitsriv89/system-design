#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8097}"

echo "=== News Feed System — Integration Test ==="

tok() {
  curl -sf -X POST "$BASE/api/auth/token" \
    -H "Content-Type: application/json" \
    -d "{\"userId\":\"$1\"}" | jq -r .token
}

# 1. Auth two users
echo "[1] Authenticating users..."
A=$(tok alice)
B=$(tok bob)
echo "  alice: ${A:0:18}...  bob: ${B:0:18}..."

# 2. Bob follows Alice
echo "[2] Bob follows Alice..."
curl -sf -X POST "$BASE/v1/follows" \
  -H "Authorization: Bearer $B" -H "Content-Type: application/json" \
  -d '{"followeeId":"alice"}' > /dev/null
echo "  ok"

# 3. Alice posts
echo "[3] Alice creates a post..."
POST=$(curl -sf -X POST "$BASE/v1/posts" \
  -H "Authorization: Bearer $A" -H "Content-Type: application/json" \
  -d '{"body":"hello from alice"}')
POST_ID=$(echo "$POST" | jq -r .id)
echo "  post_id=$POST_ID"

# 4. Wait for async fanout, then Bob reads his feed
echo "[4] Bob reads home feed (after fanout)..."
FOUND=0
for i in $(seq 1 10); do
  sleep 0.5
  FEED=$(curl -sf "$BASE/v1/feed?limit=20" -H "Authorization: Bearer $B")
  COUNT=$(echo "$FEED" | jq "[.[] | select(.postId == $POST_ID)] | length")
  if [ "$COUNT" -ge 1 ]; then FOUND=1; break; fi
done
[ "$FOUND" = "1" ] || { echo "FAIL: post did not fan out to bob's feed"; echo "$FEED" | jq .; exit 1; }
echo "  post present in bob's feed (source=$(echo "$FEED" | jq -r ".[] | select(.postId==$POST_ID) | .source"))"

# 5. Alice deletes the post → it must disappear from bob's feed
echo "[5] Alice deletes the post..."
curl -sf -X DELETE "$BASE/v1/posts/$POST_ID" -H "Authorization: Bearer $A" > /dev/null
FEED=$(curl -sf "$BASE/v1/feed?limit=20" -H "Authorization: Bearer $B")
STILL=$(echo "$FEED" | jq "[.[] | select(.postId == $POST_ID)] | length")
[ "$STILL" = "0" ] || { echo "FAIL: deleted post still in feed"; exit 1; }
echo "  deleted post correctly absent from feed"

# 6. Non-author cannot delete
echo "[6] Non-author delete is rejected..."
POST2=$(curl -sf -X POST "$BASE/v1/posts" \
  -H "Authorization: Bearer $A" -H "Content-Type: application/json" \
  -d '{"body":"second post"}')
PID2=$(echo "$POST2" | jq -r .id)
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X DELETE "$BASE/v1/posts/$PID2" -H "Authorization: Bearer $B")
[ "$STATUS" = "403" ] || { echo "FAIL: expected 403 for non-author delete, got $STATUS"; exit 1; }
echo "  correctly rejected (403)"

# 7. Unauthenticated feed read is rejected
echo "[7] Unauthenticated feed read rejected..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/feed")
[ "$STATUS" = "401" ] || [ "$STATUS" = "403" ] || { echo "FAIL: expected 401/403, got $STATUS"; exit 1; }
echo "  correctly rejected ($STATUS)"

echo ""
echo "=== All tests passed ==="

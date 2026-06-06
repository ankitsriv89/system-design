#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8100}"

echo "=== Twitter/X Timeline & Posts — Integration Test ==="

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

# 3. Alice tweets (with a hashtag, to also feed search + trends)
echo "[3] Alice posts a tweet..."
TWEET=$(curl -sf -X POST "$BASE/v1/posts" \
  -H "Authorization: Bearer $A" -H "Content-Type: application/json" \
  -d '{"text":"hello from alice about #systemdesign"}')
TWEET_ID=$(echo "$TWEET" | jq -r .id)
echo "  tweet_id=$TWEET_ID"

# 4. Wait for async fanout, then Bob reads his home timeline
echo "[4] Bob reads home timeline (after fanout)..."
FOUND=0
for i in $(seq 1 10); do
  sleep 0.5
  HOME=$(curl -sf "$BASE/v1/home?limit=20" -H "Authorization: Bearer $B")
  COUNT=$(echo "$HOME" | jq "[.[] | select(.tweetId == $TWEET_ID)] | length")
  if [ "$COUNT" -ge 1 ]; then FOUND=1; break; fi
done
[ "$FOUND" = "1" ] || { echo "FAIL: tweet did not fan out to bob's timeline"; echo "$HOME" | jq .; exit 1; }
echo "  tweet present (source=$(echo "$HOME" | jq -r ".[] | select(.tweetId==$TWEET_ID) | .source"))"

# 5. Full-text search finds the tweet (eventually consistent via OpenSearch)
echo "[5] Search for the tweet..."
SFOUND=0
for i in $(seq 1 15); do
  sleep 0.5
  HITS=$(curl -sf "$BASE/v1/search?q=alice")
  SC=$(echo "$HITS" | jq "[.[] | select(.tweetId == $TWEET_ID)] | length")
  if [ "$SC" -ge 1 ]; then SFOUND=1; break; fi
done
[ "$SFOUND" = "1" ] || { echo "FAIL: tweet not found in search"; echo "$HITS" | jq .; exit 1; }
echo "  tweet found in search index"

# 6. Trends include the hashtag
echo "[6] Trends include #systemdesign..."
TRENDS=$(curl -sf "$BASE/v1/trends")
TC=$(echo "$TRENDS" | jq '[.[] | select(.hashtag == "systemdesign")] | length')
[ "$TC" -ge 1 ] || { echo "FAIL: hashtag not in trends"; echo "$TRENDS" | jq .; exit 1; }
echo "  #systemdesign trending (count=$(echo "$TRENDS" | jq -r '.[] | select(.hashtag=="systemdesign") | .count'))"

# 7. Alice deletes the tweet → it must disappear from bob's timeline
echo "[7] Alice deletes the tweet..."
curl -sf -X DELETE "$BASE/v1/posts/$TWEET_ID" -H "Authorization: Bearer $A" > /dev/null
HOME=$(curl -sf "$BASE/v1/home?limit=20" -H "Authorization: Bearer $B")
STILL=$(echo "$HOME" | jq "[.[] | select(.tweetId == $TWEET_ID)] | length")
[ "$STILL" = "0" ] || { echo "FAIL: deleted tweet still in timeline"; exit 1; }
echo "  deleted tweet correctly absent from timeline"

# 8. Non-author cannot delete
echo "[8] Non-author delete is rejected..."
TWEET2=$(curl -sf -X POST "$BASE/v1/posts" \
  -H "Authorization: Bearer $A" -H "Content-Type: application/json" \
  -d '{"text":"second tweet"}')
TID2=$(echo "$TWEET2" | jq -r .id)
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X DELETE "$BASE/v1/posts/$TID2" -H "Authorization: Bearer $B")
[ "$STATUS" = "403" ] || { echo "FAIL: expected 403 for non-author delete, got $STATUS"; exit 1; }
echo "  correctly rejected (403)"

# 9. Unauthenticated home read is rejected; search/trends stay public
echo "[9] Auth boundaries..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/home")
[ "$STATUS" = "401" ] || [ "$STATUS" = "403" ] || { echo "FAIL: expected 401/403 for /v1/home, got $STATUS"; exit 1; }
PUB=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/trends")
[ "$PUB" = "200" ] || { echo "FAIL: /v1/trends should be public, got $PUB"; exit 1; }
echo "  home protected ($STATUS), trends public ($PUB)"

echo ""
echo "=== All tests passed ==="

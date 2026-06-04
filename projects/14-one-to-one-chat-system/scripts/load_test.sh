#!/usr/bin/env bash
# load_test.sh — Basic throughput test using REST send endpoint.
# Requires: curl, jq, bc
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8095}"
MESSAGES="${MESSAGES:-200}"
CONCURRENCY="${CONCURRENCY:-10}"

echo "=== Load Test: 14-one-to-one-chat-system ==="
echo "Endpoint: $BASE | Messages: $MESSAGES | Concurrency: $CONCURRENCY"

# Get tokens
ALICE=$(curl -sf -X POST "$BASE/api/v1/auth/token?userId=loadtest_alice" | jq -r '.token')
BOB=$(curl -sf -X POST "$BASE/api/v1/auth/token?userId=loadtest_bob" | jq -r '.token')

# Ensure conversation
CONV_ID=$(curl -sf -X POST "$BASE/api/v1/conversations?recipientId=loadtest_bob" \
  -H "Authorization: Bearer $ALICE" | jq -r '.id')

echo "Conversation: $CONV_ID"

START=$(date +%s%3N)
SUCCESS=0; FAIL=0

for i in $(seq 1 "$MESSAGES"); do
  curl -sf -X POST "$BASE/api/v1/conversations/$CONV_ID/messages" \
    -H "Authorization: Bearer $ALICE" \
    -H "Content-Type: application/json" \
    -d "{\"body\":\"load test message $i\"}" > /dev/null && ((SUCCESS++)) || ((FAIL++)) &

  if (( i % CONCURRENCY == 0 )); then wait; fi
done
wait

END=$(date +%s%3N)
ELAPSED=$(( END - START ))
RPS=$(echo "scale=1; $MESSAGES * 1000 / $ELAPSED" | bc)

echo "--- Results ---"
echo "Success: $SUCCESS | Failed: $FAIL"
echo "Elapsed: ${ELAPSED}ms | Throughput: ~${RPS} req/s"

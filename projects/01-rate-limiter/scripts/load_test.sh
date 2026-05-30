#!/usr/bin/env bash
# Simple load test for the rate limiter using curl + parallel.
# Usage: ./scripts/load_test.sh [concurrency] [total_requests]
set -euo pipefail

BASE=${BASE_URL:-http://localhost:8080}
CONCURRENCY=${1:-10}
TOTAL=${2:-500}
POLICY=${POLICY:-user-token-bucket}

echo "==> Load test: $TOTAL requests, concurrency $CONCURRENCY, policy $POLICY"

allowed=0
denied=0
errors=0

run_request() {
  local subject="user:$((RANDOM % 5))"  # 5 subjects → hot-key scenario
  local status
  status=$(curl -sf -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/limits/check" \
    -H "Content-Type: application/json" \
    -d "{\"subject\":\"$subject\",\"policy_id\":\"$POLICY\"}" 2>/dev/null || echo "000")
  echo "$status"
}

export -f run_request
export BASE POLICY

results=$(seq 1 "$TOTAL" | xargs -P "$CONCURRENCY" -I{} bash -c 'run_request')

while IFS= read -r code; do
  case "$code" in
    200) ((allowed++)) ;;
    429) ((denied++)) ;;
    *)   ((errors++)) ;;
  esac
done <<< "$results"

echo ""
echo "==> Results"
echo "   Allowed : $allowed"
echo "   Denied  : $denied"
echo "   Errors  : $errors"
echo "   Total   : $TOTAL"
echo "   Deny %%  : $(echo "scale=1; $denied * 100 / $TOTAL" | bc)%%"

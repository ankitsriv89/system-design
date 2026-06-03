#!/usr/bin/env bash
# Integration tests — requires the stack to be running via docker-compose.
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8092}"
PASS=0; FAIL=0

check() {
  local desc="$1" expected="$2" actual="$3"
  if [[ "$actual" == *"$expected"* ]]; then
    echo "  PASS: $desc"
    PASS=$((PASS+1))
  else
    echo "  FAIL: $desc (expected '$expected' in '$actual')"
    FAIL=$((FAIL+1))
  fi
}

echo "=== Integration Tests ==="

# Healthz
res=$(curl -sf "$BASE/healthz")
check "healthz returns ok" '"status":"ok"' "$res"

# Add item
res=$(curl -sf -X POST "$BASE/v1/corpus/items" \
  -H "Content-Type: application/json" \
  -d '{"text":"integration test item","category":"test","popularity":500,"locale":"en"}')
check "add item returns id" '"id"' "$res"
item_id=$(echo "$res" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)

# Suggest
res=$(curl -sf "$BASE/v1/suggest?q=integ&locale=en&limit=5")
check "suggest returns suggestions array" '"suggestions"' "$res"

# Rebuild index
res=$(curl -sf -X POST "$BASE/v1/admin/rebuild-index")
check "rebuild returns total_items" '"total_items"' "$res"
check "rebuild duration present" '"rebuild_duration_ms"' "$res"

# Suggest after rebuild
res=$(curl -sf "$BASE/v1/suggest?q=integ&locale=en&limit=5")
check "suggest after rebuild finds item" 'integration test item' "$res"

# Get stats
res=$(curl -sf "$BASE/v1/admin/stats")
check "stats returns total_prefixes" '"total_prefixes"' "$res"

# Click feedback
res=$(curl -sf -X POST "$BASE/v1/feedback/click" \
  -H "Content-Type: application/json" \
  -d "{\"prefix\":\"integ\",\"selected_item_id\":$item_id,\"latency_ms\":3,\"locale\":\"en\"}")
check "click feedback returns 204" '' "$res"

# Delete item
http_code=$(curl -sf -o /dev/null -w "%{http_code}" -X DELETE "$BASE/v1/corpus/items/$item_id")
check "delete item returns 204" '204' "$http_code"

# List items
res=$(curl -sf "$BASE/v1/corpus/items?limit=5")
check "list items returns array" '"items"' "$res"

echo "==="
echo "Passed: $PASS  Failed: $FAIL"
[[ $FAIL -eq 0 ]]

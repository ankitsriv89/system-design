#!/usr/bin/env bash
# Seed the cache with a set of sample keys for demo purposes.
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8090}"

declare -A data=(
  ["user:1001"]="Alice"
  ["user:1002"]="Bob"
  ["user:1003"]="Charlie"
  ["product:sku-001"]='{"name":"Laptop","price":999}'
  ["product:sku-002"]='{"name":"Phone","price":499}'
  ["session:abc123"]="active"
  ["session:def456"]="active"
  ["config:feature-flags"]='{"dark_mode":true,"beta":false}'
  ["rate:192.168.1.1"]="42"
  ["geo:37.7749,-122.4194"]="San Francisco"
)

for key in "${!data[@]}"; do
  value="${data[$key]}"
  status=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE/v1/cache/$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1]))' "$key")" \
    -H 'Content-Type: application/json' \
    -d "{\"value\":$(echo "$value" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read().strip()))'),\"ttl_ms\":0}")
  echo "SET $key → HTTP $status"
done

echo "Seed complete."

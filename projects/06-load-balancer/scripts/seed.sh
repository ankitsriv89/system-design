#!/usr/bin/env bash
# seed.sh — register the three echo backends with the load balancer.
set -euo pipefail

BASE="${LB_URL:-http://localhost:8086}"
SVC="demo"

echo "Seeding backends for service '$SVC' at $BASE..."

for i in 1 2 3; do
  PORT=$((8090 + i))
  curl -sf -X POST "$BASE/v1/backends/$SVC" \
    -H 'Content-Type: application/json' \
    -d "{\"url\":\"http://echo${i}:800${i}\",\"weight\":${i}}" \
    | jq .
done

echo "Done. Verify with: curl $BASE/v1/stats | jq ."

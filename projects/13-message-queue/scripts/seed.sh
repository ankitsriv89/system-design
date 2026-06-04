#!/usr/bin/env bash
# Seed the message queue with sample topics and messages.
set -euo pipefail

BASE="${MQ_BASE_URL:-http://localhost:8094}"

echo "==> Creating topics..."
for topic in orders payments notifications; do
  curl -sf -X POST "$BASE/v1/topics" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"$topic\",\"partitions\":3,\"retention_hours\":24}" \
    && echo " ✓ $topic" || echo " (already exists: $topic)"
done

echo ""
echo "==> Publishing sample messages..."
for i in $(seq 1 20); do
  user_id="user-$((RANDOM % 5))"
  topic="orders"
  payload="{\"order_id\":$i,\"user\":\"$user_id\",\"amount\":$((RANDOM % 100 + 1))}"
  curl -sf -X POST "$BASE/v1/topics/orders/messages" \
    -H "Content-Type: application/json" \
    -d "{\"key\":\"$user_id\",\"payload\":$(echo $payload | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read().strip()))')}" \
    > /dev/null
  echo -n "."
done
echo " 20 messages"

echo ""
echo "==> Stats:"
curl -sf "$BASE/v1/stats" | python3 -m json.tool

echo ""
echo "==> Queue depth for orders:"
curl -sf "$BASE/v1/topics/orders/depth" | python3 -m json.tool

#!/usr/bin/env bash
# Load test: concurrent product listings, cart adds, and checkouts.
set -euo pipefail

BASE="${ECOMMERCE_BASE_URL:-http://localhost:8104}"
CONCURRENCY="${CONCURRENCY:-10}"
REQUESTS="${REQUESTS:-200}"

echo "==> Load test — concurrency=$CONCURRENCY requests=$REQUESTS"

# Ensure a product exists
PROD_ID=$(curl -sf -X POST "$BASE/v1/products" \
  -H "Content-Type: application/json" \
  -d '{"sku":"LOAD-SKU-001","name":"Load Test Product","price":1.00,"stock":99999,"category":"test"}' \
  | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])" 2>/dev/null || \
  curl -sf "$BASE/v1/products?q=Load+Test+Product" \
  | python3 -c "import json,sys; d=json.load(sys.stdin); print(d[0]['id'] if d else '')")

echo "  Product ID: $PROD_ID"

# Benchmark GET /v1/products
echo ""
echo "==> Benchmark: GET /v1/products (${REQUESTS}×, concurrency=${CONCURRENCY})"
START=$(date +%s%N)
seq 1 "$REQUESTS" | xargs -P "$CONCURRENCY" -I{} curl -sf "$BASE/v1/products" -o /dev/null
END=$(date +%s%N)
ELAPSED=$(( (END - START) / 1000000 ))
RPS=$(( REQUESTS * 1000 / ELAPSED ))
echo "  ${REQUESTS} requests in ${ELAPSED}ms → ~${RPS} req/s"

# Benchmark cart add
echo ""
echo "==> Benchmark: POST /v1/cart/{userId}/items (${REQUESTS}×, concurrency=${CONCURRENCY})"
START=$(date +%s%N)
seq 1 "$REQUESTS" | xargs -P "$CONCURRENCY" -I{} \
  curl -sf -X POST "$BASE/v1/cart/loadtest-{}/items" \
  -H "Content-Type: application/json" \
  -d "{\"productId\":\"$PROD_ID\",\"quantity\":1}" -o /dev/null
END=$(date +%s%N)
ELAPSED=$(( (END - START) / 1000000 ))
RPS=$(( REQUESTS * 1000 / ELAPSED ))
echo "  ${REQUESTS} requests in ${ELAPSED}ms → ~${RPS} req/s"

# Benchmark checkout (unique users to avoid empty cart race)
echo ""
echo "==> Benchmark: POST /v1/orders checkout (${REQUESTS}×, concurrency=${CONCURRENCY})"
START=$(date +%s%N)
seq 1 "$REQUESTS" | xargs -P "$CONCURRENCY" -I{} \
  curl -sf -X POST "$BASE/v1/orders" \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"loadtest-{}\",\"idempotencyKey\":\"load-{}-$(date +%s)\"}" -o /dev/null
END=$(date +%s%N)
ELAPSED=$(( (END - START) / 1000000 ))
RPS=$(( REQUESTS * 1000 / ELAPSED ))
echo "  ${REQUESTS} requests in ${ELAPSED}ms → ~${RPS} req/s"

echo ""
echo "==> Done."

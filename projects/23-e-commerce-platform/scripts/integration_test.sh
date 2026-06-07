#!/usr/bin/env bash
# Integration test suite — requires a running Docker Compose stack.
set -euo pipefail

BASE="${ECOMMERCE_BASE_URL:-http://localhost:8104}"
PASS=0; FAIL=0

assert_eq() {
  local desc=$1 expected=$2 actual=$3
  if [ "$actual" = "$expected" ]; then
    echo "  PASS: $desc"
    PASS=$((PASS+1))
  else
    echo "  FAIL: $desc — expected '$expected', got '$actual'"
    FAIL=$((FAIL+1))
  fi
}

wait_healthy() {
  echo "==> Waiting for service..."
  for i in $(seq 1 60); do
    if curl -sf "$BASE/actuator/health" | grep -q '"UP"'; then return; fi
    sleep 2
  done
  echo "Service not healthy after 120s" >&2; exit 1
}

wait_healthy

echo ""
echo "==> Test: create product"
PROD=$(curl -sf -X POST "$BASE/v1/products" \
  -H "Content-Type: application/json" \
  -d '{"sku":"IT-SKU-1","name":"Integration Widget","price":9.99,"stock":10,"category":"test"}')
PROD_ID=$(echo "$PROD" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")
assert_eq "product created with id" "true" "$([ -n '$PROD_ID' ] && echo true || echo false)"

echo ""
echo "==> Test: add to cart"
CART=$(curl -sf -X POST "$BASE/v1/cart/it-user/items" \
  -H "Content-Type: application/json" \
  -d "{\"productId\":\"$PROD_ID\",\"quantity\":2}")
COUNT=$(echo "$CART" | python3 -c "import json,sys; print(len(json.load(sys.stdin)['items']))")
assert_eq "cart has 1 item" "1" "$COUNT"

echo ""
echo "==> Test: idempotent checkout"
IDEM_KEY="it-key-$(date +%s)"
ORDER1=$(curl -sf -X POST "$BASE/v1/orders" \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"it-user\",\"idempotencyKey\":\"$IDEM_KEY\"}")
ORDER2=$(curl -sf -X POST "$BASE/v1/orders" \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"it-user\",\"idempotencyKey\":\"$IDEM_KEY\"}")
ORDER_ID1=$(echo "$ORDER1" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")
ORDER_ID2=$(echo "$ORDER2" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")
assert_eq "idempotent checkout returns same order id" "$ORDER_ID1" "$ORDER_ID2"

echo ""
echo "==> Test: cart cleared after checkout"
CART_AFTER=$(curl -sf "$BASE/v1/cart/it-user")
COUNT_AFTER=$(echo "$CART_AFTER" | python3 -c "import json,sys; print(len(json.load(sys.stdin)['items']))")
assert_eq "cart is empty after checkout" "0" "$COUNT_AFTER"

echo ""
echo "==> Test: order visible in user orders"
ORDERS=$(curl -sf "$BASE/v1/orders?userId=it-user")
HAS_ORDER=$(echo "$ORDERS" | python3 -c "import json,sys; orders=json.load(sys.stdin); print('yes' if any(o['id']=='''$ORDER_ID1''' for o in orders) else 'no')")
assert_eq "order in user order list" "yes" "$HAS_ORDER"

echo ""
echo "==> Test: saga completes (wait up to 10s for Kafka consumer)"
for i in $(seq 1 10); do
  STATUS=$(curl -sf "$BASE/v1/orders/$ORDER_ID1" | python3 -c "import json,sys; print(json.load(sys.stdin)['status'])")
  if [ "$STATUS" = "CONFIRMED" ]; then break; fi
  sleep 1
done
assert_eq "order confirmed via saga" "CONFIRMED" "$STATUS"

echo ""
echo "==> Test: oversell prevention"
# Add more than available stock (stock=10, already used 2)
curl -sf -X POST "$BASE/v1/cart/it-user2/items" \
  -H "Content-Type: application/json" \
  -d "{\"productId\":\"$PROD_ID\",\"quantity\":100}" > /dev/null
OVER_RESP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/orders" \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"it-user2\",\"idempotencyKey\":\"oversell-$(date +%s)\"}")
assert_eq "oversell returns 409" "409" "$OVER_RESP"

echo ""
echo "==> Test: get single order"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/orders/$ORDER_ID1")
assert_eq "get order returns 200" "200" "$STATUS"

echo ""
echo "==> Test: 404 for unknown order"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/orders/nonexistent-id")
assert_eq "unknown order returns 404" "404" "$STATUS"

echo ""
echo "==> Test: admin ship order"
SHIP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/admin/orders/$ORDER_ID1/ship")
assert_eq "ship returns 200" "200" "$SHIP_STATUS"

echo ""
echo "============================="
echo " PASSED: $PASS  FAILED: $FAIL"
echo "============================="
[ "$FAIL" -eq 0 ]

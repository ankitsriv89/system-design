#!/usr/bin/env bash
# Integration test suite — requires a running Docker Compose stack.
set -euo pipefail

BASE="${MQ_BASE_URL:-http://localhost:8094}"
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
  for i in $(seq 1 30); do
    if curl -sf "$BASE/healthz" > /dev/null 2>&1; then return; fi
    sleep 1
  done
  echo "Service not healthy after 30s" >&2; exit 1
}

wait_healthy

echo ""
echo "==> Test: create topic"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/topics" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-it","partitions":2,"retention_hours":1}')
assert_eq "create topic returns 201" "201" "$STATUS"

echo ""
echo "==> Test: duplicate topic returns 409"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/topics" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-it","partitions":2,"retention_hours":1}')
assert_eq "duplicate topic returns 409" "409" "$STATUS"

echo ""
echo "==> Test: publish message"
RESP=$(curl -sf -X POST "$BASE/v1/topics/test-it/messages" \
  -H "Content-Type: application/json" \
  -d '{"key":"k1","payload":"hello integration"}')
MSG_ID=$(echo "$RESP" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")
assert_eq "publish returns id" "true" "$([ -n '$MSG_ID' ] && echo true || echo false)"

echo ""
echo "==> Test: poll message"
RESP=$(curl -sf -X POST "$BASE/v1/topics/test-it/messages:poll" \
  -H "Content-Type: application/json" \
  -d '{"consumer_group":"it-group","partition":-1,"max_messages":1,"visibility_timeout_seconds":30}')
COUNT=$(echo "$RESP" | python3 -c "import json,sys; print(json.load(sys.stdin)['count'])")
POLLED_ID=$(echo "$RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['messages'][0]['id'] if d['messages'] else '')")
assert_eq "poll returns 1 message" "1" "$COUNT"

echo ""
echo "==> Test: ack message"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/messages/$POLLED_ID:ack" \
  -H "Content-Type: application/json" \
  -d '{"consumer_group":"it-group"}')
assert_eq "ack returns 200" "200" "$STATUS"

echo ""
echo "==> Test: poll empty after ack"
RESP=$(curl -sf -X POST "$BASE/v1/topics/test-it/messages:poll" \
  -H "Content-Type: application/json" \
  -d '{"consumer_group":"it-group","partition":-1,"max_messages":5,"visibility_timeout_seconds":30}')
COUNT=$(echo "$RESP" | python3 -c "import json,sys; print(json.load(sys.stdin)['count'])")
assert_eq "queue empty after ack" "0" "$COUNT"

echo ""
echo "==> Test: double-ack returns 404"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/messages/$POLLED_ID:ack" \
  -H "Content-Type: application/json" \
  -d '{"consumer_group":"it-group"}')
assert_eq "double ack returns 404" "404" "$STATUS"

echo ""
echo "==> Test: visibility timeout re-delivery"
# Publish, poll with 2s timeout, wait, poll again — should see delivery_attempts=2
curl -sf -X POST "$BASE/v1/topics/test-it/messages" \
  -H "Content-Type: application/json" \
  -d '{"key":"vt","payload":"visibility-test"}' > /dev/null
curl -sf -X POST "$BASE/v1/topics/test-it/messages:poll" \
  -H "Content-Type: application/json" \
  -d '{"consumer_group":"vt-group","partition":-1,"max_messages":1,"visibility_timeout_seconds":2}' > /dev/null
echo "  Waiting 8s for reaper to restore..."
sleep 8
RESP=$(curl -sf -X POST "$BASE/v1/topics/test-it/messages:poll" \
  -H "Content-Type: application/json" \
  -d '{"consumer_group":"vt-group","partition":-1,"max_messages":1,"visibility_timeout_seconds":30}')
ATTEMPTS=$(echo "$RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['messages'][0]['delivery_attempts'] if d['messages'] else 0)")
assert_eq "delivery_attempts incremented on re-delivery" "2" "$ATTEMPTS"

echo ""
echo "==> Test: stats endpoint"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/stats")
assert_eq "stats returns 200" "200" "$STATUS"

echo ""
echo "==> Test: DLQ endpoint"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/topics/test-it/dlq")
assert_eq "dlq returns 200" "200" "$STATUS"

echo ""
echo "============================="
echo " PASSED: $PASS  FAILED: $FAIL"
echo "============================="
[ "$FAIL" -eq 0 ]

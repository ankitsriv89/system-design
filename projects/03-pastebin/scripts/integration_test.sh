#!/usr/bin/env bash
# integration_test.sh — functional + non-functional tests against a live stack.
# Run after: docker compose up -d && psql ... -f scripts/migrate.sql
# Usage: ./scripts/integration_test.sh [base_url]
set -euo pipefail

BASE=${1:-http://localhost:8082}
PASS=0
FAIL=0

ok()   { echo "  PASS: $1"; ((PASS++)); }
fail() { echo "  FAIL: $1 — $2"; ((FAIL++)); }

assert_status() {
  local label=$1 expected=$2 actual=$3
  if [[ "$actual" == "$expected" ]]; then ok "$label (HTTP $actual)"
  else fail "$label" "expected HTTP $expected, got $actual"; fi
}

assert_contains() {
  local label=$1 needle=$2 haystack=$3
  if echo "$haystack" | grep -q "$needle"; then ok "$label"
  else fail "$label" "expected '$needle' in response"; fi
}

echo "=== Pastebin integration tests against $BASE ==="
echo

# --- health ---
echo "-- Health --"
STATUS=$(curl -so /dev/null -w "%{http_code}" "$BASE/healthz")
assert_status "healthz" "200" "$STATUS"

# --- create public paste ---
echo "-- Create paste --"
RESP=$(curl -sf -X POST "$BASE/v1/pastes" \
  -H "Content-Type: application/json" \
  -d '{"title":"test","language":"go","visibility":"public","content":"hello world"}')
ID=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
assert_contains "create returns id" '"id"' "$RESP"
assert_contains "create returns content" 'hello world' "$RESP"
ok "created paste id=$ID"

# --- get paste (JSON) ---
echo "-- Get paste --"
GET_RESP=$(curl -sf "$BASE/v1/pastes/$ID")
assert_contains "get returns content" 'hello world' "$GET_RESP"
assert_contains "get returns language" '"go"' "$GET_RESP"

# --- get raw ---
echo "-- Get raw --"
RAW=$(curl -sf "$BASE/v1/pastes/$ID/raw")
if [[ "$RAW" == "hello world" ]]; then ok "raw content matches"
else fail "raw content" "expected 'hello world', got '$RAW'"; fi

# --- 404 on missing paste ---
echo "-- Not found --"
STATUS=$(curl -so /dev/null -w "%{http_code}" "$BASE/v1/pastes/doesnotexist")
assert_status "missing paste returns 404" "404" "$STATUS"

# --- create with TTL, then check expires_at present ---
echo "-- TTL paste --"
TTL_RESP=$(curl -sf -X POST "$BASE/v1/pastes" \
  -H "Content-Type: application/json" \
  -d '{"content":"expires soon","visibility":"public","ttl_seconds":3600}')
assert_contains "ttl paste has expires_at" 'expires_at' "$TTL_RESP"

# --- burn after read ---
echo "-- Burn after read --"
BURN_RESP=$(curl -sf -X POST "$BASE/v1/pastes" \
  -H "Content-Type: application/json" \
  -d '{"content":"burn me","visibility":"unlisted","burn_after_read":true}')
BURN_ID=$(echo "$BURN_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
curl -sf "$BASE/v1/pastes/$BURN_ID" > /dev/null
sleep 1  # async delete
STATUS=$(curl -so /dev/null -w "%{http_code}" "$BASE/v1/pastes/$BURN_ID")
assert_status "burn-after-read deleted after first read" "404" "$STATUS"

# --- private paste blocked (no auth) ---
echo "-- Private paste blocked --"
STATUS=$(curl -so /dev/null -w "%{http_code}" -X POST "$BASE/v1/pastes" \
  -H "Content-Type: application/json" \
  -d '{"content":"secret","visibility":"private"}')
assert_status "private paste returns 501 (auth not implemented)" "501" "$STATUS"

# --- delete anonymous paste ---
echo "-- Delete --"
DEL_RESP=$(curl -sf -X POST "$BASE/v1/pastes" \
  -H "Content-Type: application/json" \
  -d '{"content":"delete me","visibility":"public"}')
DEL_ID=$(echo "$DEL_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
STATUS=$(curl -so /dev/null -w "%{http_code}" -X DELETE "$BASE/v1/pastes/$DEL_ID")
assert_status "delete returns 204" "204" "$STATUS"
STATUS=$(curl -so /dev/null -w "%{http_code}" "$BASE/v1/pastes/$DEL_ID")
assert_status "deleted paste returns 404" "404" "$STATUS"

# --- rate limit (61 rapid requests) ---
echo "-- Rate limit --"
RL_HIT=0
for i in $(seq 1 65); do
  S=$(curl -so /dev/null -w "%{http_code}" -X POST "$BASE/v1/pastes" \
    -H "Content-Type: application/json" \
    -d '{"content":"rl","visibility":"public"}')
  if [[ "$S" == "429" ]]; then ((RL_HIT++)); fi
done
if [[ $RL_HIT -gt 0 ]]; then ok "rate limiter triggered after burst (429 count=$RL_HIT)"
else fail "rate limit" "expected at least one 429 in 65 rapid requests"; fi

# --- metrics endpoint ---
echo "-- Metrics --"
METRICS=$(curl -sf "$BASE/metrics")
assert_contains "metrics: http requests counter" 'pastebin_http_requests_total' "$METRICS"
assert_contains "metrics: pastes created counter" 'pastebin_pastes_created_total' "$METRICS"
assert_contains "metrics: rate limit rejections" 'pastebin_rate_limit_rejections_total' "$METRICS"

# --- summary ---
echo
echo "================================"
echo "Results: $PASS passed, $FAIL failed"
echo "================================"
[[ $FAIL -eq 0 ]] && exit 0 || exit 1

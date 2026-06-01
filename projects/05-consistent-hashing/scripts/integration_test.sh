#!/usr/bin/env bash
set -euo pipefail

BASE="${API_URL:-http://localhost:8084}"
RING="test-ring-$$"

pass() { echo "  PASS: $1"; }
fail() { echo "  FAIL: $1"; exit 1; }

echo "=== Consistent Hashing Integration Tests ==="

# Health
echo "--- Health check"
curl -sf "$BASE/healthz" | grep -q '"ok"' && pass "healthz" || fail "healthz"

# Create ring
echo "--- Create ring"
curl -sf -X POST "$BASE/v1/rings" \
  -H 'Content-Type: application/json' \
  -d "{\"id\":\"$RING\",\"replicas\":50}" | grep -q "\"id\"" && pass "create ring" || fail "create ring"

# Add nodes
echo "--- Add nodes"
for n in node-a node-b node-c; do
  curl -sf -X POST "$BASE/v1/rings/$RING/nodes" \
    -H 'Content-Type: application/json' \
    -d "{\"id\":\"$n\",\"weight\":1}" | grep -q "VNodeCount" && pass "add $n" || fail "add $n"
done

# Key lookup
echo "--- Key lookup"
OWNER=$(curl -sf "$BASE/v1/rings/$RING/keys/my-test-key/owner" | grep -o '"owner":"[^"]*"' | cut -d'"' -f4)
[[ -n "$OWNER" ]] && pass "lookup (owner=$OWNER)" || fail "lookup"

# Replica lookup
echo "--- Replica lookup"
REPLICAS=$(curl -sf "$BASE/v1/rings/$RING/keys/my-test-key/replicas?n=3" | grep -o '"replicas":\[[^]]*\]')
[[ -n "$REPLICAS" ]] && pass "replicas ($REPLICAS)" || fail "replicas"

# Stats
echo "--- Stats"
STDDEV=$(curl -sf "$BASE/v1/rings/$RING/stats" | grep -o '"StdDev":[0-9.e+-]*' | cut -d: -f2)
[[ -n "$STDDEV" ]] && pass "stats (stddev=$STDDEV)" || fail "stats"

# Simulate
echo "--- Simulate"
curl -sf "$BASE/v1/rings/$RING/simulate?keys=5000" | grep -q "distribution" && pass "simulate" || fail "simulate"

# Remove node
echo "--- Remove node"
curl -sf -X DELETE "$BASE/v1/rings/$RING/nodes/node-b" | grep -q "VNodeCount" && pass "remove node" || fail "remove node"

# Verify removed node no longer owns keys
echo "--- Post-removal lookup"
for i in $(seq 1 20); do
  OWNER=$(curl -sf "$BASE/v1/rings/$RING/keys/key-$i/owner" | grep -o '"owner":"[^"]*"' | cut -d'"' -f4)
  [[ "$OWNER" == "node-b" ]] && fail "removed node still owns key-$i"
done
pass "post-removal routing"

# Cleanup
curl -sf -X DELETE "$BASE/v1/rings/$RING" >/dev/null

echo ""
echo "=== All tests passed ==="

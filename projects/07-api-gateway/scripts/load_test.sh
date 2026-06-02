#!/usr/bin/env bash
# load_test.sh — Basic throughput + latency test for the API gateway.
# Requires: curl, bc, awk, parallel (apt install parallel)
# Usage: bash scripts/load_test.sh [requests] [concurrency]
set -euo pipefail

PROXY="${PROXY_URL:-http://localhost:8088}"
N="${1:-200}"
C="${2:-20}"
KEY="${API_KEY:-alice-secret-token}"

echo "=== API Gateway Load Test ==="
echo "Target:      $PROXY"
echo "Requests:    $N"
echo "Concurrency: $C"
echo "Key:         $KEY"
echo ""

TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

# ---- Scenario 1: public route (no auth) ----
echo "--- Scenario 1: Public route /api/users/1 (no auth) ---"
seq 1 "$N" | parallel -j "$C" \
  'START=$(date +%s%3N); CODE=$(curl -so /dev/null -w "%{http_code}" '"$PROXY"'/api/users/1); END=$(date +%s%3N); echo "$CODE $((END-START))"' \
  > "$TMP/s1.txt" 2>/dev/null || true

awk '{codes[$1]++; sum+=$2; n++} END {
  print "  Requests:   " n
  for (c in codes) print "  HTTP " c ": " codes[c]
  if (n>0) print "  Avg ms:     " int(sum/n)
}' "$TMP/s1.txt"

echo ""

# ---- Scenario 2: authenticated route with rate limit ----
echo "--- Scenario 2: Auth route /api/orders/1 (key=alice, quota=30/min) ---"
seq 1 "$N" | parallel -j "$C" \
  'START=$(date +%s%3N); CODE=$(curl -so /dev/null -w "%{http_code}" -H "Authorization: Bearer '"$KEY"'" '"$PROXY"'/api/orders/1); END=$(date +%s%3N); echo "$CODE $((END-START))"' \
  > "$TMP/s2.txt" 2>/dev/null || true

awk '{codes[$1]++; sum+=$2; n++} END {
  print "  Requests:   " n
  for (c in codes) print "  HTTP " c ": " codes[c]
  if (n>0) print "  Avg ms:     " int(sum/n)
}' "$TMP/s2.txt"

echo ""

# ---- Scenario 3: rate-limit exhaustion ----
echo "--- Scenario 3: Rate-limit exhaustion (key=rate-test, quota=5/min) ---"
RK="${RATE_KEY:-rate-test-token}"
seq 1 20 | parallel -j 5 \
  'CODE=$(curl -so /dev/null -w "%{http_code}" -H "Authorization: Bearer '"$RK"'" '"$PROXY"'/api/users/1); echo "$CODE"' \
  > "$TMP/s3.txt" 2>/dev/null || true

awk '{codes[$1]++} END {
  for (c in codes) print "  HTTP " c ": " codes[c]
}' "$TMP/s3.txt"
echo "  (expect mix of 200 and 429)"

echo ""
echo "=== Done ==="

#!/usr/bin/env bash
# load_test.sh — baseline throughput test using hey (https://github.com/rakyll/hey).
# Install hey: go install github.com/rakyll/hey@latest
# Usage: ./scripts/load_test.sh [base_url]
set -euo pipefail

BASE=${1:-http://localhost:8082}
CONCURRENCY=50
REQUESTS=2000

echo "=== Write path: POST /v1/pastes (c=$CONCURRENCY n=$REQUESTS) ==="
hey -n $REQUESTS -c $CONCURRENCY \
  -m POST \
  -H "Content-Type: application/json" \
  -d '{"title":"load test","content":"x","visibility":"public"}' \
  "$BASE/v1/pastes"

echo
echo "=== Read path: GET /v1/pastes/{id} ==="
# Create one paste to use as the read target.
ID=$(curl -sf -X POST "$BASE/v1/pastes" \
  -H "Content-Type: application/json" \
  -d '{"title":"hotspot","content":"hot paste content","visibility":"public"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Read target: $ID"

hey -n $REQUESTS -c $CONCURRENCY "$BASE/v1/pastes/$ID"

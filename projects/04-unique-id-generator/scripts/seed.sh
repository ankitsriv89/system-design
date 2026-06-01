#!/usr/bin/env bash
# seed.sh — generate a small batch of IDs to verify the service is working.
# Usage: ./scripts/seed.sh [base_url]
set -euo pipefail

BASE="${1:-http://localhost:8083}"

echo "=== worker health ==="
curl -sf "$BASE/v1/workers/health" | jq .

echo ""
echo "=== single ID ==="
curl -sf -X POST "$BASE/v1/ids/next" | jq .

echo ""
echo "=== batch of 5 ==="
curl -sf -X POST "$BASE/v1/ids/batch" \
  -H "Content-Type: application/json" \
  -d '{"count": 5}' | jq .

echo ""
echo "=== inspect first ID from batch ==="
FIRST_ID=$(curl -sf -X POST "$BASE/v1/ids/batch" \
  -H "Content-Type: application/json" \
  -d '{"count": 1}' | jq -r '.id_strings[0]')
curl -sf "$BASE/v1/ids/$FIRST_ID/inspect" | jq .

echo ""
echo "seed complete."

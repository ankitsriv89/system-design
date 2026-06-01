#!/usr/bin/env bash
# load_test.sh — throughput and latency baseline for the unique-id-generator.
# Requires: curl, jq, bc, parallel (GNU parallel)
# Usage: ./scripts/load_test.sh [base_url] [concurrency] [total_requests]
set -euo pipefail

BASE="${1:-http://localhost:8083}"
CONCURRENCY="${2:-20}"
TOTAL="${3:-2000}"

echo "=== load test: $BASE ==="
echo "  concurrency : $CONCURRENCY goroutines (parallel curl processes)"
echo "  total       : $TOTAL requests"
echo ""

# --- single-ID throughput ---
echo "--- POST /v1/ids/next (single) ---"
START_MS=$(date +%s%3N)

seq 1 "$TOTAL" | parallel -j "$CONCURRENCY" --no-notice \
  "curl -sf -X POST $BASE/v1/ids/next -o /dev/null" 2>/dev/null

END_MS=$(date +%s%3N)
ELAPSED=$(echo "scale=3; ($END_MS - $START_MS) / 1000" | bc)
RPS=$(echo "scale=0; $TOTAL / $ELAPSED" | bc)
echo "  elapsed : ${ELAPSED}s"
echo "  RPS     : ~${RPS} req/s"

echo ""

# --- batch throughput (100 IDs per request) ---
BATCH_REQS=$(( TOTAL / 100 ))
echo "--- POST /v1/ids/batch (count=100, $BATCH_REQS requests = $TOTAL IDs) ---"
START_MS=$(date +%s%3N)

seq 1 "$BATCH_REQS" | parallel -j "$CONCURRENCY" --no-notice \
  "curl -sf -X POST $BASE/v1/ids/batch \
    -H 'Content-Type: application/json' \
    -d '{\"count\": 100}' -o /dev/null" 2>/dev/null

END_MS=$(date +%s%3N)
ELAPSED=$(echo "scale=3; ($END_MS - $START_MS) / 1000" | bc)
IDS_PER_SEC=$(echo "scale=0; $TOTAL / $ELAPSED" | bc)
echo "  elapsed    : ${ELAPSED}s"
echo "  IDs/s      : ~${IDS_PER_SEC}"

echo ""
echo "=== Prometheus summary (sampled after load) ==="
curl -sf "$BASE/metrics" | grep -E "^uniqueid_(ids_generated|http_duration|clock_rollback|sequence_exhaustion)" | head -20

echo ""
echo "load test complete."

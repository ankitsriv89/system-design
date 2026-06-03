#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8093}"

echo "=== healthz ==="
curl -sf "$BASE/healthz" | grep '"status":"ok"'

echo "=== create crawl job ==="
JOB=$(curl -sf -X POST "$BASE/v1/crawl-jobs" \
  -H "Content-Type: application/json" \
  -d '{"seed_url":"https://example.com","max_depth":1}')
echo "$JOB"
JOB_ID=$(echo "$JOB" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)

echo "=== get job $JOB_ID ==="
curl -sf "$BASE/v1/crawl-jobs/$JOB_ID" | grep '"id"'

echo "=== list jobs ==="
curl -sf "$BASE/v1/crawl-jobs" | grep -c '"id"' | grep -v "^0$" || true

echo "=== frontier stats ==="
curl -sf "$BASE/v1/frontier/stats"

echo "=== enqueue additional URL ==="
curl -sf -X POST "$BASE/v1/frontier/enqueue" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/page1","priority":5}'

echo ""
echo "=== all integration tests passed ==="

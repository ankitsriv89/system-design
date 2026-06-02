#!/usr/bin/env bash
# load_test.sh — basic throughput + distribution check.
# Requires: curl, jq, bc.
set -euo pipefail

BASE="${LB_URL:-http://localhost:8086}"
SVC="${SVC:-demo}"
N="${N:-200}"

echo "Sending $N requests to /proxy/$SVC/..."
START=$(date +%s%N)

for i in $(seq 1 "$N"); do
  curl -sf "$BASE/proxy/$SVC/" -o /dev/null &
done
wait

END=$(date +%s%N)
ELAPSED=$(( (END - START) / 1000000 ))
RPS=$(echo "scale=1; $N * 1000 / $ELAPSED" | bc)

echo "Completed $N requests in ${ELAPSED}ms (~${RPS} req/s)"
echo ""
echo "Backend distribution:"
curl -sf "$BASE/v1/stats" | jq -r '
  .[] | select(.service == "'"$SVC"'") |
  .backends[] | "\(.url)  total=\(.total_conns)  latency=\(.latency_ewma_ms)ms"
'

#!/usr/bin/env bash
# Load test — fires N notifications and measures throughput.
set -euo pipefail

BASE=${BASE_URL:-http://localhost:8091}
N=${LOAD_N:-200}
CONCURRENCY=${LOAD_C:-20}

echo "Load test: $N notifications, $CONCURRENCY concurrent"

if command -v hey &>/dev/null; then
  hey -n "$N" -c "$CONCURRENCY" \
    -m POST -T 'application/json' \
    -d '{"user_id":"load-user","channel":"email","subject":"load","body":"load test"}' \
    "$BASE/v1/notifications"
else
  echo "'hey' not found, falling back to sequential curl"
  START=$(date +%s%N)
  for i in $(seq 1 "$N"); do
    curl -sf -X POST "$BASE/v1/notifications" \
      -H 'Content-Type: application/json' \
      -d "{\"user_id\":\"load-user\",\"channel\":\"email\",\"subject\":\"load $i\",\"body\":\"load\"}" \
      > /dev/null
  done
  END=$(date +%s%N)
  ELAPSED=$(( (END - START) / 1000000 ))
  echo "Sent $N notifications in ${ELAPSED}ms ($(( N * 1000 / ELAPSED )) req/s)"
fi

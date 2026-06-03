#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8093}"
CONCURRENCY="${CONCURRENCY:-10}"
REQUESTS="${REQUESTS:-200}"

echo "=== load test: POST /v1/frontier/enqueue ($REQUESTS reqs, $CONCURRENCY concurrent) ==="
if command -v ab &>/dev/null; then
    ab -n "$REQUESTS" -c "$CONCURRENCY" -p /dev/stdin -T application/json \
       "$BASE/v1/frontier/enqueue" <<'EOF'
{"url":"https://example.com/load-test","priority":1}
EOF
else
    echo "ab not found — running sequential curl loop"
    for i in $(seq 1 "$REQUESTS"); do
        curl -sf -X POST "$BASE/v1/frontier/enqueue" \
          -H "Content-Type: application/json" \
          -d "{\"url\":\"https://example.com/page$i\",\"priority\":1}" > /dev/null
    done
    echo "$REQUESTS enqueue requests sent sequentially"
fi

echo "=== frontier stats after load ==="
curl -sf "$BASE/v1/frontier/stats"

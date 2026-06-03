#!/usr/bin/env bash
# Load test for 08-basic-key-value-store.
# Uses curl for portability; install hey or wrk for more accurate numbers.
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8088}"
N="${N:-500}"

echo "=== Load Test: $N sequential writes ==="
start=$(date +%s%N)
for i in $(seq 1 "$N"); do
  curl -s -X PUT "$BASE/v1/kv/load-$i" --data "value-for-key-$i-padding-xxxxxxxxxxxxxxxxxxxx" > /dev/null
done
end=$(date +%s%N)
elapsed_ms=$(( (end - start) / 1000000 ))
rps=$(( N * 1000 / elapsed_ms ))
echo "Wrote $N keys in ${elapsed_ms}ms (~${rps} ops/sec)"

echo ""
echo "=== Load Test: $N sequential reads ==="
start=$(date +%s%N)
for i in $(seq 1 "$N"); do
  curl -s "$BASE/v1/kv/load-$i" > /dev/null
done
end=$(date +%s%N)
elapsed_ms=$(( (end - start) / 1000000 ))
rps=$(( N * 1000 / elapsed_ms ))
echo "Read $N keys in ${elapsed_ms}ms (~${rps} ops/sec)"

echo ""
echo "=== Final stats ==="
curl -s "$BASE/v1/admin/stats" | python3 -m json.tool 2>/dev/null || \
  curl -s "$BASE/v1/admin/stats"

#!/usr/bin/env bash
# Load test the suggest endpoint.
# Requires: curl, bc
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8092}"
CONCURRENCY="${CONCURRENCY:-10}"
REQUESTS="${REQUESTS:-500}"
PREFIXES=("go" "re" "ty" "sys" "el" "ka" "ku" "pr" "do" "lo")

echo "Load test: $REQUESTS requests, concurrency $CONCURRENCY"
echo "Target: $BASE/v1/suggest"
echo "---"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

run_request() {
  local prefix="${PREFIXES[$((RANDOM % ${#PREFIXES[@]}))]}"
  local start end ms
  start=$(date +%s%N)
  http_code=$(curl -sf -o /dev/null -w "%{http_code}" \
    "$BASE/v1/suggest?q=${prefix}&locale=en&limit=8" 2>/dev/null || echo "000")
  end=$(date +%s%N)
  ms=$(( (end - start) / 1000000 ))
  echo "$ms $http_code"
}
export -f run_request
export BASE PREFIXES

seq 1 "$REQUESTS" | xargs -P "$CONCURRENCY" -I{} bash -c 'run_request' > "$tmpdir/results.txt"

total=$(wc -l < "$tmpdir/results.txt")
ok=$(awk '$2 == 200 {c++} END {print c+0}' "$tmpdir/results.txt")
errors=$(( total - ok ))

mapfile -t latencies < <(awk '{print $1}' "$tmpdir/results.txt" | sort -n)
count=${#latencies[@]}
p50=${latencies[$((count * 50 / 100))]}
p95=${latencies[$((count * 95 / 100))]}
p99=${latencies[$((count * 99 / 100))]}
sum=0
for v in "${latencies[@]}"; do sum=$((sum + v)); done
avg=$((sum / count))

echo "Results:"
echo "  Total:   $total"
echo "  OK (200): $ok"
echo "  Errors:  $errors"
echo "  p50: ${p50}ms  p95: ${p95}ms  p99: ${p99}ms  avg: ${avg}ms"

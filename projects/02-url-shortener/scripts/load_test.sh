#!/usr/bin/env sh
set -eu

BASE_URL="${BASE_URL:-http://localhost:8081}"
CONCURRENCY="${1:-10}"
REQUESTS="${2:-500}"

created="$(curl -s -X POST "$BASE_URL/v1/links" \
  -H "Content-Type: application/json" \
  -H "X-Owner-ID: demo" \
  -d '{"long_url":"https://example.com/hotspot"}')"
code="$(printf "%s" "$created" | sed -n 's/.*"code":"\([^"]*\)".*/\1/p')"

if [ -z "$code" ]; then
  echo "failed to create link: $created" >&2
  exit 1
fi

echo "Hotspot code: $code"
echo "Running $REQUESTS requests with concurrency $CONCURRENCY"

seq "$REQUESTS" | xargs -P "$CONCURRENCY" -I {} curl -s -o /dev/null -w "%{http_code}\n" "$BASE_URL/$code" \
  | sort | uniq -c

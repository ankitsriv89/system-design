#!/usr/bin/env sh
set -eu

BASE_URL="${BASE_URL:-http://localhost:8081}"
OWNER_ID="${OWNER_ID:-demo}"

curl -s -X POST "$BASE_URL/v1/links" \
  -H "Content-Type: application/json" \
  -H "X-Owner-ID: $OWNER_ID" \
  -d '{"long_url":"https://example.com/system-design/url-shortener"}'

printf "\n"

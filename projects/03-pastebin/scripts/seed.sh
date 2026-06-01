#!/usr/bin/env bash
# seed.sh — create a handful of pastes to verify the system is working.
# Usage: ./scripts/seed.sh [base_url]
set -euo pipefail

BASE=${1:-http://localhost:8082}

echo "Seeding pastes against $BASE"
echo

create() {
  local label=$1
  local payload=$2
  local id
  id=$(curl -sf -X POST "$BASE/v1/pastes" \
    -H "Content-Type: application/json" \
    -d "$payload" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
  echo "[$label] id=$id  GET $BASE/v1/pastes/$id"
}

# Public paste — no expiry
create "public" '{"title":"Hello World","language":"go","visibility":"public","content":"package main\n\nfunc main() { println(\"hello\") }"}'

# Unlisted paste — expires in 60 seconds
create "unlisted+ttl" '{"title":"Short-lived","visibility":"unlisted","content":"This expires in 60s","ttl_seconds":60}'

# Burn-after-read
create "burn" '{"title":"Read once","visibility":"unlisted","content":"This self-destructs after first read","burn_after_read":true}'

echo
echo "Done. Check Grafana at http://localhost:3000"

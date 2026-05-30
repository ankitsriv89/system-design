#!/usr/bin/env bash
# Seed default policies into the rate limiter.
set -euo pipefail

BASE=${BASE_URL:-http://localhost:8080}

echo "==> Seeding policies at $BASE"

# Token-bucket policy: 10 req/s per user, burst up to 20
curl -sf -X PUT "$BASE/v1/policies/user-token-bucket" \
  -H "Content-Type: application/json" \
  -d '{
    "subject_type": "user",
    "algorithm": "token_bucket",
    "capacity": 20,
    "refill_rate": 10,
    "window_ms": 0,
    "action": "deny"
  }' | jq .

# Sliding-window policy: 100 req/minute per IP
curl -sf -X PUT "$BASE/v1/policies/ip-sliding-window" \
  -H "Content-Type: application/json" \
  -d '{
    "subject_type": "ip",
    "algorithm": "sliding_window",
    "capacity": 100,
    "refill_rate": 0,
    "window_ms": 60000,
    "action": "deny"
  }' | jq .

echo "==> Policies seeded"
curl -sf "$BASE/v1/policies" | jq .

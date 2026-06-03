#!/usr/bin/env bash
# Integration tests — requires a running Docker Compose stack.
set -euo pipefail

BASE=${BASE_URL:-http://localhost:8091}
PASS=0; FAIL=0

check() {
  local name="$1" got="$2" want="$3"
  if [[ "$got" == *"$want"* ]]; then
    echo "  PASS  $name"
    ((PASS++))
  else
    echo "  FAIL  $name: got '$got' want '$want'"
    ((FAIL++))
  fi
}

echo "=== Health check ==="
OUT=$(curl -sf "$BASE/healthz")
check "healthz" "$OUT" "ok"

echo "=== Create template ==="
OUT=$(curl -sf -X POST "$BASE/v1/templates" -H 'Content-Type: application/json' -d '{
  "id":"test-tmpl","channel":"email",
  "subject":"Hi {{.Name}}","body":"Code: {{.Code}}"
}')
check "create-template" "$OUT" "test-tmpl"

echo "=== List templates ==="
OUT=$(curl -sf "$BASE/v1/templates")
check "list-templates" "$OUT" "test-tmpl"

echo "=== Set preferences ==="
curl -sf -X PUT "$BASE/v1/preferences/test-user" -H 'Content-Type: application/json' \
  -d '[{"channel":"email","enabled":true,"quiet_start":-1,"quiet_end":-1}]'
OUT=$(curl -sf "$BASE/v1/preferences/test-user")
check "get-preferences" "$OUT" "email"

echo "=== Opt-out preference ==="
curl -sf -X PUT "$BASE/v1/preferences/test-user-optout" -H 'Content-Type: application/json' \
  -d '[{"channel":"sms","enabled":false,"quiet_start":-1,"quiet_end":-1}]'
OUT=$(curl -sf -X POST "$BASE/v1/notifications" -H 'Content-Type: application/json' -d '{
  "user_id":"test-user-optout","channel":"sms","body":"test","subject":"test"
}')
check "opt-out-skipped" "$OUT" "skipped"

echo "=== Send notification ==="
OUT=$(curl -sf -X POST "$BASE/v1/notifications" -H 'Content-Type: application/json' -d '{
  "user_id":"test-user","channel":"email",
  "template_id":"test-tmpl","params":{"Name":"Bob","Code":"9999"}
}')
check "send-notification" "$OUT" "queued"
NOTIF_ID=$(echo "$OUT" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

echo "=== Get notification ==="
OUT=$(curl -sf "$BASE/v1/notifications/$NOTIF_ID")
check "get-notification" "$OUT" "$NOTIF_ID"

echo "=== Idempotency ==="
curl -sf -X POST "$BASE/v1/notifications" -H 'Content-Type: application/json' -d '{
  "user_id":"test-user","channel":"email","body":"idem","idempotency_key":"idem-key-1"
}' > /dev/null
OUT=$(curl -sf -X POST "$BASE/v1/notifications" -H 'Content-Type: application/json' -d '{
  "user_id":"test-user","channel":"email","body":"idem","idempotency_key":"idem-key-1"
}')
check "idempotency-coalesced" "$OUT" "idem-key-1"

echo "=== Queue stats ==="
OUT=$(curl -sf "$BASE/v1/admin/queue/stats")
check "queue-stats" "$OUT" "queue_depth"

echo "=== Set failure rate ==="
OUT=$(curl -sf -X PUT "$BASE/v1/admin/provider/email/failure-rate" \
  -H 'Content-Type: application/json' -d '{"rate":0.5}')
check "set-failure-rate" "$OUT" "0.5"

echo ""
echo "Results: $PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]]

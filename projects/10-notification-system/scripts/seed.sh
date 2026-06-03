#!/usr/bin/env bash
# Seed demo templates and preferences for local testing.
set -euo pipefail

BASE=${BASE_URL:-http://localhost:8091}

echo "→ creating templates"
curl -sf -X POST "$BASE/v1/templates" -H 'Content-Type: application/json' -d '{
  "id": "welcome",
  "channel": "email",
  "subject": "Welcome, {{.Name}}!",
  "body": "Hi {{.Name}}, thanks for joining. Your verification code is {{.Code}}."
}' | jq .

curl -sf -X POST "$BASE/v1/templates" -H 'Content-Type: application/json' -d '{
  "id": "otp",
  "channel": "sms",
  "subject": "",
  "body": "Your OTP is {{.Code}}. Valid for 10 minutes."
}' | jq .

curl -sf -X POST "$BASE/v1/templates" -H 'Content-Type: application/json' -d '{
  "id": "push-promo",
  "channel": "push",
  "subject": "{{.Title}}",
  "body": "{{.Body}}"
}' | jq .

echo "→ creating preferences for demo-user-1 (all channels on)"
curl -sf -X PUT "$BASE/v1/preferences/demo-user-1" -H 'Content-Type: application/json' -d '[
  {"channel":"email","enabled":true,"quiet_start":-1,"quiet_end":-1},
  {"channel":"sms","enabled":true,"quiet_start":-1,"quiet_end":-1},
  {"channel":"push","enabled":true,"quiet_start":-1,"quiet_end":-1}
]'

echo "→ creating preferences for demo-user-2 (sms opted out)"
curl -sf -X PUT "$BASE/v1/preferences/demo-user-2" -H 'Content-Type: application/json' -d '[
  {"channel":"email","enabled":true,"quiet_start":-1,"quiet_end":-1},
  {"channel":"sms","enabled":false,"quiet_start":-1,"quiet_end":-1},
  {"channel":"push","enabled":true,"quiet_start":-1,"quiet_end":-1}
]'

echo "→ sending a test notification"
curl -sf -X POST "$BASE/v1/notifications" -H 'Content-Type: application/json' -d '{
  "user_id": "demo-user-1",
  "channel": "email",
  "template_id": "welcome",
  "params": {"Name":"Alice","Code":"5678"}
}' | jq .

echo "done"

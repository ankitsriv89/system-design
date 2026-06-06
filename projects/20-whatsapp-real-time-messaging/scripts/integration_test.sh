#!/usr/bin/env bash
# Integration smoke test for project 20: whatsapp-real-time-messaging.
# Assumes the stack is up (docker compose up) and reachable on :8101.
set -euo pipefail

BASE="${BASE:-http://localhost:8101}"

echo "== health =="
curl -fsS "$BASE/actuator/health" | grep -q '"status":"UP"' && echo "OK: service is UP"

# Further checks (device registration, WS session, send + receipt, sync drain)
# are added as the build milestones land. See docs/architecture.md.
echo "== done =="

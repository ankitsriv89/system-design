#!/usr/bin/env bash
# Integration smoke test for project 18 — instagram.
# Brings the service up via docker compose and exercises the core happy path:
# create user -> upload media -> create post -> read feed -> like.
# Fleshed out during the build milestones.
set -euo pipefail

BASE="${BASE:-http://localhost:8099}"

echo "== health =="
curl -fsS "${BASE}/actuator/health" && echo

echo "scaffold: integration steps added during build milestones."

#!/usr/bin/env bash
# Integration smoke test for project 18 — instagram.
# Exercises the core happy path against a running stack (docker compose up):
# begin upload -> PUT bytes -> complete -> wait for PROCESSED -> create post ->
# read feed -> like.
set -euo pipefail

BASE="${BASE:-http://localhost:8099}"
USER="${USER_ID:-1}"

say() { printf '\n== %s ==\n' "$1"; }

say "health"
curl -fsS "${BASE}/actuator/health" >/dev/null && echo "ok"

say "begin upload"
UP=$(curl -fsS -X POST "${BASE}/v1/media/uploads" \
  -H "X-User-Id: ${USER}" -H 'Content-Type: application/json' \
  -d '{"contentType":"image/png"}')
echo "$UP"
MEDIA_ID=$(echo "$UP" | grep -o '"mediaId":[0-9]*' | grep -o '[0-9]*')
UPLOAD_URL=$(echo "$UP" | sed -n 's/.*"uploadUrl":"\([^"]*\)".*/\1/p')

say "PUT bytes (1x1 png)"
printf '\x89PNG\r\n\x1a\n' > /tmp/p18_sample.png   # minimal header; real bytes via UI
# Generate a valid tiny PNG with ImageMagick if available, else skip strict check.
if command -v convert >/dev/null; then
  convert -size 64x64 xc:skyblue /tmp/p18_sample.png
fi
curl -fsS -X PUT "${UPLOAD_URL}" --data-binary @/tmp/p18_sample.png && echo "uploaded"

say "complete upload"
curl -fsS -X POST "${BASE}/v1/media/${MEDIA_ID}/complete" -H "X-User-Id: ${USER}"; echo

say "wait for PROCESSED"
for i in $(seq 1 20); do
  STATUS=$(curl -fsS "${BASE}/v1/media/${MEDIA_ID}" | grep -o '"status":"[A-Z]*"' | head -1)
  echo "  attempt $i: $STATUS"
  [[ "$STATUS" == '"status":"PROCESSED"' ]] && break
  sleep 1
done

say "create post"
POST=$(curl -fsS -X POST "${BASE}/v1/posts" \
  -H "X-User-Id: ${USER}" -H 'Content-Type: application/json' \
  -d "{\"mediaId\":${MEDIA_ID},\"caption\":\"integration test\"}")
echo "$POST"
POST_ID=$(echo "$POST" | grep -o '"id":[0-9]*' | head -1 | grep -o '[0-9]*')

say "read feed"
curl -fsS "${BASE}/v1/feed?limit=20" -H "X-User-Id: ${USER}"; echo

say "like post ${POST_ID}"
curl -fsS -X POST "${BASE}/v1/posts/${POST_ID}/likes" -H "X-User-Id: 2"; echo

echo -e "\nAll integration steps completed."

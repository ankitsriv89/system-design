#!/usr/bin/env bash
# Integration test for project 21 — Dropbox File Storage and Sync
# Usage: ./scripts/integration_test.sh [BASE_URL]
set -euo pipefail

BASE=${1:-http://localhost:8102}
API_KEY=${API_KEY:-demo-secret}
OWNER="user-$(date +%s)"
DEVICE="device-$(date +%s)"
PASS=0; FAIL=0

ok()   { echo "  PASS: $1"; ((PASS++)); }
fail() { echo "  FAIL: $1 — $2"; ((FAIL++)); }

# Common headers for all authenticated requests
AUTH=(-H "X-Api-Key: $API_KEY" -H "X-Owner-Id: $OWNER")

echo "=== Dropbox Integration Tests ==="
echo "Base URL : $BASE"
echo "Owner    : $OWNER"
echo ""

# Health
echo "[1] Health check"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${AUTH[@]}" "$BASE/v1/health")
[ "$STATUS" = "200" ] && ok "health 200" || fail "health" "$STATUS"

# Reject missing API key
echo "[2] Reject unauthenticated request"
UNAUTH=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/v1/folders")
[ "$UNAUTH" = "401" ] && ok "401 without API key" || fail "auth guard" "$UNAUTH"

# List root (empty)
echo "[3] List root folder"
BODY=$(curl -s "${AUTH[@]}" "$BASE/v1/folders")
echo "$BODY" | grep -q "\[\]" && ok "empty root" || fail "list root" "$BODY"

# Create folder
echo "[4] Create folder"
FOLDER=$(curl -s -X POST "${AUTH[@]}" "$BASE/v1/folders?name=Documents")
FOLDER_ID=$(echo "$FOLDER" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || true)
[ -n "$FOLDER_ID" ] && ok "created folder $FOLDER_ID" || fail "create folder" "$FOLDER"

# Upload file
echo "[5] Upload file"
echo "hello dropbox" > /tmp/test_upload.txt
UPLOAD=$(curl -s -X POST "${AUTH[@]}" \
  -F "file=@/tmp/test_upload.txt" \
  "$BASE/v1/files?parentId=$FOLDER_ID")
FILE_ID=$(echo "$UPLOAD" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || true)
[ -n "$FILE_ID" ] && ok "uploaded file $FILE_ID" || fail "upload file" "$UPLOAD"

# Get file metadata
echo "[6] Get file metadata"
META=$(curl -s "${AUTH[@]}" "$BASE/v1/files/$FILE_ID")
echo "$META" | grep -q "test_upload" && ok "file metadata" || fail "get file" "$META"

# Sync poll
echo "[7] Sync poll"
SYNC=$(curl -s "${AUTH[@]}" -H "X-Device-Id: $DEVICE" "$BASE/v1/sync?cursor=0")
echo "$SYNC" | grep -q "events" && ok "sync feed returned" || fail "sync" "$SYNC"

# Delete file
echo "[8] Delete file"
DEL_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "${AUTH[@]}" "$BASE/v1/files/$FILE_ID")
[ "$DEL_STATUS" = "204" ] && ok "deleted file" || fail "delete file" "$DEL_STATUS"

# Sync after delete (should contain delete event)
echo "[9] Sync shows delete event"
SYNC2=$(curl -s "${AUTH[@]}" -H "X-Device-Id: $DEVICE" "$BASE/v1/sync?cursor=0")
echo "$SYNC2" | grep -q "file.deleted" && ok "delete event in sync feed" || fail "delete in sync" "$SYNC2"

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1

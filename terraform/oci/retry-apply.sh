#!/usr/bin/env bash
# =============================================================================
# retry-apply.sh — Try to provision the OCI free-tier A1 instance.
#
# OCI free-tier Ampere A1 capacity in ap-mumbai-1 is frequently exhausted.
# This script runs `terraform apply` up to MAX_ATTEMPTS times, stopping as
# soon as the instance is successfully created or a non-capacity error occurs.
#
# Designed to be called by cron between 12:00–04:00 IST (06:30–10:30 UTC),
# when OCI typically reclaims and releases free-tier capacity.
#
# Usage (manual):
#   cd terraform/oci && bash retry-apply.sh
#
# Logs: appended to /var/log/oci-retry-apply.log (or ~/oci-retry-apply.log
# if /var/log is not writable)
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="${OCI_RETRY_LOG:-${HOME}/oci-retry-apply.log}"
MAX_ATTEMPTS="${OCI_RETRY_MAX:-8}"
SLEEP_BETWEEN="${OCI_RETRY_SLEEP:-600}"   # 10 min between attempts

# Capacity-exhaustion substrings in OCI error messages
CAPACITY_ERRORS=(
  "Out of capacity"
  "out of capacity"
  "InternalError"
  "NotAuthorizedOrNotFound"
  "LimitExceeded"
)

log() { echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] $*" | tee -a "$LOG_FILE"; }

is_capacity_error() {
  local output="$1"
  for pattern in "${CAPACITY_ERRORS[@]}"; do
    if echo "$output" | grep -q "$pattern"; then
      return 0
    fi
  done
  return 1
}

cd "$SCRIPT_DIR"

log "=== OCI retry-apply started (max $MAX_ATTEMPTS attempts, ${SLEEP_BETWEEN}s between) ==="
log "    Terraform dir: $SCRIPT_DIR"
log "    Log file:      $LOG_FILE"

# Verify credentials are reachable before burning attempts
if ! terraform version -json >/dev/null 2>&1; then
  log "ERROR: terraform not found in PATH — aborting"
  exit 1
fi

attempt=0
while [ "$attempt" -lt "$MAX_ATTEMPTS" ]; do
  attempt=$((attempt + 1))
  log "--- Attempt $attempt / $MAX_ATTEMPTS ---"

  set +e
  output=$(terraform apply -auto-approve -input=false 2>&1)
  exit_code=$?
  set -e

  echo "$output" >> "$LOG_FILE"

  if [ "$exit_code" -eq 0 ]; then
    log "SUCCESS — OCI instance provisioned on attempt $attempt"
    log "$(terraform output -no-color 2>/dev/null || true)"
    exit 0
  fi

  if is_capacity_error "$output"; then
    log "Capacity not available (attempt $attempt) — will retry in ${SLEEP_BETWEEN}s"
    if [ "$attempt" -lt "$MAX_ATTEMPTS" ]; then
      sleep "$SLEEP_BETWEEN"
    fi
  else
    log "ERROR — non-capacity failure on attempt $attempt — aborting to avoid destructive retries"
    log "Last output tail:"
    echo "$output" | tail -20 | tee -a "$LOG_FILE"
    exit 1
  fi
done

log "EXHAUSTED — $MAX_ATTEMPTS attempts made, OCI capacity still unavailable"
log "Try again later or switch to a different region (us-ashburn-1 / eu-frankfurt-1)"
exit 2

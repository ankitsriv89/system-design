#!/usr/bin/env bash
# Runs before any docker-compose.yml is written.
# Extracts ports from the incoming content and checks them against infra/ports.md.

PORTS_MD="infra/ports.md"
INPUT=$(cat)  # PreToolUse hook receives tool input as JSON on stdin

# Only check Write/Edit calls targeting docker-compose.yml
FILE=$(echo "$INPUT" | grep -o '"file_path":"[^"]*"' | head -1 | cut -d'"' -f4)
[[ "$FILE" != *docker-compose.yml ]] && exit 0

# Extract port numbers from the content being written
CONTENT=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('content','') or d.get('new_string',''))" 2>/dev/null)
PORTS=$(echo "$CONTENT" | grep -oE '(:|-)([0-9]{4,5})' | grep -oE '[0-9]{4,5}' | sort -u)

if [[ -z "$PORTS" ]]; then
  exit 0
fi

if [[ ! -f "$PORTS_MD" ]]; then
  exit 0
fi

CONFLICTS=""
for PORT in $PORTS; do
  if grep -qE "\b$PORT\b" "$PORTS_MD"; then
    # Check if it's already assigned to a *different* project
    OWNER=$(grep -E "\b$PORT\b" "$PORTS_MD" | head -1)
    PROJECT_DIR=$(echo "$FILE" | grep -oE 'projects/[^/]+')
    if ! echo "$OWNER" | grep -q "$(echo $PROJECT_DIR | cut -d/ -f2)"; then
      CONFLICTS="$CONFLICTS\n  Port $PORT already used: $OWNER"
    fi
  fi
done

if [[ -n "$CONFLICTS" ]]; then
  echo "PORT CONFLICT DETECTED — check infra/ports.md before proceeding:"
  echo -e "$CONFLICTS"
  exit 1
fi

exit 0

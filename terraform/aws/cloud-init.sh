# =============================================================================
# cloud-init.sh — Bootstrap script for AWS Spot instance
#
# NOT a shebang script — prepended by Terraform with:
#   #!/bin/bash
#   export REPO_URL='...'
#   export GITHUB_TOKEN='...'
#
# Steps:
#   1. System updates + essential packages
#   2. Install Docker + Docker Compose plugin
#   3. Clone repo
#   4. Start shared infra (Prometheus, Grafana, node-exporter, Postgres)
#   5. Run schema migrations (projects 03 + 04)
#   6. Deploy all projects
#   7. Write deployment status file
# =============================================================================

set -euo pipefail
exec > /var/log/cloud-init-user.log 2>&1

echo "=== cloud-init started at $(date) ==="

# ── 1. System packages ────────────────────────────────────────────────────────
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get upgrade -y -qq
apt-get install -y -qq \
  git curl wget unzip jq htop \
  ca-certificates gnupg lsb-release \
  prometheus-node-exporter

# ── 2. Docker ─────────────────────────────────────────────────────────────────
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
  | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
  https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
  > /etc/apt/sources.list.d/docker.list

apt-get update -qq
apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin

systemctl enable docker
systemctl start docker
usermod -aG docker ubuntu

echo "=== Docker installed: $(docker --version) ==="

# ── 3. Clone repo ─────────────────────────────────────────────────────────────
REPO_DIR="/home/ubuntu/system-design"

if [ -n "${GITHUB_TOKEN}" ]; then
  # Use HTTP header so the token is never written to .git/config or the URL
  git -c "http.extraHeader=Authorization: bearer ${GITHUB_TOKEN}" clone "${REPO_URL}" "$REPO_DIR"
else
  git clone "${REPO_URL}" "$REPO_DIR"
fi
# Scrub token from environment immediately after clone
unset GITHUB_TOKEN
chown -R ubuntu:ubuntu "$REPO_DIR"
echo "=== Repo cloned to $REPO_DIR ==="

# ── 4. Start shared infra ─────────────────────────────────────────────────────
cd "$REPO_DIR"
sudo -u ubuntu docker compose -f infra/docker-compose.yml up -d --remove-orphans
echo "=== Shared infra started ==="

echo "=== Waiting for Postgres to become healthy... ==="
for i in $(seq 1 30); do
  if docker exec infra-postgres-1 pg_isready -U admin -q 2>/dev/null; then
    echo "=== Postgres ready after ${i}s ==="
    break
  fi
  sleep 2
done

# ── 5. Run schema migrations for projects that need them ─────────────────────
# Projects 01 and 02 apply CREATE TABLE IF NOT EXISTS automatically on startup.
# Projects 03 and 04 require an explicit migrate.sql run against the shared PG.
run_migration() {
  local project="$1" user="$2" db="$3"
  local sql="$REPO_DIR/projects/$project/scripts/migrate.sql"
  if [ -f "$sql" ]; then
    echo "=== Running migrations for $project ==="
    docker exec -i infra-postgres-1 psql -U "$user" -d "$db" < "$sql"
    echo "=== Migrations done for $project ==="
  fi
}

run_migration "03-pastebin"           "paste"    "pastebin"
run_migration "04-unique-id-generator" "uniqueid" "uniqueid"

# ── 6. Deploy each project ────────────────────────────────────────────────────
PROJECTS=(
  "01-rate-limiter"
  "02-url-shortener"
  "03-pastebin"
  "04-unique-id-generator"
)

for project in "${PROJECTS[@]}"; do
  dir="$REPO_DIR/projects/$project"
  if [ -f "$dir/docker-compose.yml" ]; then
    echo "=== Deploying $project ==="
    sudo -u ubuntu docker compose -f "$dir/docker-compose.yml" build --pull -q
    sudo -u ubuntu docker compose -f "$dir/docker-compose.yml" up -d --remove-orphans
    echo "=== $project deployed ==="
  else
    echo "=== Skipping $project (no docker-compose.yml) ==="
  fi
done

# ── 7. Write status file ──────────────────────────────────────────────────────
STATUS_FILE="/home/ubuntu/deployment-status.txt"
cat > "$STATUS_FILE" <<EOF
=== System Design — AWS Spot Deployment Status ===
Bootstrapped at: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
Instance type:   $(curl -s http://169.254.169.254/latest/meta-data/instance-type)
Public IP:       $(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)
AZ:              $(curl -s http://169.254.169.254/latest/meta-data/placement/availability-zone)
AMI ID:          $(curl -s http://169.254.169.254/latest/meta-data/ami-id)
Spot notice:     $(curl -s http://169.254.169.254/latest/meta-data/spot/termination-time 2>/dev/null || echo "none")

=== Running containers ===
$(docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}")

=== Deployed projects ===
EOF

for project in "${PROJECTS[@]}"; do
  dir="$REPO_DIR/projects/$project"
  if [ -f "$dir/docker-compose.yml" ]; then
    status=$(sudo -u ubuntu docker compose -f "$dir/docker-compose.yml" ps --format "table {{.Name}}\t{{.Status}}" 2>/dev/null | tail -n +2 || echo "unknown")
    echo "  $project: $status" >> "$STATUS_FILE"
  fi
done

chown ubuntu:ubuntu "$STATUS_FILE"
echo "=== cloud-init complete at $(date) ==="
echo "=== Check ~/deployment-status.txt or run: docker ps ==="

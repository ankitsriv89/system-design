#!/bin/bash
# =============================================================================
# cloud-init-runner.sh — Bootstrap a project-runner instance (go / java / other)
#
# Injected variables (from Terraform templatefile):
#   REPO_URL, GITHUB_TOKEN, STACK_NAME (go|java|other), INFRA_HOST (private IP)
#
# What this does:
#   1. Installs Docker
#   2. Clones the repo
#   3. Writes /etc/system-design-env so all project composes pick up shared infra coords
#   4. Deploys the projects belonging to this stack
#
# Projects are assigned to stacks by STACK_NAME:
#   go    — pure Go projects (no JVM, no Python)
#   java  — Java/Spring Boot projects (JVM-heavy, Kafka-heavy)
#   other — Python, Node.js, or mixed-runtime projects
# =============================================================================

set -euo pipefail
exec > /var/log/cloud-init-user.log 2>&1
echo "=== runner (${STACK_NAME}) cloud-init started at $(date) ==="

export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get upgrade -y -qq
apt-get install -y -qq git curl wget ca-certificates gnupg lsb-release htop prometheus-node-exporter

# ── Docker ────────────────────────────────────────────────────────────────────
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
  https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" > /etc/apt/sources.list.d/docker.list
apt-get update -qq
apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
systemctl enable docker && systemctl start docker
usermod -aG docker ubuntu

# ── Clone repo ────────────────────────────────────────────────────────────────
REPO_DIR="/home/ubuntu/system-design"
if [ -n "${GITHUB_TOKEN}" ]; then
  git -c "http.extraHeader=Authorization: bearer ${GITHUB_TOKEN}" clone "${REPO_URL}" "$REPO_DIR"
else
  git clone "${REPO_URL}" "$REPO_DIR"
fi
unset GITHUB_TOKEN
chown -R ubuntu:ubuntu "$REPO_DIR"

# ── Write shared-infra coordinates so project composes can use them ───────────
# Projects read POSTGRES_HOST, REDIS_HOST, KAFKA_BOOTSTRAP from environment.
# Each project's docker-compose should use ${POSTGRES_HOST:-postgres} etc. so
# they default to localhost (single-machine mode) and work here via override.
cat > /etc/system-design-env <<EOF
POSTGRES_HOST=${INFRA_HOST}
POSTGRES_PORT=5432
REDIS_HOST=${INFRA_HOST}
REDIS_PORT=6379
KAFKA_BOOTSTRAP=${INFRA_HOST}:9092
INFRA_HOST=${INFRA_HOST}
EOF
chmod 644 /etc/system-design-env

# Make the env available to docker compose invocations
echo 'set -a; source /etc/system-design-env; set +a' >> /home/ubuntu/.bashrc
chown ubuntu:ubuntu /home/ubuntu/.bashrc

# ── Fetch public IP (for BASE_URL injection into url-shortener etc.) ──────────
IMDS_TOKEN=$(curl -s -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 60")
PUBLIC_IP=$(curl -s -H "X-aws-ec2-metadata-token: ${IMDS_TOKEN}" http://169.254.169.254/latest/meta-data/public-ipv4)

# ── Deploy projects for this stack ────────────────────────────────────────────
# Populated at build time by looking at infra/ports.md stacks column.
# Add new projects here as they are built.

case "${STACK_NAME}" in
  go)
    PROJECTS=(
      "01-rate-limiter"
      "02-url-shortener"
      "03-pastebin"
      "04-unique-id-generator"
      "05-consistent-hashing"
      "06-load-balancer"
      "07-api-gateway"
      "08-basic-key-value-store"
      "09-caching-system"
    )
    ;;
  java)
    PROJECTS=(
      "10-notification-system"
      "14-one-to-one-chat-system"
    )
    ;;
  other)
    PROJECTS=()
    # Populated as Python/Node projects are built (p24, p46, etc.)
    ;;
  *)
    echo "ERROR: unknown STACK_NAME=${STACK_NAME}"
    exit 1
    ;;
esac

source /etc/system-design-env

for project in "${PROJECTS[@]}"; do
  dir="$REPO_DIR/projects/$project"
  if [ ! -f "$dir/docker-compose.yml" ]; then
    echo "=== Skipping $project (no docker-compose.yml) ==="
    continue
  fi

  echo "=== Deploying $project ==="

  # url-shortener needs BASE_URL pointing at this runner's public IP
  if [ "$project" = "02-url-shortener" ]; then
    echo "BASE_URL=http://${PUBLIC_IP}:8085" > "$dir/.env"
    chown ubuntu:ubuntu "$dir/.env"
  fi

  # Pass shared infra env into compose
  env $(cat /etc/system-design-env | xargs) \
    sudo -u ubuntu -E docker compose -f "$dir/docker-compose.yml" build --pull -q

  env $(cat /etc/system-design-env | xargs) \
    sudo -u ubuntu -E docker compose -f "$dir/docker-compose.yml" up -d --remove-orphans

  echo "=== $project deployed ==="
done

echo "=== runner (${STACK_NAME}) cloud-init complete at $(date) ==="
echo "Shared infra at: ${INFRA_HOST} | Public IP: ${PUBLIC_IP}"

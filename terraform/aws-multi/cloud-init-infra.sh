#!/bin/bash
# =============================================================================
# cloud-init-infra.sh — Bootstrap the shared-infra instance
#
# Injected variables (from Terraform templatefile):
#   REPO_URL, GITHUB_TOKEN
#
# Runs: Postgres, Redis, Kafka (KRaft), Prometheus, Grafana, node-exporter.
# All project runner instances connect to this node over the private VPC IP.
# =============================================================================

set -euo pipefail
exec > /var/log/cloud-init-user.log 2>&1
echo "=== infra cloud-init started at $(date) ==="

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

# ── Write extended infra compose (adds Kafka KRaft + Redis) ──────────────────
# The shared infra/docker-compose.yml already has Postgres, Prometheus, Grafana.
# On the multi-instance setup we also need Redis and Kafka here.
cat > /home/ubuntu/infra-extra.yml <<'COMPOSE'
services:
  redis:
    image: redis:7-alpine
    restart: unless-stopped
    ports:
      - "6379:6379"
    networks:
      - infra

  kafka:
    image: apache/kafka:3.9.0
    restart: unless-stopped
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_LISTENERS: PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://${PRIVATE_IP}:9092
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@localhost:9093
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_LOG_DIRS: /var/lib/kafka/data
      KAFKA_HEAP_OPTS: "-Xmx512m -Xms256m"
    ports:
      - "9092:9092"
    volumes:
      - kafka_data:/var/lib/kafka/data
    networks:
      - infra

volumes:
  kafka_data:

networks:
  infra:
    name: infra
COMPOSE

# Inject private IP into Kafka advertised listeners
IMDS_TOKEN=$(curl -s -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 60")
PRIVATE_IP=$(curl -s -H "X-aws-ec2-metadata-token: ${IMDS_TOKEN}" http://169.254.169.254/latest/meta-data/local-ipv4)
sed -i "s|\${PRIVATE_IP}|${PRIVATE_IP}|g" /home/ubuntu/infra-extra.yml
chown ubuntu:ubuntu /home/ubuntu/infra-extra.yml

# ── Start shared infra ────────────────────────────────────────────────────────
cd "$REPO_DIR"
sudo -u ubuntu docker compose -f infra/docker-compose.yml up -d --remove-orphans
sudo -u ubuntu docker compose -f /home/ubuntu/infra-extra.yml up -d --remove-orphans

echo "=== Waiting for Postgres ==="
for i in $(seq 1 30); do
  docker exec infra-postgres-1 pg_isready -U admin -q 2>/dev/null && echo "=== Postgres ready ===" && break
  sleep 2
done

echo "=== infra cloud-init complete at $(date) ==="
echo "Private IP: ${PRIVATE_IP} — runners will connect to Postgres/Redis/Kafka here"

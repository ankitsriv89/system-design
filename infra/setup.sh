#!/usr/bin/env bash
# =============================================================================
# setup.sh — One-time bootstrap for the Oracle Cloud ARM VM
#
# Run this once after first SSH into a fresh Ubuntu 22.04 / 24.04 instance:
#   bash setup.sh
#
# What it does:
#   1. System updates + essential packages
#   2. Docker + Docker Compose plugin (official Docker repo, ARM64)
#   3. Caddy reverse proxy (official Caddy repo)
#   4. Firewall rules (iptables → ufw) to open ports 80 and 443
#   5. Clones (or pulls) the system-design repo
#   6. Starts the shared infra (postgres)
#   7. Installs the Caddy systemd service and config
#   8. Prints next steps
#
# Idempotent: safe to run multiple times.
# =============================================================================

set -euo pipefail

REPO_URL="https://github.com/ankitsriv89/system-design.git"
REPO_DIR="$HOME/system-design"
CADDY_CONFIG_DIR="/etc/caddy"

# ── Colour helpers ────────────────────────────────────────────────────────────
green()  { echo -e "\033[0;32m$*\033[0m"; }
yellow() { echo -e "\033[0;33m$*\033[0m"; }
red()    { echo -e "\033[0;31m$*\033[0m"; }
step()   { echo; green "▶ $*"; }

# ── 1. System packages ────────────────────────────────────────────────────────
step "Updating system packages"
sudo apt-get update -q
sudo apt-get install -y -q \
    curl wget git ca-certificates gnupg lsb-release \
    ufw htop jq unzip

# ── 2. Docker ────────────────────────────────────────────────────────────────
step "Installing Docker"
if ! command -v docker &>/dev/null; then
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
        | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
    echo \
        "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] \
        https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
        | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
    sudo apt-get update -q
    sudo apt-get install -y -q docker-ce docker-ce-cli containerd.io docker-compose-plugin
    sudo systemctl enable --now docker
    # Allow the current user to run docker without sudo
    sudo usermod -aG docker "$USER"
    green "Docker installed. NOTE: log out and back in for group membership to take effect."
else
    green "Docker already installed: $(docker --version)"
fi

# ── 3. Caddy ─────────────────────────────────────────────────────────────────
step "Installing Caddy"
if ! command -v caddy &>/dev/null; then
    curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/gpg.key \
        | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt \
        | sudo tee /etc/apt/sources.list.d/caddy-stable.list >/dev/null
    sudo apt-get update -q
    sudo apt-get install -y -q caddy
    green "Caddy installed: $(caddy version)"
else
    green "Caddy already installed: $(caddy version)"
fi

# ── 4. Firewall ───────────────────────────────────────────────────────────────
step "Configuring firewall (ufw)"
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp    # HTTP
sudo ufw allow 443/tcp   # HTTPS (ready for when you add a domain)
sudo ufw --force enable
green "Firewall rules applied"

# OCI also has a network-level security list — see DEPLOYMENT.md for the
# OCI Console steps needed to open ports 80 and 443 from the internet.

# ── 5. Clone / pull repo ──────────────────────────────────────────────────────
step "Cloning / updating system-design repo"
if [ -d "$REPO_DIR/.git" ]; then
    git -C "$REPO_DIR" pull
    green "Repo updated at $REPO_DIR"
else
    git clone "$REPO_URL" "$REPO_DIR"
    green "Repo cloned to $REPO_DIR"
fi

# ── 6. Shared infra (postgres) ────────────────────────────────────────────────
step "Starting shared infrastructure (postgres)"
cd "$REPO_DIR"

# newgrp is not available in non-interactive scripts; use sg to run docker as the docker group.
# If this fails, log out/in and re-run: cd ~/system-design/infra && docker compose up -d
if groups "$USER" | grep -q docker; then
    docker compose -f infra/docker-compose.yml up -d
    green "Shared postgres running"
else
    yellow "Not yet in docker group — run this manually after re-login:"
    yellow "  cd $REPO_DIR && docker compose -f infra/docker-compose.yml up -d"
fi

# ── 7. Caddy config ───────────────────────────────────────────────────────────
step "Installing Caddy config"
sudo mkdir -p "$CADDY_CONFIG_DIR"
sudo cp "$REPO_DIR/infra/caddy/Caddyfile" "$CADDY_CONFIG_DIR/Caddyfile"
sudo systemctl enable caddy
sudo systemctl restart caddy
green "Caddy configured and (re)started"

# ── 8. Done ───────────────────────────────────────────────────────────────────
echo
green "═══════════════════════════════════════════════════════"
green " Bootstrap complete!"
green "═══════════════════════════════════════════════════════"
echo
yellow "Next steps:"
echo "  1. If you just installed Docker, log out and back in for group membership."
echo "  2. Deploy a project:"
echo "       cd $REPO_DIR && ./infra/deploy.sh 02-url-shortener"
echo "  3. To add a domain later, edit /etc/caddy/Caddyfile — see DEPLOYMENT.md."
echo
yellow "OCI firewall reminder:"
echo "  In the OCI Console → Networking → VCN → Security List,"
echo "  add Ingress rules for TCP port 80 and 443 (source 0.0.0.0/0)."
echo "  See DEPLOYMENT.md § OCI Security List for exact steps."

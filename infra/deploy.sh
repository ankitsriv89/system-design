#!/usr/bin/env bash
# =============================================================================
# deploy.sh — Deploy or update one (or all) projects on the OCI VM
#
# Usage:
#   ./infra/deploy.sh <project-slug>       # deploy a single project
#   ./infra/deploy.sh all                  # deploy every project with a docker-compose.yml
#   ./infra/deploy.sh infra                # restart shared infra only (postgres)
#
# Examples:
#   ./infra/deploy.sh 02-url-shortener
#   ./infra/deploy.sh all
#
# What it does:
#   1. git pull (latest code)
#   2. docker compose build --pull (rebuild image if Dockerfile changed)
#   3. docker compose up -d (start / recreate changed containers only)
#   4. Prints container status
#
# Idempotent: running twice is safe — unchanged containers are not restarted.
# =============================================================================

set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PROJECTS_DIR="$REPO_DIR/projects"

green()  { echo -e "\033[0;32m$*\033[0m"; }
yellow() { echo -e "\033[0;33m$*\033[0m"; }
red()    { echo -e "\033[0;31m$*\033[0m"; }
step()   { echo; green "▶ $*"; }

# ── Validate docker is available ─────────────────────────────────────────────
if ! command -v docker &>/dev/null; then
    red "docker not found. Run infra/setup.sh first."
    exit 1
fi

# ── Pull latest code ──────────────────────────────────────────────────────────
step "Pulling latest code from GitHub"
git -C "$REPO_DIR" pull
green "Code up to date"

# ── Deploy a single named project ─────────────────────────────────────────────
deploy_project() {
    local slug="$1"
    local dir="$PROJECTS_DIR/$slug"

    if [ ! -d "$dir" ]; then
        red "Project directory not found: $dir"
        return 1
    fi

    if [ ! -f "$dir/docker-compose.yml" ]; then
        yellow "No docker-compose.yml in $slug — skipping"
        return 0
    fi

    step "Deploying $slug"

    # Build image (uses cache; --pull refreshes base images)
    docker compose -f "$dir/docker-compose.yml" build --pull

    # Start / recreate only changed services
    docker compose -f "$dir/docker-compose.yml" up -d --remove-orphans

    # Show running containers for this project
    echo
    docker compose -f "$dir/docker-compose.yml" ps
    green "$slug deployed"
}

# ── Deploy shared infra ────────────────────────────────────────────────────────
deploy_infra() {
    step "Starting shared infrastructure"
    docker compose -f "$REPO_DIR/infra/docker-compose.yml" up -d --remove-orphans
    docker compose -f "$REPO_DIR/infra/docker-compose.yml" ps
    green "Shared infra running"
}

# ── Main ──────────────────────────────────────────────────────────────────────
TARGET="${1:-}"

if [ -z "$TARGET" ]; then
    red "Usage: $0 <project-slug|all|infra>"
    echo "  Examples:"
    echo "    $0 02-url-shortener"
    echo "    $0 all"
    echo "    $0 infra"
    exit 1
fi

case "$TARGET" in
    infra)
        deploy_infra
        ;;
    all)
        # Always (re)start infra first so postgres is available before project containers start
        deploy_infra
        for project_dir in "$PROJECTS_DIR"/*/; do
            slug="$(basename "$project_dir")"
            deploy_project "$slug" || true  # continue on individual project failure
        done
        echo
        green "All projects processed."
        ;;
    *)
        # Ensure infra (postgres) is running before deploying a project that needs it
        deploy_infra
        deploy_project "$TARGET"
        ;;
esac

echo
green "Done. Use 'docker ps' to see all running containers."

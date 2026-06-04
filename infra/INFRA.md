# Infrastructure Strategy

This document is the single reference for how the 50 system-design projects are deployed, networked, and operated. Read this instead of the Terraform code.

---

## Topology overview

```
                          ┌─────────────────────────────────────┐
                          │         AWS ap-south-1 VPC           │
                          │           10.0.0.0/16                │
                          │                                      │
  ┌──────────────┐        │  ┌────────────┐  ┌────────────────┐ │
  │   Browser /  │──HTTP/S──▶│  go-runner  │  │  java-runner   │ │
  │   curl       │        │  │ t4g.medium  │  │  t4g.large     │ │
  └──────────────┘        │  │ projects    │  │  projects      │ │
                          │  │ 01-13, 25,  │  │  10, 14-23,    │ │
                          │  │ 32, 40, 45  │  │  26-30, 37,    │ │
                          │  └─────┬───────┘  │  41-43         │ │
                          │        │          └──────┬─────────┘ │
                          │        │                 │           │
                          │  ┌─────▼─────────────────▼────────┐  │
                          │  │        other-runner             │  │
                          │  │        t4g.medium               │  │
                          │  │  Python, Node.js, Rust projects │  │
                          │  │  24, 36, 38-39, 46, 49          │  │
                          │  └──────────────┬──────────────────┘  │
                          │                 │                     │
                          │  ┌──────────────▼──────────────────┐  │
                          │  │         infra (shared)          │  │
                          │  │         t4g.medium              │  │
                          │  │  Postgres · Redis · Kafka       │  │
                          │  │  Prometheus · Grafana           │  │
                          │  └─────────────────────────────────┘  │
                          └─────────────────────────────────────┘
```

All four instances share one VPC subnet (`10.0.1.0/24`) and one security group. Runners connect to shared infra services over private IPs — those ports are never exposed to the internet.

---

## Instances

| Name | Type | vCPU / RAM | Role | Spot price ceiling |
|---|---|---|---|---|
| `infra` | t4g.medium | 2 / 4 GB | Postgres, Redis, Kafka, Prometheus, Grafana | $0.0224/hr |
| `go-runner` | t4g.medium | 2 / 4 GB | Go projects | $0.0224/hr |
| `java-runner` | t4g.large | 2 / 8 GB | Java/Spring Boot projects (JVM needs headroom) | $0.0448/hr |
| `other-runner` | t4g.medium | 2 / 4 GB | Python, Node.js, Rust projects | $0.0224/hr |
| **Total** | | | | **~$0.039/hr · ~$0.94/day** |

All instances: Ubuntu 24.04 ARM64, `gp3` EBS, `persistent` spot with `stop` interruption behavior, IMDSv2 enforced.

EIPs are attached while running — free. The cost is entirely the four Spot instances.

**Cost control**: run `terraform destroy` when not actively testing. The repo and all state live in git, so nothing is lost.

---

## Project-to-runner assignment

Determined by the `Recommended stack:` line in each project's `plan.md`. Codified in `terraform/aws-multi/cloud-init-runner.sh`.

| Runner | Projects | Stacks |
|---|---|---|
| `go-runner` | 01-09, 11-13, 25, 32, 40, 45 | Go |
| `java-runner` | 10, 14-23, 26-30, 37, 41-43 | Java/Spring Boot |
| `other-runner` | 24, 36, 38-39, 46, 49 | Python/FastAPI, Rust |
| `other-runner` | 31 | TypeScript/Node.js |
| `other-runner` | 34-35, 44, 47-48, 50 | Go/Java mixed (decided at build time) |

When a new project is built, add its slug to the correct `case` block in `cloud-init-runner.sh` before committing. The `post-build` command handles this automatically.

---

## Shared infra services

All runners connect to these via the private IP written to `/etc/system-design-env` at boot.

| Service | Port | Used by |
|---|---|---|
| PostgreSQL 16 | 5432 | All projects (separate DB + user per project) |
| Redis 7 | 6379 | Projects needing cache / pubsub / sessions |
| Kafka 3.9 (KRaft) | 9092 | Java/Python projects with async messaging |
| Prometheus | 9090 | All — each project's metrics scraped here |
| Grafana | 3000 | Dashboards; one dashboard JSON per project in `infra/grafana/dashboards/` |
| node-exporter | 9100 | Host metrics on all four instances |

### Why one shared infra instance, not per-language DBs

- All 50 projects use Postgres as their durable store. Splitting by language would mean 3-4 idle Postgres instances.
- Projects are developed and tested one at a time — DB contention is never a concern.
- Logical isolation is already provided: each project gets its own Postgres database and user via `infra/initdb/<NN>_<project>.sql`.
- Cross-machine DB would add unnecessary networking complexity with no benefit.

### Environment injection

Each runner's `/etc/system-design-env` is written at boot by `cloud-init-runner.sh`:

```bash
POSTGRES_HOST=<infra-private-ip>
POSTGRES_PORT=5432
REDIS_HOST=<infra-private-ip>
REDIS_PORT=6379
KAFKA_BOOTSTRAP=<infra-private-ip>:9092
INFRA_HOST=<infra-private-ip>
```

Project `docker-compose.yml` files must use `${POSTGRES_HOST:-postgres}` style defaults so they work in both single-machine mode (local dev) and multi-instance mode (AWS). Never hardcode IPs.

---

## Database provisioning

Each project's Postgres database and user are created by an init SQL file that runs when Postgres first starts:

```
infra/initdb/
├── 00_ratelimiter.sql
├── 01_urlshortener.sql
├── 02_pastebin.sql
...
```

Naming convention: `<NN>_<projectname>.sql` (no hyphens). Schema migrations are applied separately via `docker exec` in `cloud-init-infra.sh` or the project's `scripts/migrate.sql`.

When adding a new project: create the `infra/initdb/<NN>_<project>.sql` file before `terraform apply`.

---

## Networking and ports

- One public subnet (`10.0.1.0/24`), one security group for all instances.
- App ports `8081–8131` are open to `0.0.0.0/0` — each project occupies one port.
- Infra ports (5432, 6379, 9092) are blocked from the internet by the SG; reachable only within `10.0.0.0/16`.
- Observability ports (9090 Prometheus, 3000 Grafana) are restricted to `var.observability_allowed_cidr` (default your IP).

**Port registry**: `infra/ports.md` is the authoritative source. Check it before assigning any port. Ports are sequential starting at 8081.

---

## Caddy reverse proxy

Caddy runs on each runner instance and handles:
- TLS termination (HTTPS)
- Path-based routing: `/pNN/*` → `localhost:<port>`
- WebSocket upgrade (automatic — no extra config needed)

Config lives in `infra/caddy/Caddyfile`. When a new project is built, add its `handle_path` block in project-number order.

---

## Terraform layout

```
terraform/
├── aws-multi/          ← active multi-instance setup (use this)
│   ├── main.tf         — four Spot instances + VPC + SG + EIPs
│   ├── variables.tf    — region, AZ, SSH key, CIDRs, repo URL
│   ├── outputs.tf      — SSH commands, service URLs, live cost estimate
│   ├── cloud-init-infra.sh   — bootstraps shared-infra instance
│   └── cloud-init-runner.sh  — bootstraps go/java/other runner instances
├── aws/                ← single-instance setup (used early, superseded)
└── oci/                ← Oracle Cloud (original host, currently unavailable)
```

### Common operations

```bash
# Bring up all four instances
cd terraform/aws-multi
terraform init
terraform apply

# SSH to a runner
ssh -i ~/.ssh/id_ed25519 ubuntu@<public-ip-from-terraform-output>

# Check what's running on a runner
docker ps

# Tear everything down (all data lost — DB state is ephemeral on Spot)
terraform destroy

# Check live spot prices and SSH URLs
terraform output
```

### Spot interruption behavior

All instances use `instance_interruption_behavior = "stop"` (not terminate). If AWS reclaims the instance, it stops and automatically restarts when capacity returns. Docker Compose services have `restart: unless-stopped` so they come back automatically.

If the infra instance is interrupted, runners will lose DB/Redis/Kafka connectivity until it restarts. This is acceptable for a dev/demo environment.

---

## Adding a new project — checklist

1. Check `infra/ports.md` for the next free port.
2. Create `infra/initdb/<NN>_<project>.sql` with the project's Postgres database and user.
3. Build the project (`/project:post-build <slug>` handles all remaining steps).
4. `post-build` will:
   - Mark port as `✅ built` in `infra/ports.md`
   - Add `handle_path` block to `infra/caddy/Caddyfile`
   - Add project slug to the correct runner's `PROJECTS` array in `cloud-init-runner.sh`
   - Commit and push everything

No Terraform re-apply is needed for new projects — `cloud-init-runner.sh` changes only take effect on the next instance launch (i.e. after `terraform destroy && terraform apply`). For immediate deployment, SSH to the runner and run `docker compose up -d` manually.

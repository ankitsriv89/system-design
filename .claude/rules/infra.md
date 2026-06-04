---
description: Infrastructure conventions: ports, shared infra, DB provisioning, IMDSv2, Docker networking
paths: ["projects/**/docker-compose.yml", "infra/**", "projects/**/Dockerfile"]
---

# Infrastructure conventions

## Ports (do not reuse)
The authoritative port registry is `infra/ports.md`. Always check it before assigning a port.

| Project | Port(s) | Caddy path |
|---|---|---|
| 01-rate-limiter | 8081 | /p01/ |
| 02-url-shortener | 8085 | /p02/ |
| 03-pastebin | 8082 | /p03/ |
| 04-unique-id-generator | 8083 | /p04/ |
| 05-consistent-hashing | 8084 | /p05/ |
| 06-load-balancer | 8086 (proxy), 8087 (admin) | /p06/ |
| 07-api-gateway | 8088 | /p07/ |
| 08-basic-key-value-store | 8089 | /p08/ |
| 09 onwards | next free port above 8089 — check infra/ports.md | /p09/ … |

**Port conflict rule**: Before assigning a port in a new project's `docker-compose.yml`, look up the next free port in `infra/ports.md`. Never reuse a port already taken by another project. The `LISTEN_ADDR` env var default in `main.go` must match the `docker-compose.yml` port.

## Naming
- Module path: `github.com/ankitsriv89/<project-slug>`
- Docker image tag: `<project-slug>:latest`
- Prometheus metric prefix: `<project_slug_underscored>_`

## Shared infra
- PostgreSQL, Redis, Prometheus, Grafana, MinIO live in `infra/docker-compose.yml`.
- Every project connects via the external Docker network named `infra`.
- Never bundle a redundant Postgres or Redis in a project's own `docker-compose.yml` unless the project specifically requires isolation.
- Each project's Postgres user/database is provisioned via `infra/initdb/<NN>_<project>.sql` (runs on first PG start). Schema migrations are applied explicitly in `cloud-init.sh` via `docker exec -i infra-postgres-1 psql`.
- Projects that need a dynamic public URL (e.g. BASE_URL) must use `${VAR:-default}` in docker-compose and have `cloud-init.sh` write a `.env` file using IMDSv2 before `docker compose up`. Never hardcode IPs.
- IMDSv2 is enforced (`http_tokens = required`). All IMDS calls need the two-step token fetch (`PUT /latest/api/token` then use header `X-aws-ec2-metadata-token`).

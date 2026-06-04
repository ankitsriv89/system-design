# System Design Projects — Claude Instructions

This file is read by Claude Code at the start of every session. All instructions here are mandatory for every project under `projects/`.

## Infra reference

Full deployment strategy, instance topology, port rules, and operational runbook: [infra/INFRA.md](infra/INFRA.md)

## Portfolio

**After every project build**, update `portfolio/index.html`: flip the project's `live` flag to `true`, recount the live/building stats, and update the progress bar width. The `/project:post-build` command handles this automatically.

## Rules (loaded automatically by file pattern)

Detailed rules live in `.claude/rules/` and are injected when relevant files are open:

| Rule file | Applies to |
|---|---|
| [performance.md](.claude/rules/performance.md) | All `*.go` files — allocations, goroutines, pools, struct layout |
| [go-conventions.md](.claude/rules/go-conventions.md) | All `*.go` files — errors, logging, interfaces, package layout, testing |
| [frontend.md](.claude/rules/frontend.md) | `web/**` — UI stack, XSS rules, tutorial UI requirements |
| [infra.md](.claude/rules/infra.md) | `docker-compose.yml`, `Dockerfile`, `infra/**` — ports, networking, DB provisioning |
| [docs-workflow.md](.claude/rules/docs-workflow.md) | `docs/**` — post-build artefacts and commit workflow |

## Slash commands

| Command | What it does |
|---|---|
| `/project:post-build <slug>` | Runs build+test, writes all docs/, updates infra files, commits and pushes |
| `/project:new-project <slug>` | Scaffolds a new project with correct ports, Docker, Caddy, and stub UI |

## Project quick-reference

### Port registry
Authoritative source: `infra/ports.md`. Never assign a port without checking it first.

### Package layout
```
<project-slug>/
├── main.go           — wiring only
├── <domain>/         — core logic, no HTTP/DB imports
├── api/              — HTTP handlers
├── store/            — storage adapters
├── metrics/          — Prometheus registrations
├── web/              — frontend (index.html, styles.css, app.js)
├── scripts/          — migrate.sql, integration_test.sh, load_test.sh
├── docs/             — architecture.md, code-flow.md, build-log.md, changelog.md, api.md
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

### Naming
- Module path: `github.com/ankitsriv89/<project-slug>`
- Docker image tag: `<project-slug>:latest`
- Prometheus metric prefix: `<project_slug_underscored>_`

### Shared infra
- PostgreSQL, Redis, Prometheus, Grafana, MinIO live in `infra/docker-compose.yml`.
- Every project connects via the external Docker network named `infra`.
- Never bundle a redundant Postgres or Redis unless the project specifically requires isolation.
- Each project's Postgres database is provisioned via `infra/initdb/<NN>_<project>.sql`.

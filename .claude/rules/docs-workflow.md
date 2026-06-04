---
description: Post-build documentation and infra update workflow — required before every commit
paths: ["projects/**/docs/**", "projects/**/docs/*.md"]
---

# Workflow: after every successful build + test

After the project's build and test commands pass (see language-specific conventions below), produce the following artefacts **before** committing:

**Build commands by stack:**
- Go: `go build ./...`, `go vet ./...`, `go test -race ./...`
- Java/Spring Boot: `./mvnw verify` or `./gradlew test`
- Python/FastAPI: `pytest` (with type checks via `mypy` if configured)
- TypeScript/Node: `npm run build && npm test`
- Rust: `cargo build && cargo test`

## 1. `docs/architecture.md`
- Mermaid `graph TD` diagram of the full system (client → API → domain → storage → async workers → observability).
- Mermaid `sequenceDiagram` for the primary happy-path request flow.
- Written prose: components, responsibilities, data flows, external dependencies.
- Capacity table: throughput, storage growth, key limits.

## 2. `docs/code-flow.md`
- Mermaid `flowchart TD` tracing every significant function call from `main()` through to the storage layer and back.
- One section per major operation (e.g. "Generate ID", "Acquire Lease", "Renew Lease").
- Explain *why* each call is made, not just *what* it does.
- Include a "Call graph summary" section as a `graph LR`.

## 3. `docs/build-log.md`
- Go version, module path, all direct dependencies pinned with versions and roles.
- Actual output of `go build ./...`, `go vet ./...`, `go test -race ./... -v`.
- Any build decisions, workarounds, or notable compiler flags explained.

## 4. `docs/changelog.md`
- Follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) format.
- Version starts at `0.1.0` for a new project.
- Sections: Added, Changed, Fixed, Performance.

## 5. `docs/api.md`
- Full `curl` examples for every endpoint.
- Request and response field descriptions.
- All error responses with HTTP status codes and bodies.

## 6. Update deployment infrastructure (mandatory for every new project)

**`infra/ports.md`**
- Mark the project's row as `✅ built` (status column).
- If you introduced a new port not already in the table, add a row for it.
- Resolve any port conflicts before committing.

**`infra/caddy/Caddyfile`**
- Add a `handle_path /pNN/*` block pointing at the project's actual port (from its `docker-compose.yml`).
- Update the default index page HTML to include the new project link with its tech-stack tag line.
- Keep routes in project-number order.

## Commit and push
1. `git add` only project files plus `infra/ports.md` and `infra/caddy/Caddyfile` — never `.env`, `*.secret`, or binaries.
2. Commit message: `feat(<project-slug>): initial implementation + docs`
   - For changes spanning multiple projects: `feat(03-pastebin, 04-unique-id-generator): <short description>`
3. `git push origin main`

Commit message prefixes:
- New project: `feat(<project-slug>): initial implementation + docs`
- Performance only: `perf(<project-slug>): <short description>`
- Bug fix: `fix(<project-slug>): <short description>`

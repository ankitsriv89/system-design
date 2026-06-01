# System Design Projects — Claude Instructions

## Workflow: after every successful build + test

After a project compiles and all tests pass, produce the following artefacts **before** committing:

### 1. `docs/architecture.md`
- Mermaid `graph TD` diagram of the full system (client → API → storage → async workers → observability).
- Mermaid `sequenceDiagram` for the primary happy-path request flow.
- Written prose: components, responsibilities, data flows, external dependencies.

### 2. `docs/code-flow.md`
- Mermaid `flowchart TD` tracing every significant function call from `main()` through to the storage layer and back.
- One section per major operation (e.g. "Generate ID", "Acquire Lease", "Renew Lease").
- Explain *why* each call is made, not just *what* it does.

### 3. `docs/build-log.md`
- Go version, module path, dependencies pinned.
- Output of `go build`, `go vet`, and `go test` (copy actual output).
- Any build decisions or workarounds noted.

### 4. `docs/changelog.md`
- Follows Keep a Changelog format (`## [version] — date`).
- Version starts at `0.1.0` for a new project.
- List: Added, Changed, Fixed sections.

### 5. `docs/api.md` (if not already in README)
- Full request/response examples with `curl`.
- Error responses with HTTP status codes.
- Field-level descriptions.

### Commit and push
After all docs are written:
1. `git add` only project files (never `.env` or secrets).
2. Commit with message: `feat(<project-slug>): initial implementation + docs`.
3. `git push origin main`.

## Project conventions
- Language: Go
- Port numbering: projects use ports 8081, 8082, 8083… in order.
- Shared infra (Postgres, Redis, Prometheus, Grafana, MinIO) lives in `infra/`.
- Each project connects to shared infra via the `infra` Docker network.
- Module path pattern: `github.com/ankitsriv89/<project-slug>`.
- Every source file must have a package-level comment.
- No comments explaining *what* code does — only *why* when non-obvious.

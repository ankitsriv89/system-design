Run the complete post-build workflow for the current project.

Steps (execute in order, stop and report on any failure):

1. **Verify build passes**
   Read the project's `plan.md` to determine the stack, then run the appropriate commands:
   - Go: `go build ./... && go vet ./... && go test -race ./...`
   - Java/Spring Boot: `./mvnw verify` or `./gradlew test`
   - Python/FastAPI: `pytest` (and `mypy` if configured)
   - TypeScript/Node: `npm run build && npm test`
   - Rust: `cargo build && cargo test`

   If any step fails, report the error and stop.

2. **Write `docs/architecture.md`**
   - Mermaid `graph TD` of the full system (client → API → domain → storage → async workers → observability).
   - Mermaid `sequenceDiagram` for the primary happy-path request flow.
   - Prose: components, responsibilities, data flows, external dependencies.
   - Capacity table: throughput, storage growth, key limits.

3. **Write `docs/code-flow.md`**
   - Mermaid `flowchart TD` tracing every significant function call from `main()` to the storage layer and back.
   - One section per major operation.
   - Explain *why* each call is made.
   - "Call graph summary" as `graph LR`.

4. **Write `docs/build-log.md`**
   - Go version, module path, all direct deps with versions and roles.
   - Actual captured output of `go build`, `go vet`, `go test -race -v`.

5. **Write `docs/changelog.md`**
   - Keep a Changelog format, version `0.1.0`.
   - Sections: Added, Changed, Fixed, Performance.

6. **Write `docs/api.md`**
   - Full `curl` examples for every endpoint.
   - Request/response field descriptions.
   - All error responses with HTTP status codes and bodies.

7. **Update `infra/ports.md`**
   - Mark the project row `✅ built`.
   - Add any new ports not already listed.
   - Confirm no port conflicts.

8. **Update `infra/caddy/Caddyfile`**
   - Add `handle_path /pNN/*` block for the project's port.
   - Add project link to the index page HTML.
   - Keep routes in project-number order.

9. **Update `portfolio/index.html`**
   In the `PROJECTS` array, find the entry for this project (by `id`) and set `live: true`.
   Also update the header counters:
   - `id="live-count"` inner text: count of all entries with `live: true`
   - "Building" stat value: `50 - live count`
   - `.progress-bar-fill` CSS `width`: `Math.round(liveCount / 50 * 100) + '%'`
   - Comment next to width: `/* <liveCount>/50 */`

10. **Update `terraform/aws-multi/cloud-init-runner.sh`**
   Read the project's `plan.md` to determine its stack, then add the project slug to the
   correct `case` block in `cloud-init-runner.sh`:
   - Go stack → `go)` block
   - Java/Spring Boot → `java)` block
   - Python/FastAPI, Node.js, Rust, or mixed → `other)` block

   Example: for a Go project `13-message-queue`, add `"13-message-queue"` to the `go)` PROJECTS array.

11. **Commit and push**
    ```
    git add projects/$ARGUMENTS infra/ports.md infra/caddy/Caddyfile terraform/aws-multi/cloud-init-runner.sh portfolio/index.html
    git commit -m "feat($ARGUMENTS): initial implementation + docs"
    git push origin main
    ```

Usage: `/project:post-build <project-slug>` (e.g. `/project:post-build 13-message-queue`)

Scaffold a new system design project with the correct structure and port assignment.

Steps (execute in order):

1. **Determine the next free port**
   - Read `infra/ports.md` and find the highest allocated port.
   - Assign the next available port (no gaps, no reuse).
   - Confirm the port is not used in any existing `docker-compose.yml`.

2. **Create directory structure**
   ```
   projects/$ARGUMENTS/
   ├── main.go
   ├── <domain>/
   │   ├── <domain>.go
   │   └── <domain>_test.go
   ├── api/
   │   └── handler.go
   ├── store/
   ├── metrics/
   ├── web/
   │   ├── index.html
   │   ├── styles.css
   │   └── app.js
   ├── scripts/
   │   ├── migrate.sql
   │   └── integration_test.sh
   ├── docs/
   ├── Dockerfile
   ├── docker-compose.yml
   ├── go.mod
   └── go.sum
   ```

3. **go.mod**: module path `github.com/ankitsriv89/$ARGUMENTS`

4. **docker-compose.yml**: use the assigned port, connect to `infra` external network, reference shared Postgres/Redis from infra stack.

5. **Dockerfile**: multi-stage build — `golang:1.23-alpine` builder → `alpine` final; `COPY web/ web/`.

6. **Reserve the port in `infra/ports.md`**: add a row with status `🔧 in progress`.

7. **Add Caddy placeholder** in `infra/caddy/Caddyfile`: add `handle_path /pNN/*` block (can point at port even before service is up).

8. **Stub `web/index.html`**: three-panel layout with concept explanation, visualization placeholder, and API log panel. Link `styles.css` and `app.js`.

Report the assigned port and the full path when done.

Usage: `/project:new-project <project-slug>` (e.g. `/project:new-project 14-search-engine`)

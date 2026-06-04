Scaffold a new system design project with the correct structure and port assignment.

Steps (execute in order):

1. **Determine the next free port**
   - Read `infra/ports.md` and find the highest allocated port.
   - Assign the next available port (no gaps, no reuse).
   - Confirm the port is not used in any existing `docker-compose.yml`.

2. **Determine the tech stack**
   - Read `README.md` and find the row for this project number.
   - The "Stack" column lists the recommended technologies (e.g. "Java/Spring Boot, WebSockets, Kafka, PostgreSQL, Redis").
   - If a `plan.md` already exists for the project, read it too — the "Recommended stack" line is authoritative.
   - **Projects 01–13**: Go (default)
   - **Projects 14+**: Java 21 / Spring Boot 3 / Gradle unless plan.md says otherwise
   - Never default to Go for projects 14+.

3. **Create directory structure** based on stack:

   **Go projects (01–13):**
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

   **Java/Spring Boot projects (14+):**
   ```
   projects/$ARGUMENTS/
   ├── src/
   │   ├── main/
   │   │   ├── java/com/ankitsriv89/<slug>/
   │   │   │   ├── <Slug>Application.java
   │   │   │   ├── config/
   │   │   │   ├── domain/
   │   │   │   ├── api/
   │   │   │   ├── service/
   │   │   │   ├── repository/
   │   │   │   └── store/
   │   │   └── resources/
   │   │       ├── application.yml
   │   │       └── db/migration/V1__init.sql
   │   └── test/java/com/ankitsriv89/<slug>/
   ├── web/
   │   ├── index.html
   │   ├── styles.css
   │   └── app.js
   ├── scripts/
   │   └── integration_test.sh
   ├── docs/
   ├── Dockerfile
   ├── docker-compose.yml
   ├── build.gradle
   └── settings.gradle
   ```

4. **Build file**:
   - Go: `go.mod` with module path `github.com/ankitsriv89/$ARGUMENTS`
   - Java: `build.gradle` with Spring Boot 3, Java 21, dependencies matching the stack (spring-boot-starter-web, spring-boot-starter-websocket, spring-kafka, spring-boot-starter-data-jpa, flyway, micrometer-prometheus, etc.)

5. **docker-compose.yml**: use the assigned port, connect to `infra` external network, reference shared Postgres/Redis from infra stack.

6. **Dockerfile**:
   - Go: multi-stage `golang:1.23-alpine` builder → `alpine` final; `COPY web/ web/`
   - Java: multi-stage `gradle:8-jdk21-alpine` builder → `eclipse-temurin:21-jre-alpine` final; `COPY web/ web/`; expose correct port

7. **Reserve the port in `infra/ports.md`**: update the row status from `🔲 planned` to `🔧 in progress`.

8. **Add Caddy placeholder** in `infra/caddy/Caddyfile`: add `handle_path /pNN/*` block pointing at the assigned port.

9. **Stub `web/index.html`**: three-panel layout with concept explanation, visualization placeholder, and API log panel. Link `styles.css` and `app.js`.

Report the assigned port, the stack chosen, and the full path when done.

Usage: `/project:new-project <project-slug>` (e.g. `/project:new-project 15-group-chat-system`)

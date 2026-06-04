---
description: Frontend rules for web/ directory: UI quality, XSS prevention, tutorial UI requirements
paths: ["projects/**/web/**"]
---

# Frontend (web/)

Every project must have an interactive web UI served at `GET /`.

## File structure and serving
- `web/index.html` — the page, loaded by `http.ServeFile(w, r, "web/index.html")` at `GET /`.
- Static assets served via `GET /static/*` using `http.FileServer(http.Dir("web"))`.
- The Dockerfile final stage must `COPY web/ web/` so the assets are present in the container at `/app/web/`.
- Static web assets committed under `web/data/` are excluded by `.gitignore`'s `**/data/` rule — add an `!**/web/data/` exception if needed.

## Technology
- Use whatever frontend stack produces the best result: React, Vue, Svelte, or vanilla JS are all fine.
- CSS frameworks (Tailwind, etc.) and animation libraries (GSAP, Framer Motion, D3, Three.js) are encouraged when they improve the visual quality of the tutorial UI.
- No build tools that require a separate compile step at runtime — pre-build or use CDN-delivered ESM where needed.
- Prefer a CDN-delivered bundle (e.g. via esm.sh or unpkg) over introducing a Node build pipeline, unless the project already has one.

## XSS rules (always apply, regardless of framework)
- Never set `innerHTML` with API-returned data — use `textContent`, framework templates, or DOM methods.
- Sanitize any user-controlled values before inserting into the DOM.

## UI quality bar
- The UI must exercise every meaningful API endpoint with real fetch calls — not just display documentation.
- Prioritise clarity and interactivity: users should be able to explore every feature from the browser without reading the docs.
- Reference design: project 02 (`url-shortener/web/`) for layout structure, but exceed it in visual polish.

## Tutorial UI — mandatory for every project
Every project UI must double as a visual tutorial that teaches the system design concept it implements.

- **Concept explanation panel**: a dedicated section (collapsible or always-visible) that explains the core concept in plain language — what problem it solves, why it matters, and how the implementation works. Written for a technical reader who hasn't seen the design before.
- **Animated / live visualizations**: show the system operating in real time using CSS animations, Canvas, SVG, D3, or any suitable library. Examples: animated request flow arrows between client and backends, ring diagrams for consistent hashing, token-bucket fill/drain animations for rate limiting, node status transitions for health checks. Animations must reflect actual live API state — not canned demos.
- **Algorithm walk-through**: for each major algorithm or data structure used, show a step-by-step visual of one operation updating alongside a live request.
- **Failure / edge-case demos**: interactive controls to trigger failure modes (kill a backend, exhaust tokens, create a hot key) so the viewer can watch the system respond visually.
- **No static screenshots or placeholder text**: every diagram and animation must be driven by real fetch calls to the running service.
- **Layout**: three-panel preferred — left: controls + concept text; center: live animated visualization; right: structured API output log. Collapse gracefully to single-column on narrow viewports.

# Cube Drift — v3 Design Plan

A 3D Snakes-and-Ladders space board game built as a standalone demo on top of the
project-01 rate limiter. All files in this directory document the plan. Nothing is
built yet — these are the design specifications for implementation.

## Directory index

| File | Contents |
|---|---|
| `00-overview.md` | This file — scope, goals, non-goals |
| `01-stack-and-layout.md` | Tech stack decision, folder/file layout, Go module reuse |
| `02-game-model.md` | 3D state model, coordinates, scoring |
| `03-movement-algorithm.md` | Quantum dial: goal-biased roll, clamp, win condition |
| `04-board-generation.md` | Warp (ladder) / debris (snake) randomized placement |
| `05-object-catalog.md` | ~70 curated space objects, zone mapping, cell→question resolution |
| `06-database.md` | New tables (never touching v1 tables) |
| `07-api.md` | All /v3/cube/* HTTP endpoints — request/response contracts |
| `08-frontend.md` | Three.js scene plan, UI controllers, two meters |
| `09-wiring-and-deploy.md` | cmd/cube-server, docker-compose, seed script |
| `10-risks.md` | Known risks, unknowns, tuning knobs |

## Scope

**Build:** A new standalone Go binary (`cmd/cube-server`, port `:8081`) with a new Three.js
frontend (`web/cube*.html/css/js`) — a 3D space board game in a 10×10×10 cube. New endpoints
at `/v3/cube/*`. New Postgres tables prefixed `cube_`. Reuses the project-01 token-bucket rate
limiter, Redis, Postgres, session cookie, and Groq client **by import** (no copying).

**Do not touch:** anything under `cmd/server`, `internal/api/handler.go`, `internal/api/odyssey.go`,
`internal/odyssey/*`, `web/odyssey*`, `web/index.html`, `web/app.js` — v1 is untouched.

## Core design decisions

| Decision | Choice |
|---|---|
| Move mechanic | Correct answer → spin quantum dial (1–6) and move. Wrong → dial locked, no move. |
| Fuel | HOPS = rate-limit demo (per-IP token bucket). SCORE = independent game meter. |
| Wrong-answer rule | No roll, lose 1 hop, lose 10 score points. |
| Visuals | Full 3D in Three.js. Ship glides cell-to-cell. Reuse v1 warp/shake FX. |
| 3D topology | Free (x,y,z) coordinates. Dial picks distance + goal-biased direction. |
| Hazards | Warps + debris randomized per game from a balanced pool. |
| Objects | Curated ~70 objects. Questions: seed-bank first, Groq on demand, cached. |
| Versioning | Standalone. New page, new endpoints. v1 untouched. |

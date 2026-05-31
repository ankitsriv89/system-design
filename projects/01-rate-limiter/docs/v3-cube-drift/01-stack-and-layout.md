# Stack & File Layout

## Stack decision

**New standalone Go service in the SAME Go module, reusing internal packages by import.**

- Module: `github.com/ankitsriv89/rate-limiter` (Go 1.26.3, confirmed in `go.mod`).
- Go `internal/` rule: any package in the same module may import any `internal/` package.
  A new `cmd/cube-server` and new `internal/cube/*` can import `internal/store`,
  `internal/policy`, `internal/odyssey` — **zero copying**.
- Frontend: Three.js `0.165.0` already wired via importmap in v1 odyssey.html; same CDN
  url works for cube.html. `three/addons/` is already mapped → `OrbitControls` available.
- "Standalone" is delivered by: separate binary, separate port (`:8081`), separate routes
  (`/v3/cube/*`), separate DB tables. v1 binary (`cmd/server`) is never touched.

**Why not a pure-frontend toy?**
The token-bucket rate limiter is the entire educational point of project 01. A frontend-only
build would discard it. Hops must be server-authoritative (debited via Redis Lua script,
keyed by real IP, not forgeable by the client).

## Packages reused unchanged (by import, no copy)

| Package | Reused for |
|---|---|
| `internal/store` | `RedisLimiter` — `TokenBucketAllow`, `PeekTokenBucket` |
| `internal/policy` | `Policy`, `Store`, `Cache` — the `cube-hops` policy |
| `internal/odyssey` (session.go) | `IssueSession`, `ValidateSession`, `SetSessionKey` |
| `internal/odyssey` (journey.go) | `ClientIP`, `TrustedProxyCIDRs`, `ParseTrustedProxies` |
| `internal/odyssey` (groq.go) | `GroqClient`, `GenerateQuestion` (via shim) |
| `internal/metrics` | Prometheus counters/histograms (optional, same /metrics) |
| `internal/api` (handler.go) | `Handler.Routes` mounted on cube-server for policy CRUD |

## Files to create

```
projects/01-rate-limiter/
├── cmd/
│   └── cube-server/
│       └── main.go                   # new binary, listen :8081
├── internal/
│   └── cube/
│       ├── board.go                  # Coord, Warp, Debris, Board, GameState types
│       ├── movement.go               # quantum-dial roll: goal-biased direction + clamp + win
│       ├── placement.go              # randomized warp/debris placement algorithm
│       ├── catalog.go                # ~70 curated space objects, zone map, cell→object lookup
│       ├── store.go                  # DB migrations, game CRUD, question seed/lookup
│       └── groq_topic.go             # shim: SpaceObject → odyssey.Destination → Groq
├── internal/
│   └── api/
│       └── cube.go                   # CubeHandler + Routes() /v3/cube/*
├── web/
│   ├── cube.html                     # importmap + shell HTML
│   ├── cube.css                      # dark-space theme, two meters, dial animation
│   ├── cube-ui.js                    # API calls, state, dial overlay, overlays
│   └── cube-scene.js                 # Three.js: wireframe cube, zones, ship, markers
└── scripts/
    └── seed_cube.sh                  # seed cube-hops policy at :8081
```

Files NOT created / NOT modified:
- `docker-compose.yml` — add a `cube-server` service (see `09-wiring-and-deploy.md`)
- `Dockerfile` — add `cube-server` build target (multi-stage, one image)
- Nothing under `cmd/server/`, `internal/odyssey/`, `web/odyssey*`, `web/index.html`

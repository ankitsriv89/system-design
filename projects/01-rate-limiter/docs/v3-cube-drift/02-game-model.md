# Game State Model

## Coordinate system

- Cube edge: **N = 10**. Each axis x, y, z ∈ [0, 9].
- **Start** = `(0, 0, 0)` — Earth corner.
- **Goal** = `(9, 9, 9)` — Galactic Core / Sagittarius A*.
- **D (dist-to-goal)** = `(9−x) + (9−y) + (9−z)` — Manhattan distance to Goal.
  - D = 27 at Start, D = 0 at Goal.
  - D drives: zone/difficulty, movement bias, warp/debris placement rules, win detection.
- **D_from_start** = x + y + z — used for zone bucketing (0..27).

## Go types (`internal/cube/board.go`)

```go
const N = 10

type Coord struct{ X, Y, Z int }           // 0..9 each

func (c Coord) DistToGoal() int {
    return (9-c.X) + (9-c.Y) + (9-c.Z)    // 27 at start, 0 at goal
}

func (c Coord) DistFromStart() int { return c.X + c.Y + c.Z }

type Warp struct {
    From  Coord `json:"from"`
    To    Coord `json:"to"`    // D(To) < D(From): strictly closer to goal
    Bonus int   `json:"bonus"` // added to score on traversal
}

type Debris struct {
    From    Coord `json:"from"`
    To      Coord `json:"to"`    // D(To) > D(From): strictly farther from goal
    Penalty int   `json:"penalty"` // subtracted from score on traversal
}

type Board struct {
    Seed   int64    `json:"seed"`
    Warps  []Warp   `json:"warps"`
    Debris []Debris `json:"debris"`
}

type GameState struct {
    SessionID         string    // HMAC session cookie value (reuse odyssey.IssueSession)
    Pos               Coord
    Score             int
    Turn              int
    Won               bool
    Seed              int64     // per-game RNG seed (crypto/rand generated on new-game)
    PendingQuestionID *int64    // question the player must answer THIS turn (nil = no active Q)
    StartedAt         time.Time
}
```

## What lives where

| Data | Storage | Key | Why |
|---|---|---|---|
| HOPS remaining + refill timestamp | **Redis** | `rl:tb:cube-hops:ip:<ip>` | Network identity. The rate-limit demo. Never in Postgres. |
| Position, score, turn, won, seed, pending_question_id | **Postgres** `cube_games` | `session_id` (cookie) | Authoritative progress; survives restarts. |
| Board layout (warps, debris) | **Postgres** `cube_boards` JSONB | `session_id` | Fast reads; also regenerable from seed. |
| Zone, nearest object, D-to-goal, neighborhood specials | **Derived** per request | — | Never stored; cheap to recompute. |

**Session vs IP — same split as v1:**
- Progress keyed by **session cookie** (`odyssey_sid`, shared with v1 — same HMAC key, different tables).
- Rate limit keyed by **real client IP** via `odyssey.ClientIP(r)` (XFF trusted-proxy handling).
- Changing IP does not lose progress; rate limits are per-IP and cannot be spoofed.

## Scoring constants

```go
const (
    PointsCorrect = 20  // awarded on a correct answer
    PointsWrong   = 10  // deducted on a wrong answer (no movement)
    // Warp bonus: 10 + 5 * (D_from - D_to) — scales with jump distance
    // Debris penalty: 5 * (D_to - D_from)  — scales with knockback
)
```

Score is purely a **game meter** — it does not affect rate limiting or hop cost.
Score can go negative; floor is not enforced (players can see how badly a run went).

## Two-meter UI concept

```
┌──────────────────────────────────────┐
│ HOPS  ███████░░░  7 / 30  refuels in │  ← rate-limit fuel (Redis token bucket)
│ SCORE 340 pts                        │  ← game score (Postgres)
└──────────────────────────────────────┘
```

Hops = the rate-limiter demo. Score = how skillfully you played.
When hops reach 0 → rate-limit overlay + countdown (identical to v1 UX).

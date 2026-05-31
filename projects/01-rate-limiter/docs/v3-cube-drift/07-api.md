# HTTP API (`internal/api/cube.go`)

All routes prefixed `/v3/cube/`. Registered via `CubeHandler.Routes(r *mux.Router)`.

## CubeHandler type

```go
type CubeHandler struct {
    cubeStore *cube.CubeStore
    limiter   *store.RedisLimiter
    policies  *policy.Cache
    groq      *odyssey.GroqClient // may be nil
    log       *zap.Logger
}
```

Rate-limit key for all debiting endpoints: `cube-hops:ip:<real_client_ip>`.
Real IP extracted via `odyssey.ClientIP(r)` (trusted-proxy XFF handling).
Session ID via `odyssey.IssueSession(w, r)`.

---

## `GET /v3/cube/state`

**Purpose:** Load or initialise player state. No hop cost.

**Behaviour:**
- Issue/validate session cookie.
- `PeekTokenBucket(cube-hops:ip:<ip>, capacity, refillRate)` — reads hop count without debiting.
- `GetGame(sessionID)` — returns zero-value `GameState` if new player (pos at Start, no game yet).
- Compute `zone`, `dist_to_goal`, `neighborhood` (specials + goal within radius ~3.5 cells).
- If no game exists yet (Turn==0 and Seed==0), return a prompt to call `new-game`.

**Response:**
```json
{
  "pos": {"x":3, "y":1, "z":4},
  "score": 120,
  "turn": 7,
  "won": false,
  "dist_to_goal": 14,
  "zone": 1,
  "hops_remaining": 22,
  "hops_capacity": 30,
  "rate_limited": false,
  "retry_after_ms": 0,
  "has_game": true,
  "neighborhood": {
    "warps":  [{"from":{"x":3,"y":2,"z":4},"to":{"x":7,"y":6,"z":8},"bonus":35}],
    "debris": [{"from":{"x":2,"y":1,"z":5},"to":{"x":1,"y":0,"z":3},"penalty":15}],
    "goal":   {"x":9,"y":9,"z":9}
  }
}
```

---

## `POST /v3/cube/new-game`

**Purpose:** Start a fresh game (new board). Does NOT reset hops.

**Behaviour:**
- Generate `seed = crypto/rand.Int64()`.
- `cube.GenerateBoard(seed)` → Board.
- `CubeStore.NewGame(sessionID, seed, layout)` — upserts `cube_games` + `cube_boards` in a transaction, resetting pos/score/turn/won/pending.
- Hops are NOT reset (rate limit is IP-bound, independent of sessions — same anti-cheat as v1 reset).

**Request body:** _(none)_

**Response:** Full state JSON (same shape as `/state`).

---

## `GET /v3/cube/question`

**Purpose:** Fetch the quiz question for the current cell. No hop cost.

**Behaviour:**
- Resolve cell → `SpaceObject` via `cube.ObjectForCell(pos)`.
- `CubeStore.RandomQuestion(objectID)` — if `sql.ErrNoRows` AND Groq available → generate+save.
- Set `game.PendingQuestionID = q.ID` and `SaveGame`.
- Return question WITHOUT the answer index.

**Response:**
```json
{
  "question_id": 88,
  "object": {"catalog_id":"europa","name":"Europa","kind":"moon"},
  "question": "What makes Europa one of the top candidates for extraterrestrial life?",
  "choices": ["A. Its oxygen atmosphere", "B. A subsurface liquid water ocean", "C. Volcanic hydrothermal vents on its surface", "D. Its distance from Jupiter"],
  "source": "groq",
  "zone": 1
}
```

---

## `POST /v3/cube/answer-roll`  ← **the rate-limited endpoint**

**Purpose:** Submit an answer; if correct, execute the quantum-dial roll and move.

**Request body:**
```json
{ "question_id": 88, "choice": 1 }
```

**Behaviour (in order):**
1. Look up `cube-hops` policy from `policy.Cache`.
2. `TokenBucketAllow(cube-hops:ip:<ip>)` — **1 hop debited regardless of correctness**.
   On `denied` → `429` with JSON body (rate-limit overlay trigger).
3. Validate `question_id == game.PendingQuestionID` (anti-replay; prevents submitting a
   stale or forged question ID).
4. `CubeStore.GetQuestionByID(question_id)` — compare `choice` to `q.Answer`.
5. **Correct:**
   - `rng = TurnRNG(game.Seed, game.Turn)`
   - `dist, dir = Roll(rng, game.Pos)` → `newPos = ApplyMove(game.Pos, dist, dir)`
   - Resolve special: `board.WarpAt(newPos)` or `board.DebrisAt(newPos)`
   - If special: `newPos = special.To`; adjust score by `+special.Bonus` or `-special.Penalty`
   - `game.Score += PointsCorrect` + special delta
   - `if IsWin(newPos): game.Won = true`
6. **Wrong:**
   - `game.Score -= PointsWrong`; `newPos = game.Pos` (no move); `dist, dir = nil`
7. `game.Turn++`, `game.Pos = newPos`, `game.PendingQuestionID = nil`
8. `CubeStore.SaveGame(game)`.
9. Return result.

**Response (correct answer, warp landed):**
```json
{
  "allowed": true,
  "correct": true,
  "hops_remaining": 21,
  "roll": {"dist": 4, "dir": {"x":1,"y":0,"z":0}},
  "moved": true,
  "from": {"x":3,"y":1,"z":4},
  "to_after_dial": {"x":7,"y":1,"z":4},
  "special": {"type":"warp","to":{"x":9,"y":3,"z":6},"bonus":35},
  "pos": {"x":9,"y":3,"z":6},
  "score": 175,
  "won": false,
  "correct_answer": 1,
  "correct_label": "B. A subsurface liquid water ocean",
  "hint": "Think about the evidence from Galileo spacecraft magnetometer readings."
}
```

**Response (wrong answer):**
```json
{
  "allowed": true,
  "correct": false,
  "hops_remaining": 20,
  "roll": null,
  "moved": false,
  "from": {"x":9,"y":3,"z":6},
  "to_after_dial": {"x":9,"y":3,"z":6},
  "special": null,
  "pos": {"x":9,"y":3,"z":6},
  "score": 165,
  "won": false,
  "correct_answer": 1,
  "correct_label": "B. A subsurface liquid water ocean",
  "hint": "Think about the evidence from Galileo spacecraft magnetometer readings."
}
```

**Response (429 — hop limit exceeded):**
```json
{
  "allowed": false,
  "reason": "hop_limit_exceeded",
  "retry_after_ms": 3600000,
  "retry_after_s": 3600,
  "message": "You have used all your hops. Your ship needs to refuel."
}
```

---

## `POST /v3/cube/reset`

**Purpose:** Start a new game (equivalent to `new-game`). Hops NOT reset.

Same behaviour as `POST /v3/cube/new-game`. Provided for frontend symmetry with v1.

---

## `cube-hops` policy (proposed)

Seeded by `scripts/seed_cube.sh`:

```json
{
  "id": "cube-hops",
  "subject_type": "ip",
  "algorithm": "token_bucket",
  "capacity": 30,
  "refill_rate": 0.002778,
  "window_ms": 0,
  "action": "deny"
}
```

Math: `refill_rate = 30 / (3 * 3600) = 0.002778 tokens/s` → 30 turns, full refuel in 3 hours.
Read via `policy.Cache` (10s TTL) exactly like `odyssey-hops`.

---

## Policy CRUD endpoints (shared with v1)

`cmd/cube-server/main.go` also mounts `api.New(cache, pStore, limiter, log).Routes(r)`,
which registers `PUT /v1/policies/{id}`, `GET /v1/policies`, etc. on port `:8081`.
This lets `seed_cube.sh` be self-contained and target `:8081` only.

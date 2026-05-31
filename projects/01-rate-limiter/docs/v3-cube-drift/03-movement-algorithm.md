# Movement Algorithm (`internal/cube/movement.go`)

## Overview

Each correct answer unlocks the **quantum dial**: a server-side roll that picks a
**distance** (1–6) and a **direction** (one of 6 axis-aligned unit vectors), then
moves the ship that many steps along that axis, clamped to the cube faces.

The direction is **goal-biased**: ~83% of probability mass points toward the Goal
`(9,9,9)`, guaranteeing the game is always winnable and expected progress per turn
is strictly positive.

All rolls are **server-authoritative and reproducible**: derived from `seed ^ (turn * 0x9E3779B1)`
so every turn is deterministic given `(seed, turn)` and auditable, but unpredictable to
the client (since `seed` is a server-held `int64`, never exposed).

## Constants (tuning knobs in movement.go)

```go
const (
    BiasForward  = 5  // weight given to directions that decrease D (toward goal)
    BiasBackward = 1  // weight given to directions that increase D (away from goal)
    // Ratio 5:1 → forward directions have ~83% of total probability mass when all
    // 6 directions are available. Adjust to make the game shorter/longer.
)
```

## Roll algorithm

```go
// Roll returns a (distance, direction) pair for the current turn.
// rng is seeded per-turn: rand.New(rand.NewSource(seed ^ int64(turn)*0x9E3779B1))
func Roll(rng *rand.Rand, pos Coord) (dist int, dir Coord) {
    dist = 1 + rng.Intn(6)    // quantum die: 1..6

    type option struct {
        dir    Coord
        weight int
    }
    var opts []option

    // +x / -x
    if pos.X < 9 { opts = append(opts, option{Coord{1, 0, 0}, BiasForward}) }
    if pos.X > 0 { opts = append(opts, option{Coord{-1, 0, 0}, BiasBackward}) }
    // +y / -y
    if pos.Y < 9 { opts = append(opts, option{Coord{0, 1, 0}, BiasForward}) }
    if pos.Y > 0 { opts = append(opts, option{Coord{0, -1, 0}, BiasBackward}) }
    // +z / -z
    if pos.Z < 9 { opts = append(opts, option{Coord{0, 0, 1}, BiasForward}) }
    if pos.Z > 0 { opts = append(opts, option{Coord{0, 0, -1}, BiasBackward}) }

    // Weighted random pick
    total := 0
    for _, o := range opts { total += o.weight }
    r := rng.Intn(total)
    for _, o := range opts {
        r -= o.weight
        if r < 0 { dir = o.dir; return }
    }
    dir = opts[len(opts)-1].dir   // fallback (unreachable in practice)
    return
}
```

## Move application + bounds clamping

```go
func clamp(v int) int {
    if v < 0 { return 0 }
    if v > 9 { return 9 }
    return v
}

// ApplyMove moves pos by dist steps in dir, clamping at cube faces.
// Overshoot is simply absorbed by the face (lost steps).
func ApplyMove(pos Coord, dist int, dir Coord) Coord {
    return Coord{
        clamp(pos.X + dir.X*dist),
        clamp(pos.Y + dir.Y*dist),
        clamp(pos.Z + dir.Z*dist),
    }
}
```

## Win condition

After `ApplyMove` and warp/debris resolution (see `04-board-generation.md`):

```go
func IsWin(pos Coord) bool { return pos == Goal }
```

Win triggers `GameState.Won = true`; the win overlay is shown in the frontend.

## Solvability proof (informal)

- From any non-goal cell, at least one of x, y, z is `< 9`, so at least one
  forward direction has positive weight (BiasForward > 0). The game can always
  make progress.
- Expected steps per turn toward goal: with BiasForward=5, BiasBackward=1, at a
  typical mid-game cell (all three axes progressing), the expected D-reduction per
  turn ≈ `(5/(5+1)) * 3.5 * (1 axis chosen out of ~3.5 available forward)` — roughly
  1–2 Manhattan units per turn. With 30 hops and D=27, the game is comfortably winnable
  on average. Run a Monte-Carlo `go test` (see `10-risks.md`) to confirm.
- Warps only decrease D; debris only increases D by a bounded amount. Neither breaks
  the forward-bias guarantee.

## RNG reproducibility

```go
// TurnRNG returns a fresh *rand.Rand for the given game seed and turn number.
// The XOR-shift with a golden-ratio-derived constant spreads seeds evenly so
// consecutive turns don't produce correlated sequences.
func TurnRNG(seed int64, turn int) *rand.Rand {
    return rand.New(rand.NewSource(seed ^ int64(turn)*0x9E3779B97F4A7C15))
}
```

Unit test: `TestRollDeterminism` asserts that `TurnRNG(s, t)` → `Roll(...)` gives
the same result called twice with the same inputs, and that different seeds or turns
give different results.

## Turn sequence (full)

```
1.  Client: GET /v3/cube/question     → question served, pending_question_id set
2.  Client: POST /v3/cube/answer-roll { question_id, choice }
    Server:
      a. TokenBucketAllow(cube-hops:ip:<ip>)  → debit 1 hop regardless of answer
      b. Validate question_id == pending_question_id (anti-replay)
      c. Compare choice to stored answer
      d. CORRECT:
           rng = TurnRNG(game.Seed, game.Turn)
           dist, dir = Roll(rng, game.Pos)
           newPos = ApplyMove(game.Pos, dist, dir)
           special = board.WarpAt(newPos) or board.DebrisAt(newPos)
           if special: newPos = special.To; apply bonus/penalty to score
           game.Score += PointsCorrect + (special.Bonus or -special.Penalty)
           if IsWin(newPos): game.Won = true
         WRONG:
           game.Score -= PointsWrong
           newPos = game.Pos (no movement)
           dist, dir = nil (dial stays locked)
      e. game.Turn++, game.PendingQuestionID = nil, game.Pos = newPos
      f. SaveProgress(game)
      g. Return result JSON
```

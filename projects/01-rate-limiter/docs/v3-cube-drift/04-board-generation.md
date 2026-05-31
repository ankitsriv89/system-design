# Board Generation (`internal/cube/placement.go`)

## Per-game randomized warps and debris

Every `new-game` generates a fresh `seed = crypto/rand.Int64()`. The board layout
(warps + debris) is derived entirely from this seed so it is:
- **Reproducible**: same seed → same board (auditable).
- **Unpredictable**: seed is server-held and never exposed to the client.
- **Balanced**: rules below ensure solvability and prevent cascade loops.

## Constants (tuning knobs)

```go
const (
    NumWarps  = 12  // warp portals (ladders) per game
    NumDebris = 12  // debris fields (snakes) per game
    // Total: 24 special cells out of 1000 = 2.4% — sparse, legible

    WarpMinJump  = 2   // minimum D reduction for a warp
    WarpMaxJump  = 8   // maximum D reduction for a warp
    DebrisMinPush = 2  // minimum D increase for debris
    DebrisMaxPush = 6  // maximum D increase for debris
    DebrisMaxD   = 26  // debris To cell may not have D > 26 (can't be pushed onto Start corner)
)
```

## Placement rules (solvability + termination)

1. **Forbidden cells**: Start `(0,0,0)` and Goal `(9,9,9)` are never a `From` or `To`.
2. **No overlap and no chaining**: a cell may be the `From` of at most one special and
   the `To` of at most one special. Crucially, no cell is both a `To` of one special AND
   a `From` of another — this eliminates cascade chains (you can never land on a warp that
   drops you onto debris, etc.). Resolution is guaranteed single-step.
3. **Warp invariant**: `D(To) < D(From)`, gap in `[WarpMinJump, WarpMaxJump]`.
   Bonus = `10 + 5 * (D(From) - D(To))` — scales with jump.
4. **Debris invariant**: `D(To) > D(From)`, gap in `[DebrisMinPush, DebrisMaxPush]`,
   AND `D(To) <= DebrisMaxD` (bounded knockback, never back to Start).
   Penalty = `5 * (D(To) - D(From))`.
5. **Solvability**: the dial alone (BiasForward=5) is always winnable independent of
   specials. Warps only help; debris applies bounded, non-looping setbacks.

## Algorithm (pseudocode)

```go
func GenerateBoard(seed int64) Board {
    rng  := rand.New(rand.NewSource(seed))
    used := map[Coord]bool{ Start: true, Goal: true }

    // pick a random unused cell and mark it used
    pickUnused := func() Coord {
        for {
            c := Coord{rng.Intn(N), rng.Intn(N), rng.Intn(N)}
            if !used[c] { used[c] = true; return c }
        }
    }

    // pick a cell strictly closer to Goal than 'from', D gap in [minJump, maxJump]
    pickCloser := func(from Coord) (Coord, bool) {
        for attempts := 0; attempts < 200; attempts++ {
            c := Coord{rng.Intn(N), rng.Intn(N), rng.Intn(N)}
            gap := from.DistToGoal() - c.DistToGoal()
            if gap >= WarpMinJump && gap <= WarpMaxJump && !used[c] {
                used[c] = true
                return c, true
            }
        }
        return Coord{}, false   // skip if can't find in budget
    }

    // pick a cell strictly farther from Goal than 'from', gap in [minPush, maxPush]
    pickFarther := func(from Coord) (Coord, bool) {
        for attempts := 0; attempts < 200; attempts++ {
            c := Coord{rng.Intn(N), rng.Intn(N), rng.Intn(N)}
            gap := c.DistToGoal() - from.DistToGoal()
            if gap >= DebrisMinPush && gap <= DebrisMaxPush &&
               c.DistToGoal() <= DebrisMaxD && !used[c] {
                used[c] = true
                return c, true
            }
        }
        return Coord{}, false
    }

    warps  := []Warp{}
    debris := []Debris{}

    for len(warps) < NumWarps {
        from := pickUnused()
        to, ok := pickCloser(from)
        if !ok { continue }
        gap := from.DistToGoal() - to.DistToGoal()
        warps = append(warps, Warp{from, to, 10 + 5*gap})
    }

    for len(debris) < NumDebris {
        from := pickUnused()
        to, ok := pickFarther(from)
        if !ok { continue }
        gap := to.DistToGoal() - from.DistToGoal()
        debris = append(debris, Debris{from, to, 5 * gap})
    }

    return Board{Seed: seed, Warps: warps, Debris: debris}
}
```

The 200-attempt budget per cell prevents an infinite loop if the cube is densely
filled (unlikely at 2.4% occupancy). If budget is exceeded for a slot, the slot is
simply skipped — the game will have slightly fewer specials, which is safe.

## Persistence

```
cube_games.seed   → stored on new-game; board can be regenerated from this alone
cube_boards       → layout JSON stored for fast reads on /state and /answer-roll
```

On `new-game` or `reset`, `GenerateBoard(seed)` is called and both rows are written
atomically (transaction). The board JSON is read by `answer-roll` to resolve specials.

## Neighborhood (for frontend rendering)

The `/v3/cube/state` endpoint returns a `neighborhood` object: all specials (warps +
debris) whose `From` cell has Euclidean distance ≤ radius R from the current ship
position, plus the Goal beacon. This keeps the Three.js marker count low (~5–15)
regardless of total board size.

```go
const NeighborhoodRadius = 3.5   // Euclidean distance in cell units
```

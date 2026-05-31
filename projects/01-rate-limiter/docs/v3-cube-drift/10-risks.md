# Risks, Unknowns & Tuning Knobs

## Risk 1 — 1000-cell legibility (highest priority)

**Problem:** Rendering 1000 individual cell meshes in Three.js would be illegible and
likely hit 60fps on mobile. The player wouldn't be able to read their position or understand
the board spatially.

**Mitigation (in scene design):**
- Wireframe `BoxGeometry` for the cube frame: 1 draw call.
- 4 zone shell boxes: 4 draw calls.
- Neighborhood markers only (radius ≤ 3.5 cells from ship): ≤ ~15 draw calls.
- Goal beacon: 1 draw call.
- Breadcrumb line (last 20 cells): 1 draw call.
- Ship: 1 draw call (GLTF) or ~5 procedural meshes.
- Total: ~25–30 draw calls regardless of board size.

**Residual risk:** Camera framing. The cube is 10×10×10 world units. At a glance, the
player needs to see their ship + goal + nearby specials. The `OrbitControls` auto-target
(ship–goal midpoint) may need iteration to feel good. Leave as a known UX polish item.

---

## Risk 2 — Goal-biased RNG fairness

**Problem:** `BiasForward=5 / BiasBackward=1` is a starting estimate. The game could be
too short (boring, all games win in <10 turns) or too long (players run out of hops before
winning).

**Mitigation:** Add a Monte-Carlo unit test:

```go
// movement_test.go
func TestMedianTurnsToWin(t *testing.T) {
    const runs = 10_000
    turns := 0
    for i := 0; i < runs; i++ {
        pos := Start
        seed := int64(i)
        for turn := 0; !IsWin(pos); turn++ {
            rng := TurnRNG(seed, turn)
            dist, dir := Roll(rng, pos)
            pos = ApplyMove(pos, dist, dir)
            if turn > 500 { t.Fatal("run stuck") }
        }
        turns += /* turn count */
    }
    median := turns / runs
    // Target: median turns-to-win well within cube-hops capacity (30)
    // ~15–25 turns is a good sweet spot
    if median < 10 || median > 25 {
        t.Errorf("median turns %d outside [10,25] — adjust BiasForward/BiasBackward", median)
    }
}
```

Run this during development; adjust the bias constants until the median lands in the
target range. With warps + debris the actual median will shift; run the test with a
simulated board too.

**Tuning knobs (all named constants):**
```go
BiasForward  = 5   // increase → shorter games
BiasBackward = 1   // increase → more backward movement, longer games
```

---

## Risk 3 — Warp/debris balance

**Problem:** 12/12 specials and the chosen gap margins `[2,8]` warp / `[2,6]` debris
are first estimates. If debris penalties are too large relative to the forward bias,
players may feel punished, especially when they also lose a hop.

**Mitigation:**
- All warp/debris counts and margins are named constants in `placement.go` — easy to tune.
- Run the Monte-Carlo test with warps + debris included and compare median turns-to-win.
- Start with `NumDebris = 8` (fewer snakes than ladders) for a friendlier first release;
  increase to 12 after playtesting.
- Consider adding a "safe zone" around Start (no debris within D_from_start ≤ 3) so new
  players aren't immediately knocked back.

---

## Risk 4 — Three.js perf on mobile

**Problem:** Mobile GPUs struggle with large particle counts and `backdrop-filter: blur`.

**Mitigation:**
- Star count is already capped at 3500 (proven fine in v1 odyssey).
- `renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))` — same as v1.
- Use `depthWrite: false` on all zone shell meshes (transparent overlay, no depth test cost).
- Avoid `CSS2DRenderer` for marker labels — use Three.js `SpriteMaterial` instead
  (single GPU batch, no DOM cost).
- Watch tween GC: pool or reuse tween closures rather than creating new ones per move.

---

## Risk 5 — Groq cold cache (first visit latency)

**Problem:** The first time a player visits a Zone 2–3 cell, the server calls Groq (15s
timeout). This causes a noticeable delay in `GET /v3/cube/question`.

**Mitigation:**
- Seed 3–5 questions per object for **all** Zone 0 and Zone 1 objects at startup (covers
  the most-visited cells since games start at `(0,0,0)`).
- For Zone 2–3 objects, the `503 try again` degradation path (same as v1) keeps the game
  playable even if Groq is down or slow.
- `GroqClient.http` already has a 15s timeout — no change needed.
- Future: add a background seeding goroutine that pre-generates Zone 2–3 questions at
  startup asynchronously (not in scope for v1 of this feature).

---

## Risk 6 — RNG reproducibility desync

**Problem:** If the server's `TurnRNG(seed, turn)` formula is not used consistently
(e.g., turn counter increments in a different order than the RNG call), the persisted
position could desync from a replayed roll.

**Mitigation:**
- `TurnRNG` is a pure function (`seed, turn → *rand.Rand`). The turn counter is always
  incremented **after** calling `Roll` and persisted atomically with the new position.
- Unit test `TestRollDeterminism`:
  ```go
  func TestRollDeterminism(t *testing.T) {
      for _, seed := range []int64{0, 1, 42, -1, math.MaxInt64} {
          for turn := 0; turn < 50; turn++ {
              d1, dir1 := Roll(TurnRNG(seed, turn), Coord{3,4,2})
              d2, dir2 := Roll(TurnRNG(seed, turn), Coord{3,4,2})
              if d1 != d2 || dir1 != dir2 {
                  t.Errorf("seed=%d turn=%d: non-deterministic", seed, turn)
              }
          }
      }
  }
  ```

---

## Summary table

| Risk | Severity | Mitigation |
|---|---|---|
| 1000-cell legibility | High | Wireframe + zone shells + neighborhood markers only (~30 draw calls total) |
| RNG fairness (too short/long) | Medium | Monte-Carlo unit test; `BiasForward`/`BiasBackward` tuning knobs |
| Warp/debris balance | Medium | Named constants; start with NumDebris=8; safe zone near Start |
| Three.js mobile perf | Low | Proven v1 patterns; no DOM for labels; pixel ratio cap; depthWrite:false |
| Groq cold cache latency | Low | Pre-seed Zone 0–1 questions; 503 degradation path from v1 |
| RNG desync | Low | Pure function; unit test; atomic turn increment + persist |

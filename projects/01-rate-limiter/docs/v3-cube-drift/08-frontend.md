# Frontend Plan (`web/cube*.html/css/js`)

## Files

| File | Role |
|---|---|
| `web/cube.html` | Shell: importmap, canvas, HUD panels, overlays |
| `web/cube.css` | Styles: dark-space theme, two-meter layout, dial animation |
| `web/cube-ui.js` | UI controller: API calls, state management, overlays, dial |
| `web/cube-scene.js` | Three.js scene: cube wireframe, zones, ship, markers, tweens |

---

## `cube.html` shell structure

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Cube Drift — Interstellar 3D</title>
  <link rel="stylesheet" href="/cube.css">
  <script type="importmap">
  {
    "imports": {
      "three": "https://cdn.jsdelivr.net/npm/three@0.165.0/build/three.module.js",
      "three/addons/": "https://cdn.jsdelivr.net/npm/three@0.165.0/examples/jsm/"
    }
  }
  </script>
</head>
<body>
  <!-- Full-viewport Three.js canvas -->
  <canvas id="c"></canvas>

  <!-- Top HUD: position + meters -->
  <header class="hud hud-top">
    <div id="posDisplay">Earth (0,0,0)</div>
    <div class="meter-row">
      <label>HOPS</label>
      <div class="hop-track"><div id="hopBar"></div></div>
      <span id="hopsRemaining">30</span>/30
      <span id="refillTimer"></span>
    </div>
    <div class="meter-row">
      <label>SCORE</label>
      <span id="scoreDisplay">0</span>
    </div>
    <div id="healthDot"></div>
  </header>

  <!-- Left panel: current object info + actions -->
  <section class="hud hud-left" id="infoPanel">
    <h2 id="objectName">—</h2>
    <p id="objectKind"></p>
    <p id="zoneLabel">Zone 0 · Inner System</p>
    <p id="distLabel">D to goal: 27</p>
    <button id="btnNewGame">New Game</button>
    <button id="btnQuestion">Get Question</button>
    <button id="btnReset">Reset</button>
    <p id="statusMsg"></p>
  </section>

  <!-- Right panel: question + choices + dial -->
  <section class="hud hud-right" id="questionPanel" hidden>
    <div id="qBadge" class="q-badge"></div>
    <p id="qText" class="q-text"></p>
    <ol id="qChoices" class="q-choices"></ol>
    <div id="qFeedback" class="q-feedback" hidden></div>
    <div class="dial-wrap">
      <canvas id="dialCanvas" width="120" height="120"></canvas>
      <div id="dialResult"></div>
    </div>
    <div class="q-actions">
      <button id="btnHint" class="ghost">Hint <span class="cost">(−1 hop)</span></button>
    </div>
  </section>

  <!-- Rate-limit overlay -->
  <div id="rateLimitOverlay" hidden>
    <h2>⛽ Refuelling…</h2>
    <p id="rlMsg"></p>
    <div id="rlCountdown" class="countdown"></div>
    <button id="btnRlClose">Close</button>
  </div>

  <!-- Win overlay -->
  <div id="winOverlay" hidden>
    <h1>🌌 Sagittarius A* Reached!</h1>
    <p id="finalScore"></p>
    <button id="btnPlayAgain">Play Again</button>
  </div>

  <div id="warpFlash"></div>
  <div id="denyFlash"></div>

  <script type="module" src="/cube-ui.js"></script>
</body>
</html>
```

---

## `cube-scene.js` — Three.js scene

### Reused from `odyssey-scene.js` (verbatim pattern)
- WebGL renderer, ACES filmic tone mapping, pixel ratio capping.
- 3500-star particle field (THREE.Points) with slow rotation parallax.
- GLTF Voyager ship loader + procedural fallback (same CDN URL).
- Warp-line streaks (60 lines, cyan, `THREE.LineBasicMaterial`).
- Camera shake (`shakeIntensity` decays at 0.82x/frame).
- `warpFlash` + `denyFlash` DOM divs.
- Engine light pulsing on ship.

### New: 3D cube rendering (legibility-first approach)

**Do NOT render 1000 individual cell meshes — that is illegible and a perf risk.**

```
Layer 1: Cube wireframe
  BoxGeometry(10,10,10) → EdgesGeometry → LineSegments (cyan, opacity 0.15)
  One draw call. Gives the spatial bounding box.

Layer 2: Zone shells (4 nested translucent boxes)
  Zone 0 (D_from_start 0–6):  small corner box near Start, warm orange, opacity 0.04
  Zone 1 (D_from_start 7–13): medium box, yellow, opacity 0.04
  Zone 2 (D_from_start 14–20): larger box, cyan, opacity 0.03
  Zone 3 (D_from_start 21–27): full cube, violet/deep blue, opacity 0.03
  These are BoxGeometry + MeshBasicMaterial(transparent, depthWrite:false).
  Player reads difficulty at a glance by colour.

Layer 3: Neighborhood markers (from state.neighborhood, <~15 meshes)
  Goal beacon:   pulsing SphereGeometry (bright gold, PointLight attached)
  Warp portals:  ConeGeometry pointing UP, cyan emissive, at From cell
  Debris fields: ConeGeometry pointing DOWN, red emissive, at From cell
  Each marker has a sprite label (THREE.Sprite or CSS2DRenderer) with the special's
  bonus/penalty number.

Layer 4: Ship
  Same Voyager GLTF / procedural fallback as v1. Sits at cellToWorld(pos).

Layer 5: Breadcrumb trail
  THREE.Line through up to last 20 visited cells (thin, dashed cyan, opacity 0.3).
```

### Cell → world coordinate mapping

```js
function cellToWorld(x, y, z) {
  // Maps [0,9]³ into a [-4.5, 4.5]³ cube centred at origin.
  // Cell size = 1 unit in world space.
  return new THREE.Vector3(x - 4.5, y - 4.5, z - 4.5);
}
```

### Camera

`OrbitControls` from `three/addons/controls/OrbitControls.js`.
- Default target: midpoint between ship position and Goal, weighted 60/40 to ship.
- Auto-updates every frame so camera always frames ship + goal direction.
- Player can still orbit/zoom manually.

### Move tween (no external library — plain requestAnimationFrame)

```js
function tweenShip(fromCoord, toCoord, durationMs, onComplete) {
  const from = cellToWorld(...fromCoord);
  const to   = cellToWorld(...toCoord);
  const start = performance.now();
  function step(now) {
    const t = Math.min((now - start) / durationMs, 1);
    const ease = t < 0.5 ? 2*t*t : -1+(4-2*t)*t;  // ease-in-out
    ship.position.lerpVectors(from, to, ease);
    if (t < 1) requestAnimationFrame(step);
    else onComplete?.();
  }
  requestAnimationFrame(step);
}
```

On correct answer + move:
1. `playWarp()` (warp lines + flash, same as v1).
2. `tweenShip(from, toAfterDial, 800ms)`.
3. If `special.type == "warp"`: chain `playWarp()` + `tweenShip(toAfterDial, special.to, 600ms)`.
4. If `special.type == "debris"`: chain `playDeny()` (shake + red flash) + `tweenShip(toAfterDial, special.to, 600ms)`.

### Exports

```js
export { sceneReady, updateScene, playWarp, playDeny }
// updateScene({pos, neighborhood, breadcrumb}) — called from cube-ui.js after each state change
```

---

## `cube-ui.js` — UI controller

### State object

```js
const state = {
  pos: {x:0,y:0,z:0},
  score: 0,
  turn: 0,
  won: false,
  hasGame: false,
  hopsRemaining: 30,
  rateLimited: false,
  retryAfterMs: 0,
  currentQuestionId: null,
  answered: false,
  neighborhood: {warps:[], debris:[], goal:{x:9,y:9,z:9}},
};
```

### Quantum dial (canvas overlay)

```js
// cube-ui.js: a 2D canvas dial inside #dialCanvas
// Shows a d6 face spinning at ~10fps; on unlock, slows + settles on roll.dist (1–6).
// On wrong answer: shows a lock icon + shake animation (CSS class 'dial-locked').
function animateDial(result /*null=locked, 1–6=unlocked*/) { ... }
```

### API call pattern (same as v1)

```js
async function apiFetch(path, opts = {}) {
  return fetch(path, {
    credentials: 'same-origin',
    headers: {'Content-Type':'application/json', ...opts.headers},
    ...opts,
  });
}
```

### Full turn flow (client-side)

```
loadState()           → GET /v3/cube/state → updateScene + render meters
btnNewGame click      → POST /v3/cube/new-game → loadState()
btnQuestion click     → GET /v3/cube/question → renderQuestion()
choice button click   → POST /v3/cube/answer-roll
                         correct: animateDial(roll.dist) → tweenShip → check special → loadState()
                         wrong:   animateDial(null) → show feedback → show 'Try again' button
                         429:     showRateLimitOverlay(retry_after_ms)
btnHint click         → GET /v3/cube/question?hint=1 (or separate hint endpoint — same as v1)
```

### Hop bar + countdown (reuse v1 logic)

```js
const MAX_HOPS = 30;
function renderHopBar(remaining) {
  bar.style.width = Math.max(0, (remaining / MAX_HOPS) * 100) + '%';
  bar.classList.toggle('low', remaining <= 5);
}
function startRefillCountdown(ms) { /* identical to odyssey-ui.js startRefillCountdown */ }
```

---

## `cube.css` — visual theme

Extends the v1 dark-space palette (`--bg`, `--cyan`, `--green`, `--red`, `--gold`, `--violet`).
New elements:

```css
/* Two-meter row */
.meter-row { display: flex; align-items: center; gap: 8px; }

/* Score display — bright gold, tabular nums */
#scoreDisplay { color: var(--gold); font-size: 18px; font-variant-numeric: tabular-nums; }

/* Quantum dial canvas */
.dial-wrap { display: flex; flex-direction: column; align-items: center; margin-top: 12px; }
.dial-wrap.dial-locked #dialCanvas { filter: grayscale(1) opacity(0.4); }
#dialResult { font-size: 24px; font-weight: 700; color: var(--cyan); min-height: 32px; }

/* Zone colour hints in HUD */
.zone-0 { color: #ff9f4a; }   /* warm orange — inner system */
.zone-1 { color: var(--gold); }
.zone-2 { color: var(--cyan); }
.zone-3 { color: var(--violet); }

/* Warp marker label */
.warp-label { color: var(--cyan); font-size: 11px; }
.debris-label { color: var(--red); font-size: 11px; }
```

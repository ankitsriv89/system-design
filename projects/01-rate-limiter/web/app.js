// Rate Limit Galaxy — interactive cosmic rate-limit visualizer
const canvas = document.querySelector("#universe");
const ctx = canvas.getContext("2d");

const state = {
  width: 0,
  height: 0,
  dpr: 1,
  objects: [],
  projected: [],
  stars: [],          // static backdrop star field
  trail: [],          // ship exhaust particles
  shake: 0,           // screen-shake magnitude (decays)
  ship: { x: 0, y: 0, targetX: 0, targetY: 0, angle: 0, moving: false },
  wormhole: null,
  deniedPulse: 0,
  remaining: null,
  capacity: 8,
  events: [],
  selected: null,
  pointer: { x: 0, y: 0 },
  start: performance.now(),
  cooldownEnd: 0,     // unix ms when cooldown expires
  jumping: false,     // debounce rapid clicks
};

const els = {
  subject:       document.querySelector("#subjectInput"),
  policy:        document.querySelector("#policySelect"),
  health:        document.querySelector("#healthLight"),
  jump:          document.querySelector("#jumpButton"),
  burst:         document.querySelector("#burstButton"),
  seed:          document.querySelector("#seedButton"),
  usage:         document.querySelector("#usageButton"),
  fuelFill:      document.querySelector("#fuelFill"),
  remaining:     document.querySelector("#remainingLabel"),
  cooldown:      document.querySelector("#cooldownLabel"),
  targetName:    document.querySelector("#targetName"),
  targetTracer:  document.querySelector("#targetTracer"),
  targetRedshift:document.querySelector("#targetRedshift"),
  targetRosette: document.querySelector("#targetRosette"),
  targetDistance:document.querySelector("#targetDistance"),
  oracle:        document.querySelector("#oracleText"),
  log:           document.querySelector("#eventLog"),
};

const facts = [
  "DESI maps 40 million galaxies using spectra — each jump uses a real rate-limit check.",
  "Token buckets allow short bursts then throttle; tokens refill at a steady rate.",
  "Sliding-window logs are precise but memory grows O(requests) inside the window.",
  "A 429 response is not a crash — it protects downstream capacity intentionally.",
  "Fail-open limiters preserve availability when Redis is temporarily unreachable.",
  "Hot keys are dangerous: one subject can saturate a single Redis counter shard.",
  "DESI groups sky objects into 20 sky regions called rosettes.",
  "Redshift z > 1 means light traveled over 7 billion years to reach DESI's telescope.",
  "A wormhole collapse means the limiter rejected the jump — retry after cooldown.",
  "Burst storms demonstrate the token bucket emptying faster than it refills.",
];

const tracerColors = { QSO: "#f8d06b", ELG: "#60e6ff", LRG: "#ff7ab6", BGS: "#8affad" };

// ─── Boot ────────────────────────────────────────────────────────────────────

async function boot() {
  resize();
  buildStarField();
  window.addEventListener("resize", () => { resize(); buildStarField(); });

  canvas.addEventListener("click", onCanvasClick);
  canvas.addEventListener("touchend", onCanvasTouch, { passive: false });
  canvas.addEventListener("pointermove", e => { state.pointer = { x: e.clientX, y: e.clientY }; });

  els.jump.addEventListener("click",  () => jumpTo(randomObject()));
  els.burst.addEventListener("click", burstStorm);
  els.seed.addEventListener("click",  () => seedDemoPolicies(true));
  els.usage.addEventListener("click", syncUsage);
  els.policy.addEventListener("change", updatePolicyCapacity);

  await loadCatalog();
  updatePolicyCapacity();
  resetShip();
  await checkHealth();
  await seedDemoPolicies(false);
  requestAnimationFrame(draw);
}

// ─── Data ─────────────────────────────────────────────────────────────────────

async function loadCatalog() {
  const res = await fetch("/data/desi-sample.json");
  const payload = await res.json();
  state.objects = payload.objects;
  projectObjects();
  addEvent("allowed", `Loaded ${payload.sampleSize} DESI objects`, "Click any star or use Random jump.");
}

function buildStarField() {
  const count = Math.floor((state.width * state.height) / 3200);
  state.stars = Array.from({ length: count }, () => ({
    x: Math.random() * state.width,
    y: Math.random() * state.height,
    r: Math.random() * 1.2 + 0.2,
    alpha: Math.random() * 0.6 + 0.2,
    speed: Math.random() * 0.8 + 0.2, // twinkle speed
    phase: Math.random() * Math.PI * 2,
  }));
}

// ─── Layout ───────────────────────────────────────────────────────────────────

function resize() {
  state.dpr = Math.min(window.devicePixelRatio || 1, 2);
  state.width = window.innerWidth;
  state.height = window.innerHeight;
  canvas.width  = Math.floor(state.width  * state.dpr);
  canvas.height = Math.floor(state.height * state.dpr);
  canvas.style.width  = `${state.width}px`;
  canvas.style.height = `${state.height}px`;
  ctx.setTransform(state.dpr, 0, 0, state.dpr, 0, 0);
  projectObjects();
  if (!state.ship.x) resetShip();
}

function projectObjects() {
  if (!state.objects.length || !state.width || !state.height) return;
  const scale   = Math.min(state.width, state.height) * 0.055;
  const centerX = state.width  / 2;
  const centerY = state.height / 2;
  state.projected = state.objects.map(obj => {
    const depth     = 1 / (1 + Math.max(0, obj.z) * 0.08);
    const rosetteArc= Math.sin(obj.rosette * 1.7) * 16;
    return {
      obj,
      x: centerX + obj.x * scale * 10 * depth + rosetteArc,
      y: centerY + obj.y * scale * 10 * depth + obj.z * scale * 0.22,
      r: Math.max(1.1, Math.min(3.4, 1.2 + obj.redshift * 1.2)),
      alpha: Math.max(0.28, Math.min(0.95, 0.72 - obj.redshift * 0.02)),
    };
  });
}

function resetShip() {
  state.ship.x = state.ship.targetX = state.width  * 0.5;
  state.ship.y = state.ship.targetY = state.height * 0.52;
}

// ─── Health & seeding ─────────────────────────────────────────────────────────

async function checkHealth() {
  try {
    const res = await fetch("/healthz");
    els.health.classList.toggle("online", res.ok);
  } catch { els.health.classList.remove("online"); }
  setTimeout(checkHealth, 10_000);
}

async function seedDemoPolicies(showLog = true) {
  const policies = [
    { id: "galaxy-token-bucket",  body: { subject_type:"ship", algorithm:"token_bucket",   capacity:8,  refill_rate:1.5, window_ms:0,     action:"deny" } },
    { id: "galaxy-sliding-window",body: { subject_type:"ship", algorithm:"sliding_window", capacity:12, refill_rate:0,   window_ms:15000, action:"deny" } },
  ];
  try {
    await Promise.all(policies.map(p => fetch(`/v1/policies/${p.id}`, {
      method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(p.body),
    })));
    if (showLog) addEvent("allowed", "Demo policies seeded", "Token bucket and sliding window ready.");
  } catch {
    if (showLog) addEvent("denied", "Could not seed policies", "Start docker compose, then try again.");
  }
}

// ─── Policy helpers ───────────────────────────────────────────────────────────

function updatePolicyCapacity() {
  state.capacity = els.policy.value.includes("sliding") ? 12 : 8;
  state.remaining = null;
  updateFuel({});
}

// ─── Interaction ──────────────────────────────────────────────────────────────

async function onCanvasClick(e) {
  const hit = nearestObject(e.clientX, e.clientY);
  if (hit) await jumpTo(hit.obj);
}

function onCanvasTouch(e) {
  e.preventDefault();
  const t = e.changedTouches[0];
  const hit = nearestObject(t.clientX, t.clientY, 38);
  if (hit) jumpTo(hit.obj);
}

function nearestObject(x, y, threshold = 22) {
  let best = null, bestDist = threshold;
  for (const p of state.projected) {
    const d = Math.hypot(p.x - x, p.y - y);
    if (d < bestDist) { best = p; bestDist = d; }
  }
  return best;
}

function randomObject() {
  return state.objects[Math.floor(Math.random() * state.objects.length)];
}

// ─── Jump ─────────────────────────────────────────────────────────────────────

async function jumpTo(obj) {
  if (!obj || state.jumping) return;
  state.jumping = true;
  setTimeout(() => { state.jumping = false; }, 120);

  state.selected = obj;
  updateTarget(obj);

  let res, payload;
  try {
    res     = await fetch("/v1/limits/check", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ subject: els.subject.value.trim() || "ship:andromeda", policy_id: els.policy.value }),
    });
    payload = await res.json();
  } catch {
    collapseWormhole(obj, "Backend offline — start docker compose.");
    addEvent("denied", "Jump failed", "API unreachable. Run: docker compose up");
    return;
  }

  state.remaining = payload.remaining;
  updateFuel(payload);

  if (res.ok && payload.allowed) {
    const pt = projectedFor(obj);
    state.ship.targetX = pt.x;
    state.ship.targetY = pt.y;
    state.ship.moving  = true;
    openWormhole(pt, true);
    els.oracle.textContent = facts[Math.floor(Math.random() * facts.length)];
    addEvent("allowed", `Jump → DESI ${obj.targetId.slice(-8)}`, `${obj.tracer}, z=${obj.redshift.toFixed(4)}, remaining=${payload.remaining}`);
  } else {
    collapseWormhole(obj, `Wormhole collapsed — retry in ${payload.retry_after_ms || 1000} ms`);
    triggerScreenShake(6);
    startCooldown(payload.retry_after_ms || 1000);
    addEvent("denied", "Rate limit hit", `Retry after ${payload.retry_after_ms || 1000} ms.`);
  }
}

function openWormhole(pt, allowed) {
  state.wormhole = { x: pt.x, y: pt.y, born: performance.now(), allowed };
}

function collapseWormhole(obj, msg) {
  const pt = projectedFor(obj);
  openWormhole(pt, false);
  state.deniedPulse = 1;
  els.oracle.textContent = msg;
}

// ─── Burst storm ──────────────────────────────────────────────────────────────

async function burstStorm() {
  addEvent("allowed", "Burst storm launched", "Firing 20 rapid checks to exhaust the bucket.");
  triggerScreenShake(3);
  for (let i = 0; i < 20; i++) {
    setTimeout(() => jumpTo(randomObject()), i * 80);
  }
}

// ─── Usage sync ───────────────────────────────────────────────────────────────

async function syncUsage() {
  const subject = encodeURIComponent(els.subject.value.trim() || "ship:andromeda");
  try {
    const res     = await fetch(`/v1/usage/${subject}`);
    const payload = await res.json();
    const decisions = Array.isArray(payload.decisions) ? payload.decisions : [];
    state.events = decisions.slice(0, 12).map(d => ({
      type:   d.allowed ? "allowed" : "denied",
      title:  d.allowed ? "Allowed" : "Denied",
      detail: `${d.policy_id} · ${new Date(d.decided_at).toLocaleTimeString()}`,
    }));
    renderLog();
    addEvent("allowed", `Usage synced (${decisions.length} records)`, "Showing latest audit decisions from PostgreSQL.");
  } catch {
    addEvent("denied", "Usage sync failed", "Could not read audit history from backend.");
  }
}

// ─── Fuel / cooldown ──────────────────────────────────────────────────────────

function updateFuel(payload = {}) {
  const rem = payload.remaining ?? state.remaining;
  if (rem === null || rem < 0) {
    els.remaining.textContent = "--";
    els.fuelFill.style.width = "0%";
    els.cooldown.textContent = "Click a star or press Random jump.";
    return;
  }
  const pct = Math.max(0, Math.min(100, (rem / state.capacity) * 100));
  els.remaining.textContent = `${rem} / ${state.capacity}`;
  els.fuelFill.style.width = `${pct}%`;
  // colour the bar red when denied
  els.fuelFill.classList.toggle("depleted", !payload.allowed && payload.allowed !== undefined);
}

function startCooldown(ms) {
  state.cooldownEnd = Date.now() + ms;
}

function tickCooldown() {
  if (!state.cooldownEnd) return;
  const left = state.cooldownEnd - Date.now();
  if (left <= 0) {
    state.cooldownEnd = 0;
    els.cooldown.textContent = "Cooldown expired — ready to jump.";
  } else {
    els.cooldown.textContent = `Rate limited — cooldown ${(left / 1000).toFixed(1)} s`;
  }
}

// ─── Screen shake ─────────────────────────────────────────────────────────────

function triggerScreenShake(magnitude) {
  state.shake = Math.max(state.shake, magnitude);
}

// ─── Ship trail ───────────────────────────────────────────────────────────────

function emitTrailParticle() {
  if (!state.ship.moving) return;
  state.trail.push({
    x:     state.ship.x,
    y:     state.ship.y,
    vx:    (Math.random() - 0.5) * 0.8 - Math.cos(state.ship.angle) * 1.2,
    vy:    (Math.random() - 0.5) * 0.8 - Math.sin(state.ship.angle) * 1.2,
    life:  1,
    r:     Math.random() * 2 + 1,
  });
}

// ─── Draw ─────────────────────────────────────────────────────────────────────

function draw(now) {
  const t = (now - state.start) / 1000;

  // screen-shake offset
  let sx = 0, sy = 0;
  if (state.shake > 0.2) {
    sx = (Math.random() - 0.5) * state.shake * 2;
    sy = (Math.random() - 0.5) * state.shake * 2;
    state.shake *= 0.85;
  } else { state.shake = 0; }

  ctx.save();
  ctx.translate(sx, sy);

  ctx.clearRect(-10, -10, state.width + 20, state.height + 20);
  drawBackdrop(t);
  drawStars(t);
  drawCosmicWeb(t);
  drawTrail();
  drawWormhole(now);
  drawShip(t);

  ctx.restore();

  tickCooldown();
  emitTrailParticle();
  requestAnimationFrame(draw);
}

function drawBackdrop(t) {
  const g = ctx.createRadialGradient(
    state.width * 0.5, state.height * 0.46, 0,
    state.width * 0.5, state.height * 0.5,  Math.max(state.width, state.height) * 0.75,
  );
  g.addColorStop(0,   "#14213f");
  g.addColorStop(0.5, "#060917");
  g.addColorStop(1,   "#01020a");
  ctx.fillStyle = g;
  ctx.fillRect(0, 0, state.width, state.height);

  if (state.deniedPulse > 0.01) {
    ctx.fillStyle = `rgba(255,40,82,${state.deniedPulse * 0.22})`;
    ctx.fillRect(0, 0, state.width, state.height);
    state.deniedPulse *= 0.90;
  }

  // subtle pulsing rings
  ctx.save();
  ctx.globalAlpha = 0.18;
  ctx.strokeStyle = "#2c375f";
  for (let i = 0; i < 9; i++) {
    const r = 120 + i * 95 + Math.sin(t * 0.4 + i) * 5;
    ctx.beginPath();
    ctx.arc(state.width / 2, state.height / 2, r, 0, Math.PI * 2);
    ctx.stroke();
  }
  ctx.restore();
}

function drawStars(t) {
  ctx.save();
  for (const s of state.stars) {
    const a = s.alpha * (0.7 + 0.3 * Math.sin(t * s.speed + s.phase));
    ctx.globalAlpha = a;
    ctx.fillStyle = "#ffffff";
    ctx.beginPath();
    ctx.arc(s.x, s.y, s.r, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.restore();
}

function drawCosmicWeb(t) {
  ctx.save();
  ctx.globalCompositeOperation = "lighter";

  for (let i = 0; i < state.projected.length; i++) {
    const p = state.projected[i];

    // filament lines
    if (i % 7 === 0) {
      const nb   = state.projected[(i + 31) % state.projected.length];
      const dist = Math.hypot(p.x - nb.x, p.y - nb.y);
      if (dist < 115) {
        ctx.globalAlpha = 0.07;
        ctx.strokeStyle = tracerColors[p.obj.tracer] || "#8db6ff";
        ctx.beginPath();
        ctx.moveTo(p.x, p.y);
        ctx.lineTo(nb.x, nb.y);
        ctx.stroke();
      }
    }

    const color   = tracerColors[p.obj.tracer] || "#ffffff";
    const twinkle = 0.7 + Math.sin(t * 2 + i * 0.37) * 0.22;
    ctx.globalAlpha = p.alpha * twinkle;
    ctx.fillStyle   = color;
    ctx.beginPath();
    ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2);
    ctx.fill();
  }

  // selection ring
  if (state.selected) {
    const pt = projectedFor(state.selected);
    ctx.globalAlpha = 0.9;
    ctx.strokeStyle = "#ffffff";
    ctx.lineWidth   = 1.5;
    ctx.beginPath();
    ctx.arc(pt.x, pt.y, 10 + Math.sin(t * 5) * 2, 0, Math.PI * 2);
    ctx.stroke();
    // crosshair ticks
    ctx.globalAlpha = 0.5;
    for (let a = 0; a < 4; a++) {
      const angle = (a / 4) * Math.PI * 2;
      ctx.beginPath();
      ctx.moveTo(pt.x + Math.cos(angle) * 14, pt.y + Math.sin(angle) * 14);
      ctx.lineTo(pt.x + Math.cos(angle) * 20, pt.y + Math.sin(angle) * 20);
      ctx.stroke();
    }
  }
  ctx.restore();
}

function drawTrail() {
  ctx.save();
  ctx.globalCompositeOperation = "lighter";
  state.trail = state.trail.filter(p => p.life > 0.02);
  for (const p of state.trail) {
    p.x    += p.vx;
    p.y    += p.vy;
    p.life *= 0.88;
    ctx.globalAlpha = p.life * 0.6;
    ctx.fillStyle   = `rgba(97,243,255,1)`;
    ctx.beginPath();
    ctx.arc(p.x, p.y, p.r * p.life, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.restore();
}

function drawWormhole(now) {
  if (!state.wormhole) return;
  const age      = now - state.wormhole.born;
  const progress = Math.min(1, age / 900);
  const alpha    = 1 - progress;
  const color    = state.wormhole.allowed ? "97,243,255" : "255,95,125";

  ctx.save();
  ctx.translate(state.wormhole.x, state.wormhole.y);
  ctx.rotate(progress * Math.PI * 4);
  ctx.globalCompositeOperation = "lighter";

  for (let i = 0; i < 5; i++) {
    ctx.globalAlpha = alpha * (0.42 - i * 0.055);
    ctx.strokeStyle = `rgba(${color},1)`;
    ctx.lineWidth   = 2;
    ctx.beginPath();
    ctx.ellipse(0, 0, 14 + i * 12 + progress * 60, 4 + i * 8 + progress * 25, i * 0.8, 0, Math.PI * 2);
    ctx.stroke();
  }

  // flash burst ring on open
  if (progress < 0.2) {
    ctx.globalAlpha = (0.2 - progress) * 4;
    ctx.fillStyle   = `rgba(${color},0.15)`;
    ctx.beginPath();
    ctx.arc(0, 0, 50 * progress, 0, Math.PI * 2);
    ctx.fill();
  }

  ctx.restore();
  if (progress >= 1) state.wormhole = null;
}

function drawShip(t) {
  const ship = state.ship;
  const dx   = ship.targetX - ship.x;
  const dy   = ship.targetY - ship.y;
  const dist = Math.abs(dx) + Math.abs(dy);

  ship.x += dx * 0.035;
  ship.y += dy * 0.035;
  ship.moving = dist > 1;

  if (dist > 0.5) ship.angle = Math.atan2(dy, dx);

  ctx.save();
  ctx.translate(ship.x, ship.y);
  ctx.rotate(ship.angle);
  ctx.globalCompositeOperation = "lighter";

  // engine glow
  const glowR = 3 + Math.sin(t * 8) * 1.5;
  const glow  = ctx.createRadialGradient(-14, 0, 0, -14, 0, glowR * 5);
  glow.addColorStop(0, "rgba(97,243,255,0.7)");
  glow.addColorStop(1, "rgba(97,243,255,0)");
  ctx.fillStyle = glow;
  ctx.beginPath();
  ctx.arc(-14, 0, glowR * 5, 0, Math.PI * 2);
  ctx.fill();

  // exhaust cone
  ctx.globalAlpha = 0.25;
  ctx.fillStyle   = "rgba(97,243,255,1)";
  ctx.beginPath();
  ctx.moveTo(-18, 0);
  ctx.lineTo(-42, -6);
  ctx.lineTo(-42, 6);
  ctx.closePath();
  ctx.fill();

  // hull
  ctx.globalAlpha = 1;
  ctx.fillStyle   = "#eef7ff";
  ctx.beginPath();
  ctx.moveTo(18, 0);
  ctx.lineTo(-12, -10);
  ctx.lineTo(-5, 0);
  ctx.lineTo(-12, 10);
  ctx.closePath();
  ctx.fill();

  ctx.strokeStyle = "#61f3ff";
  ctx.lineWidth   = 1.5;
  ctx.stroke();

  // cockpit dot
  ctx.globalAlpha = 0.9;
  ctx.fillStyle   = "#61f3ff";
  ctx.beginPath();
  ctx.arc(6, 0, 2.5, 0, Math.PI * 2);
  ctx.fill();

  ctx.restore();
}

// ─── UI helpers ───────────────────────────────────────────────────────────────

function projectedFor(obj) {
  return state.projected.find(p => p.obj.targetId === obj.targetId)
    || { x: state.width / 2, y: state.height / 2 };
}

function updateTarget(obj) {
  els.targetName.textContent     = `DESI ${obj.targetId.slice(-8)}`;
  els.targetTracer.textContent   = obj.tracer;
  els.targetRedshift.textContent = obj.redshift.toFixed(5);
  els.targetRosette.textContent  = String(obj.rosette);
  const dist = Math.hypot(obj.x, obj.y, obj.z);
  els.targetDistance.textContent = `${dist.toFixed(2)} Glyr`;
}

function addEvent(type, title, detail) {
  state.events.unshift({ type, title, detail });
  state.events = state.events.slice(0, 12);
  renderLog();
}

function renderLog() {
  els.log.replaceChildren();
  for (const ev of state.events) {
    const li     = document.createElement("li");
    li.className = ev.type;
    const strong = document.createElement("strong");
    strong.textContent = ev.title;
    li.appendChild(strong);
    li.appendChild(document.createTextNode(ev.detail));
    els.log.appendChild(li);
  }
}

boot();

// app.js — Load Balancer interactive tutorial UI
// Vanilla JS, no frameworks, no innerHTML with API data.

/* ── State ──────────────────────────────────────────────────────── */
const state = {
  service: 'web',
  algorithm: 'round_robin',
  backends: [],      // {url, status, weight, active_conns, total_conns, latency_ewma_ms}
  packets: [],       // animated request packets
  rrPointer: 0,      // local round-robin pointer for animation hint
  autoMode: false,
  autoTimer: null,
  healthHistory: [], // [{backend_url, status, latency_ms}]
};

/* ── DOM refs ───────────────────────────────────────────────────── */
const svcInput    = document.getElementById('svc-input');
const beUrlInput  = document.getElementById('be-url');
const beWeightInput = document.getElementById('be-weight');
const backendList = document.getElementById('backend-list');
const logList     = document.getElementById('log-list');
const statsBody   = document.getElementById('stats-body');
const stepBox     = document.getElementById('step-box');
const algoExplain = document.getElementById('algo-explain');
const btnAdd      = document.getElementById('btn-add');
const btnSingle   = document.getElementById('btn-single');
const btnAuto     = document.getElementById('btn-auto');
const btnKill     = document.getElementById('btn-kill');
const btnRevive   = document.getElementById('btn-revive');
const btnFlood    = document.getElementById('btn-flood');
const canvas      = document.getElementById('viz-canvas');
const ctx         = canvas.getContext('2d');
const healthChart = document.getElementById('health-chart');
const hctx        = healthChart.getContext('2d');

/* ── Canvas resize ──────────────────────────────────────────────── */
function resizeCanvas() {
  const panel = document.getElementById('viz-panel');
  const titleH = panel.querySelector('.panel-title').offsetHeight;
  canvas.width  = panel.offsetWidth;
  canvas.height = panel.offsetHeight - titleH;
}
window.addEventListener('resize', () => { resizeCanvas(); });
resizeCanvas();

/* ── Algorithm explanations ─────────────────────────────────────── */
// Each entry: [boldLabel, plainDescription]
const ALGO_TEXT = {
  round_robin:        ['Round Robin', '— requests are distributed sequentially across backends (1→2→3→1…). Simple and fair when all backends have equal capacity.'],
  least_connections:  ['Least Connections', '— each new request goes to the backend with the fewest active connections + lowest latency score. Best when requests vary in duration.'],
  weighted_round_robin: ['Weighted Round Robin', '— backends with higher weights receive proportionally more requests. Use when backends have different capacities (e.g. weight 3 vs 1 gets 3× the traffic).'],
  random:             ['Random', '— each request is sent to a uniformly random healthy backend. Statistically equivalent to round-robin at scale, simpler to implement in distributed systems.'],
};

function updateAlgoExplain() {
  const entry = ALGO_TEXT[state.algorithm];
  if (!entry) { algoExplain.replaceChildren(); return; }
  const b = document.createElement('b');
  b.textContent = entry[0];
  const rest = document.createTextNode(' ' + entry[1]);
  algoExplain.replaceChildren(b, rest);
}
updateAlgoExplain();

/* ── API helpers ────────────────────────────────────────────────── */
async function apiFetch(path, opts = {}) {
  const res = await fetch(path, opts);
  const text = await res.text();
  try { return { ok: res.ok, status: res.status, data: JSON.parse(text) }; }
  catch { return { ok: res.ok, status: res.status, data: text }; }
}

/* ── Add backend ────────────────────────────────────────────────── */
btnAdd.addEventListener('click', async () => {
  const svc    = svcInput.value.trim() || 'web';
  const url    = beUrlInput.value.trim();
  const weight = parseInt(beWeightInput.value, 10) || 1;
  if (!url) return;
  state.service = svc;

  const r = await apiFetch(`/v1/backends/${encodeURIComponent(svc)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url, weight }),
  });
  appendLog(svc, url, r.ok ? 'registered' : 'error', r.ok ? 'ok' : 'err', null);
  await refreshStats();
});

/* ── Algorithm pills ────────────────────────────────────────────── */
document.querySelectorAll('.algo-pill').forEach(pill => {
  pill.addEventListener('click', async () => {
    document.querySelectorAll('.algo-pill').forEach(p => p.classList.remove('active'));
    pill.classList.add('active');
    state.algorithm = pill.dataset.algo;
    updateAlgoExplain();

    const svc = svcInput.value.trim() || 'web';
    await apiFetch(`/v1/backends/${encodeURIComponent(svc)}/algorithm`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ algorithm: state.algorithm }),
    });
  });
});

/* ── Send single request (proxy demo) ───────────────────────────── */
btnSingle.addEventListener('click', () => sendProxyRequest());

async function sendProxyRequest() {
  const svc = svcInput.value.trim() || 'web';
  const healthy = state.backends.filter(b => b.status === 'healthy');
  if (healthy.length === 0) {
    setStep(['No healthy backends available — add a backend first.']);
    return;
  }

  // Pick target via our local algorithm mirror for animation
  const target = pickLocal(healthy);
  if (!target) return;

  // Spawn a packet animation
  spawnPacket(target.url);
  setStep(['Routing to ', { strong: shortURL(target.url) }, ' via ' + state.algorithm.replace(/_/g, ' ')]);

  const r = await apiFetch(`/proxy/${encodeURIComponent(svc)}/get`);
  appendLog(svc, target.url, r.status, r.status < 400 ? 'ok' : 'err', null);
  await refreshStats();
  await refreshHealthHistory(svc);
}

function pickLocal(healthy) {
  if (healthy.length === 0) return null;
  if (state.algorithm === 'least_connections') {
    return healthy.reduce((a, b) =>
      (a.active_conns + a.latency_ewma_ms / 1000) <= (b.active_conns + b.latency_ewma_ms / 1000) ? a : b
    );
  }
  if (state.algorithm === 'weighted_round_robin') {
    const total = healthy.reduce((s, b) => s + (b.weight || 1), 0);
    let r = (state.rrPointer++ % total);
    for (const b of healthy) {
      r -= (b.weight || 1);
      if (r < 0) return b;
    }
  }
  // round_robin / random
  const idx = state.rrPointer++ % healthy.length;
  return healthy[idx];
}

/* ── Auto mode ──────────────────────────────────────────────────── */
btnAuto.addEventListener('click', () => {
  state.autoMode = !state.autoMode;
  btnAuto.textContent = state.autoMode ? 'Auto: ON' : 'Auto: OFF';
  if (state.autoMode) {
    state.autoTimer = setInterval(sendProxyRequest, 800);
  } else {
    clearInterval(state.autoTimer);
  }
});

/* ── Failure injection ──────────────────────────────────────────── */
btnKill.addEventListener('click', async () => {
  const healthy = state.backends.filter(b => b.status === 'healthy');
  if (healthy.length === 0) return;
  const victim = healthy[Math.floor(Math.random() * healthy.length)];
  const svc = svcInput.value.trim() || 'web';
  // We can't truly make a backend fail (it's external), but we can remove it.
  await apiFetch(`/v1/backends/${encodeURIComponent(svc)}/${encodeURIComponent(victim.url)}`, {
    method: 'DELETE',
  });
  setStep(['Killed ', { strong: shortURL(victim.url) }, ' — watch the balancer reroute.']);
  appendLog(svc, victim.url, 'killed', 'err', null);
  await refreshStats();
});

btnRevive.addEventListener('click', async () => {
  const svc = svcInput.value.trim() || 'web';
  const all = [...state.backends];
  for (const b of all) {
    await apiFetch(`/v1/backends/${encodeURIComponent(svc)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: b.url, weight: b.weight || 1 }),
    });
  }
  setStep(['All backends re-registered.']);
  await refreshStats();
});

btnFlood.addEventListener('click', async () => {
  setStep(['Flooding with 10 concurrent requests…']);
  const svc = svcInput.value.trim() || 'web';
  const promises = Array.from({ length: 10 }, () =>
    apiFetch(`/proxy/${encodeURIComponent(svc)}/get`)
  );
  const results = await Promise.all(promises);
  results.forEach(r => appendLog(svc, '(flood)', r.status, r.status < 400 ? 'ok' : 'err', null));
  await refreshStats();
  setStep(['Flood done — check the stats table for distribution.']);
});

/* ── Stats refresh ──────────────────────────────────────────────── */
async function refreshStats() {
  const r = await apiFetch('/v1/stats');
  if (!r.ok || !Array.isArray(r.data)) return;

  const svc = svcInput.value.trim() || 'web';
  const svcView = r.data.find(s => s.service === svc);
  state.backends = svcView ? (svcView.backends || []) : [];

  renderBackendList();
  renderStatsTable();
}

function renderBackendList() {
  const frag = document.createDocumentFragment();
  for (const b of state.backends) {
    const chip = document.createElement('div');
    chip.className = 'backend-chip';

    const dot = document.createElement('span');
    dot.className = 'dot dot-' + (b.status || 'healthy');

    const urlSpan = document.createElement('span');
    urlSpan.className = 'url';
    urlSpan.textContent = shortURL(b.url);

    const conns = document.createElement('span');
    conns.className = 'conns';
    conns.textContent = b.active_conns + 'c';

    const lat = document.createElement('span');
    lat.className = 'latency';
    lat.textContent = Math.round(b.latency_ewma_ms || 0) + 'ms';

    const rm = document.createElement('button');
    rm.className = 'chip-remove';
    rm.textContent = '×';
    rm.addEventListener('click', () => removeBackend(b.url));

    chip.append(dot, urlSpan, conns, lat, rm);
    frag.appendChild(chip);
  }
  backendList.replaceChildren(frag);
}

function renderStatsTable() {
  const frag = document.createDocumentFragment();
  for (const b of state.backends) {
    const tr = document.createElement('tr');
    const td = (txt, cls) => {
      const t = document.createElement('td');
      if (cls) t.className = cls;
      t.textContent = txt;
      return t;
    };
    tr.append(
      td(shortURL(b.url)),
      td(b.active_conns),
      td(b.total_conns),
      td(Math.round(b.latency_ewma_ms || 0)),
    );
    frag.appendChild(tr);
  }
  statsBody.replaceChildren(frag);
}

async function removeBackend(url) {
  const svc = svcInput.value.trim() || 'web';
  await apiFetch(`/v1/backends/${encodeURIComponent(svc)}/${encodeURIComponent(url)}`, {
    method: 'DELETE',
  });
  appendLog(svc, url, 'removed', 'err', null);
  await refreshStats();
}

/* ── Health history ─────────────────────────────────────────────── */
async function refreshHealthHistory(svc) {
  const r = await apiFetch(`/v1/backends/${encodeURIComponent(svc)}/health?limit=30`);
  if (!r.ok || !Array.isArray(r.data)) return;
  state.healthHistory = r.data;
  drawHealthChart();
}

function drawHealthChart() {
  const W = healthChart.offsetWidth;
  const H = 40;
  healthChart.width  = W;
  healthChart.height = H;
  hctx.clearRect(0, 0, W, H);

  const rows = state.healthHistory.slice(0, 30).reverse();
  if (rows.length === 0) return;
  const barW = W / rows.length;

  rows.forEach((row, i) => {
    hctx.fillStyle = row.status === 'healthy' ? '#3fb950' : '#f85149';
    const h = Math.min(H, Math.max(4, (row.latency_ms || 20) / 3));
    hctx.fillRect(i * barW + 1, H - h, barW - 2, h);
  });
}

/* ── Log ─────────────────────────────────────────────────────────── */
function appendLog(svc, be, detail, kind, _unused) {
  const entry = document.createElement('div');
  entry.className = 'log-entry';

  const ts = document.createElement('span');
  ts.className = 'log-ts';
  ts.textContent = new Date().toTimeString().slice(0, 8);

  const svcEl = document.createElement('span');
  svcEl.className = 'log-svc';
  svcEl.textContent = svc;

  const beEl = document.createElement('span');
  beEl.className = 'log-be';
  beEl.textContent = shortURL(String(be));

  const detailEl = document.createElement('span');
  detailEl.className = kind === 'ok' ? 'log-ok' : 'log-err';
  detailEl.textContent = String(detail);

  entry.append(ts, svcEl, beEl, detailEl);
  logList.insertBefore(entry, logList.firstChild);
  if (logList.children.length > 80) logList.lastChild.remove();
}

/* ── Canvas animation ───────────────────────────────────────────── */
// Packet: animated circle flying from client → LB → backend → back.
// Each packet has a phase: 0=client→LB, 1=LB→backend, 2=backend→LB, 3=LB→client.

function spawnPacket(backendURL) {
  const id = Math.random();
  state.packets.push({ id, backendURL, phase: 0, t: 0, color: '#58a6ff' });
}

function getClientPos() {
  return { x: 60, y: canvas.height / 2 };
}

function getLBPos() {
  return { x: canvas.width / 2, y: canvas.height / 2 };
}

function getBackendPos(index, total) {
  const spread = Math.min(canvas.height * 0.7, total * 70);
  const startY = canvas.height / 2 - spread / 2 + spread / (2 * total);
  return {
    x: canvas.width - 70,
    y: startY + index * (spread / total),
  };
}

function lerp(a, b, t) { return a + (b - a) * t; }

function drawFrame() {
  ctx.clearRect(0, 0, canvas.width, canvas.height);

  const lbPos = getLBPos();
  const clientPos = getClientPos();
  const backends = state.backends;

  // Draw client node
  drawNode(clientPos.x, clientPos.y, 32, 'CLIENT', '#58a6ff', null);

  // Draw load balancer node
  drawNode(lbPos.x, lbPos.y, 40, 'LOAD\nBALANCER', '#bc8cff', state.algorithm.replace(/_/g, ' ').toUpperCase());

  // Draw connection line: client → LB
  ctx.setLineDash([4, 4]);
  ctx.strokeStyle = 'rgba(88,166,255,.25)';
  ctx.lineWidth = 1.5;
  ctx.beginPath();
  ctx.moveTo(clientPos.x + 32, clientPos.y);
  ctx.lineTo(lbPos.x - 40, lbPos.y);
  ctx.stroke();
  ctx.setLineDash([]);

  // Draw backend nodes + connection lines
  backends.forEach((b, i) => {
    const pos = getBackendPos(i, backends.length);
    const color = b.status === 'healthy' ? '#3fb950' : '#f85149';

    ctx.setLineDash([4, 4]);
    ctx.strokeStyle = b.status === 'healthy' ? 'rgba(63,185,80,.2)' : 'rgba(248,81,73,.2)';
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.moveTo(lbPos.x + 40, lbPos.y);
    ctx.lineTo(pos.x - 30, pos.y);
    ctx.stroke();
    ctx.setLineDash([]);

    const label = shortURL(b.url);
    const sub = (b.active_conns || 0) + ' conn · ' + Math.round(b.latency_ewma_ms || 0) + 'ms';
    drawNode(pos.x, pos.y, 30, label, color, sub);

    // Active connection pulse ring
    if ((b.active_conns || 0) > 0) {
      const pulse = (Date.now() % 1200) / 1200;
      ctx.beginPath();
      ctx.arc(pos.x, pos.y, 30 + pulse * 20, 0, Math.PI * 2);
      ctx.strokeStyle = `rgba(63,185,80,${0.5 - pulse * 0.5})`;
      ctx.lineWidth = 1.5;
      ctx.stroke();
    }
  });

  // Update and draw packets
  const speed = 0.04;
  state.packets = state.packets.filter(p => p.phase < 4);
  for (const p of state.packets) {
    p.t += speed;
    if (p.t >= 1) { p.t = 0; p.phase++; }

    const beIdx = backends.findIndex(b => b.url === p.backendURL);
    const bePos = beIdx >= 0 ? getBackendPos(beIdx, backends.length) : getLBPos();
    const t = easeInOut(p.t);

    let px, py;
    if (p.phase === 0) {        // client → LB
      px = lerp(clientPos.x + 32, lbPos.x - 40, t);
      py = lerp(clientPos.y, lbPos.y, t);
      p.color = '#58a6ff';
    } else if (p.phase === 1) { // LB → backend
      px = lerp(lbPos.x + 40, bePos.x - 30, t);
      py = lerp(lbPos.y, bePos.y, t);
      p.color = '#58a6ff';
    } else if (p.phase === 2) { // backend → LB (response)
      px = lerp(bePos.x - 30, lbPos.x + 40, t);
      py = lerp(bePos.y, lbPos.y, t);
      p.color = '#3fb950';
    } else if (p.phase === 3) { // LB → client
      px = lerp(lbPos.x - 40, clientPos.x + 32, t);
      py = lerp(lbPos.y, clientPos.y, t);
      p.color = '#3fb950';
    } else continue;

    drawPacket(px, py, p.color);
  }

  requestAnimationFrame(drawFrame);
}

function easeInOut(t) {
  return t < 0.5 ? 2 * t * t : -1 + (4 - 2 * t) * t;
}

function drawNode(x, y, r, label, color, sub) {
  // Glow
  const grad = ctx.createRadialGradient(x, y, r * 0.4, x, y, r * 1.5);
  grad.addColorStop(0, color + '22');
  grad.addColorStop(1, 'transparent');
  ctx.beginPath();
  ctx.arc(x, y, r * 1.5, 0, Math.PI * 2);
  ctx.fillStyle = grad;
  ctx.fill();

  // Node circle
  ctx.beginPath();
  ctx.arc(x, y, r, 0, Math.PI * 2);
  ctx.fillStyle = '#161b22';
  ctx.fill();
  ctx.strokeStyle = color;
  ctx.lineWidth = 2;
  ctx.stroke();

  // Label (multiline)
  ctx.fillStyle = color;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.font = 'bold 9px JetBrains Mono, monospace';
  const lines = label.split('\n');
  const lineH = 11;
  lines.forEach((line, i) => {
    ctx.fillText(line, x, y + (i - (lines.length - 1) / 2) * lineH);
  });

  // Sub-label below node
  if (sub) {
    ctx.font = '8px JetBrains Mono, monospace';
    ctx.fillStyle = '#8b949e';
    ctx.fillText(sub, x, y + r + 10);
  }
}

function drawPacket(x, y, color) {
  ctx.beginPath();
  ctx.arc(x, y, 5, 0, Math.PI * 2);
  ctx.fillStyle = color;
  ctx.shadowColor = color;
  ctx.shadowBlur = 8;
  ctx.fill();
  ctx.shadowBlur = 0;
}

/* ── Step box helper ────────────────────────────────────────────── */
// setStep accepts a token list: alternating plain strings and {strong: text} objects.
// Example: setStep(['Routing to ', {strong: 'http://x'}, ' via round robin'])
// This avoids innerHTML entirely while still allowing bold highlights.
function setStep(parts) {
  if (typeof parts === 'string') parts = [parts];
  const frag = document.createDocumentFragment();
  for (const part of parts) {
    if (typeof part === 'string') {
      frag.appendChild(document.createTextNode(part));
    } else if (part && typeof part.strong === 'string') {
      const s = document.createElement('strong');
      s.textContent = part.strong;
      frag.appendChild(s);
    }
  }
  stepBox.replaceChildren(frag);
}

/* ── Utilities ──────────────────────────────────────────────────── */
function shortURL(url) {
  try {
    const u = new URL(url);
    return u.host;
  } catch {
    return url.length > 24 ? url.slice(0, 22) + '…' : url;
  }
}

/* ── Periodic stats poll ────────────────────────────────────────── */
setInterval(refreshStats, 3000);
setInterval(() => {
  const svc = svcInput.value.trim() || 'web';
  refreshHealthHistory(svc);
}, 5000);

/* ── Boot ───────────────────────────────────────────────────────── */
refreshStats();
requestAnimationFrame(drawFrame);

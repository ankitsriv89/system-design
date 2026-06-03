'use strict';

// ── API helpers ───────────────────────────────────────────────────────────────

const BASE = '';

async function apiGet(path) {
  const r = await fetch(BASE + path);
  return { ok: r.ok, status: r.status, data: await r.json().catch(() => null) };
}

async function apiPut(path, body) {
  const r = await fetch(BASE + path, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  return { ok: r.ok, status: r.status, data: await r.json().catch(() => null) };
}

async function apiDelete(path) {
  const r = await fetch(BASE + path, { method: 'DELETE' });
  return { ok: r.ok, status: r.status, data: await r.json().catch(() => null) };
}

// ── Log ───────────────────────────────────────────────────────────────────────

const logEl = document.getElementById('api-log');

function log(type, parts) {
  const entry = document.createElement('div');
  entry.className = 'log-entry ' + type;

  const ts = document.createElement('span');
  ts.className = 'log-ts';
  ts.textContent = new Date().toISOString().slice(11, 23);
  entry.appendChild(ts);

  parts.forEach(p => {
    const span = document.createElement('span');
    span.className = p.cls || '';
    span.textContent = p.text;
    entry.appendChild(span);
  });

  logEl.prepend(entry);
  while (logEl.children.length > 80) logEl.removeChild(logEl.lastChild);
}

document.getElementById('btn-clear-log').addEventListener('click', () => {
  while (logEl.firstChild) logEl.removeChild(logEl.firstChild);
});

// ── Stats & gauge ─────────────────────────────────────────────────────────────

let maxBytes = 0;
let lastStats = null;

async function refreshStats() {
  const { ok, data } = await apiGet('/v1/stats');
  if (!ok || !data) return;
  lastStats = data;

  document.getElementById('stat-keys').textContent = data.keys;
  document.getElementById('stat-hitrate').textContent = (data.hit_rate * 100).toFixed(1) + '%';
  document.getElementById('stat-hits').textContent = data.hits;
  document.getElementById('stat-misses').textContent = data.misses;
  document.getElementById('stat-evictions').textContent = data.evictions;
  document.getElementById('stat-memory').textContent = fmtBytes(data.memory_bytes);
  document.getElementById('current-policy').textContent = data.policy.toUpperCase();

  drawGauge(data.hit_rate);
  updateMemBar(data.memory_bytes, data.max_bytes);
  if (data.max_bytes > 0) maxBytes = data.max_bytes;
}

function fmtBytes(n) {
  if (n < 1024) return n + ' B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB';
  return (n / 1024 / 1024).toFixed(2) + ' MB';
}

function updateMemBar(used, max) {
  const fill = document.getElementById('mem-bar-fill');
  const usedLbl = document.getElementById('mem-used-label');
  const maxLbl = document.getElementById('mem-max-label');
  usedLbl.textContent = fmtBytes(used);
  if (max <= 0) { maxLbl.textContent = 'unlimited'; fill.style.width = '0%'; return; }
  maxLbl.textContent = fmtBytes(max);
  const pct = Math.min(100, (used / max) * 100);
  fill.style.width = pct + '%';
  fill.classList.toggle('danger', pct > 80);
}

// ── Hit-rate arc gauge ────────────────────────────────────────────────────────

const gaugeCanvas = document.getElementById('gauge-canvas');
const gaugeCtx = gaugeCanvas.getContext('2d');
let currentGaugeRate = 0;

function drawGauge(rate) {
  const target = rate;
  const step = () => {
    currentGaugeRate += (target - currentGaugeRate) * 0.15;
    renderGauge(currentGaugeRate);
    if (Math.abs(target - currentGaugeRate) > 0.001) requestAnimationFrame(step);
  };
  requestAnimationFrame(step);
}

function renderGauge(rate) {
  const w = gaugeCanvas.width, h = gaugeCanvas.height;
  const cx = w / 2, cy = h - 10;
  const r = 100;
  const startAngle = Math.PI, endAngle = 2 * Math.PI;

  gaugeCtx.clearRect(0, 0, w, h);

  // background arc
  gaugeCtx.beginPath();
  gaugeCtx.arc(cx, cy, r, startAngle, endAngle);
  gaugeCtx.strokeStyle = '#30363d';
  gaugeCtx.lineWidth = 16;
  gaugeCtx.lineCap = 'round';
  gaugeCtx.stroke();

  // value arc
  const fillEnd = startAngle + rate * Math.PI;
  const grad = gaugeCtx.createLinearGradient(cx - r, cy, cx + r, cy);
  grad.addColorStop(0, '#f85149');
  grad.addColorStop(0.5, '#d29922');
  grad.addColorStop(1, '#3fb950');
  gaugeCtx.beginPath();
  gaugeCtx.arc(cx, cy, r, startAngle, fillEnd);
  gaugeCtx.strokeStyle = grad;
  gaugeCtx.lineWidth = 16;
  gaugeCtx.lineCap = 'round';
  gaugeCtx.stroke();

  // needle
  const angle = startAngle + rate * Math.PI;
  const nx = cx + (r - 8) * Math.cos(angle);
  const ny = cy + (r - 8) * Math.sin(angle);
  gaugeCtx.beginPath();
  gaugeCtx.moveTo(cx, cy);
  gaugeCtx.lineTo(nx, ny);
  gaugeCtx.strokeStyle = '#c9d1d9';
  gaugeCtx.lineWidth = 2;
  gaugeCtx.stroke();

  // centre dot
  gaugeCtx.beginPath();
  gaugeCtx.arc(cx, cy, 5, 0, 2 * Math.PI);
  gaugeCtx.fillStyle = '#c9d1d9';
  gaugeCtx.fill();

  // text
  gaugeCtx.fillStyle = '#c9d1d9';
  gaugeCtx.font = 'bold 26px monospace';
  gaugeCtx.textAlign = 'center';
  gaugeCtx.fillText((rate * 100).toFixed(1) + '%', cx, cy - 18);
}

// ── Eviction-order track ──────────────────────────────────────────────────────

const evictionTrack = document.getElementById('eviction-track');

async function refreshEntries() {
  const { ok, data } = await apiGet('/v1/entries');
  if (!ok || !Array.isArray(data)) return;

  // sort: LRU order = oldest last_access first; show up to 20 nodes
  data.sort((a, b) => new Date(a.last_access) - new Date(b.last_access));
  const nodes = data.slice(0, 20);

  // rebuild nodes without innerHTML
  while (evictionTrack.firstChild) evictionTrack.removeChild(evictionTrack.firstChild);

  nodes.forEach((e, i) => {
    const div = document.createElement('div');
    div.className = 'eviction-node' + (i === 0 ? ' victim' : i === nodes.length - 1 ? ' hot' : '');
    div.title = `key=${e.key} accesses=${e.access_count}`;
    div.textContent = e.key;
    evictionTrack.appendChild(div);
  });
}

// ── Request-flow animation ────────────────────────────────────────────────────

const flowCanvas = document.getElementById('flow-canvas');
const flowCtx = flowCanvas.getContext('2d');

// particle: { x, y, vx, label, color, age, maxAge }
const particles = [];

function addFlowParticle(label, color) {
  particles.push({ x: 10, y: 60, vx: 3.5, label, color, age: 0, maxAge: 140 });
}

function drawFlowFrame() {
  const w = flowCanvas.width, h = flowCanvas.height;
  flowCtx.clearRect(0, 0, w, h);

  // static boxes: Client → Cache → Source
  const boxes = [
    { x: 10,  y: 20, w: 80, h: 36, label: 'Client',  color: '#58a6ff' },
    { x: 210, y: 20, w: 80, h: 36, label: 'Cache',   color: '#bc8cff' },
    { x: 410, y: 20, w: 80, h: 36, label: 'Source',  color: '#d29922' },
  ];

  boxes.forEach(b => {
    flowCtx.strokeStyle = b.color;
    flowCtx.lineWidth = 1.5;
    flowCtx.strokeRect(b.x, b.y, b.w, b.h);
    flowCtx.fillStyle = b.color;
    flowCtx.font = '11px monospace';
    flowCtx.textAlign = 'center';
    flowCtx.fillText(b.label, b.x + b.w / 2, b.y + 23);
  });

  // arrows between boxes
  flowCtx.strokeStyle = '#30363d';
  flowCtx.lineWidth = 1;
  [[90, 210], [290, 410]].forEach(([x1, x2]) => {
    flowCtx.beginPath();
    flowCtx.moveTo(x1, 38);
    flowCtx.lineTo(x2, 38);
    flowCtx.stroke();
    // arrowhead
    flowCtx.beginPath();
    flowCtx.moveTo(x2, 33);
    flowCtx.lineTo(x2 + 8, 38);
    flowCtx.lineTo(x2, 43);
    flowCtx.stroke();
  });

  // move & draw particles
  for (let i = particles.length - 1; i >= 0; i--) {
    const p = particles[i];
    p.x += p.vx;
    p.age++;
    const alpha = 1 - p.age / p.maxAge;
    flowCtx.globalAlpha = alpha;
    flowCtx.beginPath();
    flowCtx.arc(p.x, p.y, 5, 0, 2 * Math.PI);
    flowCtx.fillStyle = p.color;
    flowCtx.fill();
    flowCtx.globalAlpha = 1;
    flowCtx.fillStyle = p.color;
    flowCtx.font = '9px monospace';
    flowCtx.textAlign = 'center';
    flowCtx.fillText(p.label, p.x, p.y - 9);
    if (p.age >= p.maxAge) particles.splice(i, 1);
  }

  requestAnimationFrame(drawFlowFrame);
}

requestAnimationFrame(drawFlowFrame);

// ── Operations ────────────────────────────────────────────────────────────────

document.getElementById('btn-set').addEventListener('click', async () => {
  const key = document.getElementById('set-key').value.trim();
  const value = document.getElementById('set-value').value.trim();
  const ttlMs = parseInt(document.getElementById('set-ttl').value, 10) || 0;

  if (!key || !value) { log('error', [{ cls: 'log-op', text: 'SET' }, { text: ' — key and value required' }]); return; }

  const { ok, data } = await apiPut('/v1/cache/' + encodeURIComponent(key), { value, ttl_ms: ttlMs });
  addFlowParticle('SET', '#58a6ff');
  if (ok) {
    log('set', [
      { cls: 'log-op', text: 'SET' },
      { cls: 'log-key', text: ' ' + key },
      { text: ' = ' },
      { cls: 'log-val', text: value },
      ttlMs ? { text: ' ttl=' + ttlMs + 'ms' } : { text: '' },
    ]);
  } else {
    log('error', [{ cls: 'log-op', text: 'SET ERR' }, { text: data?.error || '' }]);
  }
  refresh();
});

document.getElementById('btn-get').addEventListener('click', async () => {
  const key = document.getElementById('get-key').value.trim();
  if (!key) return;

  const { ok, data } = await apiGet('/v1/cache/' + encodeURIComponent(key));
  if (ok) {
    addFlowParticle('HIT', '#3fb950');
    log('hit', [
      { cls: 'log-op', text: 'GET' },
      { cls: 'log-key', text: ' ' + key },
      { text: ' → ' },
      { cls: 'log-val', text: data.value },
      { cls: 'log-badge hit-badge', text: 'HIT' },
    ]);
  } else {
    addFlowParticle('MISS', '#f85149');
    log('miss', [
      { cls: 'log-op', text: 'GET' },
      { cls: 'log-key', text: ' ' + key },
      { cls: 'log-badge miss-badge', text: 'MISS' },
    ]);
  }
  refresh();
});

document.getElementById('btn-delete').addEventListener('click', async () => {
  const key = document.getElementById('get-key').value.trim();
  if (!key) return;
  const { ok, data } = await apiDelete('/v1/cache/' + encodeURIComponent(key));
  if (ok) {
    log('del', [{ cls: 'log-op', text: 'DEL' }, { cls: 'log-key', text: ' ' + key }]);
  } else {
    log('error', [{ cls: 'log-op', text: 'DEL ERR' }, { text: ' ' + (data?.error || '') }]);
  }
  refresh();
});

document.getElementById('btn-flush').addEventListener('click', async () => {
  if (!confirm('Flush ALL cache entries?')) return;
  const { ok, data } = await apiDelete('/v1/cache');
  if (ok) {
    log('flush', [{ cls: 'log-op', text: 'FLUSH' }, { text: ' flushed=' + data.flushed }]);
  }
  refresh();
});

document.getElementById('btn-seed').addEventListener('click', async () => {
  const words = ['alpha','beta','gamma','delta','epsilon','zeta','eta','theta','iota','kappa',
                 'lambda','mu','nu','xi','omicron','pi','rho','sigma','tau','upsilon'];
  let count = 0;
  for (const w of words) {
    const ttl = Math.random() > 0.5 ? Math.floor(Math.random() * 30000 + 5000) : 0;
    const { ok } = await apiPut('/v1/cache/' + w, { value: 'val-' + w, ttl_ms: ttl });
    if (ok) count++;
  }
  log('info', [{ cls: 'log-op', text: 'SEED' }, { text: ' inserted ' + count + ' keys' }]);
  refresh();
});

// ── Stampede demo ─────────────────────────────────────────────────────────────

document.getElementById('btn-stampede').addEventListener('click', async () => {
  const key = document.getElementById('stampede-key').value.trim() || 'stampede-key';
  const resultEl = document.getElementById('stampede-result');

  // delete the key first so all requests miss
  await apiDelete('/v1/cache/' + encodeURIComponent(key));

  const hits = { before: 0, after: 0 };
  const start = Date.now();

  // fire 40 concurrent GETs — they all miss, but singleflight on the backend
  // means the loader (simulated by a subsequent SET from the client) fires once
  const statsBefore = (await apiGet('/v1/stats')).data;
  const promises = [];
  for (let i = 0; i < 40; i++) {
    promises.push(
      fetch(BASE + '/v1/cache/' + encodeURIComponent(key))
        .then(r => r.json())
        .then(d => { if (d.value) hits.after++; })
        .catch(() => {})
    );
  }
  await Promise.all(promises);
  const elapsed = Date.now() - start;
  const statsAfter = (await apiGet('/v1/stats')).data;

  // populate from server we can't count server-side loader calls client-side,
  // so we show the miss delta (should be 1 "group" not 40 individual loads)
  const missDelta = (statsAfter?.misses || 0) - (statsBefore?.misses || 0);

  resultEl.classList.remove('hidden');
  // build result DOM safely
  while (resultEl.firstChild) resultEl.removeChild(resultEl.firstChild);

  const title = document.createElement('div');
  title.textContent = '40 concurrent GETs fired in ' + elapsed + 'ms';
  resultEl.appendChild(title);

  const callsEl = document.createElement('div');
  callsEl.textContent = 'Miss events recorded: ';
  const calls = document.createElement('span');
  calls.className = 'loader-calls';
  calls.textContent = String(missDelta);
  callsEl.appendChild(calls);
  resultEl.appendChild(callsEl);

  const note = document.createElement('div');
  note.style.color = '#8b949e';
  note.style.fontSize = '10px';
  note.textContent = 'singleflight coalesces concurrent misses — loader called once per group, not 40×';
  resultEl.appendChild(note);

  log('info', [
    { cls: 'log-op', text: 'STAMPEDE' },
    { text: ' 40 concurrent GETs, miss-delta=' + missDelta + ', ' + elapsed + 'ms' },
  ]);
  refresh();
});

// ── Periodic refresh ──────────────────────────────────────────────────────────

function refresh() {
  refreshStats();
  refreshEntries();
}

refresh();
setInterval(refresh, 2000);

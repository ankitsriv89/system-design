'use strict';

// ---- State ---------------------------------------------------------------
const state = {
  total: 0, allowed: 0, blocked: 0, latencies: [],
  particles: [],   // animated request particles on canvas
  routes: [],
};

// ---- Canvas setup --------------------------------------------------------
const canvas = document.getElementById('gatewayCanvas');
const ctx = canvas.getContext('2d');
const W = canvas.width;
const H = canvas.height;

// Node positions (logical layout for the animation)
const NODES = {
  client:  { x: 60,  y: H / 2, label: 'Client',  color: '#58a6ff' },
  gateway: { x: W/2, y: H / 2, label: 'Gateway',  color: '#d29922' },
  upstream:{ x: W - 60, y: H / 2, label: 'Upstream', color: '#3fb950' },
  redis:   { x: W/2, y: 80,    label: 'Redis',    color: '#f0883e' },
  pg:      { x: W/2, y: H-80,  label: 'Postgres', color: '#58a6ff' },
};

const NODE_R = 38;

// ---- Draw frame ----------------------------------------------------------
function drawFrame() {
  ctx.clearRect(0, 0, W, H);

  // Background lines between nodes
  drawLine(NODES.client, NODES.gateway, '#30363d');
  drawLine(NODES.gateway, NODES.upstream, '#30363d');
  drawLine(NODES.gateway, NODES.redis, '#30363d');
  drawLine(NODES.gateway, NODES.pg, '#30363d');

  // Draw nodes
  for (const n of Object.values(NODES)) drawNode(n);

  // Draw particles
  const alive = [];
  for (const p of state.particles) {
    p.t += p.speed;
    if (p.t > 1) continue; // expired
    const nx = lerp(p.from.x, p.to.x, p.t);
    const ny = lerp(p.from.y, p.to.y, p.t);
    ctx.beginPath();
    ctx.arc(nx, ny, 5, 0, Math.PI * 2);
    ctx.fillStyle = p.color;
    ctx.fill();
    alive.push(p);
  }
  state.particles = alive;

  requestAnimationFrame(drawFrame);
}

function drawNode(n) {
  ctx.beginPath();
  ctx.arc(n.x, n.y, NODE_R, 0, Math.PI * 2);
  ctx.strokeStyle = n.color;
  ctx.lineWidth = 2;
  ctx.stroke();
  ctx.fillStyle = '#161b22';
  ctx.fill();

  ctx.fillStyle = n.color;
  ctx.font = 'bold 11px Segoe UI, sans-serif';
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.fillText(n.label, n.x, n.y);
}

function drawLine(a, b, color) {
  ctx.beginPath();
  ctx.moveTo(a.x, a.y);
  ctx.lineTo(b.x, b.y);
  ctx.strokeStyle = color;
  ctx.lineWidth = 1;
  ctx.stroke();
}

function lerp(a, b, t) { return a + (b - a) * t; }

// Spawn a particle travelling from→to with given color.
function spawnParticle(from, to, color) {
  state.particles.push({ from, to, t: 0, speed: 0.025, color });
}

// Animate a full request: client→gateway, then gateway→upstream (if allowed)
// or gateway stays red (if blocked).
function animateRequest(allowed) {
  const color = allowed ? '#3fb950' : '#f85149';
  // client → gateway
  spawnParticle(NODES.client, NODES.gateway, '#d29922');
  // gateway → redis (rate-limit check)
  setTimeout(() => spawnParticle(NODES.gateway, NODES.redis, '#f0883e'), 300);
  if (allowed) {
    // gateway → upstream
    setTimeout(() => spawnParticle(NODES.gateway, NODES.upstream, color), 600);
  }
  // Return journey
  setTimeout(() => spawnParticle(NODES.gateway, NODES.client, color), allowed ? 900 : 700);
}

drawFrame();

// ---- Helpers -------------------------------------------------------------
function adminUrl() { return document.getElementById('adminUrl').value.trim().replace(/\/$/, ''); }
function proxyUrl() { return document.getElementById('proxyUrl').value.trim().replace(/\/$/, ''); }
function adminHeaders() {
  const tok = document.getElementById('adminToken').value.trim();
  const h = { 'Content-Type': 'application/json' };
  if (tok) h['Authorization'] = 'Bearer ' + tok;
  return h;
}

function log(msg) {
  const el = document.getElementById('outputLog');
  const ts = new Date().toISOString().slice(11, 23);
  el.textContent = '[' + ts + '] ' + msg + '\n' + el.textContent;
}

function updateCounters() {
  document.getElementById('statTotal').textContent = state.total;
  document.getElementById('statAllowed').textContent = state.allowed;
  document.getElementById('statBlocked').textContent = state.blocked;
  const avg = state.latencies.length
    ? Math.round(state.latencies.reduce((a, b) => a + b, 0) / state.latencies.length)
    : '—';
  document.getElementById('statAvgMs').textContent = avg;
}

// ---- Route table refresh -------------------------------------------------
async function refreshRoutes() {
  try {
    const r = await fetch(adminUrl() + '/v1/routes', { headers: adminHeaders() });
    const routes = await r.json();
    state.routes = Array.isArray(routes) ? routes : [];
    renderRouteTable();
    log('Routes refreshed: ' + state.routes.length + ' route(s)');
  } catch (e) {
    log('ERROR refreshing routes: ' + e.message);
  }
}

function renderRouteTable() {
  const tbody = document.getElementById('routeTableBody');
  tbody.replaceChildren();
  for (const rt of state.routes) {
    const tr = document.createElement('tr');
    const cells = [rt.id, rt.path_prefix, rt.upstream, rt.auth_required ? '✓' : '—', rt.required_scope || '—', rt.active ? '✓' : '✗'];
    for (const c of cells) {
      const td = document.createElement('td');
      td.textContent = c;
      tr.appendChild(td);
    }
    tbody.appendChild(tr);
  }
}

// ---- Create / update route -----------------------------------------------
document.getElementById('btnUpsertRoute').addEventListener('click', async () => {
  const id = document.getElementById('routeId').value.trim();
  const body = {
    path_prefix:    document.getElementById('routePath').value.trim(),
    upstream:       document.getElementById('routeUpstream').value.trim(),
    strip_prefix:   document.getElementById('routeStrip').checked,
    auth_required:  document.getElementById('routeAuth').checked,
    required_scope: document.getElementById('routeScope').value.trim(),
    max_body_bytes: 0,
    timeout_secs:   0,
    active:         document.getElementById('routeActive').checked,
  };
  try {
    const r = await fetch(adminUrl() + '/v1/routes/' + encodeURIComponent(id), {
      method: 'PUT', headers: adminHeaders(), body: JSON.stringify(body),
    });
    const data = await r.json();
    log('PUT /v1/routes/' + id + ' → ' + r.status + '\n' + JSON.stringify(data, null, 2));
    if (r.ok) refreshRoutes();
  } catch (e) {
    log('ERROR: ' + e.message);
  }
});

// ---- Create API key ------------------------------------------------------
document.getElementById('btnCreateKey').addEventListener('click', async () => {
  const body = {
    owner:         document.getElementById('keyOwner').value.trim(),
    key:           document.getElementById('keyRaw').value.trim(),
    scopes:        document.getElementById('keyScopes').value.trim().split(',').map(s => s.trim()).filter(Boolean),
    quota_per_min: parseInt(document.getElementById('keyQuota').value, 10) || 0,
  };
  try {
    const r = await fetch(adminUrl() + '/v1/api-keys', {
      method: 'POST', headers: adminHeaders(), body: JSON.stringify(body),
    });
    const data = await r.json();
    log('POST /v1/api-keys → ' + r.status + '\n' + JSON.stringify(data, null, 2));
  } catch (e) {
    log('ERROR: ' + e.message);
  }
});

// ---- Send proxy request --------------------------------------------------
async function sendProxyRequest(path, apiKey) {
  const start = performance.now();
  state.total++;
  spawnParticle(NODES.client, NODES.gateway, '#d29922');

  const headers = {};
  if (apiKey) headers['Authorization'] = 'Bearer ' + apiKey;

  try {
    const r = await fetch(proxyUrl() + path, { headers });
    const elapsed = Math.round(performance.now() - start);
    state.latencies.push(elapsed);
    if (state.latencies.length > 100) state.latencies.shift();

    const allowed = r.ok;
    allowed ? state.allowed++ : state.blocked++;
    animateRequest(allowed);
    updateCounters();

    const text = await r.text();
    log((allowed ? '✓' : '✗') + ' ' + r.status + ' ' + path + ' (' + elapsed + 'ms)\n' + text.slice(0, 200));
    return { status: r.status, allowed, elapsed };
  } catch (e) {
    state.blocked++;
    animateRequest(false);
    updateCounters();
    log('ERROR ' + path + ': ' + e.message);
    return { status: 0, allowed: false, elapsed: 0 };
  }
}

document.getElementById('btnSendRequest').addEventListener('click', () => {
  const path = document.getElementById('reqPath').value.trim() || '/';
  const key  = document.getElementById('reqKey').value.trim();
  sendProxyRequest(path, key);
});

// ---- Load test -----------------------------------------------------------
document.getElementById('btnLoadTest').addEventListener('click', async () => {
  const n = parseInt(document.getElementById('loadN').value, 10) || 30;
  const concurrency = parseInt(document.getElementById('loadConcurrency').value, 10) || 5;
  const path = document.getElementById('reqPath').value.trim() || '/';
  const key  = document.getElementById('reqKey').value.trim();

  log('Load test starting: ' + n + ' requests, concurrency=' + concurrency);

  const queue = Array.from({ length: n }, (_, i) => i);
  let inFlight = 0;
  let done = 0;

  await new Promise(resolve => {
    function pump() {
      while (inFlight < concurrency && queue.length > 0) {
        queue.shift();
        inFlight++;
        sendProxyRequest(path, key).then(() => {
          inFlight--;
          done++;
          if (done >= n) return resolve();
          pump();
        });
      }
    }
    pump();
  });

  log('Load test done: ' + n + ' requests, allowed=' + state.allowed + ', blocked=' + state.blocked);
});

// ---- Clear log -----------------------------------------------------------
document.getElementById('btnClearLog').addEventListener('click', () => {
  document.getElementById('outputLog').textContent = '';
});

// ---- Refresh routes button -----------------------------------------------
document.getElementById('btnRefreshRoutes').addEventListener('click', refreshRoutes);

// ---- Auto-refresh routes on load -----------------------------------------
refreshRoutes();

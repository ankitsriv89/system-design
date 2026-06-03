'use strict';

// ─── Canvas animation state ─────────────────────────────────────────────────
const canvas = document.getElementById('vizCanvas');
const ctx = canvas.getContext('2d');

const COLORS = {
  bg: '#1a1d27',
  border: '#2e3350',
  accent: '#4f9cf9',
  green: '#22c55e',
  red: '#ef4444',
  yellow: '#f59e0b',
  purple: '#7c3aed',
  muted: '#64748b',
  text: '#e2e8f0',
};

// Nodes: Client, API, Frontier (DB), Workers, Redis, Pages (DB)
const NODES = [
  { id: 'client',   label: 'Browser',    x: 0.08, y: 0.22, color: COLORS.accent },
  { id: 'api',      label: 'API',        x: 0.28, y: 0.22, color: COLORS.purple },
  { id: 'frontier', label: 'Frontier\n(Postgres)', x: 0.52, y: 0.12, color: COLORS.yellow },
  { id: 'redis',    label: 'Redis\nSeen-Set', x: 0.52, y: 0.55, color: COLORS.red },
  { id: 'worker',   label: 'Workers\n×3',  x: 0.72, y: 0.33, color: COLORS.green },
  { id: 'pages',    label: 'PageFetches\n(Postgres)', x: 0.90, y: 0.55, color: COLORS.yellow },
  { id: 'robots',   label: 'robots.txt\ncache',       x: 0.72, y: 0.75, color: COLORS.muted },
];

// Animated packets along edges
let packets = [];
let animFrame = null;
let stepIdx = 0;
let stepTimer = null;

function resizeCanvas() {
  const rect = canvas.parentElement.getBoundingClientRect();
  canvas.width  = Math.floor(rect.width  * devicePixelRatio);
  canvas.height = Math.floor(320 * devicePixelRatio);
  canvas.style.width  = rect.width + 'px';
  canvas.style.height = '320px';
}

function nodePos(n) {
  return {
    x: n.x * canvas.width,
    y: n.y * canvas.height,
  };
}

function drawNode(n) {
  const p = nodePos(n);
  ctx.beginPath();
  ctx.arc(p.x, p.y, 22 * devicePixelRatio, 0, Math.PI * 2);
  ctx.fillStyle = n.color + '22';
  ctx.fill();
  ctx.strokeStyle = n.color;
  ctx.lineWidth = 2 * devicePixelRatio;
  ctx.stroke();

  ctx.fillStyle = COLORS.text;
  ctx.font = `${10 * devicePixelRatio}px SF Mono, monospace`;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  const lines = n.label.split('\n');
  const lh = 12 * devicePixelRatio;
  lines.forEach((line, i) => {
    ctx.fillText(line, p.x, p.y + (i - (lines.length - 1) / 2) * lh);
  });
}

function drawEdge(aId, bId, color, dashed) {
  const a = nodePos(NODES.find(n => n.id === aId));
  const b = nodePos(NODES.find(n => n.id === bId));
  ctx.beginPath();
  ctx.moveTo(a.x, a.y);
  ctx.lineTo(b.x, b.y);
  ctx.strokeStyle = color || COLORS.border;
  ctx.lineWidth = 1.5 * devicePixelRatio;
  if (dashed) ctx.setLineDash([4 * devicePixelRatio, 4 * devicePixelRatio]);
  ctx.stroke();
  ctx.setLineDash([]);
}

const EDGES = [
  ['client',   'api',      COLORS.accent],
  ['api',      'frontier', COLORS.yellow],
  ['worker',   'frontier', COLORS.green],
  ['worker',   'redis',    COLORS.red],
  ['worker',   'pages',    COLORS.yellow],
  ['worker',   'robots',   COLORS.muted, true],
];

function spawnPacket(fromId, toId, color) {
  const from = NODES.find(n => n.id === fromId);
  const to   = NODES.find(n => n.id === toId);
  if (!from || !to) return;
  packets.push({ from, to, t: 0, color: color || COLORS.accent });
}

function drawFrame() {
  ctx.clearRect(0, 0, canvas.width, canvas.height);

  EDGES.forEach(([a, b, c, d]) => drawEdge(a, b, c, d));
  NODES.forEach(drawNode);

  // Draw packets
  packets = packets.filter(p => {
    p.t += 0.025;
    if (p.t > 1) return false;
    const fp = nodePos(p.from);
    const tp = nodePos(p.to);
    const x = fp.x + (tp.x - fp.x) * p.t;
    const y = fp.y + (tp.y - fp.y) * p.t;
    ctx.beginPath();
    ctx.arc(x, y, 5 * devicePixelRatio, 0, Math.PI * 2);
    ctx.fillStyle = p.color;
    ctx.fill();
    return true;
  });

  animFrame = requestAnimationFrame(drawFrame);
}

// ─── Step highlighter ───────────────────────────────────────────────────────
const steps = document.querySelectorAll('.step');

function highlightStep(i) {
  steps.forEach((s, idx) => s.classList.toggle('active', idx === i));
}

const STEP_PACKETS = [
  null,                             // step 0: claim
  { from: 'worker', to: 'redis',    color: COLORS.red    }, // step 1: dedupe
  { from: 'worker', to: 'robots',   color: COLORS.muted  }, // step 2: robots fetch
  null,                                                       // step 3: check path
  null,                                                       // step 4: sleep
  { from: 'worker', to: 'pages',    color: COLORS.green  }, // step 5: HTTP GET
  { from: 'worker', to: 'pages',    color: COLORS.yellow }, // step 6: content hash
  { from: 'worker', to: 'frontier', color: COLORS.accent  }, // step 7: enqueue links
  { from: 'worker', to: 'redis',    color: COLORS.red    }, // step 8: mark seen
];

function advanceStep() {
  highlightStep(stepIdx);
  const pkt = STEP_PACKETS[stepIdx];
  if (pkt) spawnPacket(pkt.from, pkt.to, pkt.color);
  stepIdx = (stepIdx + 1) % steps.length;
}

// ─── Stats polling ───────────────────────────────────────────────────────────
async function pollStats() {
  try {
    const r = await fetch('/v1/frontier/stats');
    if (!r.ok) return;
    const data = await r.json();
    const f = data.frontier || {};
    document.getElementById('statPending').textContent  = f.pending  ?? 0;
    document.getElementById('statFetching').textContent = f.fetching ?? 0;
    document.getElementById('statDone').textContent     = f.done     ?? 0;
    document.getElementById('statFailed').textContent   = f.failed   ?? 0;
    document.getElementById('statSeen').textContent     = data.seen_count ?? 0;

    // Visual: burst packets proportional to pending
    const pending = f.pending || 0;
    if (pending > 0) {
      spawnPacket('worker', 'frontier', COLORS.green);
      spawnPacket('worker', 'redis',    COLORS.red);
    }
  } catch (_) {}
}

async function refreshPages() {
  try {
    const r = await fetch('/v1/pages');
    if (!r.ok) return;
    const pages = await r.json();
    const list = document.getElementById('pagesList');
    list.replaceChildren();
    (pages || []).slice(0, 15).forEach(p => {
      const div = document.createElement('div');
      div.className = 'page-item';

      const urlEl = document.createElement('div');
      urlEl.className = 'page-url';
      urlEl.textContent = p.url;

      const meta = document.createElement('div');
      meta.className = 'page-meta';

      const sc = document.createElement('span');
      sc.textContent = 'HTTP ' + (p.status_code || '?');
      sc.className = p.status_code === 200 ? 'status-200' : p.status_code >= 300 && p.status_code < 400 ? 'status-3xx' : 'status-err';

      const sz = document.createElement('span');
      sz.textContent = formatBytes(p.body_size);

      const ts = document.createElement('span');
      ts.textContent = formatTime(p.fetched_at);

      meta.append(sc, sz, ts);
      div.append(urlEl, meta);
      list.appendChild(div);
    });
  } catch (_) {}
}

async function refreshJobs() {
  try {
    const r = await fetch('/v1/crawl-jobs');
    if (!r.ok) return;
    const jobs = await r.json();
    const list = document.getElementById('jobsList');
    list.replaceChildren();
    (jobs || []).slice(0, 10).forEach(j => {
      const div = document.createElement('div');
      div.className = 'job-item';

      const seed = document.createElement('div');
      seed.className = 'job-seed';
      seed.textContent = j.seed_url;

      const meta = document.createElement('div');
      meta.className = 'job-meta';

      const st = document.createElement('span');
      st.textContent = j.status;
      st.className = 'status-' + (j.status || 'pending');

      const depth = document.createElement('span');
      depth.textContent = 'depth ' + j.max_depth;

      const ts = document.createElement('span');
      ts.textContent = formatTime(j.created_at);

      meta.append(st, depth, ts);
      div.append(seed, meta);
      list.appendChild(div);
    });
  } catch (_) {}
}

// ─── API log helpers ─────────────────────────────────────────────────────────
function logLine(text) {
  const el = document.getElementById('apiLog');
  const ts = new Date().toISOString().slice(11, 23);
  el.textContent += '[' + ts + '] ' + text + '\n';
  el.scrollTop = el.scrollHeight;
}

function formatBytes(n) {
  if (!n) return '0 B';
  if (n < 1024) return n + ' B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB';
  return (n / 1048576).toFixed(1) + ' MB';
}

function formatTime(iso) {
  if (!iso) return '';
  return new Date(iso).toLocaleTimeString();
}

// ─── Button handlers ─────────────────────────────────────────────────────────
document.getElementById('btnSubmit').addEventListener('click', async () => {
  const seedUrl  = document.getElementById('seedUrl').value.trim();
  const maxDepth = parseInt(document.getElementById('maxDepth').value, 10) || 2;
  if (!seedUrl) { logLine('ERROR: seed_url required'); return; }

  logLine('POST /v1/crawl-jobs  seed=' + seedUrl + ' depth=' + maxDepth);
  try {
    const r = await fetch('/v1/crawl-jobs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ seed_url: seedUrl, max_depth: maxDepth }),
    });
    const data = await r.json();
    logLine(r.status + ' ' + JSON.stringify(data));
    if (r.ok) {
      spawnPacket('client', 'api',      COLORS.accent);
      spawnPacket('api',    'frontier', COLORS.yellow);
      await refreshJobs();
    }
  } catch (e) { logLine('ERROR: ' + e.message); }
});

document.getElementById('btnEnqueue').addEventListener('click', async () => {
  const rawUrl   = document.getElementById('enqUrl').value.trim();
  const priority = parseInt(document.getElementById('enqPriority').value, 10) || 1;
  if (!rawUrl) { logLine('ERROR: url required'); return; }

  logLine('POST /v1/frontier/enqueue  url=' + rawUrl);
  try {
    const r = await fetch('/v1/frontier/enqueue', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: rawUrl, priority }),
    });
    const data = await r.json();
    logLine(r.status + ' ' + JSON.stringify(data));
    if (r.ok) spawnPacket('api', 'frontier', COLORS.yellow);
  } catch (e) { logLine('ERROR: ' + e.message); }
});

document.getElementById('btnClear').addEventListener('click', () => {
  document.getElementById('apiLog').textContent = '';
});

document.getElementById('btnRefreshPages').addEventListener('click', () => {
  refreshPages();
  refreshJobs();
});

// ─── Boot ────────────────────────────────────────────────────────────────────
resizeCanvas();
window.addEventListener('resize', resizeCanvas);
drawFrame();

// Advance algorithm step every 1.8s
stepTimer = setInterval(advanceStep, 1800);
advanceStep();

// Poll stats every 3s, pages/jobs every 6s
pollStats();
setInterval(pollStats, 3000);
refreshPages();
refreshJobs();
setInterval(() => { refreshPages(); refreshJobs(); }, 6000);

logLine('Web Crawler service connected — ready');

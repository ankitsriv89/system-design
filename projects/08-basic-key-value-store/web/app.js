'use strict';

// ─── State ───────────────────────────────────────────────────────────────────
const state = {
  stats: null,
  particles: [],          // animated write particles
  flushParticles: [],     // memtable→SST flush animations
  compactAnim: 0,         // compaction highlight timer
  lastFlushCount: 0,
  lastCompactCount: 0,
  loadRunning: false,
  loadDone: 0,
  loadTotal: 0,
};

// ─── Canvas setup ────────────────────────────────────────────────────────────
const canvas = document.getElementById('viz-canvas');
const ctx = canvas.getContext('2d');
let W, H;

function resize() {
  const dpr = window.devicePixelRatio || 1;
  const rect = canvas.getBoundingClientRect();
  W = rect.width;
  H = rect.height;
  canvas.width  = W * dpr;
  canvas.height = H * dpr;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
}
window.addEventListener('resize', () => { resize(); drawFrame(); });
resize();

// ─── Colour palette ──────────────────────────────────────────────────────────
const C = {
  bg:      '#0f1117',
  panel:   '#1a1d27',
  border:  '#2d3047',
  accent:  '#4f9cf9',
  accent2: '#a78bfa',
  accent3: '#34d399',
  warn:    '#fbbf24',
  danger:  '#f87171',
  muted:   '#64748b',
  text:    '#e2e8f0',
};

// ─── Draw helpers ────────────────────────────────────────────────────────────
function roundRect(x, y, w, h, r) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.lineTo(x + w - r, y);
  ctx.quadraticCurveTo(x + w, y, x + w, y + r);
  ctx.lineTo(x + w, y + h - r);
  ctx.quadraticCurveTo(x + w, y + h, x + w - r, y + h);
  ctx.lineTo(x + r, y + h);
  ctx.quadraticCurveTo(x, y + h, x, y + h - r);
  ctx.lineTo(x, y + r);
  ctx.quadraticCurveTo(x, y, x + r, y);
  ctx.closePath();
}

function label(text, x, y, color, size = 12, align = 'center') {
  ctx.save();
  ctx.font = `600 ${size}px 'Segoe UI', sans-serif`;
  ctx.fillStyle = color;
  ctx.textAlign = align;
  ctx.textBaseline = 'middle';
  ctx.fillText(text, x, y);
  ctx.restore();
}

function arrow(x1, y1, x2, y2, color, alpha = 1) {
  ctx.save();
  ctx.globalAlpha = alpha;
  ctx.strokeStyle = color;
  ctx.lineWidth = 1.5;
  ctx.beginPath();
  ctx.moveTo(x1, y1);
  ctx.lineTo(x2, y2);
  ctx.stroke();
  // arrowhead
  const angle = Math.atan2(y2 - y1, x2 - x1);
  ctx.beginPath();
  ctx.moveTo(x2, y2);
  ctx.lineTo(x2 - 8 * Math.cos(angle - 0.4), y2 - 8 * Math.sin(angle - 0.4));
  ctx.lineTo(x2 - 8 * Math.cos(angle + 0.4), y2 - 8 * Math.sin(angle + 0.4));
  ctx.closePath();
  ctx.fillStyle = color;
  ctx.fill();
  ctx.restore();
}

// ─── Main draw frame ─────────────────────────────────────────────────────────
function drawFrame() {
  ctx.clearRect(0, 0, W, H);
  ctx.fillStyle = C.bg;
  ctx.fillRect(0, 0, W, H);

  const s = state.stats;
  const memKeys   = s ? s.memtable_keys  : 0;
  const memBytes  = s ? s.memtable_bytes : 0;
  const sstCount  = s ? s.sst_count      : 0;
  const ssts      = s ? (s.sstables || []) : [];
  const walEntries = s ? s.wal_entries   : 0;

  const cx = W / 2;
  const topY = 50;

  // ── Client box ──────────────────────────────────────────────────────────
  const clientX = cx - 55, clientW = 110, clientH = 40;
  const clientCY = topY + clientH / 2;
  ctx.fillStyle = C.panel;
  roundRect(clientX, topY, clientW, clientH, 8);
  ctx.fill();
  ctx.strokeStyle = C.accent;
  ctx.lineWidth = 1.5;
  ctx.stroke();
  label('Client / API', cx, clientCY, C.accent, 13);

  // ── WAL box ─────────────────────────────────────────────────────────────
  const walY = topY + 90;
  const walX = cx + 80, walW = 110, walH = 44;
  const walCY = walY + walH / 2;
  ctx.fillStyle = C.panel;
  roundRect(walX, walY, walW, walH, 8);
  ctx.fill();
  ctx.strokeStyle = C.warn;
  ctx.lineWidth = 1.5;
  ctx.stroke();
  label('WAL', walX + walW / 2, walCY - 8, C.warn, 13);
  label(`${walEntries} entries`, walX + walW / 2, walCY + 8, C.muted, 10);

  // ── Memtable box ────────────────────────────────────────────────────────
  const memY = topY + 90;
  const memX = cx - 140, memW = 130, memH = 44;
  const memCY = memY + memH / 2;
  const memFill = Math.min(memBytes / (4 * 1024 * 1024), 1);
  ctx.fillStyle = C.panel;
  roundRect(memX, memY, memW, memH, 8);
  ctx.fill();
  // Fill bar inside memtable box
  if (memFill > 0) {
    ctx.save();
    roundRect(memX + 2, memY + memH - 8, (memW - 4) * memFill, 6, 3);
    ctx.fillStyle = memFill > 0.8 ? C.danger : C.mem;
    ctx.fill();
    ctx.restore();
  }
  ctx.strokeStyle = C.mem;
  ctx.lineWidth = 1.5;
  roundRect(memX, memY, memW, memH, 8);
  ctx.stroke();
  label('Memtable', memX + memW / 2, memCY - 8, C.mem, 13);
  label(`${memKeys} keys · ${fmtBytes(memBytes)}`, memX + memW / 2, memCY + 8, C.muted, 10);

  // ── Arrows: client → WAL, client → memtable ─────────────────────────────
  arrow(cx, topY + clientH, walX + walW / 2, walY, C.warn, 0.6);
  arrow(cx, topY + clientH, memX + memW / 2, memY, C.mem, 0.6);

  // ── SSTable level boxes ─────────────────────────────────────────────────
  const sstY = memY + memH + 70;
  const l0 = ssts.filter(s => s.level === 0);
  const l1 = ssts.filter(s => s.level >= 1);

  drawSSTLevel('L0 SSTables (newest writes)', l0, sstY, C.sst0);
  drawSSTLevel('L1 SSTables (compacted)', l1, sstY + 90, C.accent2);

  // ── Arrow: memtable → L0 ────────────────────────────────────────────────
  arrow(memX + memW / 2, memY + memH, cx, sstY, C.muted, 0.5);

  // ── Arrow: L0 → L1 (compaction) ─────────────────────────────────────────
  const compactAlpha = state.compactAnim > 0 ? 1 : 0.25;
  arrow(cx, sstY + 44, cx, sstY + 90, C.accent2, compactAlpha);
  if (state.compactAnim > 0) {
    ctx.save();
    ctx.globalAlpha = state.compactAnim / 60;
    label('⚡ compacting', cx + 40, sstY + 67, C.accent2, 11, 'left');
    ctx.restore();
    state.compactAnim--;
  }

  // ── Particles ───────────────────────────────────────────────────────────
  drawParticles();

  // ── Legend ──────────────────────────────────────────────────────────────
  drawLegend();
}

function drawSSTLevel(title, ssts, y, color) {
  const cx = W / 2;
  const boxH = 44;
  const maxSlots = 6;
  const count = Math.min(ssts.length, maxSlots);
  const slotW = 90;
  const totalW = count * slotW + (count - 1) * 8;
  let startX = cx - totalW / 2;

  label(title, cx, y - 12, C.muted, 10);

  if (count === 0) {
    ctx.save();
    ctx.strokeStyle = color;
    ctx.lineWidth = 1;
    ctx.setLineDash([4, 4]);
    roundRect(cx - 60, y, 120, boxH, 8);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.restore();
    label('empty', cx, y + boxH / 2, C.muted, 11);
    return;
  }

  for (let i = 0; i < count; i++) {
    const m = ssts[i];
    const bx = startX + i * (slotW + 8);
    const highlight = state.compactAnim > 0 && color === C.sst0;
    ctx.fillStyle = highlight ? '#1a2535' : C.panel;
    roundRect(bx, y, slotW, boxH, 8);
    ctx.fill();
    ctx.strokeStyle = highlight ? C.warn : color;
    ctx.lineWidth = highlight ? 2 : 1.5;
    roundRect(bx, y, slotW, boxH, 8);
    ctx.stroke();
    label(`SST #${m.seq}`, bx + slotW / 2, y + 13, color, 11);
    label(`${m.count} keys`, bx + slotW / 2, y + 28, C.muted, 10);
  }

  if (ssts.length > maxSlots) {
    label(`+${ssts.length - maxSlots} more`, startX + count * (slotW + 8), y + boxH / 2, C.muted, 10, 'left');
  }
}

function drawParticles() {
  for (let i = state.particles.length - 1; i >= 0; i--) {
    const p = state.particles[i];
    p.x += p.vx;
    p.y += p.vy;
    p.life--;
    ctx.save();
    ctx.globalAlpha = p.life / p.maxLife;
    ctx.beginPath();
    ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2);
    ctx.fillStyle = p.color;
    ctx.fill();
    ctx.restore();
    if (p.life <= 0) state.particles.splice(i, 1);
  }
}

function drawLegend() {
  const items = [
    { color: C.warn,    label: 'WAL' },
    { color: C.mem,     label: 'Memtable' },
    { color: C.sst0,    label: 'L0 SST' },
    { color: C.accent2, label: 'L1 SST' },
  ];
  const startX = 14;
  let x = startX;
  const y = H - 20;
  items.forEach(item => {
    ctx.beginPath();
    ctx.arc(x + 6, y, 5, 0, Math.PI * 2);
    ctx.fillStyle = item.color;
    ctx.fill();
    label(item.label, x + 14, y, C.muted, 10, 'left');
    x += 70;
  });
}

// ─── Particle factory ────────────────────────────────────────────────────────
function spawnWriteParticle(color) {
  const cx = W / 2;
  const topCY = 50 + 20;  // approx centre of client box
  for (let i = 0; i < 6; i++) {
    state.particles.push({
      x: cx + (Math.random() - .5) * 30,
      y: topCY,
      vx: (Math.random() - .5) * 3,
      vy: 1 + Math.random() * 2,
      r: 3 + Math.random() * 3,
      color,
      life: 40 + Math.random() * 20,
      maxLife: 60,
    });
  }
}

// ─── Animation loop ──────────────────────────────────────────────────────────
function loop() {
  drawFrame();
  requestAnimationFrame(loop);
}
loop();

// ─── Stats polling ───────────────────────────────────────────────────────────
const dot   = document.getElementById('health-dot');
const label_ = document.getElementById('health-label');

async function pollStats() {
  try {
    const r = await fetch('/v1/admin/stats');
    if (!r.ok) throw new Error(r.status);
    const data = await r.json();

    if (state.stats && data.flushes > state.lastFlushCount) {
      spawnWriteParticle(C.mem);
    }
    if (state.stats && data.compactions > state.lastCompactCount) {
      state.compactAnim = 90;
    }
    state.lastFlushCount = data.flushes;
    state.lastCompactCount = data.compactions;
    state.stats = data;

    dot.className = 'status-dot alive';
    label_.textContent = 'connected';
    updateStatCards(data);
  } catch {
    dot.className = 'status-dot';
    label_.textContent = 'disconnected';
  }
}

function updateStatCards(s) {
  setText('stat-writes',   s.writes);
  setText('stat-reads',    s.reads);
  setText('stat-deletes',  s.deletes);
  setText('stat-flushes',  s.flushes);
  setText('stat-compact',  s.compactions);
  setText('stat-mem',      fmtBytes(s.memtable_bytes));
  setText('stat-sstcount', s.sst_count);
  setText('stat-walent',   s.wal_entries);
}

function setText(id, val) {
  const el = document.getElementById(id);
  if (el) el.textContent = val;
}

function fmtBytes(b) {
  if (!b) return '0 B';
  if (b < 1024) return b + ' B';
  if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' KB';
  return (b / 1024 / 1024).toFixed(2) + ' MB';
}

setInterval(pollStats, 1500);
pollStats();

// ─── KV operations ───────────────────────────────────────────────────────────
async function doSet() {
  const key = document.getElementById('kv-key').value.trim();
  const val = document.getElementById('kv-val').value;
  if (!key) return log_('set', 'Key is required', 'err');

  spawnWriteParticle(C.accent3);
  walkthrough('SET', `Writing key <strong>${esc(key)}</strong> → WAL first (fsync), then memtable.`);

  try {
    const r = await fetch(`/v1/kv/${encodeURIComponent(key)}`, {
      method: 'PUT', body: val,
    });
    const ok = r.ok;
    log_('set', `PUT ${key} → ${r.status} ${ok ? '✓' : '✗'}`, ok ? 'set' : 'err');
  } catch (e) {
    log_('set', `PUT ${key} → error: ${e.message}`, 'err');
  }
}

async function doGet() {
  const key = document.getElementById('kv-key').value.trim();
  if (!key) return log_('get', 'Key is required', 'err');

  walkthrough('GET', `Searching for <strong>${esc(key)}</strong>: memtable → L0 SSTables → L1 SSTables.`);
  spawnWriteParticle(C.accent);

  try {
    const r = await fetch(`/v1/kv/${encodeURIComponent(key)}`);
    if (r.status === 404) {
      log_('get', `GET ${key} → 404 not found`, 'info');
      walkthrough('GET', `<strong>${esc(key)}</strong> not found in memtable or any SSTable.`);
      return;
    }
    const body = await r.text();
    log_('get', `GET ${key} → ${r.status} · value: ${body.substring(0, 120)}`, 'get');
    walkthrough('GET', `Found <strong>${esc(key)}</strong> = <strong>${esc(body.substring(0,60))}</strong>`);
  } catch (e) {
    log_('get', `GET ${key} → error: ${e.message}`, 'err');
  }
}

async function doDelete() {
  const key = document.getElementById('kv-key').value.trim();
  if (!key) return log_('del', 'Key is required', 'err');

  walkthrough('DELETE', `Writing tombstone for <strong>${esc(key)}</strong> to WAL + memtable.`);
  spawnWriteParticle(C.danger);

  try {
    const r = await fetch(`/v1/kv/${encodeURIComponent(key)}`, { method: 'DELETE' });
    log_('del', `DELETE ${key} → ${r.status}`, r.ok ? 'del' : 'err');
  } catch (e) {
    log_('del', `DELETE ${key} → error: ${e.message}`, 'err');
  }
}

async function doCompact() {
  walkthrough('COMPACT', 'Merging all L0 SSTables → L1. Tombstones are purged. Duplicate keys resolved to newest value.');
  state.compactAnim = 180;
  try {
    const r = await fetch('/v1/admin/compact', { method: 'POST' });
    log_('info', `Compaction → ${r.status}`, r.ok ? 'ok' : 'err');
  } catch (e) {
    log_('info', `Compaction error: ${e.message}`, 'err');
  }
  pollStats();
}

// ─── Load test ───────────────────────────────────────────────────────────────
async function doLoad() {
  if (state.loadRunning) return;
  state.loadRunning = true;
  state.loadDone = 0;
  state.loadTotal = 200;
  document.getElementById('load-btn').disabled = true;
  document.getElementById('load-bar').style.width = '0%';
  walkthrough('LOAD TEST', 'Writing 200 keys in parallel batches of 20 to fill the memtable and trigger flushes.');

  const batch = 20;
  for (let i = 0; i < state.loadTotal; i += batch) {
    const promises = [];
    for (let j = i; j < Math.min(i + batch, state.loadTotal); j++) {
      const key = `load-key-${String(j).padStart(4, '0')}`;
      const val = `load-value-${j}-${'x'.repeat(200)}`;
      promises.push(
        fetch(`/v1/kv/${key}`, { method: 'PUT', body: val })
          .then(() => { state.loadDone++; })
          .catch(() => {})
      );
    }
    await Promise.all(promises);
    spawnWriteParticle(C.warn);
    document.getElementById('load-bar').style.width =
      Math.round(state.loadDone / state.loadTotal * 100) + '%';
  }
  log_('info', `Load test: wrote ${state.loadTotal} keys`, 'ok');
  state.loadRunning = false;
  document.getElementById('load-btn').disabled = false;
  pollStats();
}

// ─── Failure demo ────────────────────────────────────────────────────────────
async function doStressDelete() {
  walkthrough('FAILURE DEMO', 'Deleting 50 keys to generate tombstones. Run compact to see them purged.');
  for (let i = 0; i < 50; i++) {
    const key = `load-key-${String(i).padStart(4, '0')}`;
    await fetch(`/v1/kv/${key}`, { method: 'DELETE' }).catch(() => {});
  }
  spawnWriteParticle(C.danger);
  log_('del', 'Deleted 50 keys — tombstones written to WAL + memtable', 'del');
  pollStats();
}

// ─── Log ─────────────────────────────────────────────────────────────────────
function log_(op, msg, cls) {
  const logEl = document.getElementById('log');
  const entry = document.createElement('div');
  entry.className = `log-entry ${cls}`;
  const ts = document.createElement('span');
  ts.className = 'ts';
  ts.textContent = new Date().toLocaleTimeString();
  const txt = document.createElement('span');
  txt.textContent = msg;
  entry.appendChild(ts);
  entry.appendChild(txt);
  logEl.prepend(entry);
  if (logEl.children.length > 200) logEl.lastChild.remove();
}

function walkthrough(op, html) {
  const el = document.getElementById('walkthrough');
  el.innerHTML = `<strong>[${op}]</strong> ` + html;
}

function esc(s) {
  return String(s)
    .replace(/&/g,'&amp;')
    .replace(/</g,'&lt;')
    .replace(/>/g,'&gt;')
    .replace(/"/g,'&quot;');
}

function clearLog() {
  document.getElementById('log').replaceChildren();
}

// ─── Wire up buttons ─────────────────────────────────────────────────────────
document.getElementById('btn-set').addEventListener('click', doSet);
document.getElementById('btn-get').addEventListener('click', doGet);
document.getElementById('btn-del').addEventListener('click', doDelete);
document.getElementById('btn-compact').addEventListener('click', doCompact);
document.getElementById('load-btn').addEventListener('click', doLoad);
document.getElementById('btn-stress-del').addEventListener('click', doStressDelete);
document.getElementById('btn-clear-log').addEventListener('click', clearLog);

// Allow Enter in key field to trigger get
document.getElementById('kv-key').addEventListener('keydown', e => {
  if (e.key === 'Enter') doGet();
});

'use strict';

// ── State ────────────────────────────────────────────────────────────────────
const state = {
  stats: { enqueued: 0, delivered: 0, failed: 0, retried: 0, dlq: 0 },
  queueDepth: 0,
  dlqDepth: 0,
  particles: [],   // animated message particles on canvas
  nodes: [],       // pipeline stage nodes
  templates: [],
  failureRates: { email: 0.1, sms: 0.15, push: 0.05 },
};

// ── Canvas setup ─────────────────────────────────────────────────────────────
const canvas = document.getElementById('pipeline-canvas');
const ctx = canvas.getContext('2d');

function resizeCanvas() {
  const rect = canvas.parentElement.getBoundingClientRect();
  // leave room for stats bar
  canvas.width  = rect.width;
  canvas.height = rect.height - document.querySelector('.stats-bar').offsetHeight - 1;
}
window.addEventListener('resize', resizeCanvas);
setTimeout(resizeCanvas, 50);

// Pipeline stages: x positions are set dynamically in drawFrame
const STAGES = [
  { id: 'client',   label: 'Client',    color: '#4f9cf9' },
  { id: 'api',      label: 'API',       color: '#22c55e' },
  { id: 'pref',     label: 'Prefs &\nTemplates', color: '#eab308' },
  { id: 'queue',    label: 'Dispatch\nQueue',  color: '#f97316' },
  { id: 'worker',   label: 'Worker\nPool',  color: '#a855f7' },
  { id: 'provider', label: 'Provider\nMock', color: '#06b6d4' },
];

// ── Particle system ───────────────────────────────────────────────────────────
let particleIdSeq = 0;

function spawnParticle(channel, status) {
  const id = ++particleIdSeq;
  const colors = {
    email: '#4f9cf9',
    sms:   '#22c55e',
    push:  '#a855f7',
  };
  state.particles.push({
    id,
    channel,
    status,            // 'ok' | 'fail' | 'retry' | 'dlq' | 'skip'
    stageIdx: 0,
    progress: 0,       // 0..1 within current segment
    speed: 0.008 + Math.random() * 0.006,
    color: colors[channel] || '#4f9cf9',
    radius: 5,
    dead: false,
    trail: [],
  });
}

function updateParticles() {
  for (const p of state.particles) {
    if (p.dead) continue;
    p.trail.push({ x: p.x, y: p.y });
    if (p.trail.length > 12) p.trail.shift();
    p.progress += p.speed;
    if (p.progress >= 1) {
      p.stageIdx++;
      p.progress = 0;
      if (p.stageIdx >= STAGES.length - 1) {
        p.dead = true;
      }
      // Skip straight to dlq stage if dlq
      if (p.status === 'dlq' && p.stageIdx === 4) {
        p.dead = true;
      }
    }
  }
  state.particles = state.particles.filter(p => !p.dead || p.trail.length > 0);
}

// ── Draw frame ────────────────────────────────────────────────────────────────
function drawFrame() {
  const W = canvas.width, H = canvas.height;
  if (W === 0 || H === 0) { requestAnimationFrame(drawFrame); return; }

  ctx.clearRect(0, 0, W, H);
  ctx.fillStyle = '#0b0e18';
  ctx.fillRect(0, 0, W, H);

  const n = STAGES.length;
  const margin = 56;
  const step = (W - margin * 2) / (n - 1);
  const cy = H * 0.5;

  // Compute x positions
  STAGES.forEach((s, i) => {
    s.x = margin + i * step;
    s.y = cy;
  });

  // Draw connecting lines
  for (let i = 0; i < STAGES.length - 1; i++) {
    const a = STAGES[i], b = STAGES[i + 1];
    ctx.beginPath();
    ctx.moveTo(a.x, a.y);
    ctx.lineTo(b.x, b.y);
    ctx.strokeStyle = '#2a2f3e';
    ctx.lineWidth = 2;
    ctx.stroke();
  }

  // Draw particles
  updateParticles();
  for (const p of state.particles) {
    if (p.stageIdx >= STAGES.length - 1) continue;
    const from = STAGES[p.stageIdx];
    const to   = STAGES[p.stageIdx + 1];
    if (!from || !to) continue;
    const x = from.x + (to.x - from.x) * p.progress;
    const y = from.y + (to.y - from.y) * p.progress;
    p.x = x; p.y = y;

    // Trail
    if (p.trail.length > 1) {
      for (let t = 1; t < p.trail.length; t++) {
        const alpha = t / p.trail.length * 0.4;
        ctx.beginPath();
        ctx.arc(p.trail[t].x, p.trail[t].y, 2, 0, Math.PI * 2);
        ctx.fillStyle = p.color + Math.floor(alpha * 255).toString(16).padStart(2, '0');
        ctx.fill();
      }
    }

    // Particle glow
    const gradient = ctx.createRadialGradient(x, y, 0, x, y, 12);
    gradient.addColorStop(0, p.color + 'cc');
    gradient.addColorStop(1, p.color + '00');
    ctx.beginPath();
    ctx.arc(x, y, 12, 0, Math.PI * 2);
    ctx.fillStyle = gradient;
    ctx.fill();

    ctx.beginPath();
    ctx.arc(x, y, p.radius, 0, Math.PI * 2);
    ctx.fillStyle = p.color;
    ctx.fill();
  }

  // Draw stage nodes
  STAGES.forEach((s) => {
    // Outer ring
    ctx.beginPath();
    ctx.arc(s.x, s.y, 26, 0, Math.PI * 2);
    ctx.fillStyle = '#161b27';
    ctx.fill();
    ctx.strokeStyle = s.color;
    ctx.lineWidth = 2;
    ctx.stroke();

    // Inner fill
    ctx.beginPath();
    ctx.arc(s.x, s.y, 20, 0, Math.PI * 2);
    ctx.fillStyle = s.color + '22';
    ctx.fill();

    // Label
    const lines = s.label.split('\n');
    ctx.fillStyle = s.color;
    ctx.font = '600 11px Segoe UI, sans-serif';
    ctx.textAlign = 'center';
    const labelY = s.y + 40;
    lines.forEach((line, li) => {
      ctx.fillText(line, s.x, labelY + li * 14);
    });
  });

  // Queue depth indicator on queue node
  const qNode = STAGES[3];
  ctx.fillStyle = '#f97316';
  ctx.font = 'bold 12px Segoe UI, sans-serif';
  ctx.textAlign = 'center';
  ctx.fillText(state.queueDepth, qNode.x, qNode.y + 5);

  requestAnimationFrame(drawFrame);
}

requestAnimationFrame(drawFrame);

// ── Log panel ────────────────────────────────────────────────────────────────
const logScroll = document.getElementById('log-scroll');
let logCount = 0;

function addLog(cls, ev, detail) {
  logCount++;
  if (logCount > 200) {
    logScroll.firstChild && logScroll.removeChild(logScroll.firstChild);
  }
  const el = document.createElement('div');
  el.className = 'log-entry ' + cls;
  const ts = document.createElement('span');
  ts.className = 'ts';
  ts.textContent = new Date().toLocaleTimeString();
  const evEl = document.createElement('span');
  evEl.className = 'ev';
  evEl.textContent = ' [' + ev + '] ';
  const det = document.createElement('span');
  det.textContent = detail;
  el.appendChild(ts);
  el.appendChild(evEl);
  el.appendChild(det);
  logScroll.appendChild(el);
  logScroll.scrollTop = logScroll.scrollHeight;
}

document.getElementById('clear-log').addEventListener('click', () => {
  logScroll.replaceChildren();
  logCount = 0;
});

// ── Stats display ─────────────────────────────────────────────────────────────
function updateStats() {
  document.getElementById('stat-enqueued').textContent  = state.stats.enqueued;
  document.getElementById('stat-delivered').textContent = state.stats.delivered;
  document.getElementById('stat-failed').textContent    = state.stats.failed;
  document.getElementById('stat-retried').textContent   = state.stats.retried;
  document.getElementById('stat-dlq').textContent       = state.stats.dlq;
}

// ── Poll queue stats ──────────────────────────────────────────────────────────
async function pollStats() {
  try {
    const r = await fetch('/v1/admin/queue/stats');
    if (!r.ok) return;
    const data = await r.json();
    state.queueDepth = data.queue_depth || 0;
    state.dlqDepth   = data.dlq_depth   || 0;

    const byStatus = data.by_status || {};
    state.stats.delivered = byStatus.delivered || 0;
    state.stats.failed    = (byStatus.failed || 0) + (byStatus.dlq || 0);
    state.stats.dlq       = byStatus.dlq || 0;
    updateStats();

    document.getElementById('stat-qdepth').textContent = state.queueDepth;
    document.getElementById('stat-dlqdepth').textContent = state.dlqDepth;
  } catch (_) {}
}
setInterval(pollStats, 1500);

// ── Send notification ─────────────────────────────────────────────────────────
document.getElementById('send-btn').addEventListener('click', async () => {
  const userID     = document.getElementById('user-id').value.trim();
  const channel    = document.getElementById('channel').value;
  const templateID = document.getElementById('template-id').value;
  const paramsRaw  = document.getElementById('params').value.trim();
  const priority   = parseInt(document.getElementById('priority').value, 10);
  const ikey       = document.getElementById('ikey').value.trim();

  if (!userID) { addLog('fail', 'validation', 'user_id is required'); return; }

  let params = {};
  if (paramsRaw) {
    try { params = JSON.parse(paramsRaw); }
    catch (_) { addLog('fail', 'validation', 'params must be valid JSON'); return; }
  }

  const body = { user_id: userID, channel, template_id: templateID, params, priority };
  if (ikey) body.idempotency_key = ikey;

  try {
    const r = await fetch('/v1/notifications', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const data = await r.json();
    if (!r.ok) {
      addLog('fail', 'send-error', data.error || 'unknown error');
      return;
    }
    state.stats.enqueued++;
    updateStats();
    const status = data.status;
    const cls = status === 'queued' ? 'info' : status === 'skipped' ? 'skip' : 'ok';
    addLog(cls, 'notification.created',
      `id=${data.id.slice(0,8)}… ch=${data.channel} status=${data.status}`);
    spawnParticle(channel, status === 'queued' ? 'ok' : 'skip');
    refreshNotifList();
  } catch (e) {
    addLog('fail', 'fetch-error', e.message);
  }
});

// ── Bulk fire ─────────────────────────────────────────────────────────────────
document.getElementById('bulk-btn').addEventListener('click', async () => {
  const count    = parseInt(document.getElementById('bulk-count').value, 10) || 5;
  const channels = ['email', 'sms', 'push'];
  addLog('info', 'bulk-start', `firing ${count} notifications`);
  for (let idx = 0; idx < count; idx++) {
    const ch = channels[idx % channels.length];
    try {
      const r = await fetch('/v1/notifications', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          user_id: 'bulk-user-' + (idx % 3 + 1),
          channel: ch,
          subject: `Bulk test #${idx + 1}`,
          body: `Bulk notification ${idx + 1} via ${ch}`,
          priority: idx % 3,
        }),
      });
      const data = await r.json();
      if (r.ok) {
        state.stats.enqueued++;
        spawnParticle(ch, 'ok');
      } else {
        addLog('warn', 'bulk-err', data.error);
      }
    } catch (e) {
      addLog('fail', 'bulk-err', e.message);
    }
    await new Promise(res => setTimeout(res, 80));
  }
  updateStats();
  addLog('ok', 'bulk-done', `${count} notifications sent`);
  refreshNotifList();
});

// ── Create template ───────────────────────────────────────────────────────────
document.getElementById('create-tmpl-btn').addEventListener('click', async () => {
  const id      = document.getElementById('tmpl-id').value.trim();
  const channel = document.getElementById('tmpl-channel').value;
  const subject = document.getElementById('tmpl-subject').value.trim();
  const body    = document.getElementById('tmpl-body').value.trim();

  if (!id || !body) { addLog('fail', 'validation', 'template id and body required'); return; }

  try {
    const r = await fetch('/v1/templates', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id, channel, subject, body }),
    });
    const data = await r.json();
    if (!r.ok) { addLog('fail', 'template-error', data.error); return; }
    addLog('ok', 'template.created', `id=${id} ch=${channel}`);
    loadTemplates();
  } catch (e) {
    addLog('fail', 'fetch-error', e.message);
  }
});

// ── Load templates ────────────────────────────────────────────────────────────
async function loadTemplates() {
  try {
    const r = await fetch('/v1/templates');
    if (!r.ok) return;
    const data = await r.json();
    state.templates = data.templates || [];
    const sel = document.getElementById('template-id');
    const current = sel.value;
    while (sel.options.length > 1) sel.remove(1);
    for (const t of state.templates) {
      const opt = document.createElement('option');
      opt.value = t.id;
      opt.textContent = `${t.id} (${t.channel})`;
      sel.appendChild(opt);
    }
    if (current) sel.value = current;
  } catch (_) {}
}
loadTemplates();
setInterval(loadTemplates, 10000);

// ── Set user preferences ──────────────────────────────────────────────────────
document.getElementById('pref-btn').addEventListener('click', async () => {
  const userID = document.getElementById('pref-user').value.trim();
  if (!userID) { addLog('fail', 'validation', 'user_id required'); return; }

  const emailOn = document.getElementById('pref-email').checked;
  const smsOn   = document.getElementById('pref-sms').checked;
  const pushOn  = document.getElementById('pref-push').checked;
  const qs = parseInt(document.getElementById('quiet-start').value, 10);
  const qe = parseInt(document.getElementById('quiet-end').value, 10);

  const prefs = [
    { channel: 'email', enabled: emailOn, quiet_start: qs, quiet_end: qe },
    { channel: 'sms',   enabled: smsOn,   quiet_start: qs, quiet_end: qe },
    { channel: 'push',  enabled: pushOn,  quiet_start: qs, quiet_end: qe },
  ];

  try {
    const r = await fetch(`/v1/preferences/${userID}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(prefs),
    });
    if (r.status === 204) {
      addLog('ok', 'prefs.saved', `user=${userID} email=${emailOn} sms=${smsOn} push=${pushOn}`);
    } else {
      const d = await r.json();
      addLog('fail', 'prefs-error', d.error);
    }
  } catch (e) {
    addLog('fail', 'fetch-error', e.message);
  }
});

// ── Failure rate sliders ──────────────────────────────────────────────────────
['email', 'sms', 'push'].forEach(name => {
  const slider = document.getElementById(`fr-${name}`);
  const valEl  = document.getElementById(`fr-${name}-val`);
  slider.addEventListener('input', () => {
    const pct = parseFloat(slider.value);
    valEl.textContent = Math.round(pct * 100) + '%';
    state.failureRates[name] = pct;
  });
  slider.addEventListener('change', async () => {
    try {
      const r = await fetch(`/v1/admin/provider/${name}/failure-rate`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ rate: parseFloat(slider.value) }),
      });
      const d = await r.json();
      if (r.ok) {
        addLog('warn', 'failure-rate', `${name} → ${Math.round(d.failure_rate * 100)}%`);
      }
    } catch (e) {
      addLog('fail', 'fetch-error', e.message);
    }
  });
});

// ── Notifications list ────────────────────────────────────────────────────────
const notifTbody = document.getElementById('notif-tbody');

function badgeHTML(status) {
  const el = document.createElement('span');
  el.className = 'badge badge-' + status;
  el.textContent = status;
  return el;
}

async function refreshNotifList() {
  try {
    const r = await fetch('/v1/notifications?limit=15');
    if (!r.ok) return;
    const data = await r.json();
    notifTbody.replaceChildren();
    for (const n of (data.notifications || [])) {
      const tr = document.createElement('tr');

      const tdId = document.createElement('td');
      tdId.textContent = n.id.slice(0, 8) + '…';

      const tdUser = document.createElement('td');
      tdUser.textContent = n.user_id;

      const tdCh = document.createElement('td');
      tdCh.textContent = n.channel;

      const tdStatus = document.createElement('td');
      tdStatus.appendChild(badgeHTML(n.status));

      tr.appendChild(tdId);
      tr.appendChild(tdUser);
      tr.appendChild(tdCh);
      tr.appendChild(tdStatus);
      notifTbody.appendChild(tr);
    }
  } catch (_) {}
}
setInterval(refreshNotifList, 2000);
refreshNotifList();

// ── Seed demo data ────────────────────────────────────────────────────────────
document.getElementById('seed-btn').addEventListener('click', async () => {
  addLog('info', 'seed', 'seeding demo templates and preferences');

  // Templates
  const templates = [
    { id: 'welcome', channel: 'email', subject: 'Welcome, {{.Name}}!',
      body: 'Hi {{.Name}}, thanks for signing up. Your code is {{.Code}}.' },
    { id: 'otp', channel: 'sms', subject: '',
      body: 'Your OTP is {{.Code}}. Expires in 10 minutes.' },
    { id: 'push-promo', channel: 'push', subject: '{{.Title}}',
      body: '{{.Body}}' },
  ];
  for (const t of templates) {
    await fetch('/v1/templates', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(t),
    });
  }

  // Preferences
  for (let i = 1; i <= 3; i++) {
    await fetch(`/v1/preferences/demo-user-${i}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify([
        { channel: 'email', enabled: true,        quiet_start: -1, quiet_end: -1 },
        { channel: 'sms',   enabled: i !== 2,     quiet_start: -1, quiet_end: -1 },
        { channel: 'push',  enabled: true,         quiet_start: -1, quiet_end: -1 },
      ]),
    });
  }

  addLog('ok', 'seed-done', '3 templates + preferences for demo-user-1/2/3');
  loadTemplates();
});

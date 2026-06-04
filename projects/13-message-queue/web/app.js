/* Message Queue — interactive tutorial + live visualization */
'use strict';

// ---- State -----------------------------------------------------------------
const state = {
  topics: [],
  polledMessages: [],  // messages currently held (not yet acked)
  currentViz: 'topology',
  depthData: {},       // { topic: { partitions: {0:n,...}, dlq: n } }
  stats: { total_messages: 0, pending_messages: 0, inflight_messages: 0, acked_messages: 0, dlq_messages: 0 },
  flowParticles: [],
  animFrame: null,
  algoStep: 0,
};

// ---- Canvas setup ----------------------------------------------------------
const canvas = document.getElementById('mainCanvas');
const ctx = canvas.getContext('2d');

function resizeCanvas() {
  const body = document.getElementById('vizBody');
  canvas.width = body.clientWidth;
  canvas.height = body.clientHeight;
}
window.addEventListener('resize', resizeCanvas);
resizeCanvas();

// ---- Logging ---------------------------------------------------------------
function log(op, body) {
  const c = document.getElementById('logContainer');
  const now = new Date();
  const t = now.toTimeString().slice(0, 8);
  const el = document.createElement('div');
  el.className = 'log-entry';

  const ts = document.createElement('span');
  ts.className = 'log-time';
  ts.textContent = t;

  const opSpan = document.createElement('span');
  opSpan.className = `log-op ${op}`;
  opSpan.textContent = op.toUpperCase();

  const bodySpan = document.createElement('span');
  bodySpan.className = 'log-body';
  bodySpan.textContent = String(body);

  el.append(ts, opSpan, bodySpan);
  c.insertBefore(el, c.firstChild);
  if (c.children.length > 200) c.removeChild(c.lastChild);
}

function escText(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function clearLog() { document.getElementById('logContainer').replaceChildren(); }

// ---- API helpers -----------------------------------------------------------
async function api(method, path, body) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body !== undefined) opts.body = JSON.stringify(body);
  const res = await fetch(path, opts);
  const text = await res.text();
  let data;
  try { data = JSON.parse(text); } catch { data = { raw: text }; }
  return { ok: res.ok, status: res.status, data };
}

// ---- Topic operations ------------------------------------------------------
async function createTopic() {
  const name = document.getElementById('topicName').value.trim();
  const partitions = parseInt(document.getElementById('topicPartitions').value) || 1;
  const retention_hours = parseInt(document.getElementById('topicRetention').value) || 24;
  if (!name) { log('error', 'topic name required'); return; }

  const { ok, data } = await api('POST', '/v1/topics', { name, partitions, retention_hours });
  if (ok) {
    log('info', `topic "${name}" created (${partitions} partitions)`);
    await loadTopics();
    triggerFlowParticle('producer', 'broker', '#6c63ff');
  } else {
    log('error', data.error || 'create topic failed');
  }
}

async function loadTopics() {
  const { ok, data } = await api('GET', '/v1/topics');
  if (!ok) return;
  state.topics = data.topics || [];
  const el = document.getElementById('topicList');
  if (!state.topics.length) { el.textContent = 'No topics yet.'; return; }
  const rows = state.topics.map(t => {
    const row = document.createElement('div');
    row.style.cssText = 'padding:3px 0;border-bottom:1px solid var(--border)';

    const name = document.createElement('span');
    name.style.color = 'var(--accent2)';
    name.textContent = t.Name;

    const info = document.createElement('span');
    info.style.color = 'var(--muted)';
    info.textContent = ` ${t.Partitions}p · ${Math.round((t.RetentionPeriod || 0) / 3600000000000)}h ret`;

    row.append(name, info);
    return row;
  });
  el.replaceChildren(...rows);
}

// ---- Publish ---------------------------------------------------------------
async function publishMessage() {
  const topic = document.getElementById('pubTopic').value.trim();
  const key = document.getElementById('pubKey').value.trim();
  const payload = document.getElementById('pubPayload').value.trim();
  if (!topic || !payload) { log('error', 'topic and payload required'); return; }

  const { ok, data } = await api('POST', `/v1/topics/${encodeURIComponent(topic)}/messages`, { key, payload });
  if (ok) {
    log('pub', `→ ${topic} p${data.partition} off=${data.offset} id=${data.id.slice(0,12)}…`);
    triggerFlowParticle('producer', 'broker', '#6c63ff');
    refreshStats();
    refreshDepth(topic);
    animateAlgoStep(0);
  } else {
    log('error', data.error || 'publish failed');
  }
}

async function publishBurst() {
  const topic = document.getElementById('pubTopic').value.trim() || 'orders';
  for (let i = 0; i < 10; i++) {
    const key = `user-${Math.floor(Math.random() * 5)}`;
    const payload = JSON.stringify({ item: `item-${i}`, qty: Math.ceil(Math.random() * 5) });
    await api('POST', `/v1/topics/${encodeURIComponent(topic)}/messages`, { key, payload });
    triggerFlowParticle('producer', 'broker', '#6c63ff');
    await sleep(60);
  }
  log('pub', `burst of 10 to ${topic}`);
  refreshStats();
  refreshDepth(topic);
}

// ---- Poll ------------------------------------------------------------------
async function pollMessages() {
  const topic = document.getElementById('pollTopic').value.trim();
  const consumer_group = document.getElementById('pollGroup').value.trim();
  const max_messages = parseInt(document.getElementById('pollMax').value) || 5;
  const visibility_timeout_seconds = parseInt(document.getElementById('pollVt').value) || 30;
  if (!topic || !consumer_group) { log('error', 'topic and consumer_group required'); return; }

  const { ok, data } = await api('POST',
    `/v1/topics/${encodeURIComponent(topic)}/messages:poll`,
    { consumer_group, partition: -1, max_messages, visibility_timeout_seconds });

  if (ok) {
    const msgs = data.messages || [];
    state.polledMessages = msgs;
    log('poll', `← ${msgs.length} msgs from ${topic} [${consumer_group}]`);
    renderPolledList(msgs);
    msgs.forEach(() => triggerFlowParticle('broker', 'consumer', '#00d4aa'));
    refreshStats();
    animateAlgoStep(1);
  } else {
    log('error', data.error || 'poll failed');
  }
}

function renderPolledList(msgs) {
  const el = document.getElementById('polledList');
  if (!msgs.length) {
    const empty = document.createElement('span');
    empty.style.color = 'var(--muted)';
    empty.textContent = 'No messages.';
    el.replaceChildren(empty);
    return;
  }
  const rows = msgs.map((m, i) => {
    const row = document.createElement('div');
    row.style.cssText = 'padding:3px 0;border-bottom:1px solid var(--border);display:flex;align-items:center;gap:6px';

    const pLabel = document.createElement('span');
    pLabel.style.cssText = 'color:var(--muted);font-size:10px';
    pLabel.textContent = `p${m.partition}`;

    const payloadSpan = document.createElement('span');
    payloadSpan.style.cssText = 'flex:1;color:var(--text);overflow:hidden;text-overflow:ellipsis;white-space:nowrap';
    payloadSpan.textContent = m.payload;

    const ackBtn = document.createElement('button');
    ackBtn.className = 'btn btn-success btn-sm';
    ackBtn.textContent = 'Ack';
    ackBtn.addEventListener('click', () => ackOne(i));

    row.append(pLabel, payloadSpan, ackBtn);
    return row;
  });
  el.replaceChildren(...rows);
}

async function ackOne(idx) {
  const m = state.polledMessages[idx];
  if (!m) return;
  const group = document.getElementById('pollGroup').value.trim() || 'billing-service';
  const { ok, data } = await api('POST', `/v1/messages/${encodeURIComponent(m.id)}:ack`, { consumer_group: group });
  if (ok) {
    log('ack', `✓ ${m.id.slice(0, 14)}… p${m.partition}`);
    state.polledMessages.splice(idx, 1);
    renderPolledList(state.polledMessages);
    triggerFlowParticle('consumer', 'broker', '#48bb78');
    refreshStats();
    animateAlgoStep(2);
  } else {
    log('error', data.error || 'ack failed');
  }
}

async function ackAll() {
  const group = document.getElementById('pollGroup').value.trim() || 'billing-service';
  const msgs = [...state.polledMessages];
  for (const m of msgs) {
    await api('POST', `/v1/messages/${encodeURIComponent(m.id)}:ack`, { consumer_group: group });
    triggerFlowParticle('consumer', 'broker', '#48bb78');
  }
  log('ack', `acked ${msgs.length} messages`);
  state.polledMessages = [];
  renderPolledList([]);
  refreshStats();
}

// ---- Failure demos ---------------------------------------------------------
async function demoNoAck() {
  // Publish a message, poll it (making it invisible), then don't ack.
  // The reaper will restore it after the visibility timeout.
  const topic = document.getElementById('pubTopic').value.trim() || 'orders';
  await api('POST', `/v1/topics/${encodeURIComponent(topic)}/messages`,
    { key: 'crash-demo', payload: '{"demo":"no-ack crash"}' });
  const { ok, data } = await api('POST',
    `/v1/topics/${encodeURIComponent(topic)}/messages:poll`,
    { consumer_group: 'crash-demo-group', partition: -1, max_messages: 1, visibility_timeout_seconds: 10 });
  if (ok && data.messages && data.messages.length) {
    log('info', `[demo] polled 1 msg — NOT acking. Reaper restores after 10s (delivery_attempts+1).`);
    log('info', `[demo] Watch delivery_attempts increment on next poll.`);
  } else {
    log('error', '[demo] no messages to poll — publish something first');
  }
}

async function demoPoisonMsg() {
  const topic = document.getElementById('pubTopic').value.trim() || 'orders';
  for (let i = 0; i < 5; i++) {
    await api('POST', `/v1/topics/${encodeURIComponent(topic)}/messages`,
      { key: `poison-${Date.now()}`, payload: `{"poison":true,"attempt":${i+1}}` });
  }
  log('info', '[demo] Published 5 msgs. Poll + don\'t-ack 5 times to trigger DLQ promotion.');
}

async function demoViewDLQ() {
  const topic = document.getElementById('pubTopic').value.trim() || 'orders';
  const { ok, data } = await api('GET', `/v1/topics/${encodeURIComponent(topic)}/dlq?limit=10`);
  if (ok) {
    log('dlq', `DLQ for "${topic}": ${data.count} messages`);
    (data.messages || []).forEach(m => {
      log('dlq', `  id=${m.ID ? m.ID.slice(0,12) : '?'}… attempts=${m.DeliveryAttempts} p${m.Partition}`);
    });
    refreshStats();
  } else {
    log('error', data.error || 'DLQ query failed');
  }
}

async function demoStats() {
  await refreshStats();
  const s = state.stats;
  log('info', `total=${s.total_messages} pending=${s.pending_messages} inflight=${s.inflight_messages} acked=${s.acked_messages} dlq=${s.dlq_messages}`);
}

// ---- Stats refresh ---------------------------------------------------------
async function refreshStats() {
  const { ok, data } = await api('GET', '/v1/stats');
  if (!ok) return;
  state.stats = data;
  document.getElementById('statTotal').textContent = data.total_messages || 0;
  document.getElementById('statPending').textContent = data.pending_messages || 0;
  document.getElementById('statInflight').textContent = data.inflight_messages || 0;
  document.getElementById('statAcked').textContent = data.acked_messages || 0;
  document.getElementById('statDLQ').textContent = data.dlq_messages || 0;
}

async function refreshDepth(topic) {
  if (!topic) return;
  const { ok, data } = await api('GET', `/v1/topics/${encodeURIComponent(topic)}/depth`);
  if (!ok) return;
  state.depthData[topic] = data;
  if (state.currentViz === 'depth') drawDepthViz(topic);
}

// ---- Viz switching ---------------------------------------------------------
function switchViz(name) {
  state.currentViz = name;
  document.querySelectorAll('.tab-btn').forEach(b => {
    b.classList.toggle('active', b.textContent.toLowerCase().replace(/\s+/g, '') === name.replace(/\s+/g, ''));
  });

  const titles = {
    topology: 'Queue Topology',
    flow: 'Message Flow',
    algo: 'Algorithm Walk-through',
    depth: 'Partition Depth',
  };
  document.getElementById('vizTitle').textContent = titles[name] || name;

  const body = document.getElementById('vizBody');
  // Remove any non-canvas children first
  Array.from(body.children).forEach(c => { if (c !== canvas) c.remove(); });
  canvas.style.display = 'block';

  if (name === 'algo') {
    canvas.style.display = 'none';
    buildAlgoPanel(body);
    return;
  }
  if (name === 'depth') {
    const t = document.getElementById('pubTopic').value.trim() || (state.topics[0] && state.topics[0].Name) || 'orders';
    refreshDepth(t);
    return;
  }
  drawFrame();
}

// ---- Animation loop --------------------------------------------------------
function drawFrame() {
  cancelAnimationFrame(state.animFrame);
  const loop = () => {
    if (state.currentViz === 'topology') drawTopology();
    else if (state.currentViz === 'flow') drawFlow();
    else if (state.currentViz === 'depth') {
      const t = document.getElementById('pubTopic').value.trim() || 'orders';
      drawDepthViz(t);
    }
    state.animFrame = requestAnimationFrame(loop);
  };
  state.animFrame = requestAnimationFrame(loop);
}

// ---- Topology viz ----------------------------------------------------------
const NODES = {
  producer: { label: 'Producer', x: 0.12, y: 0.5, color: '#6c63ff' },
  broker:   { label: 'Broker', x: 0.5, y: 0.5, color: '#00d4aa' },
  consumer: { label: 'Consumer', x: 0.88, y: 0.5, color: '#ffd166' },
  db:       { label: 'PostgreSQL', x: 0.5, y: 0.85, color: '#4a90d9' },
  redis:    { label: 'Redis', x: 0.5, y: 0.15, color: '#ff6b6b' },
  dlq:      { label: 'DLQ', x: 0.78, y: 0.82, color: '#fc8181' },
};

const EDGES = [
  ['producer', 'broker', 'publish'],
  ['broker', 'consumer', 'poll'],
  ['consumer', 'broker', 'ack'],
  ['broker', 'db', 'persist'],
  ['broker', 'redis', 'cache'],
  ['broker', 'dlq', '5 retries'],
];

function drawTopology() {
  const w = canvas.width, h = canvas.height;
  ctx.clearRect(0, 0, w, h);

  // Draw edges
  ctx.lineWidth = 1.5;
  EDGES.forEach(([from, to, label]) => {
    const a = NODES[from], b = NODES[to];
    const x1 = a.x * w, y1 = a.y * h, x2 = b.x * w, y2 = b.y * h;
    ctx.beginPath();
    ctx.strokeStyle = 'rgba(255,255,255,0.12)';
    ctx.setLineDash([4, 4]);
    ctx.moveTo(x1, y1);
    ctx.lineTo(x2, y2);
    ctx.stroke();
    ctx.setLineDash([]);
    // Edge label
    const mx = (x1 + x2) / 2, my = (y1 + y2) / 2;
    ctx.fillStyle = 'rgba(255,255,255,0.35)';
    ctx.font = '10px monospace';
    ctx.textAlign = 'center';
    ctx.fillText(label, mx, my - 6);
  });

  // Draw particles
  updateParticles(w, h);

  // Draw nodes
  Object.entries(NODES).forEach(([, n]) => {
    const x = n.x * w, y = n.y * h;
    const r = 34;
    ctx.beginPath();
    ctx.arc(x, y, r, 0, Math.PI * 2);
    ctx.fillStyle = n.color + '22';
    ctx.fill();
    ctx.strokeStyle = n.color;
    ctx.lineWidth = 2;
    ctx.stroke();
    ctx.fillStyle = n.color;
    ctx.font = 'bold 11px monospace';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(n.label, x, y);
    ctx.textBaseline = 'alphabetic';
  });

  // Draw partition indicators inside broker node
  const bx = NODES.broker.x * w, by = NODES.broker.y * h;
  const topic = document.getElementById('pubTopic').value.trim() || 'orders';
  const topicObj = state.topics.find(t => t.Name === topic);
  const parts = topicObj ? topicObj.Partitions : 3;
  const partW = 10, gap = 4;
  const totalW = parts * (partW + gap) - gap;
  for (let i = 0; i < parts; i++) {
    const px = bx - totalW / 2 + i * (partW + gap);
    const depth = (state.depthData[topic] && state.depthData[topic].partitions && state.depthData[topic].partitions[i]) || 0;
    const barH = Math.min(depth * 3 + 4, 24);
    ctx.fillStyle = depth > 0 ? '#6c63ff' : 'rgba(255,255,255,0.1)';
    ctx.fillRect(px, by + 40, partW, barH);
    ctx.fillStyle = 'rgba(255,255,255,0.4)';
    ctx.font = '8px monospace';
    ctx.textAlign = 'center';
    ctx.fillText(`p${i}`, px + partW / 2, by + 38);
  }
}

// ---- Flow viz --------------------------------------------------------------
function drawFlow() {
  const w = canvas.width, h = canvas.height;
  ctx.clearRect(0, 0, w, h);

  // Static labels
  const zones = [
    { label: 'PRODUCER', x: 0.1, color: '#6c63ff' },
    { label: 'BROKER / PARTITIONS', x: 0.5, color: '#00d4aa' },
    { label: 'CONSUMER', x: 0.9, color: '#ffd166' },
  ];
  zones.forEach(z => {
    ctx.fillStyle = z.color + '15';
    const zw = w * 0.25;
    ctx.fillRect(z.x * w - zw / 2, 0, zw, h);
    ctx.fillStyle = z.color;
    ctx.font = 'bold 11px monospace';
    ctx.textAlign = 'center';
    ctx.fillText(z.label, z.x * w, 22);
  });

  // Partition lanes
  const topicObj = state.topics.find(t => t.Name === (document.getElementById('pubTopic').value.trim() || 'orders'));
  const parts = topicObj ? topicObj.Partitions : 3;
  const laneH = (h - 60) / parts;
  for (let i = 0; i < parts; i++) {
    const ly = 40 + i * laneH;
    ctx.strokeStyle = 'rgba(255,255,255,0.06)';
    ctx.lineWidth = 1;
    ctx.setLineDash([3, 6]);
    ctx.beginPath();
    ctx.moveTo(w * 0.25, ly + laneH / 2);
    ctx.lineTo(w * 0.75, ly + laneH / 2);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = 'rgba(255,255,255,0.2)';
    ctx.font = '10px monospace';
    ctx.textAlign = 'left';
    ctx.fillText(`p${i}`, w * 0.27, ly + laneH / 2 + 4);

    // Depth bar
    const depth = (state.depthData[document.getElementById('pubTopic').value.trim()] &&
      state.depthData[document.getElementById('pubTopic').value.trim()].partitions &&
      state.depthData[document.getElementById('pubTopic').value.trim()].partitions[i]) || 0;
    if (depth > 0) {
      const barW = Math.min(depth * 8, w * 0.35);
      const grad = ctx.createLinearGradient(w * 0.3, 0, w * 0.3 + barW, 0);
      grad.addColorStop(0, '#6c63ff88');
      grad.addColorStop(1, '#6c63ff11');
      ctx.fillStyle = grad;
      ctx.fillRect(w * 0.3, ly + laneH / 2 - 8, barW, 16);
      ctx.fillStyle = '#6c63ff';
      ctx.font = '9px monospace';
      ctx.fillText(`${depth} msgs`, w * 0.32, ly + laneH / 2 + 3);
    }
  }

  updateParticles(w, h);
}

// ---- Particle system -------------------------------------------------------
function triggerFlowParticle(from, to, color) {
  const particle = {
    from, to, color,
    progress: 0,
    speed: 0.012 + Math.random() * 0.008,
  };
  state.flowParticles.push(particle);
}

function updateParticles(w, h) {
  state.flowParticles = state.flowParticles.filter(p => p.progress < 1);
  state.flowParticles.forEach(p => {
    p.progress = Math.min(p.progress + p.speed, 1);
    const a = NODES[p.from] || NODES.producer;
    const b = NODES[p.to] || NODES.broker;
    const x = (a.x + (b.x - a.x) * p.progress) * w;
    const y = (a.y + (b.y - a.y) * p.progress) * h;
    const alpha = p.progress < 0.1 ? p.progress * 10
                : p.progress > 0.9 ? (1 - p.progress) * 10 : 1;
    ctx.beginPath();
    ctx.arc(x, y, 5, 0, Math.PI * 2);
    ctx.fillStyle = p.color + Math.floor(alpha * 255).toString(16).padStart(2, '0');
    ctx.fill();
    // Trail
    ctx.beginPath();
    const tx = (a.x + (b.x - a.x) * Math.max(0, p.progress - 0.08)) * w;
    const ty = (a.y + (b.y - a.y) * Math.max(0, p.progress - 0.08)) * h;
    ctx.strokeStyle = p.color + '44';
    ctx.lineWidth = 2;
    ctx.moveTo(tx, ty);
    ctx.lineTo(x, y);
    ctx.stroke();
  });
}

// ---- Depth viz (bar chart) -------------------------------------------------
function drawDepthViz(topic) {
  const w = canvas.width, h = canvas.height;
  ctx.clearRect(0, 0, w, h);

  const data = state.depthData[topic];
  if (!data) {
    ctx.fillStyle = 'rgba(255,255,255,0.3)';
    ctx.font = '13px monospace';
    ctx.textAlign = 'center';
    ctx.fillText('No depth data — publish some messages', w / 2, h / 2);
    return;
  }

  const parts = Object.keys(data.partitions || {}).map(Number).sort((a, b) => a - b);
  if (!parts.length) {
    ctx.fillStyle = 'rgba(255,255,255,0.3)';
    ctx.font = '13px monospace';
    ctx.textAlign = 'center';
    ctx.fillText('Queue empty', w / 2, h / 2);
    return;
  }

  const maxDepth = Math.max(...parts.map(p => data.partitions[p] || 0), 1);
  const barW = Math.min(60, (w - 60) / (parts.length + 1));
  const gap = barW * 0.4;
  const totalW = parts.length * (barW + gap) - gap;
  const startX = (w - totalW) / 2;
  const maxBarH = h - 80;

  ctx.fillStyle = 'rgba(255,255,255,0.5)';
  ctx.font = 'bold 12px monospace';
  ctx.textAlign = 'center';
  ctx.fillText(`Topic: ${topic}`, w / 2, 22);

  parts.forEach((p, i) => {
    const depth = data.partitions[p] || 0;
    const bh = (depth / maxDepth) * maxBarH;
    const bx = startX + i * (barW + gap);
    const by = h - 40 - bh;

    const grad = ctx.createLinearGradient(0, by, 0, by + bh);
    grad.addColorStop(0, '#6c63ff');
    grad.addColorStop(1, '#6c63ff44');
    ctx.fillStyle = grad;
    ctx.fillRect(bx, by, barW, bh);

    ctx.fillStyle = '#6c63ff';
    ctx.font = 'bold 13px monospace';
    ctx.fillText(depth, bx + barW / 2, by - 6);

    ctx.fillStyle = 'rgba(255,255,255,0.5)';
    ctx.font = '11px monospace';
    ctx.fillText(`p${p}`, bx + barW / 2, h - 22);
  });

  // DLQ
  if (data.dlq > 0) {
    ctx.fillStyle = 'var(--accent3)';
    ctx.font = '12px monospace';
    ctx.fillText(`DLQ: ${data.dlq}`, w - 60, h - 22);
  }
}

// ---- Algorithm walk-through ------------------------------------------------
const ALGO_STEPS = [
  {
    title: 'Publish: select partition',
    detail: 'If message has a <code>key</code>, FNV-1a hash % partitions routes it deterministically ' +
            '→ all messages with the same key land on the same partition, preserving order. ' +
            'Empty key → atomic Redis INCR counter % partitions (round-robin).'
  },
  {
    title: 'Append to partition log',
    detail: 'Message is <code>INSERT</code>ed into PostgreSQL with <code>visible_at = NOW()</code>. ' +
            'The <code>offset</code> column is a BIGINT <code>SERIAL</code> — the table is the log segment. ' +
            'No update/delete on publish: the log is append-only.'
  },
  {
    title: 'Poll: acquire visibility lease',
    detail: 'Consumer sends <code>POST …:poll</code>. Server runs a CTE with <code>FOR UPDATE SKIP LOCKED</code> ' +
            'to atomically select up to N visible messages and set <code>visible_at = NOW() + timeout</code>. ' +
            'Concurrent pollers never see the same message (row-level locking).'
  },
  {
    title: 'Consumer processes & acks',
    detail: 'Consumer processes its batch. For each message, <code>POST …:ack</code> sets <code>acked_at = NOW()</code>. ' +
            'Acked messages are excluded from future polls. If the consumer crashes before acking, ' +
            'the visibility timeout expires and the message becomes visible again.'
  },
  {
    title: 'Reaper restores expired leases',
    detail: 'A background goroutine runs every 5 s. It finds messages where <code>visible_at &lt;= NOW()</code> ' +
            'and <code>acked_at IS NULL</code>. Messages with <code>delivery_attempts &lt; 5</code> are reset ' +
            'to <code>visible_at = NOW()</code> (re-deliverable). The consumer_group is cleared so any group may re-consume.'
  },
  {
    title: 'DLQ promotion',
    detail: 'Before restoring, the reaper promotes messages with <code>delivery_attempts &gt;= 5</code> to ' +
            '<code>dead_lettered = true</code>. They are permanently excluded from normal polls. ' +
            'Operators can inspect the DLQ via <code>GET /v1/topics/{topic}/dlq</code> and decide to replay or discard.'
  },
];

// parseDetail splits a developer-authored detail string on <code>…</code> tokens
// and returns an array of text nodes and <code> elements. No user data flows here.
function parseDetail(detail) {
  const nodes = [];
  const parts = detail.split(/(<code>|<\/code>)/);
  let inCode = false;
  parts.forEach(part => {
    if (part === '<code>') { inCode = true; return; }
    if (part === '</code>') { inCode = false; return; }
    if (!part) return;
    if (inCode) {
      const c = document.createElement('code');
      c.textContent = part;
      nodes.push(c);
    } else {
      nodes.push(document.createTextNode(part));
    }
  });
  return nodes;
}

function buildAlgoPanel(body) {
  const panel = document.createElement('div');
  panel.className = 'algo-panel';
  panel.id = 'algoPanel';

  ALGO_STEPS.forEach((s, i) => {
    const step = document.createElement('div');
    step.className = 'algo-step' + (i === state.algoStep ? ' active' : '');
    step.id = `astep${i}`;

    const num = document.createElement('div');
    num.className = 'algo-step-num';
    num.textContent = String(i + 1);

    const text = document.createElement('div');
    text.className = 'algo-step-text';

    const strong = document.createElement('strong');
    strong.textContent = s.title;
    text.appendChild(strong);
    text.appendChild(document.createElement('br'));
    parseDetail(s.detail).forEach(n => text.appendChild(n));

    step.append(num, text);
    panel.appendChild(step);
  });

  body.appendChild(panel);
}

function animateAlgoStep(step) {
  state.algoStep = step % ALGO_STEPS.length;
  const panel = document.getElementById('algoPanel');
  if (!panel) return;
  panel.querySelectorAll('.algo-step').forEach((el, i) => {
    el.classList.toggle('active', i === state.algoStep);
  });
}

// ---- Polling loop ----------------------------------------------------------
setInterval(async () => {
  await refreshStats();
  const topic = document.getElementById('pubTopic').value.trim() || 'orders';
  await refreshDepth(topic);
}, 3000);

setInterval(loadTopics, 10000);

// ---- Utility ---------------------------------------------------------------
function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

// ---- Init ------------------------------------------------------------------
(async function init() {
  await loadTopics();
  await refreshStats();
  drawFrame();

  // Health check
  const { ok } = await api('GET', '/healthz');
  const dot = document.getElementById('statusDot');
  dot.style.background = ok ? 'var(--success)' : 'var(--error)';
  log('info', ok ? 'service healthy' : 'service unreachable');
})();

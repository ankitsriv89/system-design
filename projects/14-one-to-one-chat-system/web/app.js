/* app.js — 14-one-to-one-chat-system tutorial frontend */
'use strict';

// ── SockJS + STOMP via CDN ────────────────────────────────────────────────────
// Loaded lazily the first time a user tries to connect.
let StompLib = null;
async function loadStomp() {
  if (StompLib) return;
  await new Promise((resolve, reject) => {
    const s1 = document.createElement('script');
    s1.src = 'https://cdn.jsdelivr.net/npm/sockjs-client@1.6.1/dist/sockjs.min.js';
    s1.onload = () => {
      const s2 = document.createElement('script');
      s2.src = 'https://cdn.jsdelivr.net/npm/@stomp/stompjs@7.0.0/bundles/stomp.umd.min.js';
      s2.onload = () => { StompLib = window.StompJs; resolve(); };
      s2.onerror = reject;
      document.head.appendChild(s2);
    };
    s1.onerror = reject;
    document.head.appendChild(s1);
  });
}

// ── State ─────────────────────────────────────────────────────────────────────
const state = {
  users: { a: null, b: null },   // { userId, token, stompClient, convId }
  messages: [],
  packets: [],                    // canvas animation queue
};

// ── Canvas setup ──────────────────────────────────────────────────────────────
const canvas  = document.getElementById('flow-canvas');
const ctx     = canvas.getContext('2d');
let RAF_ID    = null;

function resizeCanvas() {
  canvas.width  = canvas.offsetWidth  * devicePixelRatio;
  canvas.height = canvas.offsetHeight * devicePixelRatio;
  ctx.scale(devicePixelRatio, devicePixelRatio);
}
window.addEventListener('resize', resizeCanvas);
resizeCanvas();

// Node positions (logical pixels)
function nodes() {
  const w = canvas.offsetWidth, h = canvas.offsetHeight;
  return {
    clientA: { x: 55,     y: h / 2, label: 'User A', color: '#58a6ff' },
    server:  { x: w / 2,  y: h / 2, label: 'Server Hub', color: '#bc8cff' },
    kafka:   { x: w / 2,  y: h * 0.18, label: 'Kafka', color: '#d29922' },
    pg:      { x: w / 2,  y: h * 0.82, label: 'Postgres', color: '#3fb950' },
    clientB: { x: w - 55, y: h / 2, label: 'User B', color: '#f78166' },
  };
}

function drawNode(n, status) {
  ctx.beginPath();
  ctx.arc(n.x, n.y, 22, 0, Math.PI * 2);
  ctx.fillStyle = status === 'online' ? n.color : '#30363d';
  ctx.fill();
  ctx.strokeStyle = n.color;
  ctx.lineWidth = 2;
  ctx.stroke();
  ctx.fillStyle = status === 'online' ? '#0d1117' : '#6e7681';
  ctx.font = '10px Segoe UI, sans-serif';
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.fillText(n.label, n.x, n.y);
}

function drawEdge(from, to, color, opacity) {
  ctx.beginPath();
  ctx.moveTo(from.x, from.y);
  ctx.lineTo(to.x, to.y);
  ctx.strokeStyle = color || '#30363d';
  ctx.globalAlpha = opacity || 0.3;
  ctx.lineWidth = 1;
  ctx.setLineDash([4, 4]);
  ctx.stroke();
  ctx.setLineDash([]);
  ctx.globalAlpha = 1;
}

function drawPacket(p) {
  ctx.beginPath();
  ctx.arc(p.x, p.y, 5, 0, Math.PI * 2);
  ctx.fillStyle = p.color;
  ctx.globalAlpha = p.opacity;
  ctx.fill();
  ctx.globalAlpha = 1;
}

function render(_ts) {
  const w = canvas.offsetWidth, h = canvas.offsetHeight;
  ctx.clearRect(0, 0, w, h);
  const n = nodes();

  // Draw edges
  drawEdge(n.clientA, n.server, '#58a6ff', 0.2);
  drawEdge(n.clientB, n.server, '#f78166', 0.2);
  drawEdge(n.server, n.kafka,   '#d29922', 0.2);
  drawEdge(n.server, n.pg,      '#3fb950', 0.2);

  // Draw nodes
  const aOnline = state.users.a?.stompClient?.active;
  const bOnline = state.users.b?.stompClient?.active;
  drawNode(n.clientA, aOnline ? 'online' : 'offline');
  drawNode(n.server,  'online');
  drawNode(n.kafka,   'online');
  drawNode(n.pg,      'online');
  drawNode(n.clientB, bOnline ? 'online' : 'offline');

  // Animate packets
  const now = performance.now();
  state.packets = state.packets.filter(p => {
    const t = Math.min(1, (now - p.startAt) / p.duration);
    p.x = p.fromX + (p.toX - p.fromX) * easeOut(t);
    p.y = p.fromY + (p.toY - p.fromY) * easeOut(t);
    p.opacity = t < 0.9 ? 1 : 1 - (t - 0.9) * 10;
    drawPacket(p);
    return t < 1;
  });

  RAF_ID = requestAnimationFrame(render);
}
RAF_ID = requestAnimationFrame(render);

function easeOut(t) { return 1 - Math.pow(1 - t, 3); }

function spawnPacket(fromKey, toKey, color) {
  const n = nodes();
  const from = n[fromKey], to = n[toKey];
  state.packets.push({
    fromX: from.x, fromY: from.y,
    toX: to.x, toY: to.y,
    x: from.x, y: from.y,
    color: color || '#58a6ff',
    opacity: 1,
    startAt: performance.now(),
    duration: 600,
  });
}

function animateSend(sender) {
  const color = sender === 'a' ? '#58a6ff' : '#f78166';
  const clientKey = sender === 'a' ? 'clientA' : 'clientB';
  spawnPacket(clientKey, 'server', color);
  setTimeout(() => spawnPacket('server', 'pg', '#3fb950'), 150);
  setTimeout(() => spawnPacket('server', 'kafka', '#d29922'), 200);
  const recipientKey = sender === 'a' ? 'clientB' : 'clientA';
  setTimeout(() => spawnPacket('kafka', 'server', '#d29922'), 500);
  setTimeout(() => spawnPacket('server', recipientKey, color), 650);
}

// ── Logging ───────────────────────────────────────────────────────────────────
const logEl = document.getElementById('event-log');
function log(msg, type = 'info') {
  const entry = document.createElement('div');
  entry.className = `log-entry ${type}`;
  const time = new Date().toTimeString().slice(0, 8);
  const timeEl = document.createElement('span');
  timeEl.className = 'log-time';
  timeEl.textContent = time;
  const textEl = document.createElement('span');
  textEl.className = 'log-text';
  textEl.textContent = msg;
  entry.appendChild(timeEl);
  entry.appendChild(textEl);
  logEl.appendChild(entry);
  logEl.scrollTop = logEl.scrollHeight;
}

document.getElementById('clear-log-btn').addEventListener('click', () => {
  logEl.replaceChildren();
});

// ── Presence badges ───────────────────────────────────────────────────────────
function updateBadge(user, online) {
  const badge = document.getElementById(`badge-${user}`);
  const id = state.users[user]?.userId || (user === 'a' ? 'User A' : 'User B');
  badge.textContent = `${id} ●`;
  badge.className = `presence-badge ${online ? 'online' : 'offline'}`;
}

// ── Auth + STOMP connect ──────────────────────────────────────────────────────
async function getToken(userId) {
  const res = await fetch(`/api/v1/auth/token?userId=${encodeURIComponent(userId)}`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error(`Auth failed: ${res.status}`);
  const data = await res.json();
  return data.token;
}

async function connectUser(userKey) {
  await loadStomp();
  const inputId = userKey === 'a' ? 'user-a-id' : 'user-b-id';
  const userId = document.getElementById(inputId).value.trim();
  if (!userId) { log('Enter a user ID first', 'err'); return; }

  const statusEl = document.getElementById('conn-status');
  statusEl.textContent = `Connecting ${userId}…`;

  try {
    const token = await getToken(userId);
    log(`Token issued for ${userId}`, 'info');

    const client = new StompLib.Client({
      webSocketFactory: () => new SockJS('/ws'),
      connectHeaders: { Authorization: `Bearer ${token}` },
      reconnectDelay: 0,
      debug: () => {},
    });

    client.onConnect = () => {
      log(`${userId} connected via STOMP`, 'ws');
      updateBadge(userKey, true);
      statusEl.textContent = '';

      // Subscribe to personal inbox
      client.subscribe('/user/queue/inbox', (frame) => {
        const envelope = JSON.parse(frame.body);
        handleEnvelope(userKey, userId, envelope);
      });

      // Fetch missed messages if we have a conversation
      if (state.users[userKey === 'a' ? 'b' : 'a']?.convId) {
        fetchMissedMessages(userKey);
      }
    };

    client.onDisconnect = () => {
      log(`${userId} disconnected`, 'ws');
      updateBadge(userKey, false);
    };

    client.onStompError = (frame) => {
      log(`STOMP error for ${userId}: ${frame.headers.message}`, 'err');
    };

    client.activate();

    state.users[userKey] = { userId, token, stompClient: client, convId: null };

    // Create conversation between A and B if both are registered
    const otherKey = userKey === 'a' ? 'b' : 'a';
    if (state.users[otherKey]) {
      await ensureConversation();
    }

  } catch (e) {
    log(`Connect failed: ${e.message}`, 'err');
    document.getElementById('conn-status').textContent = e.message;
  }
}

async function ensureConversation() {
  const a = state.users.a, b = state.users.b;
  if (!a || !b) return;
  try {
    const res = await fetch(`/api/v1/conversations?recipientId=${encodeURIComponent(b.userId)}`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${a.token}` },
    });
    const conv = await res.json();
    state.users.a.convId = conv.id;
    state.users.b.convId = conv.id;
    log(`Conversation ${conv.id} ready (${a.userId} ↔ ${b.userId})`, 'info');

    // Load history
    await fetchHistory(conv.id, a.token, 'a');
    await fetchHistory(conv.id, b.token, 'b');
  } catch (e) {
    log(`Conversation error: ${e.message}`, 'err');
  }
}

async function fetchHistory(convId, token, userKey) {
  try {
    const res = await fetch(`/api/v1/conversations/${convId}/messages?limit=20`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    const msgs = await res.json();
    msgs.reverse().forEach(m => renderMessage(userKey, m));
  } catch (e) {
    log(`History fetch error: ${e.message}`, 'err');
  }
}

async function fetchMissedMessages(userKey) {
  const u = state.users[userKey];
  if (!u?.convId) return;
  await fetchHistory(u.convId, u.token, userKey);
}

// ── Send message ──────────────────────────────────────────────────────────────
async function sendMessage() {
  const senderKey = document.getElementById('sender-select').value;
  const body = document.getElementById('msg-body').value.trim();
  if (!body) return;
  document.getElementById('msg-body').value = '';

  const sender = state.users[senderKey];
  const otherKey = senderKey === 'a' ? 'b' : 'a';
  const other = state.users[otherKey];
  if (!sender) { log('Connect User ' + senderKey.toUpperCase() + ' first', 'err'); return; }
  if (!other)  { log('Connect both users first', 'err'); return; }

  // Use STOMP WS path if connected, else REST fallback
  if (sender.stompClient?.active) {
    sender.stompClient.publish({
      destination: '/app/chat.send',
      body: JSON.stringify({ recipientId: other.userId, body }),
    });
    log(`[WS] ${sender.userId} → ${other.userId}: "${body}"`, 'ws');
    animateSend(senderKey);
  } else {
    // REST fallback
    try {
      const res = await fetch(`/api/v1/conversations/${sender.convId}/messages`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${sender.token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ body }),
      });
      const msg = await res.json();
      log(`[REST] ${sender.userId} sent msg id=${msg.id}`, 'msg');
      renderMessage(senderKey, msg);
      animateSend(senderKey);
    } catch (e) {
      log(`Send error: ${e.message}`, 'err');
    }
  }
}

// ── Envelope handling ─────────────────────────────────────────────────────────
function handleEnvelope(userKey, userId, envelope) {
  const { type, payload } = envelope;
  if (type === 'MESSAGE') {
    log(`[WS→${userId}] msg id=${payload.id} from=${payload.senderId} seq=${payload.seq}`, 'msg');
    renderMessage(userKey, payload);
    animateSend(userKey === 'a' ? 'b' : 'a');  // reverse: packet arrives at this user
    sendDeliveryReceipt(userKey, payload.id);
  } else if (type === 'RECEIPT') {
    log(`[WS→${userId}] receipt msg=${payload.messageId} status=${payload.status}`, 'kafka');
    updateMessageStatus(payload.messageId, payload.status);
  } else if (type === 'PRESENCE') {
    log(`[WS→${userId}] presence ${payload.userId}=${payload.online ? 'online' : 'offline'}`, 'info');
  }
}

function sendDeliveryReceipt(userKey, messageId) {
  const u = state.users[userKey];
  if (u?.stompClient?.active) {
    u.stompClient.publish({
      destination: '/app/chat.receipt',
      body: JSON.stringify({ messageId, userId: u.userId, status: 'DELIVERED' }),
    });
  }
}

// ── Message rendering ─────────────────────────────────────────────────────────
const renderedIds = new Set();

function renderMessage(userKey, msg) {
  if (renderedIds.has(`${userKey}-${msg.id}`)) return;
  renderedIds.add(`${userKey}-${msg.id}`);

  const threadEl = document.getElementById(`messages-${userKey}`);
  const u = state.users[userKey];
  const isSent = u && msg.senderId === u.userId;

  const bubble = document.createElement('div');
  bubble.className = `msg-bubble ${isSent ? 'sent' : 'received'}`;
  bubble.dataset.msgId = msg.id;

  const bodyEl = document.createElement('div');
  bodyEl.textContent = msg.body;

  const meta = document.createElement('div');
  meta.className = 'meta';

  const seqEl = document.createElement('span');
  seqEl.textContent = `seq:${msg.seq}`;

  const statusEl = document.createElement('span');
  statusEl.className = `status-dot ${msg.status.toLowerCase()}`;
  statusEl.textContent = statusIcon(msg.status);
  statusEl.dataset.msgId = msg.id;

  meta.appendChild(seqEl);
  meta.appendChild(statusEl);
  bubble.appendChild(bodyEl);
  bubble.appendChild(meta);
  threadEl.appendChild(bubble);
  threadEl.scrollTop = threadEl.scrollHeight;
}

function statusIcon(status) {
  return status === 'READ' ? '✓✓' : status === 'DELIVERED' ? '✓✓' : '✓';
}

function updateMessageStatus(messageId, status) {
  document.querySelectorAll(`[data-msg-id="${messageId}"]`).forEach(el => {
    if (el.classList.contains('status-dot')) {
      el.className = `status-dot ${status.toLowerCase()}`;
      el.textContent = statusIcon(status);
    }
  });
}

// ── Disconnect / reconnect ────────────────────────────────────────────────────
function disconnectUser(userKey) {
  const u = state.users[userKey];
  if (!u) return;
  u.stompClient?.deactivate();
  updateBadge(userKey, false);
  log(`${u.userId} forcibly disconnected (simulating offline)`, 'ws');
}

async function reconnectUser(userKey) {
  const u = state.users[userKey];
  if (!u) { log('Connect User A first', 'err'); return; }
  log(`Reconnecting ${u.userId}…`, 'info');
  u.stompClient?.activate();
}

// ── Presence polling ──────────────────────────────────────────────────────────
async function pollPresence() {
  const a = state.users.a, b = state.users.b;
  if (!a || !b) return;
  try {
    const res = await fetch(`/api/v1/presence?users=${a.userId},${b.userId}`, {
      headers: { Authorization: `Bearer ${a.token}` },
    });
    const list = await res.json();
    list.forEach(p => {
      const key = p.userId === a.userId ? 'a' : 'b';
      updateBadge(key, p.online);
    });
  } catch (_) {}
}
setInterval(pollPresence, 5000);

// ── Wire up buttons ───────────────────────────────────────────────────────────
document.getElementById('connect-a-btn').addEventListener('click', () => connectUser('a'));
document.getElementById('connect-b-btn').addEventListener('click', () => connectUser('b'));
document.getElementById('send-btn').addEventListener('click', sendMessage);
document.getElementById('disconnect-a-btn').addEventListener('click', () => disconnectUser('a'));
document.getElementById('reconnect-a-btn').addEventListener('click', () => reconnectUser('a'));

document.getElementById('msg-body').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') sendMessage();
});

log('Ready — connect both users to start chatting', 'info');

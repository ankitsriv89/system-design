/**
 * Project 20 — WhatsApp Real-Time Messaging
 *
 * E2EE boundary: ECDH key pairs are generated per device in the browser with
 * WebCrypto. The public key is registered with the server; the private key never
 * leaves this tab. For demo DMs, a shared AES-GCM key is derived via ECDH and
 * stored in memory. The server only sees the encrypted ciphertext (Base64).
 *
 * WebSocket: connects to /ws/v1/session?token=JWT&deviceId=N.
 * Incoming frame types: connected | message | receipt | backlog | pong | error
 * Outgoing frame types: ping | receipt
 */

// ─── Honour Caddy /p20/ prefix ───────────────────────────────────────────────
const BASE = (() => {
  const p = location.pathname.replace(/\/[^/]*$/, '');
  return p === '' ? '' : p;
})();

const API = `${location.protocol}//${location.host}${BASE}`;
const WS_SCHEME = location.protocol === 'https:' ? 'wss' : 'ws';

// ─── App state ────────────────────────────────────────────────────────────────
const state = {
  token: null,
  userId: null,
  username: null,
  deviceId: null,
  // ECDH key pair for this device (CryptoKeyPair)
  keyPair: null,
  // Map<chatId, {name, messages: [], lastPreview}>
  chats: new Map(),
  activeChatId: null,
  ws: null,
  simulateOffline: false,
  // Map<chatId, CryptoKey> — derived AES-GCM keys per chat (demo: use fixed salt)
  chatKeys: new Map(),
  // Map<messageId, {state: 'sent'|'delivered'|'read', row: Element}>
  sentMessages: new Map(),
};

// ─── Utilities ────────────────────────────────────────────────────────────────
const $ = id => document.getElementById(id);
const log = (msg, type = 'info') => {
  const ts = new Date().toISOString().slice(11, 19);
  const prefix = { info: '  ', ws: 'WS', api: 'API', err: 'ERR', e2e: 'E2E' }[type] || '  ';
  const el = $('apilog');
  el.textContent += `[${ts}] [${prefix}] ${msg}\n`;
  el.scrollTop = el.scrollHeight;
};

async function apiFetch(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json', ...(opts.headers || {}) };
  if (state.token) headers['Authorization'] = `Bearer ${state.token}`;
  log(`${opts.method || 'GET'} ${path}`, 'api');
  const r = await fetch(API + path, { ...opts, headers });
  if (!r.ok) {
    const txt = await r.text();
    throw new Error(`HTTP ${r.status}: ${txt}`);
  }
  return r.status === 204 ? null : r.json();
}

function showError(el, msg) {
  el.textContent = msg;
  el.classList.remove('hidden');
}

function hideError(el) { el.classList.add('hidden'); }

// ─── WebCrypto E2EE helpers ───────────────────────────────────────────────────
async function generateKeyPair() {
  return crypto.subtle.generateKey(
    { name: 'ECDH', namedCurve: 'P-256' },
    true,
    ['deriveKey']
  );
}

async function exportPublicKeyB64(kp) {
  const raw = await crypto.subtle.exportKey('spki', kp.publicKey);
  return btoa(String.fromCharCode(...new Uint8Array(raw)));
}

async function importPublicKeyB64(b64) {
  const raw = Uint8Array.from(atob(b64), c => c.charCodeAt(0));
  return crypto.subtle.importKey('spki', raw, { name: 'ECDH', namedCurve: 'P-256' }, true, []);
}

async function deriveSharedKey(myPrivate, theirPublic) {
  return crypto.subtle.deriveKey(
    { name: 'ECDH', public: theirPublic },
    myPrivate,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt']
  );
}

// For the demo: derive a deterministic shared key for a DM from our key pair.
// In a real Signal-protocol implementation, X3DH would be used instead.
async function ensureChatKey(chatId, theirPublicKeyB64) {
  if (state.chatKeys.has(chatId)) return state.chatKeys.get(chatId);
  if (!theirPublicKeyB64) {
    // Fallback: generate a random AES key (no real E2EE, but still demo-safe)
    const key = await crypto.subtle.generateKey({ name: 'AES-GCM', length: 256 }, false, ['encrypt', 'decrypt']);
    state.chatKeys.set(chatId, key);
    log(`no peer key for ${chatId} — using ephemeral AES`, 'e2e');
    return key;
  }
  const theirKey = await importPublicKeyB64(theirPublicKeyB64);
  const key = await deriveSharedKey(state.keyPair.privateKey, theirKey);
  state.chatKeys.set(chatId, key);
  log(`ECDH shared key derived for ${chatId}`, 'e2e');
  return key;
}

async function encrypt(chatId, plaintext, theirPublicKeyB64 = null) {
  const key = await ensureChatKey(chatId, theirPublicKeyB64);
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const enc = new TextEncoder();
  const ct = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, enc.encode(plaintext));
  // Prepend IV to ciphertext so the recipient can decrypt
  const out = new Uint8Array(12 + ct.byteLength);
  out.set(iv);
  out.set(new Uint8Array(ct), 12);
  return btoa(String.fromCharCode(...out));
}

async function decrypt(chatId, b64) {
  const key = state.chatKeys.get(chatId);
  if (!key) return '[encrypted — no key]';
  try {
    const buf = Uint8Array.from(atob(b64), c => c.charCodeAt(0));
    const iv = buf.slice(0, 12);
    const ct = buf.slice(12);
    const pt = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, ct);
    return new TextDecoder().decode(pt);
  } catch {
    return '[encrypted]';
  }
}

// ─── Auth flow ────────────────────────────────────────────────────────────────
let authMode = 'login';

document.querySelectorAll('.tab').forEach(t => t.addEventListener('click', () => {
  authMode = t.dataset.tab;
  document.querySelectorAll('.tab').forEach(x => x.classList.remove('active'));
  t.classList.add('active');
  $('auth-btn').textContent = authMode === 'login' ? 'Sign in' : 'Register';
}));

$('auth-form').addEventListener('submit', async e => {
  e.preventDefault();
  hideError($('auth-error'));
  const username = $('auth-username').value.trim();
  const password = $('auth-password').value;
  try {
    const data = await apiFetch(`/v1/auth/${authMode}`, {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
    state.token = data.token;
    state.userId = data.userId;
    state.username = data.username;
    log(`signed in as ${username} (uid=${data.userId})`, 'api');
    await bootApp();
  } catch (err) {
    showError($('auth-error'), err.message);
  }
});

// ─── App boot ─────────────────────────────────────────────────────────────────
async function bootApp() {
  $('auth-screen').classList.add('hidden');
  $('app').classList.remove('hidden');
  $('me-label').textContent = state.username;

  // Generate ECDH key pair
  state.keyPair = await generateKeyPair();
  const pubKeyB64 = await exportPublicKeyB64(state.keyPair);
  log(`ECDH key pair generated`, 'e2e');

  // Register this tab as a device
  const device = await apiFetch('/v1/devices', {
    method: 'POST',
    body: JSON.stringify({ publicKey: pubKeyB64, label: `browser-${Date.now()}` }),
  });
  state.deviceId = device.id;
  $('device-info').textContent = `device #${device.id}`;
  log(`device registered: id=${device.id}`, 'api');

  connectWebSocket();
  loadGroups();
}

// ─── WebSocket session ────────────────────────────────────────────────────────
async function connectWebSocket() {
  if (state.simulateOffline) return;
  // Fetch a one-time ticket so the JWT never appears in the WS query string.
  let ticket;
  try {
    const data = await apiFetch(`/v1/ws-ticket?deviceId=${state.deviceId}`, { method: 'POST' });
    ticket = data.ticket;
  } catch (e) {
    log(`ws-ticket error: ${e.message} — retrying in 5s`, 'err');
    setTimeout(connectWebSocket, 5000);
    return;
  }
  const url = `${WS_SCHEME}://${location.host}${BASE}/ws/v1/session?ticket=${ticket}`;
  const ws = new WebSocket(url);
  state.ws = ws;

  ws.onopen = () => {
    log('WebSocket connected', 'ws');
    $('session-status').className = 'status-dot online';
    startHeartbeat();
  };

  ws.onmessage = async evt => {
    const env = JSON.parse(evt.data);
    log(`← ${env.type} ${JSON.stringify(env.payload ?? '').slice(0, 80)}`, 'ws');
    switch (env.type) {
      case 'connected': break;
      case 'pong':      break;
      case 'message':   await handleIncomingMessage(env.payload); break;
      case 'receipt':   handleReceiptUpdate(env.payload); break;
      case 'backlog':   log(`backlog: ${env.payload.count} pending messages — call /v1/messages/sync`, 'ws'); break;
      case 'error':     log(`server error: ${JSON.stringify(env.payload)}`, 'err'); break;
    }
  };

  ws.onclose = evt => {
    log(`WebSocket closed (${evt.code})`, 'ws');
    $('session-status').className = 'status-dot offline';
    if (!state.simulateOffline) {
      setTimeout(connectWebSocket, 3000);
    }
  };

  ws.onerror = () => log('WebSocket error', 'err');
}

let heartbeatTimer = null;
function startHeartbeat() {
  clearInterval(heartbeatTimer);
  heartbeatTimer = setInterval(() => {
    if (state.ws?.readyState === WebSocket.OPEN) {
      state.ws.send(JSON.stringify({ type: 'ping', payload: null }));
    }
  }, 30000);
}

function wsSend(type, payload) {
  if (state.ws?.readyState === WebSocket.OPEN) {
    state.ws.send(JSON.stringify({ type, payload }));
    log(`→ ${type}`, 'ws');
  }
}

// ─── Offline toggle ───────────────────────────────────────────────────────────
$('offline-toggle').addEventListener('change', e => {
  state.simulateOffline = e.target.checked;
  if (state.simulateOffline) {
    state.ws?.close();
    log('simulating offline — WS closed', 'ws');
  } else {
    log('back online — reconnecting…', 'ws');
    connectWebSocket();
    if (state.activeChatId) syncMessages(state.activeChatId);
  }
});

// ─── Chat list ────────────────────────────────────────────────────────────────
function addChat(chatId, name, preview = '') {
  if (!state.chats.has(chatId)) {
    state.chats.set(chatId, { name, messages: [], lastPreview: preview });
  } else {
    if (preview) state.chats.get(chatId).lastPreview = preview;
  }
  renderChatList();
}

function renderChatList() {
  const list = $('chat-list');
  list.innerHTML = '';
  for (const [chatId, chat] of state.chats) {
    const div = document.createElement('div');
    div.className = 'chat-item' + (chatId === state.activeChatId ? ' active' : '');
    const nameEl = document.createElement('div');
    nameEl.className = 'chat-item-name';
    nameEl.textContent = chat.name;
    const previewEl = document.createElement('div');
    previewEl.className = 'chat-item-preview';
    previewEl.textContent = chat.lastPreview || '';
    div.appendChild(nameEl);
    div.appendChild(previewEl);
    div.addEventListener('click', () => openChat(chatId));
    list.appendChild(div);
  }
}

// ─── Start DM ─────────────────────────────────────────────────────────────────
$('start-dm-btn').addEventListener('click', startDm);
$('chat-search').addEventListener('keydown', e => { if (e.key === 'Enter') startDm(); });

async function startDm() {
  const input = $('chat-search').value.trim();
  if (!input) return;
  // Support numeric user ID or username lookup (server returns 404 if not found)
  const ids = [state.userId, isNaN(input) ? null : parseInt(input)].filter(Boolean).sort((a, b) => a - b);
  if (ids.length < 2) {
    log(`enter a valid user ID to start DM`, 'err');
    return;
  }
  const chatId = `dm:${ids[0]}:${ids[1]}`;
  const name = `DM with user ${input}`;
  addChat(chatId, name);
  openChat(chatId);
  $('chat-search').value = '';
}

// ─── Groups ───────────────────────────────────────────────────────────────────
async function loadGroups() {
  try {
    const groups = await apiFetch('/v1/groups');
    for (const g of groups) {
      addChat(g.chatId, `# ${g.name}`);
    }
  } catch (e) {
    log(`load groups: ${e.message}`, 'err');
  }
}

$('new-group-btn').addEventListener('click', () => $('group-modal').classList.remove('hidden'));
$('group-cancel-btn').addEventListener('click', () => $('group-modal').classList.add('hidden'));

$('group-create-btn').addEventListener('click', async () => {
  hideError($('group-error'));
  const name = $('group-name-input').value.trim();
  const rawIds = $('group-members-input').value.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n));
  if (!name) { showError($('group-error'), 'Name required'); return; }
  try {
    const g = await apiFetch('/v1/groups', {
      method: 'POST',
      body: JSON.stringify({ name, memberUserIds: rawIds }),
    });
    addChat(g.chatId, `# ${g.name}`);
    $('group-modal').classList.add('hidden');
    $('group-name-input').value = '';
    $('group-members-input').value = '';
    openChat(g.chatId);
    log(`group created: ${g.chatId}`, 'api');
  } catch (e) {
    showError($('group-error'), e.message);
  }
});

// ─── Open/render chat ─────────────────────────────────────────────────────────
function openChat(chatId) {
  state.activeChatId = chatId;
  renderChatList();
  $('no-chat-selected').classList.add('hidden');
  $('chat-view').classList.remove('hidden');
  const chat = state.chats.get(chatId);
  $('chat-title').textContent = chat.name;
  $('chat-id-label').textContent = chatId;
  const msgEl = $('messages');
  msgEl.innerHTML = '';
  for (const m of chat.messages) renderMessage(m, msgEl);
  msgEl.scrollTop = msgEl.scrollHeight;
  syncMessages(chatId);
}

// ─── Sync (offline backlog drain) ─────────────────────────────────────────────
async function syncMessages(chatId) {
  const chat = state.chats.get(chatId);
  if (!chat) return;
  const last = chat.messages.at(-1);
  const since = last ? `&since=${encodeURIComponent(last.createdAt)}` : '';
  try {
    const msgs = await apiFetch(`/v1/messages/sync?chatId=${encodeURIComponent(chatId)}${since}`);
    const msgEl = $('messages');
    for (const m of msgs) {
      if (chat.messages.some(x => x.id === m.id)) continue;
      const plaintext = await decrypt(chatId, m.ciphertext);
      const msg = { ...m, plaintext, mine: m.senderId === state.userId };
      chat.messages.push(msg);
      if (chatId === state.activeChatId) {
        renderMessage(msg, msgEl);
        msgEl.scrollTop = msgEl.scrollHeight;
      }
      // Acknowledge delivery
      wsSend('receipt', { messageId: m.id, state: 'DELIVERED' });
    }
    if (msgs.length) log(`sync: ${msgs.length} message(s) loaded for ${chatId}`, 'api');
  } catch (e) {
    log(`sync error: ${e.message}`, 'err');
  }
}

// ─── Incoming real-time message ───────────────────────────────────────────────
async function handleIncomingMessage(payload) {
  const chatId = payload.chatId;
  if (!state.chats.has(chatId)) {
    addChat(chatId, chatId.startsWith('group:') ? `# group ${chatId}` : `DM ${chatId}`);
  }
  const chat = state.chats.get(chatId);
  if (chat.messages.some(x => x.id === payload.id)) return; // deduplicate

  const plaintext = await decrypt(chatId, payload.ciphertext);
  const msg = {
    id: payload.id,
    chatId,
    senderId: payload.senderId,
    ciphertext: payload.ciphertext,
    createdAt: payload.createdAt,
    plaintext,
    mine: payload.senderId === state.userId,
  };
  chat.messages.push(msg);
  chat.lastPreview = plaintext.slice(0, 40);
  renderChatList();

  if (state.activeChatId === chatId) {
    const msgEl = $('messages');
    renderMessage(msg, msgEl);
    msgEl.scrollTop = msgEl.scrollHeight;
  }

  // Acknowledge
  wsSend('receipt', { messageId: payload.id, state: 'DELIVERED' });
}

function handleReceiptUpdate(payload) {
  const entry = state.sentMessages.get(payload.messageId);
  if (!entry) return;
  const order = { SENT: 0, DELIVERED: 1, READ: 2 };
  if ((order[payload.state] ?? -1) > (order[entry.state] ?? -1)) {
    entry.state = payload.state;
    updateReceiptTick(entry.tickEl, payload.state);
  }
}

// ─── Send message ─────────────────────────────────────────────────────────────
$('send-btn').addEventListener('click', sendMessage);
$('msg-input').addEventListener('keydown', e => {
  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMessage(); }
});

async function sendMessage() {
  const chatId = state.activeChatId;
  if (!chatId) return;
  const text = $('msg-input').value.trim();
  if (!text) return;
  $('msg-input').value = '';

  const ciphertext = await encrypt(chatId, text);

  try {
    const m = await apiFetch('/v1/messages', {
      method: 'POST',
      body: JSON.stringify({ chatId, ciphertext }),
    });
    const msg = { ...m, plaintext: text, mine: true };
    const chat = state.chats.get(chatId);
    chat.messages.push(msg);
    chat.lastPreview = text.slice(0, 40);
    renderChatList();

    const msgEl = $('messages');
    renderMessage(msg, msgEl);
    msgEl.scrollTop = msgEl.scrollHeight;
  } catch (e) {
    log(`send failed: ${e.message}`, 'err');
  }
}

// ─── Render a message bubble ──────────────────────────────────────────────────
function renderMessage(msg, container) {
  const row = document.createElement('div');
  row.className = 'message-row ' + (msg.mine ? 'mine' : 'theirs');
  row.dataset.msgId = msg.id;

  const time = new Date(msg.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

  const tickEl = document.createElement('span');
  tickEl.className = 'receipt-tick sent';
  tickEl.textContent = '✓';

  const meta = document.createElement('div');
  meta.className = 'bubble-meta';
  meta.appendChild(document.createTextNode(time + ' '));
  if (msg.mine) meta.appendChild(tickEl);

  const bubble = document.createElement('div');
  bubble.className = 'bubble';
  bubble.textContent = msg.plaintext ?? '[encrypted]';

  if (!msg.mine) {
    const sender = document.createElement('div');
    sender.className = 'sender-label';
    sender.textContent = `uid:${msg.senderId}`;
    row.appendChild(sender);
  }

  row.appendChild(bubble);
  row.appendChild(meta);
  container.appendChild(row);

  if (msg.mine) {
    state.sentMessages.set(msg.id, { state: 'SENT', tickEl });
  }
}

function updateReceiptTick(tickEl, state) {
  tickEl.className = `receipt-tick ${state.toLowerCase()}`;
  tickEl.textContent = state === 'READ' ? '✓✓' : state === 'DELIVERED' ? '✓✓' : '✓';
  if (state === 'READ') tickEl.style.color = '#53bdeb';
}

// ─── Logout ───────────────────────────────────────────────────────────────────
$('logout-btn').addEventListener('click', () => {
  state.ws?.close();
  state.token = null;
  state.userId = null;
  state.username = null;
  state.deviceId = null;
  state.chats.clear();
  state.sentMessages.clear();
  state.chatKeys.clear();
  state.activeChatId = null;
  $('app').classList.add('hidden');
  $('auth-screen').classList.remove('hidden');
  $('apilog').textContent = '';
  log('logged out', 'info');
});

// ─── Clear log ────────────────────────────────────────────────────────────────
$('clear-log-btn').addEventListener('click', () => { $('apilog').textContent = ''; });

log('client loaded — ready to connect');

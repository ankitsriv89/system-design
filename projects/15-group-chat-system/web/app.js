'use strict';

const BASE = '';
let token = null;
let userId = null;
let stompClient = null;
let currentGroupId = null;
const subscriptions = {};

const $ = id => document.getElementById(id);

function log(msg, type = 'req') {
  const box = $('logBox');
  const el = document.createElement('div');
  el.className = `log-entry ${type}`;
  el.textContent = `${new Date().toLocaleTimeString()} ${msg}`;
  box.appendChild(el);
  box.scrollTop = box.scrollHeight;
}

async function api(method, path, body) {
  log(`${method} ${path}`, 'req');
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (token) opts.headers['Authorization'] = `Bearer ${token}`;
  if (body) opts.body = JSON.stringify(body);
  const res = await fetch(BASE + path, opts);
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) { log(`${res.status} ${text}`, 'err'); throw new Error(text); }
  log(`${res.status} ${JSON.stringify(data).slice(0, 80)}`, 'res');
  return data;
}

function span(cls, text) {
  const el = document.createElement('span');
  el.className = cls;
  el.textContent = text;
  return el;
}

function appendMessage(msg, mine) {
  const box = $('chatBox');
  const el = document.createElement('div');
  el.className = `msg${mine ? ' mine' : ''}`;
  const ts = msg.createdAt ? new Date(msg.createdAt).toLocaleTimeString() : '';
  el.appendChild(span('sender', msg.senderId ?? msg.sender_id ?? '?'));
  el.appendChild(span('body', msg.body ?? msg.content ?? ''));
  el.appendChild(span('ts', ts));
  box.appendChild(el);
  box.scrollTop = box.scrollHeight;
}

function sysMsg(text) {
  const box = $('chatBox');
  const el = document.createElement('div');
  el.className = 'msg system';
  el.textContent = text;
  box.appendChild(el);
  box.scrollTop = box.scrollHeight;
}

async function connect() {
  const uid = $('userId').value.trim();
  if (!uid) return;
  userId = uid;
  const resp = await api('POST', '/api/auth/token', { userId: uid });
  token = resp.token;

  $('group-controls').style.display = 'flex';
  $('groups-area').style.display = 'flex';

  connectStomp();
  await loadGroups();
}

function connectStomp() {
  const sock = new SockJS('/ws');
  stompClient = Stomp.over(sock);
  stompClient.debug = () => {};
  stompClient.connect({ Authorization: `Bearer ${token}` }, () => {
    log('WS connected', 'ws');
  }, err => {
    log(`WS error: ${err}`, 'err');
  });
}

function subscribeToGroup(groupId) {
  if (!stompClient || subscriptions[groupId]) return;
  subscriptions[groupId] = stompClient.subscribe(`/topic/group/${groupId}`, frame => {
    const env = JSON.parse(frame.body);
    log(`WS ← ${JSON.stringify(env).slice(0, 60)}`, 'ws');
    if (env.type === 'message' && String(env.groupId) === String(currentGroupId)) {
      appendMessage({ senderId: env.senderId, body: env.body, createdAt: null }, env.senderId === userId);
    }
  });
  log(`WS subscribed /topic/group/${groupId}`, 'ws');
}

async function loadGroups() {
  const groups = await api('GET', '/api/groups');
  const sel = $('groupSelect');
  while (sel.firstChild) sel.removeChild(sel.firstChild);
  const placeholder = document.createElement('option');
  placeholder.value = '';
  placeholder.textContent = '— select —';
  sel.appendChild(placeholder);
  groups.forEach(g => {
    const opt = document.createElement('option');
    opt.value = g.id;
    opt.textContent = g.name;
    sel.appendChild(opt);
    subscribeToGroup(g.id);
  });
}

async function createGroup() {
  const name = $('groupName').value.trim();
  if (!name) return;
  const g = await api('POST', '/api/groups', { name });
  $('groupName').value = '';
  await loadGroups();
  $('groupSelect').value = g.id;
  await selectGroup(g.id);
}

async function selectGroup(groupId) {
  if (!groupId) return;
  currentGroupId = groupId;
  const box = $('chatBox');
  while (box.firstChild) box.removeChild(box.firstChild);
  subscribeToGroup(groupId);
  const history = await api('GET', `/api/groups/${groupId}/messages`);
  sysMsg(`— history (${history.length} messages) —`);
  [...history].reverse().forEach(m => appendMessage(m, m.senderId === userId));
  await updatePresence(groupId);
}

async function updatePresence(groupId) {
  try {
    const members = await api('GET', `/api/groups/${groupId}/members`);
    const userIds = members.map(m => m.userId).join('&userIds=');
    const p = await api('GET', `/api/presence?userIds=${userIds}`);
    const bar = $('presenceBar');
    while (bar.firstChild) bar.removeChild(bar.firstChild);
    bar.appendChild(document.createTextNode('Online: '));
    if (p.online.length) {
      p.online.forEach(u => bar.appendChild(span('', u)));
    } else {
      const em = document.createElement('em');
      em.textContent = 'none';
      bar.appendChild(em);
    }
  } catch (_) {}
}

async function sendMessage() {
  const body = $('msgInput').value.trim();
  if (!body || !currentGroupId) return;
  $('msgInput').value = '';
  await api('POST', `/api/groups/${currentGroupId}/messages`, { body });
}

async function addMember() {
  const uid = $('memberUserId').value.trim();
  if (!uid || !currentGroupId) return;
  await api('POST', `/api/groups/${currentGroupId}/members`, { userId: uid });
  $('memberUserId').value = '';
  sysMsg(`${uid} added to group`);
}

// Event bindings
$('btnConnect').addEventListener('click', connect);
$('userId').addEventListener('keydown', e => e.key === 'Enter' && connect());
$('btnCreate').addEventListener('click', createGroup);
$('groupSelect').addEventListener('change', e => selectGroup(e.target.value));
$('btnSend').addEventListener('click', sendMessage);
$('msgInput').addEventListener('keydown', e => e.key === 'Enter' && sendMessage());
$('btnAddMember').addEventListener('click', addMember);

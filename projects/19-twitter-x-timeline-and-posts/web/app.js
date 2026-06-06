'use strict';

// Base path: works both at "/" (local) and behind Caddy at "/p19/".
const BASE = location.pathname.replace(/\/(index\.html)?$/, '');
const api = (p) => `${BASE}${p}`;

const state = {
  token: null,
  userId: null,
  following: new Set(),
};

// ── tiny DOM helpers (XSS-safe: text via textContent, never innerHTML) ──────
const $ = (id) => document.getElementById(id);
function el(tag, cls, text) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text != null) n.textContent = text;
  return n;
}

// ── API log ─────────────────────────────────────────────────────────────────
function log(method, path, status, body) {
  const entry = el('div', 'log-entry');
  const top = el('div');
  top.appendChild(el('span', 'ts', new Date().toLocaleTimeString() + ' '));
  top.appendChild(el('span', `method ${method}`, method + ' '));
  top.append(path + ' ');
  const ok = status >= 200 && status < 300;
  top.appendChild(el('span', ok ? 'status-ok' : 'status-err', String(status)));
  entry.appendChild(top);
  if (body !== undefined) {
    const txt = typeof body === 'string' ? body : JSON.stringify(body);
    entry.appendChild(el('div', 'body', txt.length > 600 ? txt.slice(0, 600) + '…' : txt));
  }
  const logEl = $('apiLog');
  logEl.prepend(entry);
}

async function call(method, path, body, opts = {}) {
  const headers = {};
  if (body) headers['Content-Type'] = 'application/json';
  if (state.token && !opts.noAuth) headers['Authorization'] = 'Bearer ' + state.token;
  let res, data;
  try {
    res = await fetch(api(path), { method, headers, body: body ? JSON.stringify(body) : undefined });
    const text = await res.text();
    try { data = text ? JSON.parse(text) : null; } catch { data = text; }
  } catch (e) {
    log(method, path, 0, String(e));
    throw e;
  }
  log(method, path, res.status, data);
  if (!res.ok) throw Object.assign(new Error('request failed'), { status: res.status, data });
  return data;
}

// ── auth ──────────────────────────────────────────────────────────────────
async function signIn(userId) {
  if (!userId || !userId.trim()) return;
  const data = await call('POST', '/api/auth/token', { userId: userId.trim() }, { noAuth: true });
  state.token = data.token;
  state.userId = data.userId;
  state.following = new Set();
  const label = $('session').querySelector('.session-label');
  label.textContent = 'signed in as ' + state.userId;
  label.classList.add('active');
  ['tweetBtn', 'followBtn', 'refreshHomeBtn', 'backfillBtn'].forEach((id) => ($(id).disabled = false));
  renderFollowing();
  $('timeline').replaceChildren(el('span', 'muted', 'Timeline empty — follow someone and tweet.'));
  await refreshHome();
}

// ── tweet ───────────────────────────────────────────────────────────────────
async function tweet() {
  const text = $('tweetText').value.trim();
  if (!text) return;
  await call('POST', '/v1/posts', { text });
  $('tweetText').value = '';
  updateCharCount();
  // Fanout + indexing are async; give the workers a moment, then refresh.
  setTimeout(() => { refreshHome(); loadTrends(); }, 700);
}

// ── follow ──────────────────────────────────────────────────────────────────
async function follow(id) {
  const followee = (id || $('followId').value).trim();
  if (!followee) return;
  await call('POST', '/v1/follows', { followeeId: followee });
  state.following.add(followee);
  $('followId').value = '';
  renderFollowing();
  await refreshHome();
}

function renderFollowing() {
  const row = $('graphRow');
  row.replaceChildren();
  if (state.following.size === 0) {
    row.appendChild(el('span', 'muted', `${state.userId} follows no one yet.`));
    return;
  }
  row.appendChild(el('span', 'muted', `${state.userId} →`));
  for (const f of state.following) row.appendChild(el('span', 'follow-pill', '@' + f));
}

// ── home timeline ─────────────────────────────────────────────────────────
async function refreshHome() {
  if (!state.token) return;
  const items = await call('GET', '/v1/home?limit=25');
  const tl = $('timeline');
  tl.replaceChildren();
  if (!items || items.length === 0) {
    tl.appendChild(el('span', 'muted', 'No tweets in your timeline yet.'));
    return;
  }
  for (const it of items) tl.appendChild(renderTweet(it));
}

function renderTweet(it) {
  const card = el('div', 'tweet');
  const head = el('div', 'tweet-head');
  head.appendChild(el('span', 'author', '@' + it.authorId));
  head.appendChild(el('span', `src ${it.source}`, it.source));
  head.appendChild(el('span', 'score', 'score ' + it.score.toFixed(3)));
  card.appendChild(head);
  card.appendChild(renderBodyWithTags(it.text));
  return card;
}

// Highlight #hashtags without innerHTML — split on tags, build text nodes.
function renderBodyWithTags(text) {
  const body = el('div', 'body');
  const parts = String(text).split(/(#\w+)/g);
  for (const part of parts) {
    if (/^#\w+$/.test(part)) body.appendChild(el('span', 'tag', part));
    else body.append(part);
  }
  return body;
}

async function backfill() {
  if (!state.token) return;
  await call('POST', '/v1/home/backfill?max=800');
  await refreshHome();
}

// ── search ──────────────────────────────────────────────────────────────────
async function search() {
  const q = $('searchQ').value.trim();
  if (!q) return;
  const hits = await call('GET', '/v1/search?q=' + encodeURIComponent(q) + '&limit=20', null, { noAuth: true });
  const box = $('searchResults');
  box.replaceChildren();
  if (!hits || hits.length === 0) {
    box.appendChild(el('span', 'muted', 'No matches.'));
    return;
  }
  for (const h of hits) {
    const hit = el('div', 'hit');
    hit.appendChild(renderBodyWithTags(h.text));
    hit.appendChild(el('div', 'meta', `@${h.authorId} · relevance ${h.relevance.toFixed(2)}`));
    box.appendChild(hit);
  }
}

// ── trends ──────────────────────────────────────────────────────────────────
async function loadTrends() {
  const trends = await call('GET', '/v1/trends', null, { noAuth: true });
  const ul = $('trends');
  ul.replaceChildren();
  if (!trends || trends.length === 0) {
    ul.appendChild(el('li', 'muted', 'No trends in the last 24h. Tweet with #hashtags.'));
    return;
  }
  trends.forEach((t, i) => {
    const li = el('li');
    li.appendChild(el('span', 'rank', '#' + (i + 1)));
    li.appendChild(el('span', 'tag', '#' + t.hashtag));
    li.appendChild(el('span', 'cnt', t.count + (t.count === 1 ? ' tweet' : ' tweets')));
    ul.appendChild(li);
  });
}

// ── char counter ──────────────────────────────────────────────────────────
function updateCharCount() {
  const n = $('tweetText').value.length;
  const c = $('charCount');
  c.textContent = `${n} / 280`;
  c.classList.toggle('over', n > 280);
}

// ── quick-user chips (one "celebrity" to demo the read-path) ────────────────
function renderQuickUsers() {
  const users = [
    { id: 'alice' }, { id: 'bob' }, { id: 'carol' },
    { id: 'newsbot', celebrity: true },
  ];
  const box = $('quickUsers');
  for (const u of users) {
    const chip = el('span', 'chip' + (u.celebrity ? ' celebrity' : ''), '@' + u.id);
    if (u.celebrity) chip.title = 'tag a celebrity-style account';
    chip.addEventListener('click', () => { $('userId').value = u.id; });
    box.appendChild(chip);
  }
}

// ── wiring ──────────────────────────────────────────────────────────────────
function init() {
  renderQuickUsers();
  $('loginBtn').addEventListener('click', () => signIn($('userId').value));
  $('userId').addEventListener('keydown', (e) => { if (e.key === 'Enter') signIn($('userId').value); });
  $('tweetBtn').addEventListener('click', tweet);
  $('tweetText').addEventListener('input', updateCharCount);
  $('followBtn').addEventListener('click', () => follow());
  $('followId').addEventListener('keydown', (e) => { if (e.key === 'Enter') follow(); });
  $('refreshHomeBtn').addEventListener('click', refreshHome);
  $('backfillBtn').addEventListener('click', backfill);
  $('searchBtn').addEventListener('click', search);
  $('searchQ').addEventListener('keydown', (e) => { if (e.key === 'Enter') search(); });
  $('trendsBtn').addEventListener('click', loadTrends);
  $('clearLogBtn').addEventListener('click', () => $('apiLog').replaceChildren());
  loadTrends();
}

document.addEventListener('DOMContentLoaded', init);

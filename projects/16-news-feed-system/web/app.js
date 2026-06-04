'use strict';

// Single-file demo client for the news feed system. Talks to the same-origin
// REST API. JWT is kept in memory only (demo scope). All user-supplied strings
// are rendered via textContent / DOM nodes — never innerHTML — to avoid XSS.

const state = { token: null, userId: null };

const $ = (id) => document.getElementById(id);

function log(msg, cls = 'dim') {
  const el = document.createElement('div');
  el.className = 'entry';
  const time = new Date().toLocaleTimeString();
  const t = document.createElement('span');
  t.className = 'dim';
  t.textContent = time + '  ';
  const m = document.createElement('span');
  m.className = cls;
  m.textContent = msg;
  el.appendChild(t);
  el.appendChild(m);
  const box = $('log');
  box.prepend(el);
}

async function api(method, path, body) {
  const headers = { 'Content-Type': 'application/json' };
  if (state.token) headers['Authorization'] = 'Bearer ' + state.token;
  const res = await fetch(path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = text; }
  if (!res.ok) {
    const err = (data && data.error) ? data.error : res.status + ' ' + res.statusText;
    throw new Error(err);
  }
  return data;
}

async function login() {
  const userId = $('userId').value.trim();
  if (!userId) return;
  try {
    const r = await api('POST', '/api/auth/token', { userId });
    state.token = r.token;
    state.userId = r.userId;
    $('who').textContent = 'signed in as ' + r.userId;
    $('who').classList.add('on');
    $('authRow').hidden = true;
    $('actions').hidden = false;
    log('signed in as ' + r.userId, 'ok');
    await refreshFeed();
    await refreshStats();
  } catch (e) {
    log('login failed: ' + e.message, 'err');
  }
}

async function createPost() {
  const body = $('postBody').value.trim();
  if (!body) return;
  try {
    const p = await api('POST', '/v1/posts', { body });
    $('postBody').value = '';
    log('posted #' + p.id, 'ok');
    // Fanout is async (Kafka → worker); give it a beat before refreshing.
    setTimeout(refreshFeed, 400);
  } catch (e) {
    log('post failed: ' + e.message, 'err');
  }
}

async function follow() {
  const followee = $('followee').value.trim();
  if (!followee) return;
  try {
    const r = await api('POST', '/v1/follows', { followeeId: followee });
    $('followee').value = '';
    log('followed ' + r.followee + ' (backfilled ' + r.backfilledItems + ' items)', 'ok');
    await refreshFeed();
    await refreshStats();
  } catch (e) {
    log('follow failed: ' + e.message, 'err');
  }
}

async function backfill() {
  try {
    const r = await api('POST', '/v1/feed/backfill');
    log('backfilled ' + r.backfilledItems + ' items (timeline=' + r.timelineSize + ')', 'ok');
    await refreshFeed();
    await refreshStats();
  } catch (e) {
    log('backfill failed: ' + e.message, 'err');
  }
}

async function refreshStats() {
  try {
    const s = await api('GET', '/v1/feed/stats');
    $('timelineStat').textContent = 'timeline: ' + s.timelineSize + ' items';
  } catch { /* non-critical */ }
}

async function deletePost(id) {
  try {
    await api('DELETE', '/v1/posts/' + id);
    log('deleted post #' + id, 'ok');
    await refreshFeed();
  } catch (e) {
    log('delete failed: ' + e.message, 'err');
  }
}

function renderFeed(items) {
  const feed = $('feed');
  feed.replaceChildren();
  if (!items || items.length === 0) {
    const empty = document.createElement('div');
    empty.className = 'empty';
    empty.textContent = 'No posts yet. Post something, or follow someone who has.';
    feed.appendChild(empty);
    return;
  }
  for (const it of items) {
    const li = document.createElement('li');
    li.className = 'feed-item';

    const meta = document.createElement('div');
    meta.className = 'meta';
    const author = document.createElement('span');
    author.className = 'author';
    author.textContent = '@' + it.authorId;
    const when = document.createElement('span');
    when.textContent = new Date(it.createdAt).toLocaleString();
    meta.appendChild(author);
    meta.appendChild(when);

    const body = document.createElement('div');
    body.className = 'body';
    body.textContent = it.body;

    const tags = document.createElement('div');
    tags.className = 'tags';
    const badge = document.createElement('span');
    badge.className = 'badge ' + (it.source === 'read-path' ? 'read-path' : 'materialized');
    badge.textContent = it.source;
    const score = document.createElement('span');
    score.className = 'score';
    score.textContent = 'score ' + it.score.toFixed(3);
    tags.appendChild(badge);
    tags.appendChild(score);

    // Only the author can delete; show the control on their own posts.
    if (it.authorId === state.userId) {
      const del = document.createElement('button');
      del.className = 'del';
      del.textContent = 'delete';
      del.addEventListener('click', () => deletePost(it.postId));
      tags.appendChild(del);
    }

    li.appendChild(meta);
    li.appendChild(body);
    li.appendChild(tags);
    feed.appendChild(li);
  }
}

async function refreshFeed() {
  try {
    const items = await api('GET', '/v1/feed?limit=20');
    renderFeed(items);
    log('feed loaded (' + items.length + ' items)', 'dim');
  } catch (e) {
    log('feed load failed: ' + e.message, 'err');
  }
}

$('loginBtn').addEventListener('click', login);
$('postBtn').addEventListener('click', createPost);
$('followBtn').addEventListener('click', follow);
$('refreshBtn').addEventListener('click', refreshFeed);
$('backfillBtn').addEventListener('click', backfill);
$('userId').addEventListener('keydown', (e) => { if (e.key === 'Enter') login(); });
$('postBody').addEventListener('keydown', (e) => { if (e.key === 'Enter') createPost(); });
$('followee').addEventListener('keydown', (e) => { if (e.key === 'Enter') follow(); });

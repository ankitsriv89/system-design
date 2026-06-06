'use strict';

// Base path: empty when served directly, '' also works behind Caddy because the
// page itself is served under /p18/ and all fetches are relative.
const BASE = '';

const logEl = document.getElementById('apiLog');

// ── API log panel ─────────────────────────────────────────────────────────────
function log(method, path, status, body) {
  const row = document.createElement('div');
  row.className = 'log-row';
  const ok = status >= 200 && status < 300;
  const mkSpan = (cls, text) => {
    const s = document.createElement('span');
    s.className = cls;
    s.textContent = text;
    return s;
  };
  row.appendChild(mkSpan('log-method', method));
  row.appendChild(document.createTextNode(' '));
  row.appendChild(mkSpan('log-path', path));
  row.appendChild(document.createTextNode(' '));
  row.appendChild(mkSpan(`log-status ${ok ? 'ok' : 'err'}`, String(status)));
  if (body !== undefined && body !== null && body !== '') {
    const pre = document.createElement('pre');
    pre.className = 'log-body';
    pre.textContent = typeof body === 'string' ? body : JSON.stringify(body, null, 2);
    row.appendChild(pre);
  }
  logEl.prepend(row);
}

async function api(method, path, { headers = {}, json } = {}) {
  const opts = { method, headers: { ...headers } };
  if (json !== undefined) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(json);
  }
  const res = await fetch(`${BASE}${path}`, opts);
  let body = null;
  const text = await res.text();
  try { body = text ? JSON.parse(text) : null; } catch { body = text; }
  log(method, path, res.status, body);
  if (!res.ok) throw new Error(`${method} ${path} -> ${res.status}`);
  return body;
}

function userHeader() {
  return { 'X-User-Id': document.getElementById('userId').value };
}

// ── Pipeline visualization ──────────────────────────────────────────────────
function setStage(stage, state) {
  const el = document.getElementById('stage-' + stage);
  if (el) el.dataset.state = state; // idle | active | done | failed
}
function resetStages() {
  ['upload', 'store', 'process', 'post', 'feed'].forEach(s => setStage(s, 'idle'));
}

// ── Demo flow ─────────────────────────────────────────────────────────────────
// Generate a small PNG in-browser so the demo needs no file picker.
function makeSampleImage() {
  const c = document.createElement('canvas');
  c.width = 400; c.height = 300;
  const ctx = c.getContext('2d');
  const hue = Math.floor(Math.random() * 360);
  ctx.fillStyle = `hsl(${hue},70%,55%)`;
  ctx.fillRect(0, 0, c.width, c.height);
  ctx.fillStyle = 'rgba(255,255,255,0.85)';
  ctx.font = 'bold 28px sans-serif';
  ctx.fillText('demo ' + new Date().toLocaleTimeString(), 20, 160);
  return new Promise(resolve => c.toBlob(resolve, 'image/png'));
}

async function runUploadAndPost() {
  resetStages();
  try {
    const headers = userHeader();

    // 1. begin upload -> presigned PUT URL
    setStage('upload', 'active');
    const up = await api('POST', '/v1/media/uploads', {
      headers, json: { contentType: 'image/png' },
    });

    // 2. PUT bytes directly to object storage (presigned URL)
    setStage('store', 'active');
    const blob = await makeSampleImage();
    const putRes = await fetch(up.uploadUrl, { method: 'PUT', body: blob });
    log('PUT', '(presigned MinIO URL)', putRes.status, null);

    // 3. complete -> emits media.uploaded
    const media = await api('POST', `/v1/media/${up.mediaId}/complete`, { headers });
    setStage('upload', 'done');
    setStage('store', 'done');

    // 4. poll until the variant worker marks it PROCESSED
    setStage('process', 'active');
    let processed = media;
    for (let i = 0; i < 20 && processed.status !== 'PROCESSED'; i++) {
      await new Promise(r => setTimeout(r, 500));
      processed = await api('GET', `/v1/media/${up.mediaId}`);
    }
    setStage('process', processed.status === 'PROCESSED' ? 'done' : 'failed');
    renderVariants(processed);

    // 5. create a post referencing the media -> emits post.created (fanout)
    setStage('post', 'active');
    const post = await api('POST', '/v1/posts', {
      headers, json: { mediaId: up.mediaId, caption: 'Hello from the demo!' },
    });
    setStage('post', 'done');

    window.__lastPostId = post.id;
  } catch (e) {
    log('ERROR', '(demo flow)', 0, String(e));
  }
}

function renderVariants(media) {
  const box = document.getElementById('variants');
  box.replaceChildren();
  const variants = media.variants || {};
  Object.entries(variants).forEach(([name, url]) => {
    const fig = document.createElement('figure');
    fig.className = 'variant';
    if (name !== 'original') {
      const img = document.createElement('img');
      img.src = url; img.alt = name; img.loading = 'lazy';
      fig.appendChild(img);
    }
    const cap = document.createElement('figcaption');
    cap.textContent = name;
    fig.appendChild(cap);
    box.appendChild(fig);
  });
}

async function loadFeed() {
  setStage('feed', 'active');
  try {
    const items = await api('GET', '/v1/feed?limit=20', { headers: userHeader() });
    const feed = document.getElementById('feed');
    feed.replaceChildren();
    (items || []).forEach(it => {
      const card = document.createElement('div');
      card.className = 'feed-card';
      const url = (it.mediaVariants && (it.mediaVariants.small || it.mediaVariants.medium)) || null;
      if (url) {
        const img = document.createElement('img');
        img.src = url; img.alt = it.caption || ''; img.loading = 'lazy';
        card.appendChild(img);
      }
      const meta = document.createElement('div');
      meta.className = 'feed-meta';
      meta.textContent = `post ${it.postId} · author ${it.authorId} · ♥ ${it.likeCount} · score ${it.score.toFixed(3)}`;
      card.appendChild(meta);
      const likeBtn = document.createElement('button');
      likeBtn.textContent = 'Like';
      likeBtn.onclick = () => likePost(it.postId);
      card.appendChild(likeBtn);
      feed.appendChild(card);
    });
    setStage('feed', 'done');
  } catch (e) {
    setStage('feed', 'failed');
  }
}

async function likePost(postId) {
  await api('POST', `/v1/posts/${postId}/likes`, { headers: userHeader() });
  loadFeed();
}

async function follow() {
  const target = document.getElementById('followId').value;
  await api('PUT', `/v1/follows/${target}`, { headers: userHeader() });
}

// ── wire up controls ──────────────────────────────────────────────────────────
document.getElementById('btn-run').onclick = runUploadAndPost;
document.getElementById('btn-feed').onclick = loadFeed;
document.getElementById('btn-follow').onclick = follow;

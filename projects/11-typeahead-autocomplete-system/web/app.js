'use strict';

// ── State ────────────────────────────────────────────────────────────────────
const state = {
  suggestions: [],
  focusedIndex: -1,
  latencies: [],       // ring buffer of last 60 latency values (ms)
  lastPrefix: '',
  stats: null,
};

// ── DOM refs ─────────────────────────────────────────────────────────────────
const searchInput    = document.getElementById('search-input');
const dropdown       = document.getElementById('dropdown');
const searchMeta     = document.getElementById('search-meta');
const itemText       = document.getElementById('item-text');
const itemCategory   = document.getElementById('item-category');
const itemPopularity = document.getElementById('item-popularity');
const itemLocale     = document.getElementById('item-locale');
const btnAddItem     = document.getElementById('btn-add-item');
const corpusMsg      = document.getElementById('corpus-msg');
const btnRebuild     = document.getElementById('btn-rebuild');
const btnStats       = document.getElementById('btn-stats');
const btnSeed        = document.getElementById('btn-seed');
const adminMsg       = document.getElementById('admin-msg');
const logOutput      = document.getElementById('log-output');
const btnClearLog    = document.getElementById('btn-clear-log');
const statsBox       = document.getElementById('stats-box');
const algoSteps      = document.getElementById('algo-steps');
const trieCanvas     = document.getElementById('trie-canvas');
const latencyCanvas  = document.getElementById('latency-canvas');

// ── API helper ───────────────────────────────────────────────────────────────
async function apiFetch(method, path, body) {
  const t0 = performance.now();
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body !== undefined) opts.body = JSON.stringify(body);
  let res, data;
  try {
    res  = await fetch(path, opts);
    data = await res.json().catch(() => ({}));
  } catch (e) {
    appendLog(method, path, 0, null, { error: e.message });
    return null;
  }
  const ms = Math.round(performance.now() - t0);
  appendLog(method, path, res.status, ms, data);
  return { status: res.status, data, ms };
}

// ── Log ──────────────────────────────────────────────────────────────────────
function appendLog(method, path, status, ms, data) {
  const now = new Date().toISOString().slice(11, 23);
  const entry = document.createElement('div');
  entry.className = 'log-entry';

  const timeSpan = document.createElement('span');
  timeSpan.className = 'log-time';
  timeSpan.textContent = now;

  const methodSpan = document.createElement('span');
  methodSpan.className = `log-method ${method}`;
  methodSpan.textContent = ` ${method} `;

  entry.appendChild(timeSpan);
  entry.appendChild(methodSpan);
  entry.appendChild(document.createTextNode(path));

  if (status) {
    const statusSpan = document.createElement('span');
    statusSpan.className = `log-status-${String(status)[0]}`;
    statusSpan.textContent = ` [${status}]`;
    entry.appendChild(statusSpan);
  }
  if (ms !== null) {
    const msSpan = document.createElement('span');
    msSpan.className = 'meta-row';
    msSpan.textContent = ` ${ms}ms`;
    entry.appendChild(msSpan);
  }
  if (data) {
    entry.appendChild(document.createTextNode('\n'));
    const bodySpan = document.createElement('span');
    bodySpan.className = 'log-body';
    bodySpan.textContent = JSON.stringify(data, null, 2);
    entry.appendChild(bodySpan);
  }

  logOutput.prepend(entry);
  // Keep at most 100 entries.
  while (logOutput.children.length > 100) logOutput.lastChild.remove();
}

btnClearLog.addEventListener('click', () => { logOutput.innerHTML = ''; });

// ── Suggest ──────────────────────────────────────────────────────────────────
let suggestTimer = null;

searchInput.addEventListener('input', () => {
  clearTimeout(suggestTimer);
  const q = searchInput.value.trim();
  if (!q) { hideDropdown(); return; }
  suggestTimer = setTimeout(() => suggest(q), 80);
});

searchInput.addEventListener('keydown', e => {
  if (e.key === 'ArrowDown') { moveFocus(1); e.preventDefault(); }
  else if (e.key === 'ArrowUp') { moveFocus(-1); e.preventDefault(); }
  else if (e.key === 'Enter') { selectFocused(); }
  else if (e.key === 'Escape') { hideDropdown(); }
});

async function suggest(prefix) {
  state.lastPrefix = prefix;
  showAlgoSteps(prefix);
  drawTrie(prefix, []);

  const res = await apiFetch('GET', `/v1/suggest?q=${encodeURIComponent(prefix)}&locale=${itemLocale.value}&limit=8`);
  if (!res) return;
  const ms = res.ms;

  state.suggestions = res.data.suggestions || [];
  state.latencies.push(ms);
  if (state.latencies.length > 60) state.latencies.shift();

  searchMeta.textContent = `${state.suggestions.length} results · ${ms}ms · source: ${state.suggestions.length > 0 ? 'redis/pg' : '—'}`;
  renderDropdown(prefix, state.suggestions);
  drawTrie(prefix, state.suggestions);
  drawLatencyChart();
  finishAlgoSteps(ms, state.suggestions.length);
}

function renderDropdown(prefix, items) {
  if (!items.length) { hideDropdown(); return; }
  dropdown.innerHTML = '';
  items.forEach((item, i) => {
    const div = document.createElement('div');
    div.className = 'dropdown-item';
    if (i === state.focusedIndex) div.classList.add('focused');

    const textSpan = document.createElement('span');
    textSpan.className = 'item-text';
    // Highlight matching prefix portion.
    const lo = item.text.toLowerCase();
    const lp = prefix.toLowerCase();
    if (lo.startsWith(lp)) {
      const hl = document.createElement('span');
      hl.className = 'item-highlight';
      hl.textContent = item.text.slice(0, prefix.length);
      textSpan.appendChild(hl);
      textSpan.appendChild(document.createTextNode(item.text.slice(prefix.length)));
    } else {
      textSpan.textContent = item.text;
    }

    const right = document.createElement('span');
    right.style.display = 'flex';
    right.style.gap = '4px';
    right.style.alignItems = 'center';

    if (item.category) {
      const cat = document.createElement('span');
      cat.className = 'item-cat';
      cat.textContent = item.category;
      right.appendChild(cat);
    }
    const score = document.createElement('span');
    score.className = 'item-score';
    score.textContent = item.score.toFixed(0);
    right.appendChild(score);

    div.appendChild(textSpan);
    div.appendChild(right);

    div.addEventListener('mousedown', e => { e.preventDefault(); selectItem(item, i); });
    dropdown.appendChild(div);
  });
  dropdown.classList.remove('hidden');
  state.focusedIndex = -1;
}

function hideDropdown() {
  dropdown.classList.add('hidden');
  state.suggestions = [];
  state.focusedIndex = -1;
}

function moveFocus(dir) {
  if (!state.suggestions.length) return;
  state.focusedIndex = Math.max(-1, Math.min(state.suggestions.length - 1, state.focusedIndex + dir));
  const items = dropdown.querySelectorAll('.dropdown-item');
  items.forEach((el, i) => el.classList.toggle('focused', i === state.focusedIndex));
}

function selectFocused() {
  if (state.focusedIndex >= 0 && state.suggestions[state.focusedIndex]) {
    selectItem(state.suggestions[state.focusedIndex], state.focusedIndex);
  }
}

function selectItem(item, _idx) {
  searchInput.value = item.text;
  hideDropdown();
  // Send click feedback.
  apiFetch('POST', '/v1/feedback/click', {
    prefix: state.lastPrefix,
    selected_item_id: item.item_id,
    latency_ms: state.latencies[state.latencies.length - 1] || 0,
    locale: itemLocale.value,
  });
}

// ── Corpus management ─────────────────────────────────────────────────────────
btnAddItem.addEventListener('click', async () => {
  const text = itemText.value.trim();
  if (!text) { setMsg(corpusMsg, 'Text is required', true); return; }
  const pop = parseFloat(itemPopularity.value) || 100;
  const res = await apiFetch('POST', '/v1/corpus/items', {
    text,
    category: itemCategory.value.trim() || 'general',
    popularity: pop,
    locale: itemLocale.value,
  });
  if (!res) return;
  if (res.status === 201) {
    setMsg(corpusMsg, `Added: "${text}" (id=${res.data.id})`);
    itemText.value = '';
    loadStats();
  } else {
    setMsg(corpusMsg, res.data.error || 'Error', true);
  }
});

// ── Admin ─────────────────────────────────────────────────────────────────────
btnRebuild.addEventListener('click', async () => {
  btnRebuild.disabled = true;
  setMsg(adminMsg, 'Rebuilding…');
  const res = await apiFetch('POST', '/v1/admin/rebuild-index');
  btnRebuild.disabled = false;
  if (res && res.status === 200) {
    setMsg(adminMsg, `Rebuilt ${res.data.total_items} items in ${res.data.rebuild_duration_ms}ms`);
    loadStats();
  } else {
    setMsg(adminMsg, 'Rebuild failed', true);
  }
});

btnStats.addEventListener('click', loadStats);

btnSeed.addEventListener('click', async () => {
  btnSeed.disabled = true;
  setMsg(adminMsg, 'Seeding…');
  await seedCorpus();
  btnSeed.disabled = false;
  setMsg(adminMsg, 'Seed complete — try searching "go", "redis", "type"');
  loadStats();
});

async function seedCorpus() {
  const items = [
    { text: 'golang', category: 'language', popularity: 950 },
    { text: 'google search', category: 'product', popularity: 990 },
    { text: 'goroutine', category: 'concept', popularity: 800 },
    { text: 'grpc', category: 'protocol', popularity: 850 },
    { text: 'graphql', category: 'protocol', popularity: 870 },
    { text: 'redis sorted sets', category: 'data-structure', popularity: 820 },
    { text: 'redis cluster', category: 'infra', popularity: 780 },
    { text: 'redis streams', category: 'data-structure', popularity: 750 },
    { text: 'typeahead', category: 'concept', popularity: 760 },
    { text: 'trie data structure', category: 'concept', popularity: 710 },
    { text: 'postgresql full text', category: 'database', popularity: 690 },
    { text: 'prometheus metrics', category: 'observability', popularity: 680 },
    { text: 'docker compose', category: 'infra', popularity: 900 },
    { text: 'elasticsearch', category: 'search', popularity: 880 },
    { text: 'kafka streams', category: 'messaging', popularity: 830 },
    { text: 'kubernetes pod', category: 'infra', popularity: 860 },
    { text: 'system design', category: 'concept', popularity: 930 },
    { text: 'consistent hashing', category: 'concept', popularity: 720 },
    { text: 'rate limiting', category: 'concept', popularity: 740 },
    { text: 'load balancer', category: 'infra', popularity: 800 },
    { text: 'cdn edge', category: 'infra', popularity: 770 },
    { text: 'bloom filter', category: 'data-structure', popularity: 650 },
    { text: 'lru cache', category: 'data-structure', popularity: 790 },
    { text: 'distributed tracing', category: 'observability', popularity: 700 },
    { text: 'opentelemetry', category: 'observability', popularity: 660 },
  ];
  for (const item of items) {
    await apiFetch('POST', '/v1/corpus/items', { ...item, locale: 'en' });
  }
}

async function loadStats() {
  const res = await apiFetch('GET', '/v1/admin/stats');
  if (!res || res.status !== 200) return;
  state.stats = res.data;
  renderStats(res.data);
}

function renderStats(s) {
  statsBox.innerHTML = '';
  const fields = [
    { val: s.total_items || 0, label: 'Corpus Items' },
    { val: s.total_prefixes || 0, label: 'Prefix Keys' },
    { val: s.rebuild_duration_ms ? s.rebuild_duration_ms + 'ms' : '—', label: 'Last Rebuild' },
    { val: state.latencies.length ? Math.round(state.latencies.reduce((a, b) => a + b, 0) / state.latencies.length) + 'ms' : '—', label: 'Avg Latency' },
  ];
  fields.forEach(f => {
    const box = document.createElement('div');
    box.className = 'stat-box';
    const val = document.createElement('div');
    val.className = 'stat-val';
    val.textContent = f.val;
    const lbl = document.createElement('div');
    lbl.className = 'stat-label';
    lbl.textContent = f.label;
    box.appendChild(val);
    box.appendChild(lbl);
    statsBox.appendChild(box);
  });
}

// ── Trie Canvas Visualisation ────────────────────────────────────────────────
// Draws a visual trie of the active prefix path plus suggestions.
function drawTrie(prefix, suggestions) {
  const canvas = trieCanvas;
  const dpr = window.devicePixelRatio || 1;
  const W = canvas.offsetWidth, H = canvas.offsetHeight;
  canvas.width = W * dpr;
  canvas.height = H * dpr;
  const ctx = canvas.getContext('2d');
  ctx.scale(dpr, dpr);
  ctx.clearRect(0, 0, W, H);

  if (!prefix) return;

  const chars = Array.from(prefix.toLowerCase());
  const nodeR = 16;
  const hGap  = Math.min(56, (W - 40) / Math.max(chars.length + 1, 2));
  const startX = 30;
  const midY  = H / 2;

  // Draw prefix path nodes.
  chars.forEach((_ch, i) => {
    const x1 = startX + i * hGap;
    const x2 = startX + (i + 1) * hGap;
    // Edge
    ctx.beginPath();
    ctx.moveTo(x1 + nodeR, midY);
    ctx.lineTo(x2 - nodeR, midY);
    ctx.strokeStyle = getComputedStyle(document.documentElement).getPropertyValue('--active').trim();
    ctx.lineWidth = 2;
    ctx.stroke();
    // Node circle
    const cx = x1;
    ctx.beginPath();
    ctx.arc(cx, midY, nodeR, 0, Math.PI * 2);
    ctx.fillStyle = i === 0 ? '#1c2a3a' : '#1a3050';
    ctx.fill();
    ctx.strokeStyle = getComputedStyle(document.documentElement).getPropertyValue('--active').trim();
    ctx.lineWidth = 2;
    ctx.stroke();
    // Char label
    ctx.fillStyle = '#e6edf3';
    ctx.font = `bold 13px monospace`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(i === 0 ? '⌂' : chars[i - 1], cx, midY);
  });

  // Last prefix node.
  const lastX = startX + chars.length * hGap;
  ctx.beginPath();
  ctx.arc(lastX, midY, nodeR, 0, Math.PI * 2);
  ctx.fillStyle = '#0e3a6e';
  ctx.fill();
  ctx.strokeStyle = getComputedStyle(document.documentElement).getPropertyValue('--active').trim();
  ctx.lineWidth = 3;
  ctx.stroke();
  ctx.fillStyle = '#e6edf3';
  ctx.font = `bold 12px monospace`;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.fillText(chars[chars.length - 1], lastX, midY);

  // Draw suggestion leaf nodes fanning out from the last prefix node.
  if (suggestions.length > 0) {
    const maxShow = Math.min(suggestions.length, 6);
    const fanStartX = lastX + hGap * 1.2;
    const totalHeight = (maxShow - 1) * 36;
    const yStart = midY - totalHeight / 2;

    suggestions.slice(0, maxShow).forEach((sug, i) => {
      const leafX = fanStartX + 40;
      const leafY = yStart + i * 36;

      // Edge from prefix node to leaf.
      ctx.beginPath();
      ctx.moveTo(lastX + nodeR, midY);
      ctx.lineTo(leafX - nodeR - 2, leafY);
      ctx.strokeStyle = getComputedStyle(document.documentElement).getPropertyValue('--match').trim();
      ctx.lineWidth = 1;
      ctx.globalAlpha = 0.5;
      ctx.stroke();
      ctx.globalAlpha = 1;

      // Leaf node.
      ctx.beginPath();
      ctx.arc(leafX, leafY, nodeR, 0, Math.PI * 2);
      ctx.fillStyle = '#0d2a1a';
      ctx.fill();
      ctx.strokeStyle = getComputedStyle(document.documentElement).getPropertyValue('--match').trim();
      ctx.lineWidth = 1.5;
      ctx.stroke();

      // Score inside leaf.
      ctx.fillStyle = getComputedStyle(document.documentElement).getPropertyValue('--match').trim();
      ctx.font = `bold 9px monospace`;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(Math.round(sug.score), leafX, leafY);

      // Text label.
      ctx.fillStyle = '#e6edf3';
      ctx.font = `12px sans-serif`;
      ctx.textAlign = 'left';
      const label = sug.text.length > 18 ? sug.text.slice(0, 17) + '…' : sug.text;
      ctx.fillText(label, leafX + nodeR + 6, leafY + 1);
    });
  }

  // Animated "typing" pulse on the last prefix node.
  const pulse = (Date.now() % 1200) / 1200;
  const pulseR = nodeR + 6 + pulse * 8;
  ctx.beginPath();
  ctx.arc(lastX, midY, pulseR, 0, Math.PI * 2);
  ctx.strokeStyle = getComputedStyle(document.documentElement).getPropertyValue('--active').trim();
  ctx.lineWidth = 1.5;
  ctx.globalAlpha = 1 - pulse;
  ctx.stroke();
  ctx.globalAlpha = 1;
}

// Animate the trie pulse even when idle.
let animFrame;
function animateCanvas() {
  if (state.lastPrefix) drawTrie(state.lastPrefix, state.suggestions);
  animFrame = requestAnimationFrame(animateCanvas);
}
animateCanvas();

// ── Latency Chart ────────────────────────────────────────────────────────────
function drawLatencyChart() {
  const canvas = latencyCanvas;
  const dpr = window.devicePixelRatio || 1;
  const W = canvas.offsetWidth, H = canvas.offsetHeight;
  canvas.width = W * dpr;
  canvas.height = H * dpr;
  const ctx = canvas.getContext('2d');
  ctx.scale(dpr, dpr);
  ctx.clearRect(0, 0, W, H);

  const data = state.latencies;
  if (data.length < 2) return;

  const max = Math.max(...data, 10);
  const pad = 4;
  const xStep = (W - pad * 2) / (data.length - 1);

  // Grid line at max/2
  ctx.strokeStyle = 'rgba(255,255,255,0.06)';
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(pad, H / 2);
  ctx.lineTo(W - pad, H / 2);
  ctx.stroke();

  // Sparkline
  ctx.beginPath();
  data.forEach((v, i) => {
    const x = pad + i * xStep;
    const y = H - pad - ((v / max) * (H - pad * 2));
    if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
  });
  ctx.strokeStyle = getComputedStyle(document.documentElement).getPropertyValue('--accent').trim();
  ctx.lineWidth = 1.5;
  ctx.stroke();

  // Last value dot
  const lx = pad + (data.length - 1) * xStep;
  const lv = data[data.length - 1];
  const ly = H - pad - ((lv / max) * (H - pad * 2));
  ctx.beginPath();
  ctx.arc(lx, ly, 3, 0, Math.PI * 2);
  ctx.fillStyle = getComputedStyle(document.documentElement).getPropertyValue('--accent').trim();
  ctx.fill();

  const avg = Math.round(data.reduce((a, b) => a + b, 0) / data.length);
  const p99 = data.slice().sort((a, b) => a - b)[Math.floor(data.length * 0.99)] || lv;
  document.getElementById('latency-stats').textContent =
    `last: ${lv}ms  avg: ${avg}ms  p99: ${p99}ms  max: ${max}ms`;
}

// ── Algorithm Walkthrough ─────────────────────────────────────────────────────
function showAlgoSteps(prefix) {
  algoSteps.innerHTML = '';
  const steps = [
    `Normalize prefix "${prefix}" → lowercase, strip whitespace`,
    `Compute Redis key: ac:pfx:${itemLocale.value}:${prefix.toLowerCase()}`,
    `ZREVRANGE key 0 7 WITHSCORES — O(log N + K)`,
    `Cache hit? Return top-K suggestions`,
    `Cache miss → LIKE query on PostgreSQL, back-fill Redis`,
    `Return JSON with latency_ms`,
  ];
  steps.forEach((text, i) => {
    const div = document.createElement('div');
    div.className = 'algo-step' + (i === 0 ? ' active' : '');
    const numSpan = document.createElement('span');
    numSpan.className = 'step-num';
    numSpan.textContent = String(i + 1);
    const textSpan = document.createElement('span');
    textSpan.className = 'step-text';
    textSpan.textContent = text;
    div.appendChild(numSpan);
    div.appendChild(textSpan);
    algoSteps.appendChild(div);
    setTimeout(() => div.classList.add('active'), i * 60);
  });
}

function finishAlgoSteps(ms, count) {
  const items = algoSteps.querySelectorAll('.algo-step');
  items.forEach((el, i) => {
    setTimeout(() => el.classList.add('done'), i * 40);
  });
  // Annotate step 4 or 5 based on latency heuristic.
  const source = ms < 15 ? 'Redis hit ✓' : 'PG fallback ✓';
  if (items[3]) items[3].querySelector('.step-text').textContent += ` — ${count} results (${source})`;
  if (items[5]) items[5].querySelector('.step-text').textContent += ` — ${ms}ms`;
}

// ── Utility ───────────────────────────────────────────────────────────────────
function setMsg(el, text, isError = false) {
  el.textContent = text;
  el.className = 'msg' + (isError ? ' error' : '');
  setTimeout(() => { if (el.textContent === text) el.textContent = ''; }, 5000);
}

// ── Init ──────────────────────────────────────────────────────────────────────
loadStats();

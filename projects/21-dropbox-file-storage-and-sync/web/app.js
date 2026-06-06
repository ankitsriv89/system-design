'use strict';

const BASE = '';
let syncCursor = 0;
let autoPollTimer = null;
let stats = { files: 0, folders: 0, events: 0 };

// ── DOM refs ──────────────────────────────────────────────────────────────────
const $ownerId   = () => document.getElementById('owner-id').value.trim() || 'demo-user';
const $deviceId  = () => document.getElementById('device-id').value.trim() || 'browser-1';
const $apiKey    = () => document.getElementById('api-key').value.trim() || 'demo-secret';
const $tree      = document.getElementById('file-tree');
const $syncFeed  = document.getElementById('sync-feed');
const $log       = document.getElementById('api-log');
const $cursorBadge = document.getElementById('cursor-badge');

// ── Logging ───────────────────────────────────────────────────────────────────
function logEntry(method, path, status, body) {
  const ok = status >= 200 && status < 300;
  const ts = new Date().toLocaleTimeString('en-US', { hour12: false });
  const div = document.createElement('div');
  div.className = `log-entry ${ok ? 'success' : 'error'}`;

  const time = document.createElement('span');
  time.className = 'log-time';
  time.textContent = ts;

  const meth = document.createElement('span');
  meth.className = `log-method ${method}`;
  meth.textContent = method;

  const p = document.createElement('span');
  p.textContent = path;

  const statusSpan = document.createElement('span');
  statusSpan.className = ok ? 'log-status-ok' : 'log-status-err';
  statusSpan.textContent = status;

  div.append(time, ' ', meth, ' ', p, ' ', statusSpan);

  if (body) {
    const bodyDiv = document.createElement('div');
    bodyDiv.className = 'log-body';
    bodyDiv.textContent = typeof body === 'string' ? body : JSON.stringify(body, null, 2);
    div.appendChild(bodyDiv);
  }

  $log.prepend(div);
}

function logInfo(msg) {
  const div = document.createElement('div');
  div.className = 'log-entry info';
  div.textContent = msg;
  $log.prepend(div);
}


// ── API helpers ───────────────────────────────────────────────────────────────
async function api(method, path, body, isFormData) {
  const headers = { 'X-Owner-Id': $ownerId(), 'X-Device-Id': $deviceId(), 'X-Api-Key': $apiKey() };
  const opts = { method, headers };
  if (body && !isFormData) {
    headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  } else if (isFormData) {
    opts.body = body;
  }

  const res = await fetch(BASE + path, opts);
  let data;
  try { data = await res.json(); } catch { data = null; }
  logEntry(method, path, res.status, data);
  return { status: res.status, data };
}

// ── File Tree ─────────────────────────────────────────────────────────────────
async function refreshTree() {
  const { data } = await api('GET', '/v1/folders');
  renderTree(Array.isArray(data) ? data : []);
  updateStats(data);
}

function renderTree(nodes) {
  $tree.replaceChildren();
  if (!nodes.length) {
    const empty = document.createElement('span');
    empty.className = 'tree-empty';
    empty.textContent = 'No files or folders yet';
    $tree.appendChild(empty);
    return;
  }
  nodes.forEach(node => {
    const row = document.createElement('div');
    row.className = 'tree-item';

    const iconSpan = document.createElement('span');
    iconSpan.className = 'tree-icon';
    iconSpan.textContent = node.type === 'FOLDER' ? '📁' : '📄';

    const nameSpan = document.createElement('span');
    nameSpan.className = 'tree-name';
    nameSpan.textContent = node.name;

    const sizeSpan = document.createElement('span');
    sizeSpan.className = 'tree-size';
    sizeSpan.textContent = node.type === 'FILE' ? formatBytes(node.sizeBytes) : '';

    const del = document.createElement('span');
    del.className = 'tree-delete';
    del.title = 'Delete';
    del.textContent = '✕';
    del.addEventListener('click', (e) => {
      e.stopPropagation();
      deleteFile(node.id);
    });

    row.append(iconSpan, nameSpan, sizeSpan, del);
    $tree.appendChild(row);
  });
}

function updateStats(nodes) {
  if (!Array.isArray(nodes)) return;
  stats.files   = nodes.filter(n => n.type === 'FILE').length;
  stats.folders = nodes.filter(n => n.type === 'FOLDER').length;
  document.getElementById('stat-files').textContent   = stats.files;
  document.getElementById('stat-folders').textContent = stats.folders;
}

async function createFolder() {
  const name = prompt('Folder name:');
  if (!name) return;
  await api('POST', `/v1/folders?name=${encodeURIComponent(name)}`);
  await refreshTree();
}

async function uploadFile() {
  const input = document.getElementById('file-input');
  input.onchange = async () => {
    const file = input.files[0];
    if (!file) return;
    const fd = new FormData();
    fd.append('file', file);
    await api('POST', '/v1/files', fd, true);
    await refreshTree();
    input.value = '';
  };
  input.click();
}

async function deleteFile(id) {
  if (!confirm('Delete this item?')) return;
  await api('DELETE', `/v1/files/${id}`);
  await refreshTree();
}

// ── Sync Feed ─────────────────────────────────────────────────────────────────
async function pollSync() {
  const { data } = await api('GET', `/v1/sync?cursor=${syncCursor}`);
  if (!data || !Array.isArray(data.events)) return;
  if (data.events.length === 0) { logInfo('Sync: no new events'); return; }

  data.events.forEach(ev => appendSyncEvent(ev));
  syncCursor = data.newCursor ?? syncCursor;
  $cursorBadge.textContent = `cursor: ${syncCursor}`;
  document.getElementById('stat-cursor').textContent = syncCursor;
  stats.events += data.events.length;
  document.getElementById('stat-events').textContent = stats.events;
}

function appendSyncEvent(ev) {
  const wrap = document.createElement('div');
  wrap.className = 'sync-event';

  const typeClass = ev.eventType?.includes('deleted') ? 'event-deleted'
                  : ev.eventType?.includes('version') ? 'event-version'
                  : 'event-created';

  const badge = document.createElement('span');
  badge.className = `event-type-badge ${typeClass}`;
  badge.textContent = ev.eventType ?? '';

  const idSpan = document.createElement('span');
  idSpan.style.cssText = 'color:var(--muted);font-size:10px';
  idSpan.textContent = `#${ev.id}`;

  const fileSpan = document.createElement('span');
  fileSpan.style.cssText = 'color:var(--text);font-size:10px';
  fileSpan.textContent = ev.fileId ? ev.fileId.slice(0, 8) + '…' : '';

  wrap.append(badge, idSpan, fileSpan);

  const placeholder = $syncFeed.querySelector('.tree-empty');
  if (placeholder) placeholder.remove();
  $syncFeed.prepend(wrap);
}

// ── Auto-poll ─────────────────────────────────────────────────────────────────
document.getElementById('toggle-auto-poll').addEventListener('change', function () {
  if (this.checked) {
    autoPollTimer = setInterval(pollSync, 5000);
    logInfo('Auto-poll enabled (5s)');
  } else {
    clearInterval(autoPollTimer);
    logInfo('Auto-poll disabled');
  }
});

// ── Wire up buttons ───────────────────────────────────────────────────────────
document.getElementById('btn-refresh').addEventListener('click', refreshTree);
document.getElementById('btn-new-folder').addEventListener('click', createFolder);
document.getElementById('btn-upload').addEventListener('click', uploadFile);
document.getElementById('btn-poll').addEventListener('click', pollSync);
document.getElementById('btn-clear-log').addEventListener('click', () => { $log.replaceChildren(); });

// ── Helpers ───────────────────────────────────────────────────────────────────
function formatBytes(n) {
  if (!n) return '0 B';
  const units = ['B','KB','MB','GB'];
  let i = 0;
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++; }
  return n.toFixed(1) + ' ' + units[i];
}

// ── Init ──────────────────────────────────────────────────────────────────────
logInfo('Demo ready — click Refresh to load file tree');

'use strict';

const BASE = '';

// ── Canvas map state ──────────────────────────────────────────────────────────
const canvas = document.getElementById('mapCanvas');
const ctx = canvas.getContext('2d');
const hint = document.getElementById('mapHint');

// lat/lng bounds for SF demo area
const BOUNDS = { minLat: 37.72, maxLat: 37.83, minLng: -122.52, maxLng: -122.38 };

const users = new Map(); // userId → {lat, lng, color}
const COLORS = ['#388bfd','#3fb950','#f78166','#e3b341','#bc8cff','#79c0ff','#56d364'];
let colorIdx = 0;
let queryCircle = null;

function projectPoint(lat, lng) {
  const x = ((lng - BOUNDS.minLng) / (BOUNDS.maxLng - BOUNDS.minLng)) * canvas.width;
  const y = (1 - (lat - BOUNDS.minLat) / (BOUNDS.maxLat - BOUNDS.minLat)) * canvas.height;
  return { x, y };
}

function drawMap() {
  ctx.clearRect(0, 0, canvas.width, canvas.height);

  // grid
  ctx.strokeStyle = '#21262d';
  ctx.lineWidth = 0.5;
  for (let i = 0; i <= 4; i++) {
    const x = (i / 4) * canvas.width;
    ctx.beginPath(); ctx.moveTo(x, 0); ctx.lineTo(x, canvas.height); ctx.stroke();
    const y = (i / 4) * canvas.height;
    ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(canvas.width, y); ctx.stroke();
  }

  // query radius circle
  if (queryCircle) {
    const { x, y, radiusPx } = queryCircle;
    ctx.beginPath();
    ctx.arc(x, y, radiusPx, 0, Math.PI * 2);
    ctx.strokeStyle = 'rgba(56,139,253,0.5)';
    ctx.lineWidth = 1.5;
    ctx.stroke();
    ctx.fillStyle = 'rgba(56,139,253,0.07)';
    ctx.fill();
  }

  // user dots
  for (const [uid, u] of users) {
    const { x, y } = projectPoint(u.lat, u.lng);
    ctx.beginPath();
    ctx.arc(x, y, 6, 0, Math.PI * 2);
    ctx.fillStyle = u.highlighted ? '#f0e060' : u.color;
    ctx.fill();
    ctx.fillStyle = '#fff';
    ctx.font = '9px monospace';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(uid.charAt(0).toUpperCase(), x, y);

    ctx.fillStyle = u.color;
    ctx.font = '10px sans-serif';
    ctx.fillText(uid, x + 9, y - 7);
  }

  hint.style.display = users.size === 0 ? 'flex' : 'none';
}

function metersToPixels(radiusM) {
  const degPerMeter = 1 / 111320;
  const pxPerDeg = canvas.height / (BOUNDS.maxLat - BOUNDS.minLat);
  return radiusM * degPerMeter * pxPerDeg;
}

// ── API helpers ───────────────────────────────────────────────────────────────
function logEntry(method, path, status, body) {
  const log = document.getElementById('apiLog');
  const entry = document.createElement('div');
  entry.className = 'log-entry';

  const tsSpan = document.createElement('span');
  tsSpan.className = 'ts';
  tsSpan.textContent = new Date().toLocaleTimeString();

  const methodSpan = document.createElement('span');
  methodSpan.className = `method ${method}`;
  methodSpan.textContent = method;

  const pathText = document.createTextNode(' ' + path + ' ');

  const statusSpan = document.createElement('span');
  statusSpan.className = status >= 200 && status < 300 ? 'status-ok' : 'status-err';
  statusSpan.textContent = String(status);

  entry.append(tsSpan, ' ', methodSpan, pathText, statusSpan);

  if (body) {
    const bodyDiv = document.createElement('div');
    bodyDiv.className = 'body';
    bodyDiv.textContent = JSON.stringify(body, null, 2);
    entry.appendChild(bodyDiv);
  }

  log.prepend(entry);
}

async function apiCall(method, path, data) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (data) opts.body = JSON.stringify(data);
  try {
    const res = await fetch(BASE + path, opts);
    let json;
    try { json = await res.json(); } catch { json = null; }
    logEntry(method, path, res.status, json);
    return { ok: res.ok, status: res.status, data: json };
  } catch (e) {
    logEntry(method, path, 0, { error: e.message });
    return { ok: false, status: 0, data: null };
  }
}

// ── Button handlers ───────────────────────────────────────────────────────────
document.getElementById('btnUpdate').addEventListener('click', async () => {
  const userId = document.getElementById('userId').value.trim();
  const lat = parseFloat(document.getElementById('lat').value);
  const lng = parseFloat(document.getElementById('lng').value);
  if (!userId) return;

  const res = await apiCall('POST', '/v1/locations', { userId, lat, lng });
  if (res.ok) {
    if (!users.has(userId)) {
      users.set(userId, { lat, lng, color: COLORS[colorIdx++ % COLORS.length] });
    } else {
      users.get(userId).lat = lat;
      users.get(userId).lng = lng;
    }
    drawMap();
  }
});

document.getElementById('btnSeed').addEventListener('click', async () => {
  const seedUsers = [
    { userId: 'alice',   lat: 37.7749, lng: -122.4194 },
    { userId: 'bob',     lat: 37.7800, lng: -122.4100 },
    { userId: 'carol',   lat: 37.7700, lng: -122.4300 },
    { userId: 'dave',    lat: 37.7850, lng: -122.4250 },
    { userId: 'eve',     lat: 37.7650, lng: -122.4050 },
  ];
  for (const u of seedUsers) {
    const res = await apiCall('POST', '/v1/locations', u);
    if (res.ok) {
      if (!users.has(u.userId)) {
        users.set(u.userId, { ...u, color: COLORS[colorIdx++ % COLORS.length] });
      }
    }
  }
  drawMap();
});

document.getElementById('btnNearby').addEventListener('click', async () => {
  const userId = document.getElementById('queryUserId').value.trim();
  const lat = parseFloat(document.getElementById('lat').value);
  const lng = parseFloat(document.getElementById('lng').value);
  const radius = parseInt(document.getElementById('radius').value, 10);

  const res = await apiCall('GET',
    `/v1/nearby?userId=${encodeURIComponent(userId)}&lat=${lat}&lng=${lng}&radiusMeters=${radius}`, null);

  if (res.ok && res.data) {
    const nearby = res.data.nearby || [];

    // highlight nearby users
    for (const [, u] of users) u.highlighted = false;
    for (const uid of nearby) {
      if (users.has(uid)) users.get(uid).highlighted = true;
    }

    // draw query circle
    const { x, y } = projectPoint(lat, lng);
    queryCircle = { x, y, radiusPx: metersToPixels(radius) };
    drawMap();

    const list = document.getElementById('nearbyList');
    list.textContent = '';
    if (nearby.length === 0) {
      const li = document.createElement('li');
      li.textContent = 'No nearby users';
      list.appendChild(li);
    } else {
      for (const uid of nearby) {
        const li = document.createElement('li');
        li.textContent = uid;
        list.appendChild(li);
      }
    }
    document.getElementById('nearbyResult').classList.remove('hidden');
  }
});

document.getElementById('btnClear').addEventListener('click', () => {
  document.getElementById('apiLog').innerHTML = '';
});

drawMap();

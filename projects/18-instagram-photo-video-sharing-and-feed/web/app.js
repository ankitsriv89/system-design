'use strict';

// Base path: empty when served directly, '/p18' behind Caddy.
const BASE = '';

const logEl = document.getElementById('apiLog');

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
  row.appendChild(mkSpan(`log-status ${ok ? 'ok' : 'err'}`, status));
  if (body) {
    const pre = document.createElement('pre');
    pre.className = 'log-body';
    pre.textContent = typeof body === 'string' ? body : JSON.stringify(body, null, 2);
    row.appendChild(pre);
  }
  logEl.prepend(row);
}

// Demo interactions (upload, post, feed, like) are wired here during the
// build milestones. For now, confirm the service is reachable.
async function ping() {
  try {
    const res = await fetch(`${BASE}/actuator/health`);
    const json = await res.json();
    log('GET', '/actuator/health', res.status, json);
  } catch (e) {
    log('GET', '/actuator/health', 0, String(e));
  }
}

ping();

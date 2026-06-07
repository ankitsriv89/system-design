(() => {
  const BASE = '';
  let selectedEventId = null;
  let selectedSeatId = null;

  // ── Logging ─────────────────────────────────────────────────────────────────
  function logEntry(method, url, status, body) {
    const log = document.getElementById('api-log');
    const el = document.createElement('div');
    const ok = status >= 200 && status < 300;
    el.className = `log-entry ${ok ? 'success' : 'error'}`;

    const ts = document.createElement('span');
    ts.className = 'ts';
    ts.textContent = new Date().toLocaleTimeString();

    const meth = document.createElement('span');
    meth.className = 'method';
    meth.textContent = method;

    const statusSpan = document.createElement('span');
    statusSpan.className = ok ? 'status-ok' : 'status-err';
    statusSpan.textContent = String(status);

    const body_text = document.createTextNode(' ' + url + ' ');
    const detail = document.createTextNode('\n' + JSON.stringify(body, null, 2));

    el.append(ts, meth, body_text, statusSpan, detail);
    log.prepend(el);
  }

  // userId is sent as X-User-Id header (simulating upstream auth gateway injection).
  function getUserId() {
    return document.getElementById('hold-user-id')?.value.trim() || 'user-001';
  }

  async function apiFetch(method, path, body) {
    const opts = {
      method,
      headers: { 'Content-Type': 'application/json', 'X-User-Id': getUserId() }
    };
    if (body) opts.body = JSON.stringify(body);
    const res = await fetch(BASE + path, opts);
    let data;
    try { data = await res.json(); } catch { data = {}; }
    logEntry(method, path, res.status, data);
    return { ok: res.ok, status: res.status, data };
  }

  // ── Events ──────────────────────────────────────────────────────────────────
  async function loadEvents() {
    const { ok, data } = await apiFetch('GET', '/v1/events');
    if (!ok) return;
    const list = document.getElementById('events-list');
    list.innerHTML = '';
    (data || []).forEach(ev => {
      const el = document.createElement('div');
      el.className = 'entity-item' + (ev.id === selectedEventId ? ' selected' : '');

      const nameSpan = document.createElement('span');
      const venueSpan = document.createElement('span');
      venueSpan.style.color = 'var(--muted)';
      venueSpan.textContent = ev.venue;
      nameSpan.textContent = ev.name + ' ';
      nameSpan.appendChild(venueSpan);

      const eidSpan = document.createElement('span');
      eidSpan.className = 'eid';
      eidSpan.textContent = (ev.id || '').slice(0, 8) + '…';

      el.append(nameSpan, eidSpan);
      el.addEventListener('click', () => selectEvent(ev.id, el));
      list.appendChild(el);
    });
  }

  function selectEvent(id, el) {
    selectedEventId = id;
    document.querySelectorAll('#events-list .entity-item').forEach(e => e.classList.remove('selected'));
    el.classList.add('selected');
  }

  document.getElementById('btn-create-event').addEventListener('click', async () => {
    const name = document.getElementById('event-name').value.trim();
    const venue = document.getElementById('event-venue').value.trim();
    const totalSeats = parseInt(document.getElementById('event-seats').value);
    if (!name || !venue || isNaN(totalSeats)) return;
    const eventTime = new Date(Date.now() + 7 * 24 * 3600 * 1000).toISOString();
    const { ok } = await apiFetch('POST', '/v1/events', { name, venue, eventTime, totalSeats });
    if (ok) loadEvents();
  });

  // ── Seat Map ─────────────────────────────────────────────────────────────────
  async function loadSeats() {
    if (!selectedEventId) { alert('Select an event first.'); return; }
    const { ok: ok1, data: seats } = await apiFetch('GET', `/v1/events/${selectedEventId}/seats`);
    const { ok: ok2, data: stats } = await apiFetch('GET', `/v1/events/${selectedEventId}/seats/stats`);
    if (!ok1) return;

    const grid = document.getElementById('seat-map');
    grid.innerHTML = '';
    (seats || []).forEach(seat => {
      const el = document.createElement('div');
      el.className = `seat ${seat.status.toLowerCase()}${seat.id === selectedSeatId ? ' selected' : ''}`;
      el.title = `${seat.section}${seat.rowLabel}-${seat.seatNumber} [${seat.status}]`;
      el.textContent = `${seat.section}${seat.seatNumber}`;
      if (seat.status === 'AVAILABLE') {
        el.addEventListener('click', () => {
          selectedSeatId = seat.id;
          document.querySelectorAll('.seat').forEach(s => s.classList.remove('selected'));
          el.classList.add('selected');
          document.getElementById('hold-seat-id').value = seat.id;
        });
      }
      grid.appendChild(el);
    });

    if (ok2 && stats) {
      const statsEl = document.getElementById('seat-stats');
      statsEl.innerHTML = '';
      [['available', stats.available], ['held', stats.held], ['booked', stats.booked]].forEach(([cls, count]) => {
        const stat = document.createElement('div');
        stat.className = 'stat';
        const dot = document.createElement('div');
        dot.className = `stat-dot ${cls}`;
        stat.appendChild(dot);
        stat.appendChild(document.createTextNode(Number(count) + ' ' + cls));
        statsEl.appendChild(stat);
      });
    }
  }

  document.getElementById('btn-refresh-seats').addEventListener('click', loadSeats);

  // ── Hold ─────────────────────────────────────────────────────────────────────
  document.getElementById('btn-create-hold').addEventListener('click', async () => {
    const seatId = document.getElementById('hold-seat-id').value.trim();
    const resultEl = document.getElementById('hold-result');
    if (!seatId) return;

    // userId flows via X-User-Id header (set by getUserId()), not the body.
    const { ok, data } = await apiFetch('POST', '/v1/holds', { seatId });
    resultEl.className = 'result-box' + (ok ? '' : ' error');
    resultEl.textContent = JSON.stringify(data, null, 2);
    if (ok && data.id) {
      document.getElementById('checkout-hold-id').value = data.id;
      document.getElementById('checkout-idem-key').value = 'idem-' + data.id.slice(0, 8);
    }
    loadSeats();
  });

  // ── Checkout ─────────────────────────────────────────────────────────────────
  document.getElementById('btn-checkout').addEventListener('click', async () => {
    const holdId = document.getElementById('checkout-hold-id').value.trim();
    const amount = document.getElementById('checkout-amount').value.trim();
    const idempotencyKey = document.getElementById('checkout-idem-key').value.trim() || undefined;
    const resultEl = document.getElementById('checkout-result');
    if (!holdId || !amount) return;

    // userId flows via X-User-Id header, not the body.
    const { ok, data } = await apiFetch('POST', '/v1/bookings', { holdId, amount, idempotencyKey });
    resultEl.className = 'result-box' + (ok ? '' : ' error');
    resultEl.textContent = JSON.stringify(data, null, 2);
    if (ok) loadSeats();
  });

  // ── Clear log ────────────────────────────────────────────────────────────────
  document.getElementById('btn-clear-log').addEventListener('click', () => {
    document.getElementById('api-log').innerHTML = '';
  });

  // Initial load
  loadEvents();
})();

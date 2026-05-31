/**
 * odyssey-ui.js — UI controller for Interstellar Odyssey.
 * Handles API calls, state updates, question rendering, and overlays.
 */

import { sceneReady, setDestination, playWarp, playDeny } from '/odyssey-scene.js';

// ── Constants ─────────────────────────────────────────────────────────────
const MAX_HOPS = 6;
const REFILL_MS = 6 * 60 * 60 * 1000; // 6 hours

const DESTINATION_NAMES = [
  'Earth', 'Moon', 'Mars', 'Asteroid Belt', 'Jupiter',
  'Saturn', 'Uranus', 'Neptune', 'Pluto', 'Alpha Centauri', 'Sagittarius A*',
];

// ── DOM refs ──────────────────────────────────────────────────────────────
const els = {
  healthDot:       () => document.getElementById('healthDot'),
  destSubtitle:    () => document.getElementById('destSubtitle'),
  destName:        () => document.getElementById('destName'),
  destDistance:    () => document.getElementById('destDistance'),
  hopsRemaining:   () => document.getElementById('hopsRemaining'),
  hopsUsed:        () => document.getElementById('hopsUsed'),
  refillTimer:     () => document.getElementById('refillTimer'),
  hopBar:          () => document.getElementById('hopBar'),
  statusMsg:       () => document.getElementById('statusMsg'),
  btnQuestion:     () => document.getElementById('btnQuestion'),
  btnReset:        () => document.getElementById('btnReset'),
  journeyNav:      () => document.getElementById('journeyNav'),
  questionPanel:   () => document.getElementById('questionPanel'),
  qBadge:          () => document.getElementById('qBadge'),
  qDest:           () => document.getElementById('qDest'),
  qText:           () => document.getElementById('qText'),
  qChoices:        () => document.getElementById('qChoices'),
  qFeedback:       () => document.getElementById('qFeedback'),
  btnHint:         () => document.getElementById('btnHint'),
  btnNextQ:        () => document.getElementById('btnNextQ'),
  rateLimitOverlay:() => document.getElementById('rateLimitOverlay'),
  rlMsg:           () => document.getElementById('rlMsg'),
  rlCountdown:     () => document.getElementById('rlCountdown'),
  btnRlClose:      () => document.getElementById('btnRlClose'),
  completeOverlay: () => document.getElementById('completeOverlay'),
  btnPlayAgain:    () => document.getElementById('btnPlayAgain'),
};

// ── App state ─────────────────────────────────────────────────────────────
const state = {
  destIdx: 0,
  hopsUsed: 0,
  completed: false,
  currentQuestionId: null,
  currentAnswer: null,
  answered: false,
  rateLimitRetryMs: 0,
  lastActivity: null,
};

// ── Helpers ───────────────────────────────────────────────────────────────
function setStatus(msg, color = '') {
  const el = els.statusMsg();
  if (el) { el.textContent = msg; el.style.color = color || ''; }
}

function safeText(el, text) {
  if (el) el.textContent = text ?? '—';
}

async function apiFetch(path, opts = {}) {
  const r = await fetch(path, {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) },
    ...opts,
  });
  return r;
}

// ── Health check ──────────────────────────────────────────────────────────
async function checkHealth() {
  try {
    const r = await apiFetch('/healthz');
    els.healthDot()?.classList.toggle('online', r.ok);
  } catch {
    els.healthDot()?.classList.remove('online');
  }
  setTimeout(checkHealth, 12_000);
}

// ── Journey nav ───────────────────────────────────────────────────────────
function renderJourneyNav(destIdx) {
  const nav = els.journeyNav();
  if (!nav) return;
  nav.replaceChildren();
  DESTINATION_NAMES.forEach((name, i) => {
    if (i > 0) {
      const conn = document.createElement('div');
      conn.className = 'nav-connector';
      nav.appendChild(conn);
    }
    const stop = document.createElement('div');
    stop.className = 'nav-stop';

    const dot = document.createElement('div');
    dot.className = 'nav-dot' + (i < destIdx ? ' done' : i === destIdx ? ' current' : '');

    const label = document.createElement('div');
    label.className = 'nav-label';
    label.textContent = name.split(' ')[0]; // first word only for brevity

    stop.appendChild(dot);
    stop.appendChild(label);
    nav.appendChild(stop);
  });
}

// ── Hop bar ───────────────────────────────────────────────────────────────
function renderHopBar(remaining) {
  const bar = els.hopBar();
  if (!bar) return;
  const pct = Math.max(0, Math.min(100, (remaining / MAX_HOPS) * 100));
  bar.style.width = pct + '%';
  bar.classList.toggle('low', remaining <= 2);
}

// ── Refill countdown ──────────────────────────────────────────────────────
let refillInterval = null;

function startRefillCountdown(retryAfterMs) {
  if (refillInterval) clearInterval(refillInterval);
  const deadline = Date.now() + retryAfterMs;

  const tick = () => {
    const left = deadline - Date.now();
    if (left <= 0) {
      clearInterval(refillInterval);
      safeText(els.refillTimer(), 'Now!');
      safeText(els.rlCountdown(), '00:00');
      return;
    }
    const h = Math.floor(left / 3600000);
    const m = Math.floor((left % 3600000) / 60000);
    const s = Math.floor((left % 60000) / 1000);
    const fmt = h > 0
      ? `${h}h ${String(m).padStart(2,'0')}m`
      : `${String(m).padStart(2,'0')}:${String(s).padStart(2,'0')}`;
    safeText(els.refillTimer(), fmt);
    safeText(els.rlCountdown(), fmt);
  };
  tick();
  refillInterval = setInterval(tick, 1000);
}

// ── Load state ────────────────────────────────────────────────────────────
async function loadState() {
  setStatus('Loading…');
  try {
    const r = await apiFetch('/v1/odyssey/state');
    if (!r.ok) { setStatus('Server error loading state', 'var(--red)'); return; }
    const data = await r.json();

    state.destIdx = data.destination_idx ?? 0;
    state.hopsUsed = data.hops_used ?? 0;
    state.completed = data.completed ?? false;

    // Use real remaining count from Redis (PeekTokenBucket), not computed from hops_used.
    // This survives resets — the IP-based rate limit persists independent of session progress.
    const remaining = data.hops_remaining ?? (MAX_HOPS - state.hopsUsed);
    const dest = data.destination;

    // If already rate-limited on load, show the overlay immediately
    if (data.rate_limited && data.retry_after_ms > 0) {
      showRateLimitOverlay(data.retry_after_ms);
    }

    safeText(els.destName(), dest?.name ?? DESTINATION_NAMES[state.destIdx]);
    safeText(els.destDistance(), dest?.distance ?? '—');
    safeText(els.destSubtitle(), dest?.description ?? '');
    safeText(els.hopsRemaining(), String(Math.max(0, remaining)));
    safeText(els.hopsUsed(), String(state.hopsUsed));

    renderHopBar(remaining);
    renderJourneyNav(state.destIdx);
    setDestination(state.destIdx);

    const btn = els.btnQuestion();
    if (btn) {
      btn.disabled = state.completed || remaining <= 0;
      btn.textContent = state.completed ? 'Odyssey complete!' : 'Get question';
    }

    if (state.completed) {
      els.completeOverlay()?.removeAttribute('hidden');
    }

    setStatus('');
  } catch (e) {
    setStatus('Could not reach server', 'var(--red)');
  }
}

// ── Load question ─────────────────────────────────────────────────────────
async function loadQuestion() {
  els.btnQuestion()?.setAttribute('disabled', '');
  setStatus('Fetching question…');

  try {
    const r = await apiFetch('/v1/odyssey/question');
    if (!r.ok) {
      setStatus('Could not load question — try again', 'var(--red)');
      els.btnQuestion()?.removeAttribute('disabled');
      return;
    }
    const data = await r.json();
    if (data.completed) { setStatus('Odyssey already complete!'); return; }

    state.currentQuestionId = data.question_id;
    state.currentAnswer = null;
    state.answered = false;

    renderQuestion(data);
    setStatus('');
    els.btnQuestion()?.removeAttribute('disabled');
  } catch (e) {
    setStatus('Network error', 'var(--red)');
    els.btnQuestion()?.removeAttribute('disabled');
  }
}

// ── Render question ───────────────────────────────────────────────────────
function renderQuestion(data) {
  const panel = els.questionPanel();
  if (!panel) return;
  panel.removeAttribute('hidden');

  const badge = els.qBadge();
  if (badge) {
    badge.textContent = data.source === 'groq' ? 'AI Generated' : 'Question Bank';
    badge.className = 'q-badge ' + (data.source === 'groq' ? 'groq' : 'seed');
  }
  safeText(els.qDest(), data.destination?.name ?? '');
  safeText(els.qText(), data.question);

  const choicesList = els.qChoices();
  if (!choicesList) return;
  choicesList.replaceChildren();

  (data.choices || []).forEach((choice, i) => {
    const li = document.createElement('li');
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = choice;
    btn.addEventListener('click', () => submitAnswer(i));
    li.appendChild(btn);
    choicesList.appendChild(li);
  });

  const feedback = els.qFeedback();
  if (feedback) { feedback.setAttribute('hidden', ''); feedback.className = 'q-feedback'; feedback.textContent = ''; }

  els.btnHint()?.removeAttribute('hidden');
  els.btnNextQ()?.setAttribute('hidden', '');
}

// ── Submit answer ─────────────────────────────────────────────────────────
async function submitAnswer(choice) {
  if (state.answered || state.currentQuestionId == null) return;
  state.answered = true;

  // Disable all choice buttons
  els.qChoices()?.querySelectorAll('button').forEach(b => b.setAttribute('disabled', ''));
  els.btnHint()?.setAttribute('hidden', '');
  setStatus('Checking answer…');

  try {
    const r = await apiFetch('/v1/odyssey/answer', {
      method: 'POST',
      body: JSON.stringify({ question_id: state.currentQuestionId, choice }),
    });

    const data = await r.json();

    if (r.status === 429 || data.reason === 'hop_limit_exceeded') {
      // Rate limited
      playDeny();
      state.answered = false; // allow retry once refuelled
      showRateLimitOverlay(data.retry_after_ms ?? REFILL_MS);
      setStatus('Hop fuel depleted — rate limited', 'var(--red)');
      return;
    }

    // Update hop count from server response
    const remaining = data.remaining_hops ?? (MAX_HOPS - (state.hopsUsed + 1));
    state.hopsUsed = MAX_HOPS - Math.max(0, remaining);
    safeText(els.hopsRemaining(), String(Math.max(0, remaining)));
    safeText(els.hopsUsed(), String(state.hopsUsed));
    renderHopBar(remaining);

    if (data.correct) {
      showFeedback('correct', '✓ Correct! Initiating jump…');
      markChoiceResult(choice, 'correct');
      playWarp();

      setTimeout(async () => {
        state.destIdx = data.destination_idx;
        state.completed = data.completed;
        setDestination(state.destIdx);
        renderJourneyNav(state.destIdx);
        safeText(els.destName(), data.next_destination?.name ?? DESTINATION_NAMES[state.destIdx]);
        safeText(els.destDistance(), data.next_destination?.distance ?? '—');
        safeText(els.destSubtitle(), data.next_destination?.description ?? '');

        if (data.completed) {
          setTimeout(() => els.completeOverlay()?.removeAttribute('hidden'), 1500);
          return;
        }

        // Auto-load next question after arriving at new destination
        setStatus('Arrived! Loading next question…', 'var(--cyan)');
        setTimeout(() => loadQuestion(), 1200);
      }, 1400);
    } else {
      showFeedback('wrong', `✗ Wrong — the correct answer was: ${data.correct_label}`);
      markChoiceResult(choice, 'wrong');
      markChoiceResult(data.correct_answer, 'correct');
      playDeny();
      // Show try-again button — loads a fresh question (costs 1 more hop)
      const btnNext = els.btnNextQ();
      if (btnNext) {
        btnNext.textContent = 'Try again (−1 hop)';
        btnNext.removeAttribute('hidden');
      }
      setStatus('');
    }

    if (remaining <= 0) {
      els.btnQuestion()?.setAttribute('disabled', '');
      showRateLimitOverlay(data.retry_after_ms ?? REFILL_MS);
    }
  } catch (e) {
    setStatus('Network error', 'var(--red)');
    state.answered = false;
    els.qChoices()?.querySelectorAll('button').forEach(b => b.removeAttribute('disabled'));
  }
}

function markChoiceResult(idx, cls) {
  const items = els.qChoices()?.querySelectorAll('li');
  if (items && items[idx]) items[idx].className = cls;
}

function showFeedback(type, msg) {
  const fb = els.qFeedback();
  if (!fb) return;
  fb.className = 'q-feedback ' + type;
  fb.textContent = msg;
  fb.removeAttribute('hidden');
}

// ── Get hint ──────────────────────────────────────────────────────────────
async function getHint() {
  if (state.currentQuestionId == null) return;
  els.btnHint()?.setAttribute('disabled', '');
  setStatus('Getting hint…');

  try {
    const r = await apiFetch(`/v1/odyssey/hint?question_id=${state.currentQuestionId}`);
    const data = await r.json();

    if (r.status === 429) {
      playDeny();
      showRateLimitOverlay(data.retry_after_ms ?? REFILL_MS);
      setStatus('');
      return;
    }

    const fb = els.qFeedback();
    if (fb) {
      fb.className = 'q-feedback hint';
      fb.textContent = '💡 Hint: ' + (data.hint ?? '');
      fb.removeAttribute('hidden');
    }

    const remaining = data.remaining_hops ?? 0;
    safeText(els.hopsRemaining(), String(Math.max(0, remaining)));
    renderHopBar(remaining);
    setStatus('');
  } catch {
    setStatus('Network error', 'var(--red)');
  } finally {
    els.btnHint()?.removeAttribute('disabled');
  }
}

// ── Rate-limit overlay ────────────────────────────────────────────────────
function showRateLimitOverlay(retryMs) {
  state.rateLimitRetryMs = retryMs;
  const overlay = els.rateLimitOverlay();
  if (!overlay) return;
  overlay.removeAttribute('hidden');
  safeText(els.rlMsg(), `You have used all ${MAX_HOPS} hops for this session (rate-limited by your IP address).`);
  startRefillCountdown(retryMs);
  els.btnQuestion()?.setAttribute('disabled', '');
}

// ── Reset journey ─────────────────────────────────────────────────────────
async function resetJourney() {
  if (!confirm('Reset your journey back to Earth? This also resets your hop counter.')) return;
  setStatus('Resetting…');
  try {
    await apiFetch('/v1/odyssey/reset', { method: 'POST' });
    els.questionPanel()?.setAttribute('hidden', '');
    els.rateLimitOverlay()?.setAttribute('hidden', '');
    els.completeOverlay()?.setAttribute('hidden', '');
    if (refillInterval) { clearInterval(refillInterval); refillInterval = null; }
    await loadState();
    setStatus('Journey reset — back to Earth', 'var(--cyan)');
    setTimeout(() => setStatus(''), 3000);
  } catch {
    setStatus('Reset failed', 'var(--red)');
  }
}

// ── Boot ──────────────────────────────────────────────────────────────────
await sceneReady;
await checkHealth();
await loadState();

els.btnQuestion()?.addEventListener('click', loadQuestion);
els.btnReset()?.addEventListener('click', resetJourney);
els.btnHint()?.addEventListener('click', getHint);
els.btnNextQ()?.addEventListener('click', () => {
  els.btnNextQ()?.setAttribute('hidden', '');
  loadQuestion();
});
els.btnRlClose()?.addEventListener('click', () => els.rateLimitOverlay()?.setAttribute('hidden', ''));
els.btnPlayAgain()?.addEventListener('click', resetJourney);

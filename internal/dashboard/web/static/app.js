// Spec: S-016 | Req: I-008d — CVM Dashboard frontend. Vanilla ES2020+, no frameworks.

'use strict';

// ---- State ----

const state = {
  tab: 'timeline',
  timelineScope: 'both',
  browserScope: 'both',
  browserQuery: '',
  browserTag: '',
  browserSelectedKey: null,
  sessionId: null,
  sseConnected: false,
};

let debounceTimer = null;
let statsRefreshTimer = null;

// ---- DOM helpers ----

const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => document.querySelectorAll(sel);

function el(tag, cls, text) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

// ---- Time helpers ----

function relativeTime(isoStr) {
  const now = Date.now();
  const then = new Date(isoStr).getTime();
  const diff = Math.max(0, now - then);
  const s = Math.floor(diff / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  return `${d}d ago`;
}

function formatTokens(n) {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k tokens`;
  return `${n} tokens`;
}

// ---- Badge helpers ----

const TAG_COLORS = {
  learning: 'badge-learning',
  gotcha: 'badge-gotcha',
  decision: 'badge-decision',
  session: 'badge-session',
  summary: 'badge-summary',
  'spec-gap': 'badge-spec-gap',
};

function badgeClass(tag) {
  const base = tag.replace(/^type:/, '');
  return TAG_COLORS[base] || 'badge-default';
}

function renderBadge(tag) {
  const span = el('span', `badge ${badgeClass(tag)}`);
  span.textContent = tag;
  return span;
}

function renderScopeBadge(scope) {
  return el('span', `badge badge-${scope}`, scope);
}

// ---- Tab navigation ----
// Spec: S-016 | Req: I-003a, I-003b, I-003c

function initTabs() {
  $$('nav button[data-tab]').forEach(btn => {
    btn.addEventListener('click', () => {
      switchTab(btn.dataset.tab);
    });
  });

  window.addEventListener('hashchange', () => {
    const hash = window.location.hash.replace('#', '') || 'timeline';
    if (hash !== state.tab) switchTab(hash, false);
  });

  const initial = window.location.hash.replace('#', '') || 'timeline';
  switchTab(initial, false);
}

function switchTab(tab, updateHash = true) {
  state.tab = tab;
  if (updateHash) window.location.hash = tab;

  $$('nav button[data-tab]').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.tab === tab);
  });
  $$('.view[data-tab]').forEach(view => {
    view.classList.toggle('active', view.dataset.tab === tab);
  });

  // Refresh data when switching tabs
  if (tab === 'timeline') loadTimeline();
  if (tab === 'session') loadSession();
  if (tab === 'browser') loadEntries();
  if (tab === 'stats') loadStats();
}

// ---- SSE ----
// Spec: S-016 | Req: I-002p..I-002u, I-004b, I-004g

function initSSE() {
  const src = new EventSource('/api/events');

  src.addEventListener('open', () => {
    state.sseConnected = true;
    updateSSEIndicator();
  });

  src.addEventListener('error', () => {
    state.sseConnected = false;
    updateSSEIndicator();
  });

  src.addEventListener('tick', () => {
    // Only refresh on tick if timeline is active — avoid unnecessary fetches
    // Don't auto-refresh timeline on tick (causes flicker). Only refresh on actual changes.
  });

  src.addEventListener('entry_added', (e) => {
    const data = JSON.parse(e.data || '{}');
    if (state.tab === 'timeline') loadTimeline();
    if (state.tab === 'browser') loadEntries();
    console.debug('[SSE] entry_added', data);
  });

  src.addEventListener('session_updated', (e) => {
    const data = JSON.parse(e.data || '{}');
    if (state.tab === 'session') loadSession();
    console.debug('[SSE] session_updated', data);
  });
}

function updateSSEIndicator() {
  const dot = $('#sse-dot');
  if (dot) {
    dot.classList.toggle('connected', state.sseConnected);
  }
  const label = $('#sse-label');
  if (label) {
    label.textContent = state.sseConnected ? 'live' : 'disconnected';
  }
}

// ---- Timeline ----
// Spec: S-016 | Req: I-004a..I-004g

async function loadTimeline() {
  const scope = state.timelineScope;
  let url = `/api/timeline?limit=50&scope=${scope}`;
  let data;
  try {
    const res = await fetch(url);
    if (!res.ok) return; // Don't clear DOM on error
    data = await res.json();
  } catch (e) {
    console.error('Timeline fetch failed', e);
    return; // Don't clear DOM on error
  }

  // Don't replace valid data with empty response (transient backend errors)
  if ((!data.days || data.days.length === 0) && $('#timeline-list')?.children.length > 0) {
    return;
  }

  const container = $('#timeline-list');
  if (!container) return;

  // Build new content in a fragment first, then swap
  const frag = document.createDocumentFragment();

  if (!data.days || data.days.length === 0) {
    frag.appendChild(el('div', 'empty-state', 'No entries in the selected timeframe.'));
  } else {
    for (const day of data.days) {
      const group = el('div', 'day-group');
      const header = el('div', 'day-header', day.date);
      group.appendChild(header);
      for (const entry of day.entries) {
        group.appendChild(renderEntryCard(entry));
      }
      frag.appendChild(group);
    }
  }

  // Preserve scroll position — Spec: S-016 | Req: I-004f
  const scrollTop = container.scrollTop;
  container.innerHTML = '';
  container.appendChild(frag);
  container.scrollTop = scrollTop;
}

function renderEntryCard(entry) {
  const card = el('div', 'entry-card');

  const header = el('div', 'entry-header');
  const keyEl = el('span', 'entry-key', entry.key);
  header.appendChild(keyEl);

  (entry.tags || []).forEach(tag => header.appendChild(renderBadge(tag)));
  if (entry.scope) header.appendChild(renderScopeBadge(entry.scope));
  card.appendChild(header);

  const meta = el('div', 'entry-meta');
  meta.appendChild(el('span', '', relativeTime(entry.updated_at)));
  if (entry.token_estimate) {
    meta.appendChild(el('span', '', `~${entry.token_estimate} tok`));
  }
  card.appendChild(meta);

  if (entry.first_line) {
    card.appendChild(el('div', 'first-line', entry.first_line));
  }

  // Body (lazy-loaded on click) — Spec: S-016 | Req: I-004d
  const bodyDiv = el('div', 'body-content');
  card.appendChild(bodyDiv);

  let bodyLoaded = false;

  card.addEventListener('click', async () => {
    card.classList.toggle('expanded');
    if (card.classList.contains('expanded') && !bodyLoaded) {
      bodyLoaded = true;
      bodyDiv.textContent = 'Loading...';
      try {
        const scope = entry.scope || 'global';
        const res = await fetch(`/api/entries?q=&scope=${scope}&limit=1&offset=0`);
        // Try to find the entry by key in entries endpoint
        const data = await fetch(`/api/entries?scope=${scope}&limit=500`);
        const json = await data.json();
        const found = (json.entries || []).find(e => e.key === entry.key);
        bodyDiv.textContent = found ? found.body : '(body not available)';
      } catch {
        bodyDiv.textContent = '(failed to load body)';
      }
    }
  });

  return card;
}

// ---- Session ----
// Spec: S-016 | Req: I-005a..I-005g

async function loadSession() {
  // First, get all active buffers
  let listData;
  try {
    const res = await fetch('/api/session');
    listData = await res.json();
  } catch {
    return;
  }

  const buffers = listData.buffers || [];
  const selector = $('#session-selector');
  const container = $('#session-lines');

  if (!buffers.length) {
    // Spec: S-016 | Req: I-005f
    if (selector) selector.innerHTML = '';
    if (container) {
      container.innerHTML = '';
      container.appendChild(el('div', 'session-empty', 'No active session buffer found.'));
    }
    return;
  }

  // Populate selector — Spec: S-016 | Req: I-005b, I-005c
  if (selector) {
    const currentId = state.sessionId || buffers[0].session_id;
    selector.innerHTML = '';
    buffers.forEach(buf => {
      const opt = document.createElement('option');
      opt.value = buf.session_id;
      opt.textContent = `${buf.session_id} (${buf.line_count} lines)`;
      if (buf.session_id === currentId) opt.selected = true;
      selector.appendChild(opt);
    });
    state.sessionId = currentId;
    selector.onchange = () => {
      state.sessionId = selector.value;
      fetchSessionDetail(state.sessionId);
    };
  }

  fetchSessionDetail(state.sessionId || buffers[0].session_id);
}

async function fetchSessionDetail(id) {
  if (!id) return;
  state.sessionId = id;

  const container = $('#session-lines');
  if (!container) return;

  let data;
  try {
    const res = await fetch(`/api/session?id=${encodeURIComponent(id)}`);
    if (!res.ok) return;
    data = await res.json();
  } catch {
    return;
  }

  // Don't replace valid data with empty response
  if ((!data.found || !data.lines || data.lines.length === 0) && container.children.length > 1) {
    return;
  }

  const wasAtBottom = container.scrollHeight - container.scrollTop <= container.clientHeight + 40;

  container.innerHTML = '';

  if (!data.found || !data.lines || data.lines.length === 0) {
    container.appendChild(el('div', 'session-empty', 'Buffer is empty.'));
    return;
  }

  data.lines.forEach(line => {
    const row = el('div', 'session-line');

    const ts = el('span', 'session-ts', line.timestamp || '');
    row.appendChild(ts);

    const type = el('span', 'session-type');
    if (line.type === 'TOOL') {
      type.appendChild(renderBadge(line.tool || 'TOOL'));
    } else if (line.type === 'USER') {
      type.appendChild(el('span', 'badge badge-decision', 'USER'));
    } else {
      type.appendChild(el('span', 'badge badge-default', 'RAW'));
    }
    row.appendChild(type);

    row.appendChild(el('span', 'session-content', line.content || ''));
    container.appendChild(row);
  });

  // Auto-scroll to bottom — Spec: S-016 | Req: I-005g
  if (wasAtBottom) {
    container.scrollTop = container.scrollHeight;
  }
}

// ---- Browser ----
// Spec: S-016 | Req: I-006a..I-006g

function initBrowser() {
  const qInput = $('#browser-q');
  if (qInput) {
    qInput.addEventListener('input', () => {
      state.browserQuery = qInput.value;
      clearTimeout(debounceTimer);
      // Debounce 300ms — Spec: S-016 | Req: I-006a
      debounceTimer = setTimeout(() => loadEntries(), 300);
    });
  }

  const tagInput = $('#browser-tag');
  if (tagInput) {
    tagInput.addEventListener('change', () => {
      state.browserTag = tagInput.value;
      loadEntries();
    });
  }

  $$('#browser-scope .scope-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      state.browserScope = btn.dataset.scope;
      $$('#browser-scope .scope-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      loadEntries();
    });
  });
}

async function loadEntries() {
  const t0 = performance.now();
  const q = state.browserQuery;
  const tag = state.browserTag;
  const scope = state.browserScope;

  let url = `/api/entries?scope=${scope}&limit=100`;
  if (q) url += `&q=${encodeURIComponent(q)}`;
  if (tag) url += `&tag=${encodeURIComponent(tag)}`;

  let data;
  try {
    const res = await fetch(url);
    if (!res.ok) return;
    data = await res.json();
  } catch {
    return;
  }

  // Don't replace valid data with empty response on transient errors
  if ((!data.entries || !data.entries.length) && !q && !tag && $('#browser-list')?.children.length > 1) {
    return;
  }

  const elapsed = Math.round(performance.now() - t0);

  const list = $('#browser-list');
  if (!list) return;

  const meta = $('#browser-meta');
  if (meta) {
    meta.textContent = `${data.total || 0} entries · ${elapsed}ms`;
  }

  list.innerHTML = '';

  if (!data.entries || !data.entries.length) {
    list.appendChild(el('div', 'empty-state', 'No entries found.'));
    return;
  }

  data.entries.forEach(entry => {
    const card = el('div', 'entry-card');

    const header = el('div', 'entry-header');
    header.appendChild(el('span', 'entry-key', entry.key));
    (entry.tags || []).forEach(tag => header.appendChild(renderBadge(tag)));
    header.appendChild(renderScopeBadge(entry.scope));
    card.appendChild(header);

    const meta = el('div', 'entry-meta');
    meta.appendChild(el('span', '', relativeTime(entry.updated_at)));
    meta.appendChild(el('span', '', `~${entry.token_estimate} tok`));
    card.appendChild(meta);

    // Click shows detail — Spec: S-016 | Req: I-006e
    card.addEventListener('click', () => {
      state.browserSelectedKey = entry.key;
      $$('#browser-list .entry-card').forEach(c => c.classList.remove('expanded'));
      card.classList.add('expanded');
      showBrowserDetail(entry);
    });

    list.appendChild(card);
  });
}

function showBrowserDetail(entry) {
  const panel = $('#browser-detail');
  if (!panel) return;

  panel.classList.remove('empty');
  panel.innerHTML = '';

  const keyEl = el('div', 'detail-key', entry.key);
  panel.appendChild(keyEl);

  const meta = el('div', 'detail-meta');
  meta.appendChild(el('span', '', `Scope: ${entry.scope}`));
  meta.appendChild(el('span', '', `Updated: ${new Date(entry.updated_at).toLocaleString()}`));
  meta.appendChild(el('span', '', `~${entry.token_estimate} tokens`));
  panel.appendChild(meta);

  const tagsRow = el('div', 'entry-header');
  (entry.tags || []).forEach(tag => tagsRow.appendChild(renderBadge(tag)));
  panel.appendChild(tagsRow);

  const body = el('div', 'detail-body', entry.body || '');
  panel.appendChild(body);
}

// ---- Stats ----
// Spec: S-016 | Req: I-007a..I-007e

async function loadStats() {
  let data;
  try {
    const res = await fetch('/api/stats');
    data = await res.json();
  } catch {
    return;
  }

  renderScopeStats('global', data.global || {});
  renderScopeStats('local', data.local || {});

  const activeSessEl = $('#stats-active-sessions');
  if (activeSessEl) {
    activeSessEl.textContent = data.active_sessions || 0;
  }
}

function renderScopeStats(scope, stats) {
  const totalEl = $(`#stats-${scope}-total`);
  const enabledEl = $(`#stats-${scope}-enabled`);
  const staleEl = $(`#stats-${scope}-stale`);
  const tokensEl = $(`#stats-${scope}-tokens`);
  const tagsEl = $(`#stats-${scope}-tags`);

  if (totalEl) totalEl.textContent = stats.total || 0;
  if (enabledEl) enabledEl.textContent = stats.enabled || 0;
  if (staleEl) staleEl.textContent = stats.stale || 0;
  if (tokensEl) tokensEl.textContent = formatTokens(stats.total_tokens || 0);

  if (tagsEl && stats.by_tag) {
    tagsEl.innerHTML = '';
    const sorted = Object.entries(stats.by_tag)
      .sort((a, b) => b[1] - a[1]);

    sorted.forEach(([tag, count]) => {
      const row = el('div', 'tag-row');
      row.appendChild(el('span', 'tag-name', tag));
      row.appendChild(el('span', 'tag-count', String(count)));
      tagsEl.appendChild(row);
    });

    if (!sorted.length) {
      tagsEl.appendChild(el('span', '', 'No tags'));
    }
  }
}

// ---- Scope toggle for timeline ----

function initTimelineScope() {
  $$('#timeline-scope .scope-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      state.timelineScope = btn.dataset.scope;
      $$('#timeline-scope .scope-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      loadTimeline();
    });
  });
}

// ---- Stats auto-refresh ----
// Spec: S-016 | Req: I-007d

function startStatsRefresh() {
  clearInterval(statsRefreshTimer);
  statsRefreshTimer = setInterval(() => {
    if (state.tab === 'stats') loadStats();
  }, 10000);
}

// ---- Init ----

document.addEventListener('DOMContentLoaded', () => {
  initTabs();
  initSSE();
  initBrowser();
  initTimelineScope();
  startStatsRefresh();
});

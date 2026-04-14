// Spec: S-016 | Req: I-008d — CVM Dashboard frontend. Vanilla ES2020+, no frameworks.

'use strict';

// ---- State ----

const state = {
  tab: 'sessions',
  knowledgeScope: 'both',
  knowledgeQuery: '',
  knowledgeTag: '',
  knowledgeSelectedKey: null,
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

function formatDuration(startIso, endIso) {
  const start = new Date(startIso).getTime();
  const end = new Date(endIso).getTime();
  const diffMin = Math.round(Math.max(0, end - start) / 60000);
  if (diffMin < 60) return `${diffMin}m`;
  const h = Math.floor(diffMin / 60);
  const m = diffMin % 60;
  return m > 0 ? `${h}h ${m}m` : `${h}h`;
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

function initTabs() {
  $$('nav button[data-tab]').forEach(btn => {
    btn.addEventListener('click', () => {
      switchTab(btn.dataset.tab);
    });
  });

  window.addEventListener('hashchange', () => {
    const hash = window.location.hash.replace('#', '') || 'sessions';
    if (hash !== state.tab) switchTab(hash, false);
  });

  const initial = window.location.hash.replace('#', '') || 'sessions';
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

  if (tab === 'sessions') loadSessions();
  if (tab === 'knowledge') loadKnowledge();
  if (tab === 'stats') loadStats();
}

// ---- SSE ----

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

  src.addEventListener('entry_added', (e) => {
    const data = JSON.parse(e.data || '{}');
    if (state.tab === 'sessions') loadSessions();
    if (state.tab === 'knowledge') loadKnowledge();
    console.debug('[SSE] entry_added', data);
  });

  src.addEventListener('session_updated', (e) => {
    const data = JSON.parse(e.data || '{}');
    if (state.tab === 'sessions') loadSessions();
    console.debug('[SSE] session_updated', data);
  });
}

function updateSSEIndicator() {
  const dot = $('#sse-dot');
  if (dot) dot.classList.toggle('connected', state.sseConnected);
  const label = $('#sse-label');
  if (label) label.textContent = state.sseConnected ? 'live' : 'disconnected';
}

// ---- Sessions tab ----

async function loadSessions() {
  let data;
  try {
    const res = await fetch('/api/sessions');
    if (!res.ok) return;
    data = await res.json();
  } catch (e) {
    console.error('Sessions fetch failed', e);
    return;
  }

  const sessions = data.sessions || [];
  const meta = $('#sessions-meta');
  const list = $('#sessions-list');
  if (!list) return;

  const activeCount = sessions.filter(s => s.status === 'active').length;
  const totalCount = sessions.length;
  if (meta) {
    meta.textContent = `${totalCount} session${totalCount !== 1 ? 's' : ''} · ${activeCount} active`;
  }

  const frag = document.createDocumentFragment();

  if (!sessions.length) {
    frag.appendChild(el('div', 'empty-state', 'No sessions found.'));
  } else {
    sessions.forEach(session => frag.appendChild(renderSessionCard(session)));
  }

  list.innerHTML = '';
  list.appendChild(frag);
}

function renderSessionCard(session) {
  const isActive = session.status === 'active';
  const isEnded = session.status === 'ended';
  const isLegacy = session.status === 'legacy';
  const isSummarized = session.status === 'summarized';
  const isStale = session.status === 'stale';

  let cardClass = 'session-card';
  if (isActive) cardClass += ' session-card--active';
  else if (isStale) cardClass += ' session-card--stale';
  else cardClass += ' session-card--summarized';

  const card = el('div', cardClass);

  // --- Header row ---
  const header = el('div', 'session-card__header');

  const idEl = el('span', 'session-card__id', session.id.substring(0, 16) + (session.id.length > 16 ? '…' : ''));
  idEl.title = session.id;
  header.appendChild(idEl);

  const statusLabel = isActive ? 'ACTIVE' : isStale ? 'STALE' : 'SUMMARIZED';
  const badgeClass = isActive ? 'badge-active' : isStale ? 'badge-stale' : 'badge-summarized';
  header.appendChild(el('span', `badge ${badgeClass}`, statusLabel));

  if (session.project_dir) {
    header.appendChild(el('span', 'session-card__project', session.project_dir));
  }

  card.appendChild(header);

  // --- Meta row ---
  const metaRow = el('div', 'session-card__meta');
  if (session.created_at) {
    metaRow.appendChild(el('span', '', new Date(session.created_at).toLocaleString()));
  }
  // Duration for active/stale from timestamps; for summarized from meta time_range
  if ((isActive || isStale) && session.created_at && session.updated_at) {
    metaRow.appendChild(el('span', 'session-card__duration', formatDuration(session.created_at, session.updated_at)));
  } else if (isSummarized && session.meta?.time_range) {
    metaRow.appendChild(el('span', 'session-card__duration', session.meta.time_range));
  }
  // Event count
  if ((isActive || isStale) && session.line_count != null) {
    metaRow.appendChild(el('span', '', `${session.line_count} events`));
  } else if (isSummarized && session.meta?.event_count) {
    metaRow.appendChild(el('span', '', `${session.meta.event_count} events`));
  }
  // Estimated tokens
  if (isSummarized && session.meta?.est_tokens) {
    metaRow.appendChild(el('span', '', `~${session.meta.est_tokens} tok`));
  }
  if (isActive) {
    metaRow.appendChild(el('span', 'session-card__live', `last activity ${relativeTime(session.updated_at)}`));
  }
  card.appendChild(metaRow);

  // --- Summary preview (summarized sessions) ---
  if (!isActive && session.summary_body) {
    const summaryEl = el('div', 'session-card__summary');
    // Show first 3 lines of the summary body
    const lines = session.summary_body.split('\n').filter(l => l.trim()).slice(0, 3);
    lines.forEach(line => {
      summaryEl.appendChild(el('div', 'session-card__summary-line', line));
    });
    card.appendChild(summaryEl);
  }

  // --- Knowledge section ---
  const knowledge = session.knowledge || [];
  if (knowledge.length > 0) {
    const kSection = el('div', 'session-card__knowledge');
    const kToggle = el('button', 'session-card__knowledge-toggle',
      `${knowledge.length} linked knowledge entr${knowledge.length === 1 ? 'y' : 'ies'}`);
    kSection.appendChild(kToggle);

    const kList = el('div', 'session-card__knowledge-list');
    kList.style.display = 'none';
    knowledge.forEach(kEntry => {
      kList.appendChild(renderKnowledgePill(kEntry));
    });
    kSection.appendChild(kList);

    kToggle.addEventListener('click', (e) => {
      e.stopPropagation();
      const isOpen = kList.style.display !== 'none';
      kList.style.display = isOpen ? 'none' : 'block';
      kToggle.classList.toggle('open', !isOpen);
    });

    card.appendChild(kSection);
  }

  // --- Expandable detail ---
  const detail = el('div', 'session-card__detail');
  card.appendChild(detail);

  let detailLoaded = false;

  card.addEventListener('click', (e) => {
    if (e.target.closest('.session-card__knowledge')) return;
    card.classList.toggle('expanded');
    if (card.classList.contains('expanded') && !detailLoaded) {
      detailLoaded = true;
      renderSessionDetail(detail, session);
    }
  });

  return card;
}

function renderKnowledgePill(entry) {
  const pill = el('div', 'knowledge-pill');

  const header = el('div', 'knowledge-pill__header');
  header.appendChild(el('span', 'entry-key', entry.key));
  (entry.tags || []).forEach(tag => header.appendChild(renderBadge(tag)));
  header.appendChild(renderScopeBadge(entry.scope));
  pill.appendChild(header);

  if (entry.body) {
    const preview = entry.body.split('\n').filter(l => l.trim())[0] || '';
    if (preview) {
      pill.appendChild(el('div', 'knowledge-pill__preview', preview.substring(0, 120)));
    }
  }

  return pill;
}

function renderSessionDetail(container, session) {
  container.innerHTML = '';

  if (session.status === 'active') {
    // Show full buffer lines
    if (!session.lines || !session.lines.length) {
      container.appendChild(el('div', 'session-empty', 'Buffer is empty.'));
      return;
    }
    const bufContainer = el('div', 'session-container');
    session.lines.forEach(line => {
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
      bufContainer.appendChild(row);
    });
    container.appendChild(bufContainer);
  } else {
    // Show full summary
    if (!session.summary_body) {
      container.appendChild(el('div', 'session-empty', 'No summary body.'));
      return;
    }
    const summaryFull = el('div', 'session-summary-full');
    summaryFull.textContent = session.summary_body;
    container.appendChild(summaryFull);
  }

  // Show full knowledge entries
  const knowledge = session.knowledge || [];
  if (knowledge.length > 0) {
    const kHeader = el('div', 'session-detail-section-header', `Linked Knowledge (${knowledge.length})`);
    container.appendChild(kHeader);
    knowledge.forEach(kEntry => {
      const entryEl = el('div', 'session-detail-knowledge-entry');
      const eh = el('div', 'entry-header');
      eh.appendChild(el('span', 'entry-key', kEntry.key));
      (kEntry.tags || []).forEach(tag => eh.appendChild(renderBadge(tag)));
      eh.appendChild(renderScopeBadge(kEntry.scope));
      entryEl.appendChild(eh);
      const meta = el('div', 'entry-meta');
      meta.appendChild(el('span', '', relativeTime(kEntry.updated_at)));
      meta.appendChild(el('span', '', `~${kEntry.token_estimate} tok`));
      entryEl.appendChild(meta);
      if (kEntry.body) {
        entryEl.appendChild(el('div', 'detail-body', kEntry.body));
      }
      container.appendChild(entryEl);
    });
  }
}

// ---- Knowledge tab (merged Timeline + Browser) ----

function initKnowledge() {
  const qInput = $('#knowledge-q');
  if (qInput) {
    qInput.addEventListener('input', () => {
      state.knowledgeQuery = qInput.value;
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => loadKnowledge(), 300);
    });
  }

  const tagInput = $('#knowledge-tag');
  if (tagInput) {
    tagInput.addEventListener('change', () => {
      state.knowledgeTag = tagInput.value;
      loadKnowledge();
    });
  }

  $$('#knowledge-scope .scope-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      state.knowledgeScope = btn.dataset.scope;
      $$('#knowledge-scope .scope-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      loadKnowledge();
    });
  });
}

async function loadKnowledge() {
  const t0 = performance.now();
  const q = state.knowledgeQuery;
  const tag = state.knowledgeTag;
  const scope = state.knowledgeScope;

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

  if ((!data.entries || !data.entries.length) && !q && !tag && $('#knowledge-list')?.children.length > 1) {
    return;
  }

  const elapsed = Math.round(performance.now() - t0);
  const list = $('#knowledge-list');
  if (!list) return;

  const meta = $('#knowledge-meta');
  if (meta) meta.textContent = `${data.total || 0} entries · ${elapsed}ms`;

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

    const metaEl = el('div', 'entry-meta');
    metaEl.appendChild(el('span', '', relativeTime(entry.updated_at)));
    metaEl.appendChild(el('span', '', `~${entry.token_estimate} tok`));
    card.appendChild(metaEl);

    card.addEventListener('click', () => {
      state.knowledgeSelectedKey = entry.key;
      $$('#knowledge-list .entry-card').forEach(c => c.classList.remove('expanded'));
      card.classList.add('expanded');
      showKnowledgeDetail(entry);
    });

    list.appendChild(card);
  });
}

function showKnowledgeDetail(entry) {
  const panel = $('#knowledge-detail');
  if (!panel) return;

  panel.classList.remove('empty');
  panel.innerHTML = '';

  panel.appendChild(el('div', 'detail-key', entry.key));

  const meta = el('div', 'detail-meta');
  meta.appendChild(el('span', '', `Scope: ${entry.scope}`));
  meta.appendChild(el('span', '', `Updated: ${new Date(entry.updated_at).toLocaleString()}`));
  meta.appendChild(el('span', '', `~${entry.token_estimate} tokens`));
  panel.appendChild(meta);

  const tagsRow = el('div', 'entry-header');
  (entry.tags || []).forEach(tag => tagsRow.appendChild(renderBadge(tag)));
  panel.appendChild(tagsRow);

  panel.appendChild(el('div', 'detail-body', entry.body || ''));
}

// ---- Stats ----

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
  if (activeSessEl) activeSessEl.textContent = data.active_sessions || 0;
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
    const sorted = Object.entries(stats.by_tag).sort((a, b) => b[1] - a[1]);
    sorted.forEach(([tag, count]) => {
      const row = el('div', 'tag-row');
      row.appendChild(el('span', 'tag-name', tag));
      row.appendChild(el('span', 'tag-count', String(count)));
      tagsEl.appendChild(row);
    });
    if (!sorted.length) tagsEl.appendChild(el('span', '', 'No tags'));
  }
}

// ---- Stats auto-refresh ----

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
  initKnowledge();
  startStatsRefresh();
});

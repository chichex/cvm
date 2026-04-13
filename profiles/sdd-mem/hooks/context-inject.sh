#!/usr/bin/env bash
# Spec: S-010 | Req: B-002 | Spec: S-011 | Req: E-002 | Context injection hook
# Injects a compact KB summary into the session context at startup.
# Also cleans up orphaned session buffers older than 24h (E-002).
# Pure shell + single python3 call — no LLM calls (I-002). Must complete in < 2s (I-003).

set -euo pipefail

# --- E-002: Orphaned session buffer cleanup ---
# Scan KB local for entries with tag "session-buffer" older than 24h and delete them.
CVM_HOME_CLEANUP="${HOME}/.cvm"
cwd_cleanup="$(pwd)"
safe_cleanup=$(echo "$cwd_cleanup" | sed 's|/|-|g' | sed 's|^-||')
safe_cleanup="${safe_cleanup: -100}"
local_kb_cleanup="${CVM_HOME_CLEANUP}/local/kb/${safe_cleanup}"

if [ -d "$local_kb_cleanup" ]; then
  index_file="${local_kb_cleanup}/.index.json"
  if [ -f "$index_file" ]; then
    now_epoch=$(date +%s)
    # Find session-buffer entries older than 24h and delete them
    orphans=$(python3 -c "
import json, sys, os
from datetime import datetime, timezone

index_file = sys.argv[1]
now_epoch = int(sys.argv[2])
cutoff = now_epoch - 86400  # 24h in seconds

with open(index_file) as f:
    idx = json.load(f)

for e in idx.get('entries', []):
    tags = e.get('tags', [])
    if 'session-buffer' not in tags:
        continue
    created = e.get('created_at', e.get('updated_at', ''))
    if not created:
        continue
    try:
        dt = datetime.fromisoformat(created)
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        epoch = int(dt.timestamp())
        if epoch < cutoff:
            print(e['key'])
    except Exception:
        pass
" "$index_file" "$now_epoch" 2>/dev/null || true)

    if [ -n "$orphans" ]; then
      while IFS= read -r orphan_key; do
        if [ -n "$orphan_key" ]; then
          echo "[context-inject] warning: deleting orphaned session buffer: ${orphan_key}" >&2
          cvm kb rm "$orphan_key" --local 2>/dev/null || true
        fi
      done <<< "$orphans"
    fi
  fi
fi

MAX_ENTRIES="${CVM_CONTEXT_ENTRY_COUNT:-10}"
MAX_TOKENS="${CVM_CONTEXT_MAX_TOKENS:-2000}"

# --- Locate KB directories ---
CVM_HOME="${HOME}/.cvm"
GLOBAL_KB="${CVM_HOME}/global/kb"
LOCAL_KB=""

# Derive local KB path from cwd (same hashing as config.hashPath in Go — keeps underscores, strips leading -)
cwd="$(pwd)"
safe=$(echo "$cwd" | sed 's|/|-|g' | sed 's|^-||')
safe="${safe: -100}"
local_kb_candidate="${CVM_HOME}/local/kb/${safe}"
if [ -d "$local_kb_candidate" ]; then
  LOCAL_KB="$local_kb_candidate"
fi

# --- Single python3 call to process everything (I-003: < 2s) ---
result=$(python3 -c "
import json, sys, os
from datetime import datetime, timezone

max_entries = int(sys.argv[1])
max_tokens = int(sys.argv[2])
kb_dirs = []
if sys.argv[3] and os.path.isdir(sys.argv[3]):
    kb_dirs.append(sys.argv[3])
if sys.argv[4] and os.path.isdir(sys.argv[4]):
    kb_dirs.append(sys.argv[4])

# Collect all enabled entries from all scopes
entries = []
for kb_dir in kb_dirs:
    index_file = os.path.join(kb_dir, '.index.json')
    if not os.path.isfile(index_file):
        continue
    try:
        with open(index_file) as f:
            idx = json.load(f)
    except (json.JSONDecodeError, IOError):
        continue
    for e in idx.get('entries', []):
        if e.get('enabled', True):
            e['_kb_dir'] = kb_dir
            entries.append(e)

if not entries:
    sys.exit(0)

total_entries = len(entries)

# Sort: last_referenced desc (non-zero first), then updated_at desc
def sort_key(e):
    lr = e.get('last_referenced', '')
    ua = e.get('updated_at', '')
    if lr and not lr.startswith('0001'):
        primary = lr
    else:
        primary = '0001-01-01'
    return (primary, ua)

entries.sort(key=sort_key, reverse=True)

# Build context table with token budget
now = datetime.now(timezone.utc)
lines = []
total_tokens = 0

for e in entries[:max_entries * 2]:  # read extra in case some fail
    if len(lines) >= max_entries:
        break

    key = e['key']
    kb_dir = e['_kb_dir']
    tags = e.get('tags', [])

    # Relative time
    ua = e.get('updated_at', '')
    updated = '?'
    if ua:
        try:
            dt = datetime.fromisoformat(ua)
            if dt.tzinfo is None:
                dt = dt.replace(tzinfo=timezone.utc)
            delta = now - dt
            if delta.days > 0:
                updated = f'{delta.days}d ago'
            elif delta.seconds >= 3600:
                updated = f'{delta.seconds // 3600}h ago'
            else:
                updated = f'{max(1, delta.seconds // 60)}m ago'
        except Exception:
            pass

    # Extract type from tags
    entry_type = ''
    for t in tags:
        if t.startswith('type:'):
            entry_type = t[5:]
            break
    if not entry_type and tags:
        entry_type = tags[0]

    # Read first line of body
    entry_file = os.path.join(kb_dir, 'entries', key + '.md')
    first_line = ''
    try:
        with open(entry_file) as f:
            content = f.read()
        parts = content.split('---\n\n', 1)
        body = parts[1] if len(parts) > 1 else parts[0]
        for bline in body.strip().split('\n'):
            bline = bline.strip()
            if bline:
                first_line = bline[:80]
                break
    except (IOError, IndexError):
        pass

    # Token budget check
    line = f'| {key} | {entry_type} | {updated} | {first_line} |'
    line_tokens = len(line) // 4
    new_total = total_tokens + line_tokens
    if new_total > max_tokens and len(lines) > 0:
        break

    lines.append(line)
    total_tokens = new_total

if not lines:
    sys.exit(0)

entry_count = len(lines)

# Build context block
ctx = f'<cvm-context>\n## Recent KB (showing {entry_count} of {total_entries} entries, ~{total_tokens}t estimated)\n| Key | Type | Updated | Summary |\n|-----|------|---------|---------|'
for line in lines:
    ctx += '\n' + line
ctx += '\n</cvm-context>'

# Output as hookSpecificOutput JSON
output = {
    'hookSpecificOutput': {
        'hookEventName': 'SessionStart',
        'additionalContext': ctx
    }
}
print(json.dumps(output))
" "$MAX_ENTRIES" "$MAX_TOKENS" "$GLOBAL_KB" "${LOCAL_KB:-}" 2>/dev/null) || exit 0

if [ -n "$result" ]; then
  echo "$result"
fi

#!/usr/bin/env bash
# Spec: S-011 | Req: B-004 | SessionEnd: read buffer, generate summary with haiku, cleanup
# Replaces session-digest.sh + auto-summary.sh.
# MUST NOT block session end (I-003). Cleans up buffer even on failure (E-003).

set -euo pipefail

INPUT=$(cat)

# Extract session_id
session_id=$(echo "$INPUT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('session_id',''))" 2>/dev/null) || true
if [ -z "$session_id" ]; then
  echo "[session-summary] warning: session_id missing, skipping" >&2
  exit 0
fi

buffer_key="session-buffer-${session_id}"

# Read buffer body (strip frontmatter: everything up to and including the first blank line after ---)
buffer=$(cvm kb show "$buffer_key" --local 2>/dev/null | sed '1,/^$/d' || true)

# Cleanup buffer regardless of what happens next
cleanup_buffer() {
  cvm kb rm "$buffer_key" --local 2>/dev/null || true
}
trap cleanup_buffer EXIT

# E-001: short session — skip LLM call
if [ -z "$buffer" ]; then
  exit 0
fi

line_count=$(echo "$buffer" | wc -l | tr -d ' ')
if [ "$line_count" -lt 3 ]; then
  cleanup_buffer
  exit 0
fi

# B-004: call claude -p with buffer content
prompt="Summarize this coding session from the captured events.
Generate JSON: {\"request\": \"...\", \"accomplished\": \"...\", \"discovered\": \"...\", \"next_steps\": \"...\"}
Max 1-2 sentences per field. Output ONLY the JSON.

<events>
${buffer}
</events>"

summary=""
if command -v claude &>/dev/null; then
  MODEL="${CVM_AUTOSUMMARY_MODEL:-haiku}"
  summary=$(echo "$prompt" | claude -p --model "$MODEL" 2>/dev/null) || summary=""
fi

# E-003: claude -p failed
if [ -z "$summary" ]; then
  echo "[session-summary] warning: claude -p failed or not available, buffer will be cleaned up" >&2
  cleanup_buffer
  exit 0
fi

# Parse JSON response; if malformed, store raw text
parsed=$(python3 -c "
import json, sys
raw = sys.stdin.read().strip()
# Strip markdown code fences if present
if raw.startswith('\`\`\`'):
    raw = raw.split('\n', 1)[1] if '\n' in raw else raw
    if raw.endswith('\`\`\`'):
        raw = raw[:-3]
    raw = raw.strip()
try:
    data = json.loads(raw)
    parts = []
    if data.get('request'):
        parts.append(f\"Request: {data['request']}\")
    if data.get('accomplished'):
        parts.append(f\"Accomplished: {data['accomplished']}\")
    if data.get('discovered'):
        parts.append(f\"Discovered: {data['discovered']}\")
    if data.get('next_steps'):
        parts.append(f\"Next: {data['next_steps']}\")
    print(' | '.join(parts))
except (json.JSONDecodeError, KeyError):
    print(raw[:500])
" <<< "$summary" 2>/dev/null) || parsed="$summary"

if [ -n "$parsed" ]; then
  ts=$(date +%Y%m%d-%H%M%S)

  # Enrich with session metadata for dashboard
  project_dir="${PWD:-unknown}"
  event_count="$line_count"
  buffer_chars=${#buffer}
  est_tokens=$((buffer_chars / 4))

  # Extract duration from buffer timestamps (first and last [HH:MM] entries)
  first_ts=$(echo "$buffer" | grep -oE '^\[([0-9]{2}:[0-9]{2})\]' | head -1 | tr -d '[]') || true
  last_ts=$(echo "$buffer" | grep -oE '^\[([0-9]{2}:[0-9]{2})\]' | tail -1 | tr -d '[]') || true

  # Build metadata header
  meta="[meta] project=${project_dir} | events=${event_count} | est_tokens=${est_tokens}"
  if [ -n "$first_ts" ] && [ -n "$last_ts" ]; then
    meta="${meta} | time_range=${first_ts}-${last_ts}"
  fi

  enriched_body="${meta}
${parsed}"

  cvm kb put "session-${ts}" --body "$enriched_body" --tag "session,summary" 2>/dev/null || \
    echo "[session-summary] warning: failed to store summary in KB" >&2
fi

cleanup_buffer
exit 0

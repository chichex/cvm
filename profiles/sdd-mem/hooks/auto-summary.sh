#!/usr/bin/env bash
# Spec: S-010 | Req: B-004 | Auto session summary
# Generates a structured summary from the session digest using Haiku.
# MUST NOT block session end (I-001). Idempotent (I-005).

set -euo pipefail

ENABLED="${CVM_AUTOSUMMARY_ENABLED:-true}"
MODEL="${CVM_AUTOSUMMARY_MODEL:-haiku}"
MAX_TOKENS="${CVM_AUTOSUMMARY_MAX_TOKENS:-500}"

# Derive project dir name (same logic as session-digest.sh and Claude Code)
cwd="$(pwd)"
project_dir=$(echo "$cwd" | sed 's|/|-|g; s|_|-|g')
DIGEST_FILE="/tmp/cvm-session-digest-${project_dir}.txt"

# --- Guard clauses ---
if [ "$ENABLED" != "true" ]; then
  exit 0
fi

if [ ! -f "$DIGEST_FILE" ]; then
  # No digest = session was too short or transcript missing. Already logged by session-digest.sh.
  exit 0
fi

digest_size=$(wc -c < "$DIGEST_FILE" | tr -d ' ')
if [ "$digest_size" -lt 100 ]; then
  echo "digest too small (${digest_size} chars), skipping auto-summary" >&2
  rm -f "$DIGEST_FILE"
  exit 0
fi

digest_content=$(cat "$DIGEST_FILE")

# --- Call claude -p for summary ---
prompt="Given this session digest, generate a JSON summary:
{\"request\": \"...\", \"accomplished\": \"...\", \"discovered\": \"...\", \"next_steps\": \"...\"}
Be concise. Max 1-2 sentences per field. Output ONLY the JSON.

<digest>
${digest_content}
</digest>"

summary=""
if command -v claude &>/dev/null; then
  summary=$(echo "$prompt" | claude -p --model "$MODEL" 2>/dev/null) || true
fi

if [ -z "$summary" ]; then
  echo "claude -p failed or not available, skipping auto-summary" >&2
  rm -f "$DIGEST_FILE"
  exit 0
fi

# --- Parse and store ---
# Try to extract JSON fields; if parsing fails, store raw summary
parsed=$(python3 -c "
import json, sys
raw = sys.stdin.read().strip()
# Handle markdown code blocks
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
  cvm kb put "session-${ts}" --body "$parsed" --tag "session,summary" 2>/dev/null || \
    echo "failed to store auto-summary in KB" >&2
fi

# --- Cleanup ---
rm -f "$DIGEST_FILE"

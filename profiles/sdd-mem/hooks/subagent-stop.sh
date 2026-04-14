#!/usr/bin/env bash
# Spec: S-017 | Req: B-004, B-014 | SubagentStop: agent capture + Key Learnings extraction
# Appends agent summary via cvm session append, then extracts Key Learnings from stdout.
# MUST complete in < 500ms for capture portion (I-001).

set -euo pipefail

# Read hook input from stdin (shared by both B-004 capture and Key Learnings logic)
INPUT=$(cat)

# --- B-004, B-014: Capture agent summary via cvm session append ---
session_id=$(echo "$INPUT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('session_id',''))" 2>/dev/null) || true

if [ -z "$session_id" ]; then
  echo "[subagent-stop] warning: session_id missing, skipping capture" >&2
fi

if [ -n "$session_id" ]; then
  agent_type=$(echo "$INPUT" | python3 -c "
import json, sys
data = json.load(sys.stdin)
print(data.get('agent_type', data.get('type', 'unknown')))
" 2>/dev/null) || agent_type="unknown"

  last_msg=$(echo "$INPUT" | python3 -c "
import json, sys
data = json.load(sys.stdin)
msg = data.get('last_assistant_message', data.get('output', ''))
if isinstance(msg, list):
    texts = [b.get('text','') for b in msg if isinstance(b,dict) and b.get('type')=='text']
    msg = ' '.join(texts)
msg = str(msg).strip()
print(msg[:300])
" 2>/dev/null) || last_msg=""

  if [ -n "$session_id" ] && [ -n "$last_msg" ]; then
    cvm session append "$session_id" --type agent --agent-type "$agent_type" --content "$last_msg" 2>/dev/null || true
  fi
fi

# --- Key Learnings extraction (existing behavior) ---

# Extract the subagent's stdout from hook input
STDOUT=$(echo "$INPUT" | perl -0777 -ne 'if (/"stdout"\s*:\s*"((?:[^"\\]|\\.)*)"/s) { $s=$1; $s=~s/\\n/\n/g; $s=~s/\\"/"/g; $s=~s/\\\\/\\/g; print $s }')

if [ -z "$STDOUT" ]; then
  exit 0
fi

# Check if there's a Key Learnings section
if ! echo "$STDOUT" | grep -q "## Key Learnings:"; then
  exit 0
fi

# Extract lines after "## Key Learnings:" that start with "- " (macOS-compatible)
LEARNINGS=$(echo "$STDOUT" | perl -ne 'if (/^## Key Learnings:/..$found_end) { $found_end=1 if /^(##[^#]|$)/ and !/^## Key Learnings:/; print if /^- / and !$found_end }')

if [ -z "$LEARNINGS" ]; then
  exit 0
fi

# Persist each learning via cvm kb put
COUNT=0
while IFS= read -r line; do
  # Strip the leading "- " prefix
  BODY=$(echo "$line" | sed 's/^- //')

  if [ -z "$BODY" ]; then
    continue
  fi

  # Generate a key from first few words
  KEY=$(echo "$BODY" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9 ]//g' | awk '{for(i=1;i<=4&&i<=NF;i++) printf "%s-",$i; print ""}' | sed 's/-$//' | head -c 50)

  # Check for duplicates silently
  SEARCH_OUT=$(cvm kb search "$KEY" 2>/dev/null)
  if [ -n "$SEARCH_OUT" ] && ! echo "$SEARCH_OUT" | grep -qiE '(no matches|no results|0 results)'; then
    continue
  fi

  # Link entry to current session. Spec: S-017 | Req: B-016
  if [ -n "$session_id" ]; then
    cvm kb put "$KEY" --body "$BODY" --type learning --tag "auto-captured" --session-id "$session_id" 2>/dev/null || true
  else
    cvm kb put "$KEY" --body "$BODY" --type learning --tag "auto-captured" 2>/dev/null || true
  fi
  COUNT=$((COUNT + 1))
done <<< "$LEARNINGS"

if [ "$COUNT" -gt 0 ]; then
  cat <<HOOK
{
  "hookSpecificOutput": {
    "hookEventName": "SubagentStop",
    "additionalContext": "[auto-captured] $COUNT learning(s) extraidos del subagent y guardados en KB."
  }
}
HOOK
fi

exit 0

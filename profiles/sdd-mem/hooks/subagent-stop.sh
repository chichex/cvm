#!/bin/bash
# Spec: S-011 | Req: B-003 | SubagentStop: agent capture + Key Learnings extraction
# Appends agent summary to session buffer, then extracts Key Learnings from stdout.
# MUST complete in < 500ms for capture portion (I-002).

set -euo pipefail

# Read hook input from stdin (shared by both B-003 capture and Key Learnings logic)
INPUT=$(cat)

# --- B-003: Capture agent summary to session buffer ---
session_id=$(echo "$INPUT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('session_id',''))" 2>/dev/null) || true

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
print(msg[:200])
" 2>/dev/null) || last_msg=""

  if [ -n "$last_msg" ]; then
    timestamp=$(date +%H:%M)
    new_line="[${timestamp}] AGENT(${agent_type}): ${last_msg}"
    buffer_key="session-buffer-${session_id}"

    existing=$(cvm kb show "$buffer_key" --local 2>/dev/null | sed '1,/^$/d' || true)

    if [ -n "$existing" ]; then
      line_count=$(echo "$existing" | wc -l | tr -d ' ')
      if [ "$line_count" -ge 100 ]; then
        existing=$(echo "$existing" | tail -n 99)
      fi
      new_body="${existing}
${new_line}"
    else
      new_body="$new_line"
    fi

    cvm kb put "$buffer_key" --body "$new_body" --tag "session-buffer" --local 2>/dev/null || true
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

  cvm kb put "$KEY" --body "$BODY" --tag "learning,auto-captured" 2>/dev/null || true
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

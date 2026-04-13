#!/usr/bin/env bash
# Spec: S-011 | Req: B-001 | PostToolUse: realtime tool capture to session buffer
# Appends a one-line observation to KB entry "session-buffer-<session_id>" for
# each significant tool use (Bash, Write, Edit, NotebookEdit).
# MUST complete in < 500ms (I-001). No LLM calls. No network.

set -euo pipefail

INPUT=$(cat)

# Extract session_id
session_id=$(echo "$INPUT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('session_id',''))" 2>/dev/null) || true
if [ -z "$session_id" ]; then
  echo "[tool-capture] warning: session_id missing, skipping" >&2
  exit 0
fi

# Extract tool_name
tool_name=$(echo "$INPUT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('tool_name',''))" 2>/dev/null) || true

# Filter: only capture Bash, Write, Edit, NotebookEdit
case "$tool_name" in
  Bash|Write|Edit|NotebookEdit) ;;
  *) exit 0 ;;
esac

# Build summary line depending on tool
summary=""
case "$tool_name" in
  Bash)
    cmd=$(echo "$INPUT" | python3 -c "
import json, sys
data = json.load(sys.stdin)
inp = data.get('tool_input', {})
cmd = inp.get('command', '') if isinstance(inp, dict) else ''
print(cmd[:120])
" 2>/dev/null) || cmd=""
    summary="Bash: ${cmd}"
    ;;
  Write)
    fp=$(echo "$INPUT" | python3 -c "
import json, sys
data = json.load(sys.stdin)
inp = data.get('tool_input', {})
print(inp.get('file_path', '') if isinstance(inp, dict) else '')
" 2>/dev/null) || fp=""
    summary="Write: wrote ${fp}"
    ;;
  Edit)
    fp=$(echo "$INPUT" | python3 -c "
import json, sys
data = json.load(sys.stdin)
inp = data.get('tool_input', {})
print(inp.get('file_path', '') if isinstance(inp, dict) else '')
" 2>/dev/null) || fp=""
    summary="Edit: edited ${fp}"
    ;;
  NotebookEdit)
    fp=$(echo "$INPUT" | python3 -c "
import json, sys
data = json.load(sys.stdin)
inp = data.get('tool_input', {})
print(inp.get('file_path', inp.get('notebook_path', '')) if isinstance(inp, dict) else '')
" 2>/dev/null) || fp=""
    summary="NotebookEdit: edited notebook ${fp}"
    ;;
esac

if [ -z "$summary" ]; then
  exit 0
fi

timestamp=$(date +%H:%M)
new_line="[${timestamp}] ${summary}"
buffer_key="session-buffer-${session_id}"

# Read existing body (strip frontmatter)
existing=$(cvm kb show "$buffer_key" --local 2>/dev/null | sed '1,/^$/d' || true)

# Cap at 100 lines: drop oldest if needed
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

cvm kb put "$buffer_key" --body "$new_body" --tag "session-buffer" --local 2>/dev/null || \
  echo "[tool-capture] warning: cvm kb put failed" >&2

exit 0

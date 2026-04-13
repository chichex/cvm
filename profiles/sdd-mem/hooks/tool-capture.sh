#!/usr/bin/env bash
# Spec: S-011 | Req: B-001 | PostToolUse: realtime tool capture to session buffer
# Appends a one-line observation to KB entry "session-buffer-<session_id>" for
# each significant tool use (Bash, Write, Edit, NotebookEdit).
# MUST complete in < 500ms (I-001). No LLM calls. No network.
# Note: settings.json matcher limits this to Bash|Write|Edit|NotebookEdit,
# so filtered tools never reach this script.

set -euo pipefail

INPUT=$(cat)

# Single python3 call to extract session_id, tool_name, and build summary
summary=$(echo "$INPUT" | python3 -c "
import json, sys
data = json.load(sys.stdin)
sid = data.get('session_id', '')
tool = data.get('tool_name', '')
inp = data.get('tool_input', {})
if not isinstance(inp, dict):
    inp = {}

if not sid or not tool:
    sys.exit(1)

if tool == 'Bash':
    detail = inp.get('command', '')[:120]
    line = f'Bash: {detail}'
elif tool == 'Write':
    line = f\"Write: wrote {inp.get('file_path', '')}\"
elif tool == 'Edit':
    line = f\"Edit: edited {inp.get('file_path', '')}\"
elif tool == 'NotebookEdit':
    line = f\"NotebookEdit: edited notebook {inp.get('file_path', inp.get('notebook_path', ''))}\"
else:
    sys.exit(1)

print(f'{sid}\n{line}')
" 2>/dev/null) || exit 0

session_id=$(echo "$summary" | head -1)
tool_summary=$(echo "$summary" | tail -1)

if [ -z "$session_id" ] || [ -z "$tool_summary" ]; then
  exit 0
fi

timestamp=$(date +%H:%M)
new_line="[${timestamp}] ${tool_summary}"
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

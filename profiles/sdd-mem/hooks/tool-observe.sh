#!/usr/bin/env bash
# Spec: S-015 | Req: B-001 | PostToolUse: structured tool observation capture to session buffer
# Appends a [HH:MM] [TOOL:<name>] <summary> line to KB entry "session-buffer-<session_id>"
# for each tool use in CVM_OBSERVE_TOOLS (default: Bash,Write,Edit).
# MUST complete in < 50ms (I-001). No LLM calls. No network.
# Replaces tool-capture.sh for Bash|Write|Edit subset (Option A — S-015 B-007).

INPUT=$(cat)

# Single python3 call: validate, filter by allowlist, build summary line
# Outputs: "<session_id>\n<summary>" or exits non-zero to trigger graceful skip
result=$(echo "$INPUT" | python3 -c "
import json, sys, os

try:
    data = json.loads(sys.stdin.read())
except Exception:
    sys.stderr.write('[tool-observe] warning: malformed JSON on stdin\n')
    sys.exit(1)

sid = data.get('session_id', '')
tool = data.get('tool_name', '')

if not sid:
    sys.stderr.write('[tool-observe] warning: session_id missing, skipping\n')
    sys.exit(1)

if not tool:
    sys.stderr.write('[tool-observe] warning: tool_name missing, skipping\n')
    sys.exit(1)

# Allowlist filter (B-005)
observe_env = os.environ.get('CVM_OBSERVE_TOOLS', '').strip()
if not observe_env:
    observe_env = 'Bash,Write,Edit'
allowed = [t.strip() for t in observe_env.split(',') if t.strip()]

if tool not in allowed:
    sys.exit(2)  # Not in allowlist — silent skip

inp = data.get('tool_input', {})
if not isinstance(inp, dict):
    inp = {}

# Build per-tool summary (B-002, B-003, B-004)
if tool == 'Bash':
    cmd = inp.get('command')
    if cmd is None:
        summary = '(no command)'
    elif cmd == '':
        summary = '(empty command)'
    else:
        cmd_str = str(cmd)
        if len(cmd_str) > 200:
            summary = cmd_str[:200] + '\u2026'
        else:
            summary = cmd_str

elif tool == 'Write':
    fp = inp.get('file_path', '')
    if not fp:
        summary = 'wrote (unknown path)'
    else:
        summary = f'wrote {fp}'

elif tool == 'Edit':
    fp = inp.get('file_path', '')
    if not fp:
        fp_str = '(unknown path)'
    else:
        fp_str = fp

    old = inp.get('old_string')
    if old is None:
        snippet = '(no old_string)'
    elif old == '':
        snippet = '(empty)'
    else:
        old_str = str(old).replace('\n', '\u21b5').replace('\r', '')
        if len(old_str) > 50:
            snippet = old_str[:50] + '\u2026'
        else:
            snippet = old_str

    summary = f'edited {fp_str} \u2014 \"{snippet}\"'

else:
    # Tool is in allowlist but has no specific handler — generic capture
    summary = f'used {tool}'

print(f'{sid}')
print(f'[TOOL:{tool}] {summary}')
" 2>&1)

exit_code=$?

# Exit code 2 = not in allowlist (silent skip); exit code 1 = warning written to stderr
if [ $exit_code -ne 0 ]; then
  exit 0
fi

session_id=$(echo "$result" | head -1)
tool_summary=$(echo "$result" | tail -1)

if [ -z "$session_id" ] || [ -z "$tool_summary" ]; then
  exit 0
fi

timestamp=$(date +%H:%M)
new_line="[${timestamp}] ${tool_summary}"
buffer_key="session-buffer-${session_id}"

# Read existing buffer body, strip frontmatter (B-006 read-modify-write)
existing=$(cvm kb show "$buffer_key" --local 2>/dev/null | sed '1,/^$/d' || true)

# Cap at 100 lines: drop oldest if at/above cap (B-006, I-004, B-010)
if [ -n "$existing" ]; then
  line_count=$(echo "$existing" | wc -l | tr -d ' ')
  if [ "$line_count" -ge 500 ]; then
    existing=$(echo "$existing" | tail -n 499)
  fi
  new_body="${existing}
${new_line}"
else
  new_body="$new_line"
fi

# Write back (B-006)
cvm kb put "$buffer_key" --body "$new_body" --tag "session-buffer" --local 2>/dev/null || \
  echo "[tool-observe] warning: cvm kb put failed" >&2

exit 0

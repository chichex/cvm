#!/usr/bin/env bash
# Spec: S-017 | Req: B-003, B-014 | PostToolUse: structured tool observation capture via cvm session append
# Appends a tool event to the CVM session for each tool use in CVM_OBSERVE_TOOLS (default: Bash,Write,Edit).
# MUST complete in < 50ms (I-001). No LLM calls. No network.
# Replaces session-buffer KB writes (S-015 superseded by S-017).

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
print(f'{tool}')
print(f'{summary}')
" 2>&1)

exit_code=$?

# Exit code 2 = not in allowlist (silent skip); exit code 1 = warning written to stderr
if [ $exit_code -ne 0 ]; then
  exit 0
fi

session_id=$(echo "$result" | sed -n '1p')
tool_name=$(echo "$result" | sed -n '2p')
tool_summary=$(echo "$result" | sed -n '3p')

if [ -z "$session_id" ] || [ -z "$tool_name" ] || [ -z "$tool_summary" ]; then
  exit 0
fi

# Append tool event to session (B-003, B-014: cvm session append replaces kb put)
cvm session append "$session_id" --type tool --tool "$tool_name" --content "$tool_summary" 2>/dev/null || \
  echo "[tool-observe] warning: cvm session append failed" >&2

exit 0

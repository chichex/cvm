#!/usr/bin/env bash
# Spec: S-017 | Req: B-005 | SessionEnd: close CVM session with summary
set -euo pipefail

INPUT=$(cat)

session_id=$(echo "$INPUT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('session_id',''))" 2>/dev/null) || true
if [ -z "$session_id" ]; then
  echo "[session-end] warning: session_id missing" >&2
  exit 0
fi

# Use full path in case PATH is incomplete in hook context
CVM="${HOME}/go/bin/cvm"
if command -v cvm &>/dev/null; then
  CVM="cvm"
fi

# Run in background — hooks have a short timeout and cvm session end
# runs retro (claude -p) which can take 30s+. The End() function marks
# the session as ended BEFORE running retro, so even if the background
# process is killed, the session is properly closed.
nohup "$CVM" session end "$session_id" >/dev/null 2>&1 &

exit 0

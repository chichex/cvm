#!/usr/bin/env bash
# Spec: S-017 | Req: B-001, B-014 | SessionStart: create CVM session
set -euo pipefail

INPUT=$(cat)

session_id=$(echo "$INPUT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('session_id',''))" 2>/dev/null) || true
if [ -z "$session_id" ]; then
  echo "[session-start] warning: session_id missing from stdin" >&2
  exit 0
fi

# Extract project from cwd (hooks run in the project directory)
project="$(pwd)"

# Use full path in case PATH is incomplete in hook context
CVM="${HOME}/go/bin/cvm"
if command -v cvm &>/dev/null; then
  CVM="cvm"
fi

profile=$("$CVM" profile 2>/dev/null | grep -m1 "Local:" | awk '{print $2}' || echo "unknown")

"$CVM" session start --session-id "$session_id" --project "$project" --profile "$profile" 2>/dev/null || \
  echo "[session-start] warning: cvm session start failed" >&2

exit 0

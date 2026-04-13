#!/usr/bin/env bash
# Spec: S-017 | Req: B-005 | SessionEnd: close CVM session with summary
set -euo pipefail

INPUT=$(cat)

session_id=$(echo "$INPUT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('session_id',''))" 2>/dev/null) || true
if [ -z "$session_id" ]; then
  echo "[session-end] warning: session_id missing" >&2
  exit 0
fi

cvm session end "$session_id" 2>/dev/null || \
  echo "[session-end] warning: cvm session end failed" >&2

exit 0

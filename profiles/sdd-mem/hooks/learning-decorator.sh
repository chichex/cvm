#!/usr/bin/env bash
# Spec: S-017 | Req: B-002, B-014 | UserPromptSubmit: on-the-fly learning protocol + prompt capture
# Captures the user prompt via cvm session append, then injects the learning protocol.
# MUST complete in < 500ms for capture portion (I-001).

set -euo pipefail

# --- B-002, B-014: Capture user prompt to session via cvm session append ---
INPUT=$(cat)

session_id=$(echo "$INPUT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('session_id',''))" 2>/dev/null) || true

if [ -z "$session_id" ]; then
  echo "[learning-decorator] warning: session_id missing, skipping capture" >&2
fi

if [ -n "$session_id" ]; then
  # Extract user prompt (may be a string or array of content blocks)
  user_prompt=$(echo "$INPUT" | python3 -c "
import json, sys
data = json.load(sys.stdin)
prompt = data.get('content', data.get('prompt', data.get('user_prompt', data.get('message', ''))))
if isinstance(prompt, list):
    texts = []
    for block in prompt:
        if isinstance(block, dict) and block.get('type') == 'text':
            texts.append(block.get('text', ''))
    prompt = ' '.join(texts)
prompt = str(prompt).strip()
print(prompt[:300])
" 2>/dev/null) || user_prompt=""

  # Filter out system-reminder prompts; pass the rest to cvm session append
  if [ -n "$user_prompt" ] && [[ "$user_prompt" != \<system* ]]; then
    cvm session append "$session_id" --type prompt --content "$user_prompt" 2>/dev/null || true
  fi
fi

# --- Learning protocol injection (writes to stdout for Claude Code to consume) ---
PULSE_FILE="$HOME/.cvm/learning-pulse"

# First prompt of session: full protocol
if [ ! -f "$PULSE_FILE" ]; then
  date +%s > "$PULSE_FILE"
  cat <<'HOOK'
{
  "hookSpecificOutput": {
    "hookEventName": "UserPromptSubmit",
    "additionalContext": "[learning-protocol] On-the-fly learning activo.\n\nSELF-CHECK obligatorio — preguntate despues de cada interaccion significativa:\n- ¿Tome decisiones de diseno, resolvi bugs, o descubri algo no-obvio?\n\nSi la respuesta es SI → ejecutar /retro (mid-session) AHORA, no despues.\n\n/retro captura automaticamente learnings, decisions, y gotchas y los persiste en KB con session_id.\n\nReglas: calidad > cantidad. Solo si genuinamente util para futuras sesiones."
  }
}
HOOK
  exit 0
fi

# Subsequent prompts: self-check every 15+ min
LAST=$(cat "$PULSE_FILE" 2>/dev/null || echo 0)
NOW=$(date +%s)
ELAPSED=$(( NOW - LAST ))

if [ "$ELAPSED" -gt 900 ]; then
  date +%s > "$PULSE_FILE"
  cat <<'HOOK'
{
  "hookSpecificOutput": {
    "hookEventName": "UserPromptSubmit",
    "additionalContext": "[learning-pulse] +15min desde ultimo checkpoint. SELF-CHECK: ¿tomaste decisiones, resolviste bugs, o descubriste algo no-obvio en los ultimos 15min? Si SI → ejecutar /retro AHORA. No esperar al final."
  }
}
HOOK
  exit 0
fi

# <15min since last pulse: no injection, save tokens
exit 0

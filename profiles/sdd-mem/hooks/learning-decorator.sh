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
    "additionalContext": "[learning-protocol] On-the-fly learning activo.\n\nSELF-CHECK obligatorio — preguntate despues de cada interaccion significativa:\n- Acabo de tomar una decision de diseno? → cvm kb put con tag decision\n- Acabo de resolver un bug o encontrar la causa? → cvm kb put con tag learning\n- Algo no funciono como esperaba? → cvm kb put con tag gotcha\n- El usuario confirmo o rechazo un approach? → cvm kb put con tag decision\n\nSi la respuesta a cualquiera es SI → guardar AHORA, no despues.\n\nAccion:\n1. Verificar duplicados: cvm kb search \"<terminos>\"\n2. Guardar: cvm kb put \"<key>\" --body \"<descripcion con el POR QUE>\" --tag \"<tipo>,<area>\" [--local]\n3. Reportar: [learned] key — descripcion\n\nTipos: learning, gotcha, decision\nReglas: calidad > cantidad. Solo si genuinamente util para futuras sesiones.\n\nSESSION SUMMARY: Antes de cerrar la sesion (cuando el usuario dice listo/done/chau/exit), DEBES persistir un resumen con:\ncvm kb put \"session-summary-$(date +%Y%m%d)\" --body \"Goal: ... | Accomplished: ... | Discoveries: ... | Next: ...\" --tag \"session,summary\"\nEsto NO es opcional. Si lo salteas, la proxima sesion arranca ciega."
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
    "additionalContext": "[learning-pulse] +15min desde ultimo checkpoint. SELF-CHECK: ¿tomaste decisiones, resolviste bugs, o descubriste algo no-obvio en los ultimos 15min? Si SI → cvm kb put AHORA. No esperar al final."
  }
}
HOOK
  exit 0
fi

# <15min since last pulse: no injection, save tokens
exit 0

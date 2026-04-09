#!/bin/bash
# UserPromptSubmit hook: on-the-fly learning protocol
# Inyecta conciencia de learning en cada prompt.
# Primer prompt: protocolo completo. Despues: nudge cada 15+ min.
# Reemplaza el headless retro — Claude guarda directo con cvm kb put.

PULSE_FILE="$HOME/.cvm/learning-pulse"

# First prompt of session: full protocol
if [ ! -f "$PULSE_FILE" ]; then
  date +%s > "$PULSE_FILE"
  cat <<'HOOK'
{
  "hookSpecificOutput": {
    "hookEventName": "UserPromptSubmit",
    "additionalContext": "[learning-protocol] On-the-fly learning activo. Si durante esta interaccion identificas algo no-obvio worth persisting:\n\nTipos:\n- learning: comportamiento inesperado, patron que funciono/fallo, workaround\n- gotcha: trampa silenciosa, config que rompe, error costoso de diagnosticar\n- decision: eleccion de diseno con trade-offs aceptados\n\nAccion inmediata:\n1. Verificar duplicados: cvm kb search \"<terminos>\"\n2. Guardar: cvm kb put \"<key>\" --body \"<descripcion con el POR QUE>\" --tag \"<tipo>,<area>\" [--local]\n3. Reportar al usuario: [learned] key — descripcion\n\nReglas: calidad > cantidad. No forzar. Solo si genuinamente util para futuras sesiones."
  }
}
HOOK
  exit 0
fi

# Subsequent prompts: nudge only if >15min since last pulse
LAST=$(cat "$PULSE_FILE" 2>/dev/null || echo 0)
NOW=$(date +%s)
ELAPSED=$(( NOW - LAST ))

if [ "$ELAPSED" -gt 900 ]; then
  date +%s > "$PULSE_FILE"
  cat <<'HOOK'
{
  "hookSpecificOutput": {
    "hookEventName": "UserPromptSubmit",
    "additionalContext": "[learning-pulse] +15min desde ultimo checkpoint. Si hubo descubrimientos no-obvios, persistirlos ahora con cvm kb put."
  }
}
HOOK
  exit 0
fi

# <15min since last pulse: no injection, save tokens
exit 0

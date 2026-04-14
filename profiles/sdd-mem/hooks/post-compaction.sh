#!/bin/bash
# SessionStart hook (matcher: compact): re-inyecta protocolo de learning
# despues de que Claude Code compacta el contexto.
# Sin esto, el protocolo inyectado en el primer prompt se pierde.

cat <<'HOOK'
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "[post-compaction] El contexto fue compactado. Re-inyectando protocolo de learning.\n\nSELF-CHECK obligatorio — preguntate despues de cada interaccion significativa:\n- Acabo de tomar una decision de diseno?\n- Acabo de resolver un bug o encontrar la causa?\n- Algo no funciono como esperaba?\n- El usuario confirmo o rechazo un approach?\n\nSi la respuesta a cualquiera es SI → ejecutar /retro con scope mid-session AHORA.\n/retro es el UNICO mecanismo de captura de conocimiento. No usar cvm kb put directamente.\n\nANTES DE CONTINUAR: Si tenes trabajo significativo de antes de la compaction que no fue capturado, ejecutar /retro ahora."
  }
}
HOOK

exit 0

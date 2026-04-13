#!/bin/bash
# SessionStart hook (matcher: compact): re-inyecta protocolo de learning
# despues de que Claude Code compacta el contexto.
# Sin esto, el protocolo inyectado en el primer prompt se pierde.

cat <<'HOOK'
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "[post-compaction] El contexto fue compactado. Re-inyectando protocolo de learning.\n\nSELF-CHECK obligatorio — preguntate despues de cada interaccion significativa:\n- Acabo de tomar una decision de diseno? → cvm kb put con tag decision\n- Acabo de resolver un bug o encontrar la causa? → cvm kb put con tag learning\n- Algo no funciono como esperaba? → cvm kb put con tag gotcha\n- El usuario confirmo o rechazo un approach? → cvm kb put con tag decision\n\nSi la respuesta a cualquiera es SI → guardar AHORA.\n\nAccion:\n1. Verificar duplicados: cvm kb search \"<terminos>\"\n2. Guardar: cvm kb put \"<key>\" --body \"<POR QUE>\" --tag \"<tipo>,<area>\" [--local]\n3. Reportar: [learned] key — descripcion\n\nANTES DE CONTINUAR: Si tenes trabajo significativo de antes de la compaction que no fue guardado, persistirlo ahora con cvm kb put."
  }
}
HOOK

exit 0

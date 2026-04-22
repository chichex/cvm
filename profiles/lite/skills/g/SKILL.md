[DEPRECATED] Shim de compatibilidad para Gemini. `/g` quedo deprecado — usa `/o --gemini` en su lugar. $ARGUMENTS es la tarea.

## Proceso

1. Avisar al usuario: "`/g` esta deprecated. Redirigiendo a `/o --gemini`. Usa `/o --gemini <tarea>` directamente la proxima vez."
2. Sanear $ARGUMENTS: remover cualquier flag de provider previo (`--codex`, `--gemini`, `--opus`) para evitar conflicto con la validacion de `/o`. `/g` fuerza `--gemini` — si el usuario paso otro flag de provider, el de `/g` gana.
3. Delegar al skill `/o` con el flag `--gemini` anteponiendolo al $ARGUMENTS saneado (ejecutar internamente la rama Gemini de `/o`, ver `profiles/lite/skills/o/SKILL.md`).

No duplicar logica: toda la ejecucion (availability check con fallback, prompts en `/tmp/`, Bash calls) vive en `/o`.

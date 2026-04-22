[DEPRECATED] Shim de compatibilidad para Codex. `/c` quedo deprecado — usa `/o --codex` en su lugar. $ARGUMENTS es la tarea.

## Proceso

1. Avisar al usuario: "`/c` esta deprecated. Redirigiendo a `/o --codex`. Usa `/o --codex <tarea>` directamente la proxima vez."
2. Sanear $ARGUMENTS: remover cualquier flag de provider previo (`--codex`, `--gemini`, `--opus`) para evitar conflicto con la validacion de `/o`. `/c` fuerza `--codex` — si el usuario paso otro flag de provider, el de `/c` gana.
3. Delegar al skill `/o` con el flag `--codex` anteponiendolo al $ARGUMENTS saneado (ejecutar internamente la rama Codex de `/o`, ver `profiles/lite/skills/o/SKILL.md`).

No duplicar logica: toda la ejecucion (availability check, prompts en `/tmp/`, Bash calls) vive en `/o`.

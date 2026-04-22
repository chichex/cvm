[DEPRECATED] Shim de compatibilidad para Codex. `/c` quedo deprecado — usa `/o --codex` en su lugar. $ARGUMENTS es la tarea.

## Proceso

1. Avisar al usuario: "`/c` esta deprecated. Redirigiendo a `/o --codex`. Usa `/o --codex <tarea>` directamente la proxima vez."
2. Delegar al skill `/o` con el flag `--codex` anteponiendolo a $ARGUMENTS (ejecutar internamente la rama Codex de `/o`, ver `profiles/lite/skills/o/SKILL.md`).

No duplicar logica: toda la ejecucion (availability check, prompts en `/tmp/`, Bash calls) vive en `/o`.

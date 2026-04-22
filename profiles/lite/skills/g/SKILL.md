[DEPRECATED] Shim de compatibilidad para Gemini. `/g` quedo deprecado — usa `/o --gemini` en su lugar. $ARGUMENTS es la tarea.

## Proceso

1. Avisar al usuario: "`/g` esta deprecated. Redirigiendo a `/o --gemini`. Usa `/o --gemini <tarea>` directamente la proxima vez."
2. Delegar al skill `/o` con el flag `--gemini` anteponiendolo a $ARGUMENTS (ejecutar internamente la rama Gemini de `/o`, ver `profiles/lite/skills/o/SKILL.md`).

No duplicar logica: toda la ejecucion (availability check con fallback, prompts en `/tmp/`, Bash calls) vive en `/o`.

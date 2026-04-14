# Lite Profile

## Skills

| Skill | Que hace |
|-------|----------|
| `/s` | Selector inteligente — entry point recomendado cuando no sabes que agente usar |
| `/o` | Subagent Opus — razonamiento profundo, review, arquitectura |
| `/c` | Subagent Codex — validacion externa |
| `/g` | Subagent Gemini — validacion externa |
| `/r` | Review de sesion + persistencia de learnings en project memory. Soporta `/r --dry-run` |
| `/ux` | Iteracion UX con validacion multi + HTML de alternativas |
| `/issue` | Crear GitHub issue con label `ct:plan` |
| `/pr` | Crear PR, pregunta si ejecutar `/r` antes, espera GH Actions |

Usa `/o`, `/c`, `/g` directamente cuando ya sabes que agente necesitas. Usa `/s` cuando quieras recomendacion o combinar agentes.

## Subagentes

Los skills parsean el input y arman un prompt estructurado. No son pass-through.
Codex y Gemini tienen acceso al filesystem. Darles paths, no contenido inline.
Seguridad shell: NUNCA interpolar texto del usuario en double-quoted commands. Usar Write tool para archivos temporales.

## Persistencia

- Todo va a la auto-memory del proyecto: `~/.claude/projects/<path>/memory/`
- MEMORY.md se carga automaticamente al inicio de cada sesion (built-in de Claude Code)
- `/r` mantiene MEMORY.md y los archivos de memory del proyecto
- CLAUDE.md (este archivo) NUNCA se modifica. Los CLAUDE.md de proyectos tampoco.

## Reglas

- Sacar ambiguedades — si algo puede interpretarse de mas de una forma, clarificar antes de actuar
- TDD siempre que sea posible
- No agregar lo que no se pidio
- No especular sobre codigo sin leerlo
- Respuestas cortas y directas
- macOS — evitar flags GNU-only (`grep -P`). Usar `grep -E`

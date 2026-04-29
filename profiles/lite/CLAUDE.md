# Lite Profile

## Skills

| Skill | Que hace |
|-------|----------|
| `/go` | Subagent unificado — default Opus; `--codex` / `--gemini` para validacion externa |
| `/r` | Review de sesion + persistencia de learnings en project memory. Soporta `/r --dry-run` |
| `/ux` | Iteracion UX con validacion multi + HTML de alternativas |
| `/che-idea` | Crear GitHub issue desde idea vaga (subagent Opus); aplica `che:idea` + `ct:plan` |
| `/che-explore` | Analizar issue + prepender plan consolidado al body + comment con paths/riesgos; aplica `che:planning→che:plan` |
| `/che-execute` | Implementar issue/tarea en worktree aislado + abrir PR draft; aplica `che:executing→che:executed` |
| `/che-validate` | Revisar PR/issue (subagentes paralelos); aplica `che:validating→che:validated` + verdict |
| `/che-iterate` | Aplicar comments de PR/issue (subagent Opus); aplica transitions `che:executing\|planning` |
| `/che-loop` | Automatiza `che-validate→che-iterate→...` hasta verdict=approve, 0 comments accionables, `--max N` (default 3), o idempotencia. No toca labels `che:*` directamente — compone los skills hermanos |
| `/che-close` | Cerrar PR (ready→CI→merge→close issues linkeados); aplica `che:closing→che:closed` |

Los skills "che" que tocan la state machine (`/che-idea`, `/che-explore`, `/che-execute`, `/che-validate`, `/che-iterate`, `/che-close`) replican el workflow de [che-cli](https://github.com/chichex/che-cli) en modo lenient — aplican las mismas transitions de la state machine `che:*` (ver `che-cli/internal/labels/labels.go`) pero no abortan si current state no calza con `from` (warnean y aplican igual). `/che-loop` es un orquestador puro que compone `/che-validate` y `/che-iterate` via Skill tool y NO aplica labels `che:*` por su cuenta.

Usa `/go` directamente cuando sabes que agente necesitas (default Opus; agrega `--codex` o `--gemini` para CLIs externos).

## Subagentes

Los skills parsean el input y arman un prompt estructurado. No son pass-through.
Codex y Gemini tienen acceso al filesystem. Darles paths, no contenido inline.
Seguridad shell: NUNCA interpolar texto del usuario en double-quoted commands. Usar Write tool para archivos temporales.

## Persistencia

- Todo va a la auto-memory del proyecto: `~/.claude/projects/<path>/memory/`
- MEMORY.md se carga automaticamente al inicio de cada sesion (built-in de Claude Code)
- `/r` mantiene MEMORY.md y los archivos de memory del proyecto
- La copia desplegada de CLAUDE.md (`~/.claude/CLAUDE.md` y los CLAUDE.md de proyectos) NUNCA se modifica en runtime: ni `/r` ni los skills la editan.
- Este archivo (`profiles/lite/CLAUDE.md`) es la **fuente** del profile lite y SI es editable: cambios deliberados al profile (agregar skills, ajustar reglas) van por PR sobre este archivo y se redespliegan via `cvm`. La regla de arriba habla del runtime, no del workflow de mantenimiento del profile.

## Reglas

- Sacar ambiguedades — si algo puede interpretarse de mas de una forma, clarificar antes de actuar
- Preguntas de desambiguacion SIEMPRE en formato multiple choice (opciones numeradas + opcion libre "otro"). Nunca preguntas abiertas cuando se puede enumerar.
- TDD siempre que sea posible
- No agregar lo que no se pidio
- No especular sobre codigo sin leerlo
- Respuestas cortas y directas
- macOS — evitar flags GNU-only (`grep -P`). Usar `grep -E`

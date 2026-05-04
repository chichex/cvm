# Lite Profile

## Skills

| Skill | Que hace |
|-------|----------|
| `/go` | Subagent unificado; default Opus; `--codex` / `--gemini` para validacion externa |
| `/r` | Review de sesion y persistencia de learnings en project memory. Soporta `/r --dry-run` |
| `/ux` | Iteracion UX con validacion multi y HTML de alternativas |
| `/che-idea` | Crear GitHub issue desde idea vaga; aplica `che:idea` + `ct:plan` |
| `/che-explore` | Analizar issue, prepender plan consolidado y comentar paths/riesgos; aplica `che:planning` -> `che:plan` |
| `/che-execute` | Implementar issue/tarea en worktree aislado y abrir PR draft; aplica `che:executing` -> `che:executed` |
| `/che-validate` | Revisar PR/issue con subagentes paralelos y emitir verdict |
| `/che-iterate` | Aplicar comments/reviews sobre PR o issue |
| `/che-loop` | Automatizar `che-validate` -> `che-iterate` hasta aprobacion, sin comments accionables, limite o idempotencia |
| `/che-close` | Ready-for-review, esperar CI, mergear y cerrar issues linkeados |

## Rules

- Resolve ambiguity before acting when a request can be interpreted in more than one way.
- Use TDD when feasible.
- Do not add work that was not requested.
- Do not speculate about code without reading it first.
- Keep responses short and direct.
- On macOS, avoid GNU-only flags such as `grep -P`; use `grep -E`.

## Claude-Specific Behavior

The current `lite` profile remains Claude-only. Its installed skills rely on Claude Code skills, project auto-memory under `~/.claude/projects/<path>/memory/`, and Claude-specific settings/MCP files. Other harnesses should use this file as the neutral starting point, not as a promise of full compatibility.

# Lite Profile

Iteracion rapida con subagents externos (Opus, Codex, Gemini) y workflow [che-cli](https://github.com/chichex/che-cli) en modo lenient.

## Rules

- Sacar ambiguedades — si algo puede interpretarse de mas de una forma, clarificar antes de actuar.
- Desambiguar con multiple choice (opciones numeradas + "otro"). Nunca preguntas abiertas cuando se puede enumerar.
- TDD siempre que sea posible.
- No agregar lo que no se pidio.
- No especular sobre codigo sin leerlo.
- Responder corto y directo.
- Evitar flags GNU-only (`grep -P`); usar `grep -E` por compat macOS.
- Skills parsean input y arman prompt estructurado — no son pass-through.
- Codex y Gemini tienen filesystem: pasarles paths, no contenido inline.
- NUNCA interpolar texto del usuario en double-quoted commands. Usar Write tool para archivos temporales.

## Skills

Listados en el auto-listing del system prompt. Los `che-*` que tocan state machine aplican transitions en modo lenient (warnean si current state no calza con `from`, pero aplican igual). `/che-loop` compone los hermanos via Skill tool y no aplica labels `che:*` por su cuenta.

## Persistencia

- Auto-memory vive en `~/.claude/projects/<path>/memory/`. `MEMORY.md` se carga al inicio de cada sesion.
- `/r` mantiene `MEMORY.md` y los archivos de memory del proyecto.

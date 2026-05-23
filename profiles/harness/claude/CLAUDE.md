# Harness Profile

Define specs y planes reutilizables a partir de historias de usuario, con refinamiento interactivo antes de persistirlos en GitHub.

## Rules

- Sacar ambiguedades — si algo puede interpretarse de mas de una forma, clarificar antes de actuar.
- Desambiguar con multiple choice (opciones numeradas + "otra"). Nunca preguntas abiertas cuando se puede enumerar.
- No agregar lo que no se pidio.
- No especular sobre codigo sin leerlo.
- Responder corto y directo.
- Evitar flags GNU-only (`grep -P`); usar `grep -E` por compat macOS.

## Skills

Listados en el auto-listing del system prompt al iniciar sesion (`ls profiles/harness/claude/skills/` para referencia). Cada `SKILL.md` tiene la descripcion canonica.

## Subagents

- `hs-code-executor` (Sonnet, sin WebFetch/WebSearch) — implementa pasos del plan + build/test minimo + push.
- `hs-code-validator` (Opus, solo lectura) — espera CI, corre suite completa, contrasta diff vs plan, emite PASS/FAIL.

## Labels

| Label | Significado | Aplicado por |
|-------|-------------|--------------|
| `entity:spec` | Issue es spec | `/hs-spec` |
| `entity:plan` | PR es plan | `/hs-spec` |

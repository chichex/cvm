# Exec notes — issue #25 `/iterate` skill

## Completado
- `profiles/lite/skills/iterate/SKILL.md` creado siguiendo el formato de `/o` y `/pr` (descripcion de una linea, `## Proceso` con pasos numerados, `## MUST DO`, `## MUST NOT DO`).
- Fila `/iterate` agregada a la tabla de skills en `profiles/lite/CLAUDE.md`.
- Todos los criterios de aceptacion documentados en el skill:
  - Parsing de `$ARGUMENTS` (numero, URL, vacio) con detection PR vs issue via `gh pr view` → fallback `gh issue view`.
  - Fetch via `gh api --paginate` de los tres endpoints para PR (`issues/N/comments`, `pulls/N/comments`, `pulls/N/reviews`) y solo `issues/N/comments` para issue.
  - Filtrado: regex case-insensitive `^(lgtm|\+1|👍|thanks?|ty)\.?$`, comments del autor, reviews `APPROVED` con body vacio.
  - Contexto escrito a `/tmp/cvm-iterate-context.md` via Write tool; prompt del agente referencia el path.
  - Lanzamiento directo de `Agent(subagent_type='general-purpose', model='opus')` con instruccion de (a) evaluar, (b) aplicar, (c) reportar aplicados/ignorados con razon, (d) `## Key Learnings:`.
  - Invocacion final de `/r` via Skill tool.
  - NO commits automaticos — cambios quedan en working tree.
  - Sin interpolacion de bodies/`$ARGUMENTS` en double-quoted shell commands.

## Pendiente (bloqueado por permisos)
- **Paso 9 del plan: replicar `SKILL.md` a `~/.claude/skills/iterate/SKILL.md`.** El harness bloquea writes a `/Users/ayrtonmarini/.claude/` con error "sensitive file". El usuario debe correr manualmente tras el merge:

  ```bash
  mkdir -p ~/.claude/skills/iterate
  cp profiles/lite/skills/iterate/SKILL.md ~/.claude/skills/iterate/SKILL.md
  ```

  Sin esto, `/iterate` no queda visible como slash-command global — solo el archivo del repo queda versionado.

- **Paso 10 del plan: prueba end-to-end contra un PR real con comments mixtos.** No ejecutada porque requiere (a) el skill ya instalado globalmente (bloqueado arriba) y (b) un PR con comments reales para validar el filtrado y el dispatch. Se recomienda probar manualmente tras la instalacion dual-location.

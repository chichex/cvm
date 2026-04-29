# EXEC_NOTES — issue #29 (`/che-loop`)

## Lo que se hizo

- Nuevo skill `profiles/lite/skills/che-loop/SKILL.md` (264 lineas).
- Nueva fila `/che-loop` en `profiles/lite/CLAUDE.md`, ubicada entre `/che-iterate` y `/che-close` (orden logico del flujo: validate → iterate → loop sobre ambos → close).
- Texto del bullet "los 6 skills che" actualizado: ahora distingue los 6 que tocan state machine vs `/che-loop` que es orquestador puro.
- `profiles/lite/install.sh` NO se modifico — ya itera `*/SKILL.md` automaticamente y va a sincronizar `che-loop` al redeploy.

## Decisiones de diseno destacadas

1. **Composicion via Skill tool, no reimplementacion.** El skill invoca `/che-validate` y `/che-iterate` directamente. Cualquier cambio futuro en el filtrado de comments, fetch o transitions vive en los hermanos, no aca.
2. **Lectura del verdict desde labels (`validated:*` / `plan-validated:*`), no del stdout del Skill tool.** Los labels son el contrato persistido en GitHub que `/che-validate` aplica al final; leerlos es deterministico y resiliente al formato del reporte.
3. **No tocar `che:*` directamente.** El loop no aplica locks ni transitions propias — `che:looping` no existe en la state machine de che-cli (`labels.go`). Documentado en MUST NOT DO.
4. **Idempotencia por IDs crudos de comments.** Comparar IDs sin filtrar puede dar false positives (ej: alguien dejo un `+1` nuevo) pero evita acoplar el loop al filtrado interno de `/che-iterate`. Conservador en favor de simplicidad.
5. **`--max=3` default.** Cubre "primera review pide cambios → fix → segunda review aprueba" con buffer extra. Mas que eso suele indicar idempotencia o needs-human. Override explicito disponible (rango `1 ≤ N ≤ 20`).
6. **Salida inmediata en `needs-human`.** No iteramos; `/che-iterate` no resuelve trade-offs que requieren decision humana.
7. **Precheck de PR (branch == headRefName + working tree limpio) ANTES del primer `/che-validate`.** `/che-iterate` ya aborta si esto no se cumple, pero hacerlo arriba evita gastar una validate. Para issues no aplica.

## Dual location (`~/.claude/skills/`)

Segun la memoria `lite_profile_dual_location.md`, los skills viven tanto en `~/.claude/skills/` (runtime) como en `profiles/lite/skills/` (fuente del repo). En este worktree solo se modifico la fuente del repo. El sync a `~/.claude/skills/che-loop/SKILL.md` lo hace `cvm install` / `bash profiles/lite/install.sh` post-merge.

## Pendiente

Nada bloqueante. La verificacion end-to-end del loop (validate → iterate → validate sobre un PR real con cambios pedidos) requiere correrlo despues del merge, idealmente sobre un PR de prueba con comments accionables.

## Validacion ejecutada en este run

- No hay markdown linter ni skill validator en el repo (`Makefile` solo corre `go test`).
- Frontmatter consistente con los hermanos: primera linea es la descripcion en una sola oracion (ningun skill del profile lite usa YAML frontmatter).
- `install.sh` no requiere cambios — usa glob `*/SKILL.md` y picks up `che-loop` automaticamente.

# Plan: Renombrar profile portable a harness con skills /hs-*

Refs #70 · https://github.com/chichex/cvm/issues/70

## Contexto

Rename del profile `portable` (y sus 7 skills + 2 subagents) al nuevo nombre `harness`, con prefijo de slash-commands `/hs-*`. Forward-only, sin aliases ni mensajes de deprecation.

## Objetivo

Ejecutar el rename completo del profile en `profiles/portable/` → `profiles/harness/` y de todos sus assets (skills, subagents, paths internos, refs cruzadas) a la nueva convencion `hs-*`, sin tocar el comportamiento funcional ni el mecanismo interno de "portable assets" del profile manager (que es ortogonal y reusa la palabra por casualidad).

## Approach

- Renombrar el directorio del profile entero con `git mv` (preserva historia).
- Recorrer skills y subagents (en `claude/` y `opencode/`) y renombrarlos uno por uno con `git mv` directorio→directorio.
- Editar contenido: header de `CLAUDE.md`/`AGENTS.md`, manifest TOML, frontmatter de subagents, refs cruzadas dentro de cada SKILL.md (`/portable-*` → `/hs-*`, `.portable/plans/` → `.harness/plans/`, branch `portable-plan/<N>` → `hs-plan/<N>`, mktemp `cvm-portable-*` → `cvm-hs-*`, marker comments `<!-- portable-* -->` → `<!-- hs-* -->`).
- Inspeccionar `claude/settings.json`, `claude/statusline-command.sh` y `README.md` raiz para detectar y actualizar menciones del profile (si existen).
- NO tocar `internal/profile/{authoring,manifest,profile}.go` ni los tests Go: la palabra "portable" ahi se refiere al mecanismo de assets multi-harness, no al nombre del profile.
- Validar con `go build ./...` y tests Go acotados (`internal/profile/...`, `internal/remote/...`).

## Pasos

- [ ] `git mv profiles/portable profiles/harness`
- [ ] Actualizar `profiles/harness/cvm.profile.toml`: `name = "harness"`
- [ ] Renombrar 7 skills en `claude/skills/` y 7 en `opencode/skills/`: `portable-*` → `hs-*` (con `git mv`)
- [ ] Renombrar 2 subagents en `claude/agents/` y 2 en `opencode/agents/`: `portable-code-{executor,validator}.md` → `hs-code-{executor,validator}.md`
- [ ] Editar contenido de cada SKILL.md (14 archivos): refs cruzadas `/portable-*` → `/hs-*`, paths `.portable/plans/` → `.harness/plans/`, branch `portable-plan/<N>` → `hs-plan/<N>`, mktemp `cvm-portable-*` → `cvm-hs-*`, markers `<!-- portable-* -->` → `<!-- hs-* -->`
- [ ] Editar frontmatter + contenido de cada agent.md (4 archivos): `name:` en YAML, refs cruzadas
- [ ] Editar `claude/CLAUDE.md` y `opencode/AGENTS.md`: header `# Harness Profile`, incorporar frase "harness engineering", tabla de skills con nuevos nombres
- [ ] Inspeccionar y actualizar si corresponde: `claude/settings.json`, `claude/statusline-command.sh`, `README.md` raiz
- [ ] Validar: `go build ./...` + `go test ./internal/profile/... ./internal/remote/...`

## Archivos afectados

- `profiles/portable/` — rename del directorio entero a `profiles/harness/` con `git mv`
- `profiles/harness/cvm.profile.toml` — modificar campo `name`
- `profiles/harness/claude/CLAUDE.md` — modificar header + tabla de skills + intro con "harness engineering"
- `profiles/harness/opencode/AGENTS.md` — modificar header + tabla de skills + intro con "harness engineering"
- `profiles/harness/claude/skills/portable-spec/SKILL.md` — rename a `hs-spec/SKILL.md` + edit refs cruzadas
- `profiles/harness/claude/skills/portable-plan/SKILL.md` — rename a `hs-plan/SKILL.md` + edit refs (incluye branch name `portable-plan/<N>` → `hs-plan/<N>`)
- `profiles/harness/claude/skills/portable-code-loop/SKILL.md` — rename a `hs-code-loop/SKILL.md` + edit refs
- `profiles/harness/claude/skills/portable-code-exec/SKILL.md` — rename a `hs-code-exec/SKILL.md` + edit refs
- `profiles/harness/claude/skills/portable-code-validate/SKILL.md` — rename a `hs-code-validate/SKILL.md` + edit refs (incluye marker `<!-- portable-code-validate:feedback -->`)
- `profiles/harness/claude/skills/portable-recover/SKILL.md` — rename a `hs-recover/SKILL.md` + edit refs
- `profiles/harness/claude/skills/portable-auto/SKILL.md` — rename a `hs-auto/SKILL.md` + edit refs
- `profiles/harness/opencode/skills/portable-spec/SKILL.md` — rename a `hs-spec/SKILL.md` + edit refs cruzadas
- `profiles/harness/opencode/skills/portable-plan/SKILL.md` — rename a `hs-plan/SKILL.md` + edit refs
- `profiles/harness/opencode/skills/portable-code-loop/SKILL.md` — rename a `hs-code-loop/SKILL.md` + edit refs
- `profiles/harness/opencode/skills/portable-code-exec/SKILL.md` — rename a `hs-code-exec/SKILL.md` + edit refs
- `profiles/harness/opencode/skills/portable-code-validate/SKILL.md` — rename a `hs-code-validate/SKILL.md` + edit refs
- `profiles/harness/opencode/skills/portable-recover/SKILL.md` — rename a `hs-recover/SKILL.md` + edit refs
- `profiles/harness/opencode/skills/portable-auto/SKILL.md` — rename a `hs-auto/SKILL.md` + edit refs
- `profiles/harness/claude/agents/portable-code-executor.md` — rename a `hs-code-executor.md` + edit frontmatter `name:`
- `profiles/harness/claude/agents/portable-code-validator.md` — rename a `hs-code-validator.md` + edit frontmatter `name:`
- `profiles/harness/opencode/agents/portable-code-executor.md` — rename a `hs-code-executor.md` + edit frontmatter `name:`
- `profiles/harness/opencode/agents/portable-code-validator.md` — rename a `hs-code-validator.md` + edit frontmatter `name:`
- `profiles/harness/claude/settings.json` — inspeccionar; modificar si tiene refs a "portable" en hooks/permissions/env
- `profiles/harness/claude/statusline-command.sh` — inspeccionar; modificar si tiene refs a "portable"
- `README.md` (raiz del repo) — inspeccionar; modificar si menciona el profile `portable` por nombre (NO tocar refs a "portable assets" como concepto)

## Riesgos

- Confusion entre "portable" como nombre del profile y "portable" como concepto del manager (assets layer multi-harness). Si se hace search-and-replace masivo, se rompe el manager. **Mitigacion**: editar archivo por archivo con `Edit replace_all`, NO tocar nada bajo `internal/`.
- Tests Go (`e2e_test.go`, `manifest_test.go`, `remote_test.go`) usan strings literales `"portable"` y `"portable-plan"` como nombres arbitrarios de profile/skill en fixtures, independientes del profile real. **Mitigacion**: dejarlos intactos; validar con `go test ./internal/profile/... ./internal/remote/...`.
- El branch name del propio plan-PR (`portable-plan/<N>`) sigue la convencion vieja en este PR — el rename pasa a `hs-plan/<N>` "going forward" y NO se aplica retroactivamente a este PR ni a otros ya abiertos.
- Usuarios con el profile instalado pierden los slash-commands viejos al actualizar; es esperable y se documenta en `CLAUDE.md`.

## Out of scope

- Migrar issues/PRs historicos del repo que citen `/portable-*` o `.portable/plans/`.
- Renombrar el concepto interno "portable assets" del profile manager (`internal/profile/...`).
- Renombrar/migrar el directorio `.portable/plans/` ya existente en otros repos que usaron el profile viejo.
- Agregar campo `description` al manifest TOML (requiere cambios en `internal/profile/manifest.go`, fuera de scope).
- Renombrar labels de GitHub (`entity:*`, `code:*`).

## Asunciones tecnicas validadas

1. Para preservar history, todos los renames se hacen con `git mv` (directorio entero del profile y luego cada skill/agent dir individualmente), no `cp + rm`.
2. El manifest `cvm.profile.toml` solo cambia el campo `name = "harness"`. NO se agrega `description` (requeriria cambios en `internal/profile/manifest.go`, fuera de scope).
3. La frase "harness engineering" vive en `CLAUDE.md` y `AGENTS.md` como header/intro, no como campo estructurado del manifest.
4. El concepto interno "portable assets" del profile manager (refs en `internal/profile/{authoring.go,manifest.go,profile.go}`, comentarios en `README.md` sobre "portable assets") NO se toca — es un mecanismo de assets multi-harness, ortogonal al nombre del profile.
5. Los tests Go (`e2e_test.go`, `internal/profile/manifest_test.go`, `internal/remote/remote_test.go`) que usan el string literal `"portable"` como nombre arbitrario de profile en fixtures NO se modifican — usan el string como literal independiente del profile real.
6. Las refs cruzadas dentro de cada `SKILL.md` (ej: `/portable-spec` mencionado desde el body de `/portable-plan`) se actualizan a `/hs-*` con `Edit replace_all` archivo por archivo.
7. El path convencional `.portable/plans/<N>-<slug>.md` hardcoded en multiples SKILLs (`/portable-plan`, `/portable-recover`, `/portable-auto`, agents) se actualiza a `.harness/plans/<N>-<slug>.md`.
8. El branch name `portable-plan/<N>` hardcoded en `/portable-plan` se actualiza a `hs-plan/<N>`.
9. Los prefijos de mktemp (`cvm-portable-spec-body`, `cvm-portable-plan-pr`, etc.) se actualizan a `cvm-hs-*`.
10. El YAML frontmatter de cada subagent (`name: portable-code-executor`, etc.) se actualiza al nuevo nombre `hs-code-{executor,validator}`.
11. El marker comment del feedback del validator (`<!-- portable-code-validate:feedback ... -->`) se actualiza a `<!-- hs-code-validate:feedback ... -->`.
12. Los archivos `claude/settings.json` y `claude/statusline-command.sh` se inspeccionan; si referencian "portable" se actualizan, si no quedan intactos.
13. La validacion post-cambio es minima: `go build ./...` + tests acotados de `internal/profile/...` y `internal/remote/...`. NO se corren tests e2e completos en este loop (eso es trabajo del validator del workflow code-loop).
14. Las ediciones se hacen archivo por archivo con `Edit replace_all` o `Write`, NO con `sed -i` masivo (macOS no tiene GNU sed compatible y los SKILLs tienen strings que podrian colisionar de forma indeseada).
15. El `README.md` raiz del repo se chequea con grep — si menciona el profile `portable` por nombre (ej: como ejemplo), se actualiza; las menciones de "portable assets" como concepto se dejan intactas.

---

_Plan generado por `/portable-plan` a partir de #70._

# Plan: Adoptar issues y PRs preexistentes al workflow portable

Refs #60 · https://github.com/chichex/cvm/issues/60

## Contexto

El profile portable necesita un nuevo skill `/portable-recover` que adopte issues y PRs preexistentes al flujo portable: aplica los labels `entity:*` faltantes y, cuando hace falta, genera los artefactos del workflow (ej: `.portable/plans/<N>-<slug>.md` derivado del diff de un PR). El caso motivador es el PR #59, que no tiene archivo de plan y por lo tanto `/portable-code-validate` lo rechaza; el skill debe poder "destrabarlo" generando el archivo y aplicando `entity:plan` en un solo flujo.

## Objetivo

Implementar `/portable-recover <issue|pr|url>` como skill interactivo multi-turno en los harnesses Claude Code y OpenCode del profile portable. El skill:

- Detecta el tipo de input (issue vs PR) y resuelve el repo destino.
- Diagnostica el estado actual (labels presentes, archivos del diff, shape del body).
- Adopta autónomamente cuando el shape es claramente compatible (genera artefactos + commit + push + label).
- Pregunta al usuario en casos ambiguos antes de forzar adopción.
- Sugiere el siguiente comando del workflow portable.

## Approach

Skill 100% en `SKILL.md` (sin código nuevo en el binario `cvm`), espejado en los dos harnesses:

- `profiles/portable/claude/skills/portable-recover/SKILL.md` — formato Claude Code, sin frontmatter YAML.
- `profiles/portable/opencode/skills/portable-recover/SKILL.md` — formato OpenCode con frontmatter YAML (`name`, `description`).

Mismo flujo lógico en ambos, sin auto-sync entre ellos (mantenimiento manual, alineado al patrón existente del profile).

Flujo runtime:

1. **Pre-flight**: `gh repo view`, `gh auth status`, working tree limpio. Aborta con mensaje accionable si falla cualquiera.
2. **Parse del input**: acepta `<N>`, `#<N>` o URL completa de GitHub. Para URLs apuntando a un repo distinto, extrae `owner/name` y lo usa con `--repo`.
3. **Detección issue vs PR**: intenta `gh issue view <N> --json ...` con fallback a `gh pr view <N> --json ...`. Si ambos fallan, aborta.
4. **Carga de contexto**: `labels`, `title`, `body`, `state`, `url`, y para PR adicionalmente `files`, `headRepositoryOwner`, `headRefName`, `baseRefName`.
5. **Diagnóstico**: clasifica como **compatible** (auto-adoptar) o **ambiguo** (pedir confirmación de force-adopt) según la heurística (ver asunción 11 abajo).
6. **Idempotencia**: si los artefactos + labels ya están en estado consistente, reporta "ya adoptado" + sugerencia de next step y termina sin tocar nada.
7. **Preview**: muestra al usuario el diagnóstico y la acción propuesta.
8. **Confirmación**: una sola pregunta sí/no.
9. **Ejecución**:
   - **Issue + shape compatible**: `gh issue edit --add-label entity:spec`. No toca el body.
   - **Issue + shape no compatible**: aborta con mensaje sugiriendo correr `/portable-spec` desde cero.
   - **PR + shape compatible**: guarda branch actual; `gh pr checkout <N>`; fetch + check de divergencia; escribir `.portable/plans/<N>-<slug>.md` derivado de title + lista de files; `git add/commit/push`; `gh pr edit --add-label entity:plan`; volver a la branch original.
   - **PR + shape ambiguo**: pregunta "¿forzar adopción igual?" y, si sí, sigue el flujo de PR compatible.
10. **Reporte**: bloque `## Result` con `target`, `diagnosis`, `actions_applied`, `next_step`.

## Pasos

- [ ] Crear `profiles/portable/claude/skills/portable-recover/SKILL.md` con el flujo completo en español (mismo estilo que `portable-spec`/`portable-plan`).
- [ ] Crear `profiles/portable/opencode/skills/portable-recover/SKILL.md` con frontmatter YAML adecuado al harness OpenCode.
- [ ] Actualizar `profiles/portable/CLAUDE.md`: agregar fila a la tabla de skills con descripción de `/portable-recover`.
- [ ] Actualizar `profiles/portable/opencode/AGENTS.md`: agregar fila equivalente para el harness OpenCode.
- [ ] Smoke test manual: correr `/portable-recover 59` sobre el PR #59; verificar que aplica `entity:plan` y crea `.portable/plans/59-<slug>.md` en el branch del PR.
- [ ] Smoke test idempotencia: re-correr y verificar que reporta "ya adoptado" sin tocar nada.
- [ ] Smoke test caso ambiguo: correr sobre un PR de docs y verificar que pregunta al usuario antes de adoptar.

## Archivos afectados

- `profiles/portable/claude/skills/portable-recover/SKILL.md` — **crear** — fuente del skill para Claude Code.
- `profiles/portable/opencode/skills/portable-recover/SKILL.md` — **crear** — fuente del skill para OpenCode.
- `profiles/portable/CLAUDE.md` — **modificar** — registrar el skill nuevo en la tabla.
- `profiles/portable/opencode/AGENTS.md` — **modificar** — registrar el skill nuevo en la tabla del harness OpenCode.

## Riesgos

- Pushear al branch del PR puede conflictuar con commits en curso del autor. Mitigación: `git fetch origin <branch>` + comparación con `git rev-list` antes de pushear; abortar si hay divergencia.
- Generar el plan desde solo `title + files` (sin el diff completo) puede producir un artefacto pobre. Mitigación: el plan generado es minimal y declara explícitamente "adoptado post-hoc" en una nota; el usuario puede enriquecerlo antes de validar.
- PRs desde forks pueden no permitir push del adopter. Mitigación: pre-check de `headRepositoryOwner` vs el repo destino y abortar si no hay permisos.
- PR con muchísimos archivos puede saturar la lista del plan. Mitigación: truncar a los primeros 50 + nota "(+N archivos más)".
- Los dos `SKILL.md` (Claude vs OpenCode) deben mantenerse sincronizados a mano. Mitigación: documentar la duplicación en el plan; no se introduce auto-sync para no romper el patrón existente del profile.
- Generar contenido de spec a partir de un issue ambiguo podría inducir interpretaciones erróneas. Mitigación: para issues, el skill solo aplica label cuando el shape ya está; nunca modifica el body.

## Out of scope

- No abrir issues nuevos ni crear PRs nuevos.
- No ejecutar `/portable-code-*` automáticamente — solo sugerir el next step.
- No tocar código de aplicación; el único archivo escrito es `.portable/plans/<N>-<slug>.md`.
- No tocar labels `code:*` (responsabilidad de los skills `/portable-code-*`).
- No modificar el body del issue al adoptarlo como spec; si no tiene shape, abortar y derivar a `/portable-spec`.
- No agregar tests automatizados — validación es smoke manual contra fixtures.
- No tocar el binario `cvm` (Go) ni dependencias del `go.mod`.
- No agregar lógica de auto-sync entre los `SKILL.md` de Claude y OpenCode.

## Asunciones técnicas validadas

1. **Ubicación dual del source**: el skill vive en `profiles/portable/claude/skills/portable-recover/SKILL.md` (Claude Code) y en `profiles/portable/opencode/skills/portable-recover/SKILL.md` (OpenCode con frontmatter YAML). Mismo flujo lógico, distinta sintaxis del header.
2. **Modelo de ejecución**: interactivo multi-turno en el orquestador principal del harness; no se delega a subagent. Mismo patrón de `/portable-spec` y `/portable-plan`.
3. **Detección issue vs PR**: el skill primero intenta `gh issue view <N> --json ...`; si falla por not-found, cae a `gh pr view <N> --json ...`. Si ambos fallan, aborta.
4. **Parser del input**: acepta `<N>`, `#<N>` o URL completa de GitHub. Para URLs apuntando a un repo distinto del actual, extrae `owner/name` y lo pasa como `--repo` a los `gh` commands.
5. **Numeración del archivo de plan al adoptar PR**: usa el número del PR como `<N>` para `.portable/plans/<N>-<slug>.md`. Si el body del PR contiene `Closes #X` (o `Fixes #X`), usa X como `<N>` en su lugar.
6. **Slug del archivo de plan**: derivado del título del PR via la misma regex que `/portable-plan` (lowercase + espacios→`-` + strip non-`[a-z0-9-]` + colapsar `-` repetidos + trim a 50 chars al último word boundary).
7. **Generación del contenido del plan**: el skill carga `gh pr view <N> --json title,body,files` (no el diff completo). Genera secciones: Contexto (link al PR + resumen del body si existe), Objetivo (extraído del título), Approach (placeholder con nota "adoptado post-hoc"), Pasos (un checkbox por archivo afectado, marcado como done), Archivos afectados (lista de `files`), Riesgos (placeholder), Out of scope (placeholder), Asunciones validadas (nota explicando que el plan fue adoptado).
8. **Adopción de issue**: si el issue NO tiene shape de spec (sin secciones `## Historia` / `## Asunciones` / `## Criterios`), el skill aborta con mensaje accionable sugiriendo correr `/portable-spec` desde cero. NO modifica el body del issue. Solo aplica `entity:spec` cuando el body YA tiene secciones reconocibles.
9. **Operaciones git en branch del PR**: el skill guarda la branch actual con `git rev-parse --abbrev-ref HEAD`, hace `gh pr checkout <N>`, escribe el archivo, `git add` + `git commit -m "Add adopted plan for #<N>"` + `git push`, y vuelve a la branch original con `git checkout <prev>`.
10. **Detección de divergencia**: antes de pushear, ejecuta `git fetch origin <branch>` y compara con `git rev-list`. Si hay divergencia, aborta con mensaje al usuario.
11. **Heurística "compatible" vs "ambiguo"**:
    - PR compatible: el diff contiene al menos un archivo de código (no solo `.md`, `docs/`, `README`).
    - PR ambiguo: solo cambia archivos de docs.
    - Issue compatible: body con secciones `## Historia` / `## Asunciones` / `## Criterios` reconocibles por regex.
    - Issue ambiguo: body sin secciones reconocibles.
12. **Validación de auth**: pre-flight `gh auth status`. NO chequea explícitamente permisos de write — deja que el primer comando con write falle con mensaje claro de `gh`.
13. **Idempotencia**: si el PR ya tiene `entity:plan` Y existe `.portable/plans/<N>-<slug>.md` en su branch → solo muestra "PR ya adoptado" + sugerencia. Issue con `entity:spec` → mismo comportamiento.
14. **Idioma del output**: español, alineado con los demás skills del profile portable.
15. **Confirmación**: una sola confirmación sí/no después del diagnóstico. Sin barras de progreso ni multi-turno largo.
16. **Mapa de sugerencias de next step** (lookup table):
    - Issue con `entity:spec` → `/portable-plan <N>`
    - PR con `entity:plan` (sin `code:*`) → `/portable-code-loop <PR>` o `/portable-code-exec <PR>`
    - PR con `entity:plan` + `code:exec` → `/portable-code-validate <PR>`
    - PR con `code:failed` → `/portable-code-loop <PR>`
    - PR con `code:passed` → "no acción; revisar y mergear"
17. **Output del Result**: bloque `## Result` con campos `target`, `diagnosis`, `actions_applied`, `next_step` — alineado con el formato de los otros skills.
18. **Tests**: validación manual (smoke contra PR fixture #59 entre otros). No se agregan tests automatizados ni Go. La capa cvm core no necesita cambios.
19. **Documentación**: actualizar `profiles/portable/CLAUDE.md` (tabla de skills) y `profiles/portable/opencode/AGENTS.md`.
20. **Install/scaffold**: el skill se incluye automáticamente por estar en `profiles/portable/{claude,opencode}/skills/` (zero-config glob). No requiere cambios al installer ni al binario.
21. **Argumento**: el skill recibe un único argumento string (el identificador). Si está vacío, lo pide en el primer turno.
22. **Backup del estado git**: guarda branch original antes del checkout, la restaura al final (incluso en error). Pre-flight aborta si working tree no está limpio.
23. **No tocar `code:*`**: usa `gh pr edit --add-label entity:plan` sin `--remove-label`. Solo agrega; nunca quita.
24. **Manejo de PRs cross-fork**: pre-check de `headRepositoryOwner`; si el PR viene de fork sin permisos de push del adopter, aborta con mensaje accionable.
25. **Límite del diff cargado**: si el PR tiene >50 archivos cambiados, trunca la lista a los primeros 50 + nota "(+N archivos más)" en el plan.

---

_Plan generado por `/portable-plan` a partir de #60._

Una sola pasada de validacion sobre un PR con label `entity:plan`. Delega al subagent `portable-code-validator` (Opus) que espera `gh pr checks`, contrasta el diff vs cada paso/archivo/riesgo del plan, corre la suite completa local, y emite verdict PASS|FAIL con feedback accionable. NO ejecuta — para implementar usar `/portable-code-exec` o el loop completo `/portable-code-loop`. `$ARGUMENTS` es el numero de PR (puede venir vacio — en ese caso se pide).

Skill **exclusivo para Claude Code** (depende del subagent `portable-code-validator` en `~/.claude/agents/`). Wrapper thin sobre el agent. Sirve para auditar PRs propios o ajenos sin tocar codigo.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar:
```
No hay un repo GitHub configurado en este directorio. /portable-code-validate necesita un PR de plan en GitHub.
```

### 2. Validar working tree limpio
```bash
git status --porcelain
```
Si hay cambios sin commitear, abortar:
```
Working tree no esta limpio. /portable-code-validate hace checkout de la branch del PR para correr tests; commiteá o stashé los cambios pendientes antes de correr.
```

### 3. Parsear `$ARGUMENTS`
- Si esta vacio: pedir `Pasame el numero del PR a validar (ej: 42).` y esperar respuesta.
- Normalizar: aceptar `42`, `#42`, o URL — extraer solo el numero. Guardar como `PR`.

### 4. Cargar el PR
```bash
gh pr view "$PR" --json number,title,url,state,labels,headRefName,baseRefName
```
- Si no existe: abortar.
- Si `state != "OPEN"`: avisar `El PR #<PR> no esta OPEN (state: <X>). Validar igual? (si/no)`. Si dice no, abortar. (Auditar PRs cerrados es valido — ej: post-mortem.)
- Si los labels no incluyen `entity:plan`: avisar y pedir confirmacion para continuar.

Guardar `BRANCH`, `BASE`, `PR_TITLE`, `PR_URL`.

### 5. Localizar el archivo de plan
```bash
gh pr diff "$PR" --name-only
```
Buscar un unico `.portable/plans/<N>-<slug>.md`. Si no aparece exactamente uno, abortar. Guardar como `PLAN_FILE`.

### 6. Checkout
```bash
gh pr checkout "$PR"
git pull --ff-only origin "$BRANCH" 2>/dev/null || true
```

### 6b. Asegurar labels de estado (idempotente)
```bash
gh label create "code:exec"   --color "FBCA04" --description "portable-code: last op was exec, pending validate" 2>/dev/null || true
gh label create "code:passed" --color "0E8A16" --description "portable-code: last validate emitted PASS"          2>/dev/null || true
gh label create "code:failed" --color "B60205" --description "portable-code: last validate emitted FAIL"          2>/dev/null || true
```

### 7. Leer el plan
`Read` sobre `PLAN_FILE` → `PLAN_TEXT`. **No** imprimirlo al usuario.

Confirmar:
```
PR #<PR>: <PR_TITLE>
Branch: <BRANCH> (base: <BASE>)
Plan: <PLAN_FILE>

Voy a delegar a `portable-code-validator` (Opus) para una sola pasada de validacion: espera `gh pr checks`, corre suite completa local, contrasta diff vs plan. Sin ejecucion — usa `/portable-code-exec <PR>` o `/portable-code-loop <PR>` para implementar.

Confirmas? (si/no)
```
Si dice `no`, abortar.

## Delegar al validator

Llamar `Agent` tool con:
- `subagent_type: "portable-code-validator"`
- `description: "portable-code-validate single-shot"`
- `prompt`:

```
iter: single-shot
pr_number: <PR>
branch: <BRANCH>

plan_text:
---
<PLAN_TEXT>
---

exec_report (vacio en single-shot):
---
(vacio)
---
```

Esperar el resultado. Parsear el `## Validate report`. Guardar `verdict` y el bloque completo como `validate_report`.

## Aplicar label segun verdict (mutuamente exclusivo)
```bash
if [ "$verdict" = "PASS" ]; then
  gh pr edit "$PR" --add-label "code:passed" --remove-label "code:exec" --remove-label "code:failed" 2>/dev/null
else
  gh pr edit "$PR" --add-label "code:failed" --remove-label "code:exec" --remove-label "code:passed" 2>/dev/null
fi
```

## Postear el feedback como comment del PR

Generar via `Write` a un tempfile (NUNCA interpolar el reporte en double-quoted shell commands):
```bash
COMMENT_FILE="$(mktemp -t cvm-portable-code-validate-feedback.XXXXXX).md"
```
Body del comment:
```markdown
<!-- portable-code-validate:feedback iter=single-shot verdict=<verdict> -->
## Validate feedback (single-shot) — <verdict>

<validate_report>

---
_Posted by `/portable-code-validate`._
```
Postear:
```bash
gh pr comment "$PR" --body-file "$COMMENT_FILE"
```

## Cierre

Imprimir el reporte tal cual lo devolvio el agent (es la salida principal del skill), seguido de:

```
## Result
- pr_url: <PR_URL>
- pr: #<PR>
- branch: <BRANCH>
- mode: single-shot validate (sin ejecucion)
- verdict: <PASS|FAIL>
- label_applied: <code:passed|code:failed>
- feedback_persisted: yes (PR comment)

<si FAIL:>
Para resolver el feedback: /portable-code-exec <PR>
Para loop completo (exec + validate iterativo): /portable-code-loop <PR>
</si>
<si PASS:>
PR listo para review/merge: <PR_URL>
</si>
```

## MUST DO

- Validar repo / working tree / PR / plan ANTES de delegar.
- Pedir confirmacion explicita al usuario antes de delegar.
- Delegar a `portable-code-validator` (no a `general-purpose`).
- Pasar el plan completo en el prompt.
- Imprimir el `## Validate report` tal cual.

## MUST NOT DO

- No ejecutar nada vos mismo (no escribir codigo, no commitear). Es responsabilidad de `/portable-code-exec`.
- No hacer mas de una pasada — para iterar usar `/portable-code-loop`.
- No correr build/test/lint desde el orquestador.
- No tocar labels del PR distintos de `code:passed` / `code:failed` / `code:exec`.
- No interpolar el reporte del validator en double-quoted shell commands — siempre via `--body-file` con Write tool.
- No persistir nada en auto-memory.

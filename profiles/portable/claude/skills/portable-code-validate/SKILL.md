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

Esperar el resultado. Parsear el `## Validate report`.

## Cierre

Imprimir el reporte tal cual lo devolvio el agent (es la salida principal del skill), seguido de:

```
## Result
- pr_url: <PR_URL>
- pr: #<PR>
- branch: <BRANCH>
- mode: single-shot validate (sin ejecucion)
- verdict: <PASS|FAIL>

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
- No tocar labels del PR.
- No persistir nada en auto-memory.

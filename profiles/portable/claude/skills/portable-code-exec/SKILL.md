Una sola pasada de implementacion sobre un PR con label `entity:plan`. Delega al subagent `portable-code-executor` (Sonnet) que escribe codigo segun el plan, hace chequeos minimos (build/typecheck + 1-3 unit tests acotados), commitea y pushea. NO valida — para eso esta `/portable-code-validate` o el loop completo `/portable-code-loop`. `$ARGUMENTS` es el numero de PR (puede venir vacio — en ese caso se pide).

Skill **exclusivo para Claude Code** (depende del subagent `portable-code-executor` en `~/.claude/agents/`). Wrapper thin sobre el agent — el orquestador solo hace pre-flight, delega, y reporta.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar:
```
No hay un repo GitHub configurado en este directorio. /portable-code-exec necesita un PR de plan en GitHub.
```

### 2. Validar working tree limpio
```bash
git status --porcelain
```
Si hay cambios sin commitear, abortar:
```
Working tree no esta limpio. /portable-code-exec hace checkout de la branch del PR; commiteá o stashé los cambios pendientes antes de correr.
```

### 3. Parsear `$ARGUMENTS`
- Si esta vacio: pedir `Pasame el numero del PR de plan (ej: 42).` y esperar respuesta.
- Normalizar: aceptar `42`, `#42`, o URL — extraer solo el numero. Guardar como `PR`.

### 4. Cargar el PR
```bash
gh pr view "$PR" --json number,title,url,state,labels,headRefName,baseRefName
```
- Si no existe: abortar.
- Si `state != "OPEN"`: abortar `El PR #<PR> no esta OPEN.`
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

Voy a delegar a `portable-code-executor` (Sonnet) para una sola pasada de implementacion. Sin validacion al final — usa `/portable-code-validate <PR>` o `/portable-code-loop <PR>` para auditar.

Confirmas? (si/no)
```
Si dice `no`, abortar.

## Delegar al executor

Llamar `Agent` tool con:
- `subagent_type: "portable-code-executor"`
- `description: "portable-code-exec single-shot"`
- `prompt`:

```
iter: single-shot
pr_number: <PR>
branch: <BRANCH>

plan_text:
---
<PLAN_TEXT>
---

last_feedback (vacio en single-shot):
---
(vacio)
---
```

Esperar el resultado. Parsear el `## Exec report`.

## Aplicar label de estado

```bash
gh pr edit "$PR" --add-label "code:exec" --remove-label "code:passed" --remove-label "code:failed" 2>/dev/null
```
(`--remove-label` no falla si el label no estaba aplicado.)

## Cierre

Imprimir el reporte tal cual lo devolvio el agent (es informacion util para el usuario), seguido de:

```
## Result
- pr_url: <PR_URL>
- pr: #<PR>
- branch: <BRANCH>
- mode: single-shot exec (sin validacion)
- label_applied: code:exec

Para validar: /portable-code-validate <PR>
Para loop completo (exec + validate iterativo): /portable-code-loop <PR>
```

## MUST DO

- Validar repo / working tree / PR / plan ANTES de delegar.
- Pedir confirmacion explicita al usuario antes de delegar.
- Delegar a `portable-code-executor` (no a `general-purpose`).
- Pasar el plan completo en el prompt.
- Imprimir el `## Exec report` tal cual.

## MUST NOT DO

- No validar nada vos mismo (no correr tests/checks). Es responsabilidad de `/portable-code-validate`.
- No hacer mas de una pasada — para iterar usar `/portable-code-loop`.
- No correr build/test/lint desde el orquestador.
- No tocar labels del PR distintos de `code:exec` / `code:passed` / `code:failed`.
- No persistir nada en auto-memory.

A partir de un PR con label `entity:plan`, ejecuta el plan iterativamente: en cada vuelta delega la implementacion al subagent `portable-code-executor` (Sonnet, escribe codigo + chequeos minimos: build/typecheck y unos pocos unit tests, commitea y pushea); despues delega la validacion al subagent `portable-code-validator` (Opus, espera `gh pr checks`, corre la suite completa, contrasta diff vs plan, emite PASS/FAIL + feedback). Por default, hasta 5 iteraciones. Auto-detecta si arrancar por exec o validate segun el estado actual del PR. `$ARGUMENTS` es el numero de PR (puede venir vacio — en ese caso se pide). Soporta flag opcional `--max N` para cambiar el tope de iteraciones.

Skill **exclusivo para Claude Code** (depende de los subagents `portable-code-executor` y `portable-code-validator` desplegados en `~/.claude/agents/`). El orquestador (Claude principal) **debe mantener su contexto bajo**: toda la ejecucion y validacion vive dentro de subagents; el orquestador solo trackea iteracion, verdict y un feedback compacto entre vueltas.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar:
```
No hay un repo GitHub configurado en este directorio. /portable-code-loop necesita un PR de plan en GitHub para iterar.
```

### 2. Validar working tree limpio
```bash
git status --porcelain
```
Si hay cambios sin commitear, abortar:
```
Working tree no esta limpio. /portable-code-loop hace checkout de la branch del PR; commiteá o stashé los cambios pendientes antes de correr.
```

### 3. Parsear `$ARGUMENTS`
- Forma esperada: `<pr-number>` o `<pr-number> --max N` o `--max N <pr-number>`.
- Si esta vacio: pedir al usuario `Pasame el numero del PR de plan (ej: 42).` y esperar respuesta. **No** continuar hasta tenerlo.
- Normalizar el PR: aceptar `42`, `#42`, o URL completa — extraer solo el numero. Guardar como `PR`.
- `--max N`: si esta presente y es un entero `1..20`, guardar como `MAX_ITER`. Default `MAX_ITER=5`. Si esta fuera de rango, abortar pidiendo un valor valido.

### 4. Cargar el PR
```bash
gh pr view "$PR" --json number,title,url,state,labels,headRefName,baseRefName,isDraft
```
Validaciones:
- Si el PR no existe: abortar con el error de `gh`.
- Si `state != "OPEN"`: abortar `El PR #<PR> no esta OPEN (state: <X>). /portable-code-loop solo opera sobre PRs abiertos.`
- Si los labels **no** incluyen `entity:plan`: avisar `El PR #<PR> no tiene label entity:plan. /portable-code-loop esta pensado para PRs de plan. Continuar igual? (si/no)`. Si dice no, abortar.

Guardar `BRANCH = headRefName`, `BASE = baseRefName`, `PR_TITLE = title`, `PR_URL = url`.

### 5. Localizar el archivo de plan en el PR
```bash
gh pr diff "$PR" --name-only
```
Buscar un archivo `.portable/plans/<N>-<slug>.md`. Si no aparece exactamente uno, abortar:
```
No pude localizar un unico archivo .portable/plans/*.md en el PR #<PR>. /portable-code-loop necesita ese archivo como fuente del plan.
```
Guardar como `PLAN_FILE`.

### 5b. Asegurar labels de estado (idempotente)
```bash
gh label create "code:exec"   --color "FBCA04" --description "portable-code: last op was exec, pending validate" 2>/dev/null || true
gh label create "code:passed" --color "0E8A16" --description "portable-code: last validate emitted PASS"          2>/dev/null || true
gh label create "code:failed" --color "B60205" --description "portable-code: last validate emitted FAIL"          2>/dev/null || true
```

### 5c. Auto-detect del arranque

Estrategia en dos capas — labels primero, diff como fallback.

**Capa 1: leer labels del PR** (ya cargados en step 4 como `LABELS`).
- Si tiene `code:passed`: abortar con
  ```
  PR #<PR> ya tiene label code:passed (validate previo dio PASS). No hay nada que iterar.
  Si querés re-validar igual, corré: /portable-code-validate <PR>
  ```
- Si tiene `code:failed`: `START_WITH = "exec"`. Recuperar el ultimo feedback del validator:
  ```bash
  gh pr view "$PR" --json comments --jq '.comments | map(select(.body | startswith("<!-- portable-code-validate:feedback"))) | last | .body // ""'
  ```
  Si devuelve un body, extraer el `## Validate report` que esta despues del marker y guardarlo como `last_feedback` (asi el primer exec arranca con contexto, no ciego).
- Si tiene `code:exec`: `START_WITH = "validate"`.
- Si NO tiene ninguno de los tres: **fallback heurística diff**:
  - Si el diff del PR contiene **solo** `PLAN_FILE`: `START_WITH = "exec"` (PR todavia sin implementacion).
  - Si el diff contiene `PLAN_FILE` **+ otros archivos**: `START_WITH = "validate"` (ya hay implementacion sin label de estado — auditarla).

### 6. Checkout local de la branch del PR
```bash
gh pr checkout "$PR"
git pull --ff-only origin "$BRANCH" 2>/dev/null || true
```
Si falla el checkout, abortar con el error.

### 7. Leer el plan
Usar la `Read` tool sobre `PLAN_FILE`. Guardar el contenido en una variable conceptual `PLAN_TEXT` — **no** lo imprimas entero al usuario. Confirmá:
```
PR #<PR> cargado: <PR_TITLE>
Branch: <BRANCH> (base: <BASE>)
Plan: <PLAN_FILE>
Iteraciones maximas: <MAX_ITER>
Arranco por: <START_WITH>  ← auto-detectado: <razon corta>

Voy a delegar a los subagents `portable-code-executor` (Sonnet) y `portable-code-validator` (Opus). Vas a ver un resumen compacto por vuelta.

Confirmas? (si/no)
```
Si dice `no`, abortar sin tocar nada.

## Loop principal

Inicializar:
- `iter = 0`
- `last_feedback = ""` (vacio en la primera vuelta)
- `last_verdict = ""`
- `last_exec_report = ""` (vacio si arrancas por validate sin pasar por exec primero)
- `next_phase = START_WITH`

Mientras `iter < MAX_ITER` y `last_verdict != "PASS"`:

1. `iter = iter + 1`
2. Anunciar al usuario: `--- Iteracion <iter>/<MAX_ITER> ---`
3. Si `next_phase == "exec"`:
   - Ejecutar **Fase EXEC** (subagent `portable-code-executor`) — ver mas abajo.
   - Despues de exec, esta misma vuelta sigue con validate.
4. Ejecutar **Fase VALIDATE** (subagent `portable-code-validator`).
5. Mostrar resumen compacto al usuario (3-6 lineas max).
6. Si `last_verdict == "PASS"`, romper.
7. Si `iter == MAX_ITER`: romper con resultado FAIL.
8. Para la proxima vuelta: `next_phase = "exec"` (siempre — despues de la primera vuelta, todo ciclo es exec→validate).

### Fase EXEC — subagent portable-code-executor

Llamar `Agent` tool con:
- `subagent_type: "portable-code-executor"`
- `description: "portable-code-loop exec iter <iter>"`
- `prompt`: el delta de esta vuelta (NO repetir las reglas — viven en el system prompt del agent)

Template del prompt (reemplazar los `<...>`):

```
iter: <iter>/<MAX_ITER>
pr_number: <PR>
branch: <BRANCH>

plan_text:
---
<PLAN_TEXT>
---

last_feedback (resolve esto primero, vacio si es la primera vuelta):
---
<last_feedback o "(vacio)">
---
```

Esperar el resultado. Parsear el `## Exec report` y guardar `last_exec_report = <bloque completo>`.

**Aplicar label `code:exec`** (idempotente, mutuamente exclusivo con los otros dos):
```bash
gh pr edit "$PR" --add-label "code:exec" --remove-label "code:passed" --remove-label "code:failed" 2>/dev/null
```
(`--remove-label` no falla si el label no estaba aplicado.)

### Fase VALIDATE — subagent portable-code-validator

Llamar `Agent` tool con:
- `subagent_type: "portable-code-validator"`
- `description: "portable-code-loop validate iter <iter>"`
- `prompt`: el delta de esta vuelta

Template del prompt:

```
iter: <iter>/<MAX_ITER>
pr_number: <PR>
branch: <BRANCH>

plan_text:
---
<PLAN_TEXT>
---

exec_report (vacio si esta vuelta arranco por validate sin exec previo):
---
<last_exec_report o "(vacio)">
---
```

Esperar el resultado. Parsear el `## Validate report`. Guardar:
- `last_verdict = verdict`
- `last_feedback = feedback_for_next_exec` (vacio si PASS)
- `last_validate_report = <bloque completo>`

**Aplicar label segun verdict** (mutuamente exclusivo):
```bash
if [ "$last_verdict" = "PASS" ]; then
  gh pr edit "$PR" --add-label "code:passed" --remove-label "code:exec" --remove-label "code:failed" 2>/dev/null
else
  gh pr edit "$PR" --add-label "code:failed" --remove-label "code:exec" --remove-label "code:passed" 2>/dev/null
fi
```

**Postear el feedback como comment del PR** (para que sobreviva a la sesion del orquestador). Generar via `Write` a un tempfile (NUNCA interpolar el reporte en double-quoted shell commands):
```bash
COMMENT_FILE="$(mktemp -t cvm-portable-code-loop-feedback.XXXXXX).md"
```
Body del comment:
```markdown
<!-- portable-code-validate:feedback iter=<iter> verdict=<last_verdict> -->
## Validate feedback (iter <iter>/<MAX_ITER>) — <last_verdict>

<last_validate_report>

---
_Posted by `/portable-code-loop`._
```
Postear:
```bash
gh pr comment "$PR" --body-file "$COMMENT_FILE"
```

### Resumen compacto al usuario por iteracion

Imprimir (no mas que esto, salvo que el usuario pida detalles):

```
## Iter <iter>/<MAX_ITER>
- exec: <N> commits, compile=<ok|fail>, pasos: <lista corta>  ← omitir esta linea si la iteracion arranco directo por validate
- validate: <PASS|FAIL> — checks=<resumen>, tests=<resumen>
<si FAIL:>
- proximo focus: <last_feedback resumido a 1-2 lineas>
</si>
```

## Cierre

### Caso PASS
```
## Result
- pr_url: <PR_URL>
- pr: #<PR>
- branch: <BRANCH>
- started_with: <START_WITH>
- iterations_used: <iter>/<MAX_ITER>
- verdict: PASS
- final_pr_checks: <resumen>

PR listo para review/merge: <PR_URL>
```

### Caso FAIL (agotadas las iteraciones)
```
## Result
- pr_url: <PR_URL>
- pr: #<PR>
- branch: <BRANCH>
- started_with: <START_WITH>
- iterations_used: <MAX_ITER>/<MAX_ITER>
- verdict: FAIL
- last_feedback: <last_feedback completo>

Loop agotado sin alcanzar PASS. El PR quedo en su ultimo estado en <BRANCH>. Revisa el feedback arriba y decidi si correr de nuevo (con mas iteraciones via `--max`), retomar a mano, o cerrar el PR.
```

El estado del PR queda reflejado en el label `code:passed` o `code:failed` (aplicado en cada vuelta del validate). El ultimo feedback queda persistido como comment del PR con marker `<!-- portable-code-validate:feedback ... -->`, asi una invocacion futura de `/portable-code-loop` puede recuperarlo.

## MUST DO

- Validar `gh repo view`, working tree limpio y existencia/estado del PR ANTES de empezar el loop.
- Verificar que el PR tiene label `entity:plan` (o pedir confirmacion si no).
- Localizar exactamente un `.portable/plans/<N>-<slug>.md` en el diff del PR.
- Crear los labels `code:exec`, `code:passed`, `code:failed` (idempotente) antes del loop.
- Auto-detectar el arranque: labels primero (`code:passed` → abortar; `code:failed` → exec con feedback recuperado del comment marker; `code:exec` → validate); si no hay label, fallback a heurística del diff.
- Aplicar label de estado mutuamente exclusivo despues de cada exec (`code:exec`) y validate (`code:passed`/`code:failed`).
- Postear el `## Validate report` como comment del PR con marker `<!-- portable-code-validate:feedback ... -->` despues de cada validate, via `--body-file` (Write tool genera el archivo).
- Hacer `gh pr checkout` antes de delegar a los subagents.
- Delegar exec a `portable-code-executor` (no a `general-purpose`).
- Delegar validate a `portable-code-validator` (no a `general-purpose`).
- Pasar el plan completo a ambos subagents en el prompt (es la fuente de verdad).
- Pasar el `feedback_for_next_exec` del validador previo al executor de la siguiente vuelta.
- Mantener el contexto del orquestador minimo: solo iter, verdict, feedback y exec_report compactos entre vueltas.
- Mostrar al usuario un resumen de 3-6 lineas por iteracion.
- Respetar `MAX_ITER` (default 5).
- Al cerrar, reportar PR URL, iteraciones usadas y verdict final.

## MUST NOT DO

- No correr build/test/lint vos mismo desde el orquestador — eso es trabajo de los subagents.
- No leer ni imprimir el diff completo del PR en el thread principal — vive dentro del validador.
- No imprimir el contenido completo del plan al usuario (solo confirmar el path).
- No mezclar roles ni tipos de subagent.
- No usar `git push --force` ni mergear el PR ni tocar otras branches.
- No agregar/quitar labels al PR distintos de `code:exec` / `code:passed` / `code:failed`. NO tocar `entity:plan` ni otros.
- No interpolar el reporte del validator en double-quoted shell commands — siempre via `--body-file` con Write tool.
- No avanzar del pre-flight sin confirmacion explicita del usuario.
- No persistir estado entre invocaciones (cada `/portable-code-loop` arranca de cero).
- No persistir nada en auto-memory.

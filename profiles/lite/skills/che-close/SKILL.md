Cerrar un PR siguiendo el flujo `che close` de che-cli: ready-for-review (si era draft), espera CI, merge, cierra issues linkeados, y aplica las transitions de la state machine `che:*`. `$ARGUMENTS` es el PR a cerrar: numero, URL, o vacio (usa el PR de la branch actual). Flag opcional `--merge-type=squash|merge|rebase` (default `squash`).

## Contrato de labels (de `che-cli/internal/labels/labels.go`)

Transitions aplicadas (modo lenient — ver "Lenient mode" abajo):

- **Pre (lock)**: `<current>→che:closing` donde `<current>` es `che:executed` o `che:validated`.
- **Success**: `che:closing→che:closed`.
- **Rollback** (si subagent falla): `che:closing→<previous>` con `<previous>` = el state que tenia antes del lock.

Para cada issue linkeado al PR, al cerrarlo aplicar la transition correspondiente:
- Si issue estaba en `che:validated` → `che:validated→che:closing→che:closed`.
- Si issue estaba en otro state → lenient: limpiar `che:*` y aplicar `che:closed`.

## Proceso

### Paso 1: Parsear input y resolver PR

**Precheck auth**: `gh auth status` una vez. Si falla, abortar: "gh no esta autenticado o no hay red — corre `gh auth login` y reintenta."

Detectar formato:
- **Vacio** → `gh pr view --json number,title,url,headRefName,isDraft,labels,closingIssuesReferences,baseRefName,state 2>/dev/null`. Si falla, abortar: "No hay PR asociado a la branch actual. Pasa `/che-close <numero>` o URL."
- **URL** (`https://github.com/<owner>/<repo>/pull/N`) → parsear localmente (split por `/`), guardar `OWNER`, `REPO`, `N`.
- **Numero puro** (validar `^[0-9]+$` ANTES de pasar a shell) → `N`. Resolver `OWNER/REPO` via `gh repo view --json owner,name --jq '"\(.owner.login)/\(.name)"'`.
- **Cualquier otro input** → abortar: "Uso: `/che-close [N | URL | vacio para branch actual]` [--merge-type=squash|merge|rebase]"

Parsear flag `--merge-type=`. Solo aceptar `squash`, `merge`, `rebase`. Default `squash`. Cualquier otro valor → abortar.

Guardar: `N`, `OWNER`, `REPO`, `MERGE_TYPE`, `IS_DRAFT`, `PR_LABELS`, `LINKED_ISSUES` (lista de numbers de `closingIssuesReferences`).

### Paso 2: Resolver state actual (stateref)

Replica `che-cli/internal/flow/stateref`:

1. Buscar en `PR_LABELS` cualquiera de `che:idea`, `che:planning`, `che:plan`, `che:executing`, `che:executed`, `che:validating`, `che:validated`, `che:closing`, `che:closed`. Si encuentra, ese es `CURRENT_STATE`.
2. Si el PR NO tiene `che:*`, iterar `LINKED_ISSUES`. Para cada uno: `gh issue view "$num" --repo "$OWNER/$REPO" --json labels`. Si tiene `che:*`, usar ese como `CURRENT_STATE` y guardar el issue como `STATE_ISSUE`.
3. Si nadie tiene `che:*` → `CURRENT_STATE=""` (warn al usuario: "PR ni issues linkeados tienen label `che:*`. Procediendo en modo lenient.").

### Paso 3: Pre-transition (lock → `che:closing`)

Si `CURRENT_STATE` es `che:executed` o `che:validated`:
```bash
gh api -X DELETE "repos/$OWNER/$REPO/issues/$N/labels/$CURRENT_STATE" 2>/dev/null
gh api -X POST "repos/$OWNER/$REPO/issues/$N/labels" -f "labels[]=che:closing"
```
Antes del POST, asegurar que `che:closing` existe en el repo:
```bash
gh label create "che:closing" --force 2>/dev/null
```

Si `CURRENT_STATE` es otro o vacio (lenient mode):
- Warn: "PR no esta en che:executed ni che:validated (esta en `<CURRENT_STATE>` o sin che:*). Aplicando lock igual."
- Remover TODOS los `che:*` que tenga el PR (loop sobre los 9 estados, DELETE cada uno con tolerancia a 404).
- Aplicar `che:closing`.

Guardar `PREVIOUS_STATE=$CURRENT_STATE` para usar en rollback.

### Paso 4: Armar contexto y lanzar subagent

Escribir `/tmp/cvm-close-<N>-context.md` con `Write` tool:

```markdown
# Close PR #<N>

**Repo**: <OWNER>/<REPO>
**PR**: #<N>
**Merge type**: <MERGE_TYPE>
**Is draft**: <IS_DRAFT>
**Linked issues**: <LINKED_ISSUES o "ninguno">
**Previous state**: <PREVIOUS_STATE o "sin che:*">

## Tarea
Llevar el PR #<N> hasta merge + close, en este orden:

1. Si `IS_DRAFT=true`: `gh pr ready <N> --repo <OWNER>/<REPO>`. Si falla, reportar y abortar (rollback).
2. Esperar CI: poll `gh pr checks <N> --repo <OWNER>/<REPO> --json state,conclusion,name --jq '.[]'` cada 30s. CI esta verde cuando ningun check tiene `state=PENDING|QUEUED|IN_PROGRESS` Y todos los completados tienen `conclusion=SUCCESS|SKIPPED|NEUTRAL`. Timeout 15min — si se agota, abortar (rollback).
   - Tolerar exit code 8 de `gh pr checks` (= "hay checks fallando"); leer el JSON igual.
   - Si algun check tiene `conclusion=FAILURE|CANCELLED|TIMED_OUT|ACTION_REQUIRED|STALE`, abortar (rollback) con detalle.
3. Mergear: `gh pr merge <N> --repo <OWNER>/<REPO> --<MERGE_TYPE>`. Si falla, abortar (rollback).
4. Para cada issue en `<LINKED_ISSUES>`: `gh issue close <num> --repo <OWNER>/<REPO>`. Loggear si alguno falla (no rollback — el merge ya pasó).

Output requerido (markdown estricto):

## Result
- merged: true|false
- merge_url: <URL si paso, vacio si no>
- closed_issues: [<lista de numbers cerrados OK>]
- failed_issues: [<lista de numbers que fallaron>]
- ci_wait_seconds: <segundos esperados>

## Errors
<lista de errores con contexto, o "ninguno">
```

Lanzar:
```
Agent(
  subagent_type: "general-purpose",
  model: "opus",
  description: "close PR #<N>",
  prompt: "Lee /tmp/cvm-close-<N>-context.md y ejecuta exactamente los pasos descritos. NO modifiques codigo. Solo gh CLI calls. Reporta el resultado en el formato especificado."
)
```

### Paso 5: Post-transition (success o rollback)

Parsear el output del subagent buscando `merged: true|false`.

**Si `merged: true`:**
1. Aplicar `che:closing→che:closed` en el PR:
   ```bash
   gh label create "che:closed" --force 2>/dev/null
   gh api -X DELETE "repos/$OWNER/$REPO/issues/$N/labels/che:closing" 2>/dev/null
   gh api -X POST "repos/$OWNER/$REPO/issues/$N/labels" -f "labels[]=che:closed"
   ```
2. Para cada issue cerrado OK (`closed_issues`): aplicar transition al state final segun su current state:
   - Leer labels actuales del issue
   - Si tenia `che:validated` → remover, agregar `che:closed`
   - Else (lenient) → remover todos los `che:*`, agregar `che:closed`

**Si `merged: false` (rollback):**
1. Restaurar `PREVIOUS_STATE` en el PR:
   ```bash
   gh api -X DELETE "repos/$OWNER/$REPO/issues/$N/labels/che:closing" 2>/dev/null
   if [[ -n "$PREVIOUS_STATE" ]]; then
     gh api -X POST "repos/$OWNER/$REPO/issues/$N/labels" -f "labels[]=$PREVIOUS_STATE"
   fi
   ```
2. NO tocar issues linkeados (no se cerraron).

### Paso 6: Reportar

Mostrar al usuario:

```
Close de PR #<N> <completado|fallido>.

- Merged: <true|false> (<merge_url si aplica>)
- Issues cerrados: <lista>
- Issues fallidos: <lista o "ninguno">
- CI wait: <segundos>s
- Estado final del PR: <che:closed | che:executed | che:validated>

Errores (si hubo): <lista>
```

No ejecutar `/r` automaticamente.

## MUST DO
- Validar `N` con `^[0-9]+$` ANTES de interpolarlo en cualquier comando shell.
- Validar `--merge-type` contra `{squash, merge, rebase}` antes de usarlo.
- Pre-check `gh auth status` una vez.
- Resolver state via stateref (PR labels → linked issue labels → vacio).
- Lenient: si `CURRENT_STATE` no es `che:executed` ni `che:validated`, warnear + limpiar `che:*` previos + aplicar `che:closing`.
- Pasar el path del contexto al subagent — NUNCA inline.
- Subagent solo hace `gh` calls, NO modifica codigo.
- Timeout CI 15min con poll cada 30s.
- Tolerar exit code 8 de `gh pr checks` (significa "hay checks rojos", el JSON sigue siendo valido).
- Aplicar `che:closing→che:closed` solo si `merged: true`.
- Cerrar issues linkeados con transition correspondiente (preferir `validated→closed`, fallback lenient `→closed`).
- Rollback (`closing→<previous>`) si `merged: false`. NO tocar issues linkeados en rollback.
- Usar `gh api` REST (no `gh issue edit --add-label`) — REST evita scope `read:org` que `gh auth login` no entrega por default.

## MUST NOT DO
- No interpolar `$ARGUMENTS` ni texto libre en double-quoted shell commands.
- No mergear sin esperar CI verde (excepto si subagent decide que CI no aplica — pero default: esperar).
- No mergear si `IS_DRAFT=true` y `gh pr ready` falla.
- No saltar la pre-transition `→che:closing` (es el lock optimista del workflow).
- No persistir nada en auto-memory (el resultado vive en GitHub).
- No soportar GitLab/Bitbucket — solo GitHub via `gh`.
- No correr `/r` al final.
- No delegar a `/go`, `/che-iterate`, etc — el skill maneja todo el flow con un solo subagent Opus.
- No bloquear el rollback en errores de label REST: `DELETE` debe tolerar 404 (label no existe en el repo o no aplicado al PR).

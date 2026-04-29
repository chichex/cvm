Automatiza el ciclo `che-validate → che-iterate → che-validate → ...` sobre un PR o issue hasta que el verdict consolidado sea `approve`, no queden comments accionables, se alcance `--max N` iteraciones, o se detecte idempotencia (mismos comments dos iteraciones seguidas). `$ARGUMENTS` acepta el mismo formato que `/che-validate` y `/che-iterate`: numero (`24`), URL completa, `pr 24`, `issue 24`. Flag opcional `--max N` (default `3`). El skill **no toca labels `che:*` directamente** — eso lo hacen los skills internos via composicion (Skill tool).

## Composicion (no reimplementar)

Este skill es un orquestador puro. Invoca via Skill tool a:

- `/che-validate` — para revisar y obtener verdict + labels `validated:*` / `plan-validated:*`.
- `/che-iterate` — para aplicar comments accionables y persistir (PR: commit+push; issue: `gh issue edit --body-file`).

NO duplica logica de fetch de comments, parseo de verdict, transitions de state machine, ni filtrado. Si algo del flujo `validate-iterate` cambia, se cambia en los skills internos, no aca.

## Default `--max=3`

Justificacion: en la practica, 3 iteraciones cubren los casos comunes (verdict `changes-requested` → fix → `approve`, con un buffer extra por si el primer fix es incompleto). Mas que eso suele indicar idempotencia (mismo comment vuelve) o un trade-off arquitectural que requiere humano. Mantener bajo evita gastar tokens y minutos de subagentes en loops poco productivos. Override con `--max N` si la tarea lo amerita.

## Proceso

### Paso 1: Parsear `$ARGUMENTS`

Aceptar los mismos formatos que `/che-validate` y `/che-iterate`. Detectar y separar:

- `--max N` (donde `N` matchea `^[0-9]+$` y `1 ≤ N ≤ 20`): si no matchea o esta fuera de rango, abortar: "Uso: `/che-loop <N | URL | pr N | issue N> [--max <1-20>]`". Default `MAX=3`.
- Resto del input → `TARGET_INPUT` (lo que se pasa tal cual a `/che-validate` y `/che-iterate`).

Si `TARGET_INPUT` queda vacio tras quitar el flag, abortar con mensaje de uso.

**Validar regex `^[0-9]+$` para `N`** ANTES de pasarlo a cualquier comando shell. NUNCA interpolar `$ARGUMENTS` crudo en double-quoted shell.

### Paso 2: Resolver target una vez (KIND + N + OWNER/REPO)

Replicar el parseo de `/che-validate`/`/che-iterate` aca para evitar re-resolverlo en cada iteracion:

1. **Precheck auth**: `gh auth status` una vez. Si falla, abortar: "gh no esta autenticado o no hay red — corre `gh auth login` y reintenta.".
2. Parsear `TARGET_INPUT`:
   - URL `https://github.com/<owner>/<repo>/(pull|issues)/<N>` → split por `/` localmente. NO interpolar la URL en shell.
   - `pr N` / `issue N` (case-insensitive) → forzar KIND, `N` validado por regex.
   - `^[0-9]+$` → `N`, KIND a detectar via fallback `gh pr view "$N"` → `gh issue view "$N"` (con `>/dev/null 2>&1` y separando exit code de stdout, igual que `/che-validate`).
   - Cualquier otra cosa → abortar con uso.
3. Si no vino URL, resolver `OWNER/REPO`:
   ```bash
   gh repo view --json owner,name --jq '"\(.owner.login)/\(.name)"' 2>/dev/null
   ```
4. Para `KIND=pr`: capturar `HEAD_REF` (`gh pr view "$N" $REPO_FLAG --json headRefName`) — se usa en Paso 4 (precheck PR) y para detectar push fallido.

Guardar: `KIND` (`pr` | `issue`), `N`, `OWNER`, `REPO`, `HEAD_REF` (solo PR), `TARGET_INPUT_FOR_SKILLS` (string que se pasa intacto a `/che-validate` / `/che-iterate`; conviene canonicalizar a `pr N` o `issue N` para que los skills internos no tengan que volver a inferir KIND).

### Paso 3: Precheck especifico para PR (working tree)

Si `KIND=pr`:

```bash
CURRENT_BRANCH=$(git branch --show-current)
if [[ "$CURRENT_BRANCH" != "$HEAD_REF" ]]; then
  echo "Branch local '$CURRENT_BRANCH' != headRefName del PR '$HEAD_REF'."
  echo "Hace 'git checkout $HEAD_REF' (o 'gh pr checkout $N') antes de /che-loop."
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Working tree no limpio. Stashea o commitea antes de /che-loop."
  exit 1
fi
```

`/che-iterate` ya hace este check internamente y aborta — pero al fallar dentro del primer `/che-iterate` ya gastamos un `/che-validate`. Hacerlo arriba evita ese gasto.

Para `KIND=issue` no hace falta — `/che-iterate` solo edita el body via `gh issue edit`.

### Paso 4: Loop principal

Inicializar:

```
ITER=0
PREV_COMMENT_IDS=()      # set de IDs de la iteracion anterior (para idempotencia)
LAST_VERDICT=""
LAST_VALIDATE_URLS=()    # URLs de los comments posteados por el ultimo che-validate
EXIT_REASON=""           # se llena al salir
```

Mientras `ITER < MAX`:

#### 4.1 Incrementar y loggear

```
ITER=$((ITER + 1))
echo "=== /che-loop iteracion $ITER de $MAX ==="
```

#### 4.2 Invocar `/che-validate`

Via Skill tool: `Skill(skill: "che-validate", args: "$TARGET_INPUT_FOR_SKILLS")`.

`/che-validate` se encarga de:
- Lock `che:*ing` → fetch contexto → lanzar agentes → postear comments → consolidar verdict → aplicar `che:validated` + `<ns>:<verdict>`.

Esperar a que termine. Si falla catastroficamente (todos los agentes failed), `/che-validate` ya hizo rollback del lock; en ese caso terminar el loop con `EXIT_REASON="validate-failed"` y reportar.

Para que el orquestador del che-loop sepa el verdict aplicado, **usar el agente default de `/che-validate`** (Opus) leyendo despues el label aplicado:

```bash
# Determinar namespace
if [[ "$KIND" == "pr" ]]; then NS=validated; else NS=plan-validated; fi

# Leer labels actuales
LABELS=$(gh api "repos/$OWNER/$REPO/issues/$N/labels" --jq '.[].name')

VERDICT=""
for v in approve changes-requested needs-human; do
  if echo "$LABELS" | grep -qx "$NS:$v"; then VERDICT="$v"; break; fi
done
```

Si `VERDICT` queda vacio (rollback de `/che-validate`), terminar con `EXIT_REASON="validate-no-verdict"`.

Guardar `LAST_VERDICT=$VERDICT`.

#### 4.3 Capturar URLs de las reviews recien posteadas

Para el reporte final:

```bash
# Comments posteados en la iteracion: filtrar por timestamp posterior al inicio de la iteracion
ITER_START_ISO=<timestamp ISO al inicio del 4.2>
gh api --paginate "repos/$OWNER/$REPO/issues/$N/comments" \
  --jq ".[] | select(.created_at >= \"$ITER_START_ISO\") | .html_url"
```

Guardar en `LAST_VALIDATE_URLS`.

#### 4.4 Exit por verdict approve

Si `VERDICT == approve`:
- `EXIT_REASON="approved"`
- Salir del loop.

#### 4.5 Exit por verdict needs-human

Si `VERDICT == needs-human`:
- `EXIT_REASON="needs-human"`
- Salir del loop. No iteramos: `/che-iterate` no resuelve trade-offs que requieren decision humana.

#### 4.6 Caso `changes-requested`: detectar idempotencia

Antes de invocar `/che-iterate`, fetch los comments accionables que `/che-iterate` veria y compararlos con la iteracion anterior:

```bash
# Mismos endpoints que /che-iterate Paso 2:
gh api --paginate "repos/$OWNER/$REPO/issues/$N/comments" --jq '.[].id' > /tmp/cvm-loop-issue-ids.txt
if [[ "$KIND" == "pr" ]]; then
  gh api --paginate "repos/$OWNER/$REPO/pulls/$N/comments" --jq '.[].id' >> /tmp/cvm-loop-issue-ids.txt
  gh api --paginate "repos/$OWNER/$REPO/pulls/$N/reviews" --jq '.[].id' >> /tmp/cvm-loop-issue-ids.txt
fi
sort -u /tmp/cvm-loop-issue-ids.txt > /tmp/cvm-loop-current-ids.txt
```

Aplicar el **mismo filtro barato** que `/che-iterate` Paso 3 (autor del PR/issue, reacciones puras, reviews APPROVED vacios). Como esto requiere mas que `id` (necesita `user.login`, `body`, `state`), en la practica fetch el JSON completo y filtrarlo localmente con `jq`. Para mantener este skill simple, **delegar el filtrado a la salida de `/che-iterate`** y comparar **antes** vs **despues**:

Alternativa simpler (preferida): comparar los `id`s de comments **crudos** (sin filtrar). Si son iguales que la iteracion anterior, asumir idempotencia. Esto puede dar falsos positivos cuando un comment nuevo es 100% reaccional (ruido) — aceptable, porque el caso comun donde queremos detectar idempotencia es el reviewer dejando los mismos comentarios accionables sin que el commit los resuelva.

```bash
CURRENT_IDS=$(cat /tmp/cvm-loop-current-ids.txt)

if [[ "$ITER" -gt 1 && "$CURRENT_IDS" == "$PREV_COMMENT_IDS" ]]; then
  EXIT_REASON="idempotent"
  break
fi

PREV_COMMENT_IDS="$CURRENT_IDS"
```

#### 4.7 Exit por 0 comments accionables

Si `/che-iterate` no encontraria nada accionable (todos filtrados), no tiene sentido invocarlo. Detectarlo aca es costoso (replicar el filtrado). Estrategia mas simple: invocar `/che-iterate` igual y dejar que el reporte indique 0 aplicados → tratarlo como exit reason en 4.9.

#### 4.8 Invocar `/che-iterate`

Via Skill tool: `Skill(skill: "che-iterate", args: "$TARGET_INPUT_FOR_SKILLS")`.

`/che-iterate` se encarga de:
- Lock `che:*ing` → fetch + filtrar comments → lanzar agente Opus → editar codigo (PR) o body (issue) → **persistir** (PR: `git commit + git push`; issue: `gh issue edit --body-file`) → aplicar `che:executed` / `che:plan` (success) o rollback.

Esperar a que termine.

#### 4.9 Detectar 0 aplicados / fallo de iterate

Leer el reporte de `/che-iterate`. Si el resumen indica:

- "0 aplicados" / "no hay comments accionables" → `EXIT_REASON="no-actionable"`, salir.
- Falla de commit/push (PR) o `gh issue edit` (issue) → `EXIT_REASON="iterate-persist-failed"`, salir. El humano resuelve a mano.
- Fallo del subagent → `EXIT_REASON="iterate-failed"`, salir.

Si `/che-iterate` fue exitoso (≥1 aplicado y persistido), volver al inicio del loop (Paso 4.1) — el siguiente `/che-validate` ya vera el commit nuevo (PR) o el body actualizado (issue) en GitHub.

#### 4.10 Exit por `--max` alcanzado

Si el `while` sale por `ITER == MAX` sin haber matcheado approve/needs-human/idempotent/no-actionable, `EXIT_REASON="max-iter"`.

### Paso 5: Reporte final

Mostrar al usuario:

```
/che-loop sobre <KIND> #<N> finalizado.

Iteraciones ejecutadas: <ITER> de <MAX>
Verdict final: <LAST_VERDICT o "(sin verdict)">
Razon de salida: <EXIT_REASON>
  - approved: el ultimo che-validate retorno verdict=approve.
  - needs-human: subagent emitio needs-human; revisar a mano.
  - changes-requested + max-iter: se acabaron las iteraciones con cambios pendientes.
  - idempotent: dos iteraciones consecutivas con el mismo set de comments. Posible loop infinito — humano requerido.
  - no-actionable: che-iterate no encontro nada accionable tras filtrado.
  - iterate-persist-failed: commit/push o gh issue edit fallo. Reintenta a mano.
  - iterate-failed / validate-failed / validate-no-verdict: skill interno fallo. Ver logs arriba.

URLs de la ultima review:
- <LAST_VALIDATE_URLS[0]>
- ...

Recomendacion:
  - approved → listo para `/che-close <N>` (si KIND=pr).
  - needs-human / idempotent → revisar comments a mano y decidir.
  - max-iter / changes-requested → re-correr `/che-loop <N> --max N` con mas iteraciones, o `/che-iterate` manual.
  - iterate-persist-failed → resolver el git/gh issue edit y reintentar.
```

No invocar `/r` automaticamente — cada `/che-validate` y `/che-iterate` interno ya decide su propia persistencia de learnings (en el caso de `/che-iterate`, llama a `/r` al final). El loop en si no genera learnings nuevos sobre el codebase.

## MUST DO

- Componer via Skill tool: invocar `/che-validate` y `/che-iterate`. NO duplicar logica de fetch de comments, filtrado, locks, transitions, ni parseo de verdict.
- Parsear `--max N` con `^[0-9]+$` y rango `1 ≤ N ≤ 20`. Default `3`.
- Validar `N` (numero del target) con `^[0-9]+$` ANTES de pasarlo a cualquier comando shell.
- **Precheck PR**: si `KIND=pr`, verificar branch local == `HEAD_REF` y working tree limpio ANTES del primer `/che-validate` (evita gastar una validate cuando `/che-iterate` va a abortar igual).
- Detectar `KIND` una sola vez (Paso 2) y reusarlo. No re-resolver en cada iteracion.
- Leer el verdict de `/che-validate` desde los labels `validated:<v>` / `plan-validated:<v>` aplicados en GitHub (no parsear stdout del subagent — esos labels son el contrato persistido).
- Detectar idempotencia comparando los `id`s de comments entre iteraciones consecutivas. Si son iguales tras `/che-iterate`, abortar con `EXIT_REASON=idempotent`.
- Salir inmediatamente si `verdict=needs-human` (no iterar — requiere humano).
- Salir si `/che-iterate` reporta 0 aplicados (no tiene sentido re-validar sin cambios).
- Salir si `/che-iterate` falla en persistir (commit/push o `gh issue edit`) — el lock ya lo rolleo `/che-iterate`; reportar al humano para que resuelva a mano.
- Reportar al final: iteraciones ejecutadas, verdict final, exit reason, URLs de la ultima review, recomendacion accionable.
- Tolerar exit code 8 de `gh pr checks` si en algun momento se pollea CI (no aplica directamente aca, pero aplica si se compone con `/che-close` en el futuro).
- Usar `gh api` REST para cualquier lectura de labels (mismo gotcha que el resto de los skills che-*).

## MUST NOT DO

- **No tocar labels `che:*` directamente.** Lo hacen `/che-validate` y `/che-iterate` internamente. El loop solo lee labels post-validate para extraer verdict.
- No aplicar locks ni transitions propias del loop (`che:looping` no existe en la state machine de che-cli).
- No commitear, no pushear, no editar bodies de issues directamente. `/che-iterate` lo hace.
- No interpolar `$ARGUMENTS` ni el target del usuario en double-quoted shell. Pasar via Skill tool args (string) que el skill interno parsea con sus propias garantias.
- No duplicar el filtrado de comments de `/che-iterate` Paso 3 — comparar idempotencia sobre IDs crudos es suficiente (acepta falsos positivos en favor de simplicidad).
- No reintentar `/che-iterate` si fallo en persistir. El working tree puede quedar en estado raro; humano resuelve.
- No correr el loop con `MAX > 20` — riesgo de quemar tokens/minutos en algo que un humano deberia mirar despues de 3-5.
- No invocar `/r` al final del loop. Los skills internos ya manejan su propia persistencia.
- No soportar GitLab/Bitbucket — solo GitHub via `gh`, igual que los hermanos.
- No correr en paralelo `/che-validate` y `/che-iterate` — el loop es estrictamente secuencial (cada validate depende del commit/body del iterate anterior).

## Notas de diseno

- **Por que leer verdict de labels y no del stdout del Skill tool**: el Skill tool devuelve la salida del skill al orquestador, pero esa salida es markdown libre. Los labels `validated:*` / `plan-validated:*` son el contrato persistido en GitHub que `/che-validate` aplica antes de retornar — leerlos es deterministico y resiliente a cambios de formato del reporte.
- **Por que comparar IDs crudos para idempotencia**: replicar el filtrado de `/che-iterate` Paso 3 aca acoplaria el loop a la implementacion interna del iterate. Comparar IDs crudos da false positives (ej: alguien dejo un `+1` nuevo entre iteraciones) que en la practica son raros y conservadores (preferimos abortar de mas que loopear de menos).
- **Por que no soportar `KIND=issue` sin verdict ciclo**: para issues, `/che-iterate` reescribe el body — el siguiente `/che-validate` valida el body nuevo y si los reviewers dejaron mas comments, los aplica. El ciclo es identico al de PR conceptualmente; solo cambia el medio (body vs codigo).
- **Por que `--max=3` y no `--max=5`**: 3 cubre el caso "primera review pidio cambios → fix → segunda review aprueba" con un buffer extra. 5 ya es zona donde la idempotencia o el needs-human son mas probables — preferimos cortar antes y que el humano decida. Override explicito si la tarea lo necesita.

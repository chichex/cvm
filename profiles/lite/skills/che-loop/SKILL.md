Automatiza el ciclo `che-validate → che-iterate → che-validate → ...` sobre un PR o issue hasta que el verdict consolidado sea `approve`, no queden comments accionables, se alcance `--max N` iteraciones, o se detecte idempotencia (fingerprint del feedback humano accionable repetido entre iteraciones). `$ARGUMENTS` acepta el mismo formato que `/che-validate` y `/che-iterate`: numero (`24`), URL completa, `pr 24`, `issue 24`. Flag opcional `--max N` (default `3`). El skill **no toca labels `che:*` directamente** — eso lo hacen los skills internos via composicion (Skill tool).

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

Guardar: `KIND` (`pr` | `issue`), `N`, `OWNER`, `REPO`, `HEAD_REF` (solo PR), `TARGET_INPUT_FOR_SKILLS` (string que se pasa intacto a `/che-validate` / `/che-iterate`).

**Reglas para `TARGET_INPUT_FOR_SKILLS`** (importante para soportar cross-repo):

- Si el input vino como **URL** completa (`https://github.com/<owner>/<repo>/(pull|issues)/<N>`): **mantener la URL tal cual**. No canonicalizar a `pr N` / `issue N` — los skills internos solo preservan el repo ajeno cuando reciben la URL completa; con `pr N` re-resuelven `gh repo view` y caen en el repo actual del cwd, que NO es necesariamente el target.
- Si el input vino como `pr N` / `issue N` o numero puro: detectar si `OWNER/REPO` resuelto coincide con el repo del cwd actual (`gh repo view --json owner,name`):
  - Coincide → canonicalizar a `pr N` / `issue N` (mas legible para los skills internos).
  - No coincide (caso raro: `gh` apuntando a otro repo via env/flags) → usar la URL `https://github.com/<OWNER>/<REPO>/(pull|issues)/<N>` para preservar el contexto.

En resumen: el orquestador **nunca degrada** una URL cross-repo a forma corta. Esto evita que `/che-validate`/`/che-iterate` operen contra el repo equivocado.

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

> **Duplicado deliberado**: este precheck replica `/che-iterate` Paso 4.6. Si el contrato de `/che-iterate` cambia (ej: aprende a `git stash` automaticamente, deja de exigir branch == `headRefName`), sincronizar este Paso 3 a mano. La duplicacion es un trade-off explicito (DRY vs short-circuit antes de gastar una validate); preferimos short-circuit.

Para `KIND=issue` no hace falta — `/che-iterate` solo edita el body via `gh issue edit`.

### Paso 4: Loop principal

Inicializar:

```
ITER=0
PREV_FP=""               # fingerprint (sha) del feedback accionable de la iter anterior, para idempotencia
LAST_VERDICT=""
LAST_VALIDATE_URLS=()    # URLs de los comments posteados por el ultimo che-validate
EXIT_REASON=""           # se llena al salir
LOOP_TMP=$(mktemp -d -t cvm-loop-XXXXXX)   # paths del loop, namespaced por PID — no choquea con runs concurrentes
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

> **Convencion de interactividad**: `/che-validate` puede pedir al usuario que elija agentes (prompt `[O/c/g/a]`). Cuando se invoca desde `/che-loop`, **el orquestador del loop responde automaticamente con el default (Opus solo)** — no detiene la ejecucion para preguntar. Esto se logra dejando que el Skill tool propague la respuesta vacia / aceptando default. Si en el futuro se quiere un agente distinto en cada iteracion, agregar una flag al loop (ej `--agents O,c`) y forwardearlo al validate; por ahora, todos los validates dentro de un loop usan Opus.

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

Antes de invocar `/che-iterate`, computar un fingerprint **estable** del feedback accionable y compararlo con el de la iteracion anterior. Detalles importantes:

- **Los IDs crudos NO sirven**: `/che-validate` postea comments nuevos en cada iteracion del loop (header `## Review: <agent>`), asi que el set de IDs cambia siempre, aunque el feedback semantico del reviewer humano sea identico. Comparar IDs nunca dispararia idempotencia en el caso real.
- **Senal correcta: hash del body de los comments accionables, excluyendo los del propio `/che-validate`.** Filtramos los comments cuyo body (despues de strip de whitespace al inicio) empieza con `## Review:` — esos los genera `/che-validate` y son los que cambian entre iteraciones. Lo que queda es el feedback del reviewer humano (o de otros bots), que es lo que iterate va a procesar.

Usamos `$LOOP_TMP` (creado al inicializar el loop con `mktemp -d`) — los paths quedan **namespaced por PID** y no chocan con runs concurrentes (a diferencia de los `/tmp/cvm-loop-*.txt` globales que tenia la version anterior).

```bash
FINGERPRINT_FILE="$LOOP_TMP/fp-iter-$ITER.txt"
```

Fetch el **JSON completo** (no solo `.id`) de los tres endpoints:

```bash
gh api --paginate "repos/$OWNER/$REPO/issues/$N/comments" > "$LOOP_TMP/issue-comments.json"
if [[ "$KIND" == "pr" ]]; then
  gh api --paginate "repos/$OWNER/$REPO/pulls/$N/comments" > "$LOOP_TMP/pr-comments.json"
  gh api --paginate "repos/$OWNER/$REPO/pulls/$N/reviews"  > "$LOOP_TMP/pr-reviews.json"
fi
```

Computar fingerprint con `jq`: filtrar los comments con body que empieza por `## Review:` (machine-generated por `/che-validate`), tomar el body crudo del resto, ordenar y hashear:

```bash
# jq filter: el body crudo si NO empieza con "## Review:" (tras strip), vacio si si empieza
JQ_FILTER='.[] | (.body // "") | select((. | ltrimstr(" ") | ltrimstr("\n") | ltrimstr("\t") | startswith("## Review:")) | not)'

{
  jq -r "$JQ_FILTER" "$LOOP_TMP/issue-comments.json"
  if [[ "$KIND" == "pr" ]]; then
    jq -r "$JQ_FILTER" "$LOOP_TMP/pr-comments.json"
    jq -r "$JQ_FILTER" "$LOOP_TMP/pr-reviews.json"
  fi
} | sort > "$FINGERPRINT_FILE"

CURRENT_FP=$(shasum "$FINGERPRINT_FILE" | awk '{print $1}')

if [[ "$ITER" -gt 1 && "$CURRENT_FP" == "$PREV_FP" ]]; then
  EXIT_REASON="idempotent"
  break
fi

PREV_FP="$CURRENT_FP"
```

Esto da: false positives bajos (solo si dos sets distintos hashearan al mismo valor — extremadamente raro con SHA-1) y cubre el caso real (reviewer humano dejando los mismos comments accionables sin que el commit los resuelva). El loop sigue siendo conservador (preferimos abortar de mas que loopear de menos), pero ahora la senal **si dispara**.

> Nota: el filtro `## Review:` aca es el mismo que `/che-iterate` Paso 3 usa para preservar reviews machine-generated. Aca lo invertimos (las excluimos del fingerprint) porque queremos detectar si el feedback **del reviewer**, no del propio loop, se repite. Si el contrato del header `## Review:` cambia en `/che-validate`, sincronizar aca tambien.

#### 4.7 Exit por 0 comments accionables

Si `/che-iterate` no encontraria nada accionable (todos filtrados), no tiene sentido invocarlo. Detectarlo aca es costoso (replicar el filtrado). Estrategia mas simple: invocar `/che-iterate` igual y dejar que el reporte indique 0 aplicados → tratarlo como exit reason en 4.9.

#### 4.8 Invocar `/che-iterate`

Via Skill tool: `Skill(skill: "che-iterate", args: "$TARGET_INPUT_FOR_SKILLS")`.

`/che-iterate` se encarga de:
- Lock `che:*ing` → fetch + filtrar comments → lanzar agente Opus → editar codigo (PR) o body (issue) → **persistir** (PR: `git commit + git push`; issue: `gh issue edit --body-file`) → aplicar `che:executed` / `che:plan` (success) o rollback.

Esperar a que termine.

#### 4.9 Detectar 0 aplicados / fallo de iterate

Leer el reporte de `/che-iterate` parseando el contrato formalizado en su `## Contract` section. Las dos senales canonicas son:

1. **Linea `Procesados por el agente: <Z>`** — `Z` es entero. Lo extraemos con un grep simple (no parse de markdown):
   ```bash
   ITERATE_OUTPUT=<stdout del Skill tool>
   APPLIED=$(echo "$ITERATE_OUTPUT" | grep -E '^- Procesados por el agente: [0-9]+$' | tail -1 | grep -oE '[0-9]+$')
   ```
   - `APPLIED == 0` → `EXIT_REASON="no-actionable"`, salir.
   - `APPLIED >= 1` → continuar al chequeo de persistencia.
   - `APPLIED` vacio (no se encontro la linea — contrato roto) → `EXIT_REASON="iterate-failed"`, salir y warnear al humano.

2. **Linea `Persistencia: ...`** — si arranca con `failed:` (segun el contrato), la persistencia fallo aunque haya cambios:
   ```bash
   if echo "$ITERATE_OUTPUT" | grep -qE '^- Persistencia: failed:'; then
     EXIT_REASON="iterate-persist-failed"
     break
   fi
   ```

Si `APPLIED >= 1` y `Persistencia` no es `failed:`, `/che-iterate` fue exitoso → volver al inicio del loop (Paso 4.1). El siguiente `/che-validate` vera el commit nuevo (PR) o el body actualizado (issue) en GitHub (el contrato garantiza persistencia previa al return).

> **Por que parsear contrato en vez de frases libres**: la version anterior buscaba "0 aplicados" / "no hay comments accionables" en el resumen markdown. Eso es fragil: cualquier cambio de wording en `/che-iterate` (refactor, traduccion, etc) rompe el loop silenciosamente. El bloque `## Contract` de `/che-iterate` documenta los strings exactos como contrato estable y obliga a sincronizar consumers en cualquier breaking change.

#### 4.10 Exit por `--max` alcanzado

Si el `while` sale por `ITER == MAX` sin haber matcheado approve/needs-human/idempotent/no-actionable, `EXIT_REASON="max-iter"`.

### Paso 5: Reporte final

Antes de imprimir, leer el `che:*` actual del target para reportar state final accurate (especialmente util en `max-iter`):

```bash
FINAL_STATE=$(gh api "repos/$OWNER/$REPO/issues/$N/labels" --jq '.[].name' | grep -E '^che:' | head -1)
```

Mostrar al usuario:

```
/che-loop sobre <KIND> #<N> finalizado.

Iteraciones ejecutadas: <ITER> de <MAX>
Verdict final: <LAST_VERDICT o "(sin verdict)">
Estado final: <FINAL_STATE o "(sin che:*)">
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
- Detectar idempotencia hasheando el body de los comments accionables (excluyendo los machine-generated por `/che-validate` con header `## Review:`) entre iteraciones consecutivas. Si el fingerprint repite, abortar con `EXIT_REASON=idempotent`.
- Mantener URLs cross-repo intactas en `TARGET_INPUT_FOR_SKILLS`: nunca degradar una URL completa a `pr N` / `issue N` cuando el repo del target no es el del cwd.
- Usar `mktemp -d` para los archivos temporales del loop. NUNCA paths globales tipo `/tmp/cvm-loop-*.txt` (corrompen runs concurrentes).
- Parsear el output de `/che-iterate` segun su `## Contract` section (linea `Procesados por el agente: <Z>` y `Persistencia: ...`). NUNCA buscar frases libres como "0 aplicados" en el resumen markdown.
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
- No duplicar el filtrado completo de comments de `/che-iterate` Paso 3 — para idempotencia alcanza con hashear bodies excluyendo los `## Review:` (es decir, lo que el reviewer humano dijo, no lo que el propio loop posteo).
- No buscar frases libres ("0 aplicados", "no hay comments accionables") en el output de `/che-iterate`. Usar el contrato formal (`## Contract` section).
- No reintentar `/che-iterate` si fallo en persistir. El working tree puede quedar en estado raro; humano resuelve.
- No correr el loop con `MAX > 20` — riesgo de quemar tokens/minutos en algo que un humano deberia mirar despues de 3-5.
- No invocar `/r` al final del loop. Los skills internos ya manejan su propia persistencia.
- No soportar GitLab/Bitbucket — solo GitHub via `gh`, igual que los hermanos.
- No correr en paralelo `/che-validate` y `/che-iterate` — el loop es estrictamente secuencial (cada validate depende del commit/body del iterate anterior).
- **No asumir aislamiento entre iteraciones del loop.** Entre la salida de `/che-iterate` (`che:executed` / `che:plan`) y la entrada del proximo `/che-validate` (`che:executed→che:validating`), hay una ventana sin lock. Un humano puede correr `/che-validate` o `/che-iterate` manualmente justo en esa ventana sin que el loop se entere. El peor caso es que el loop detecte una iteracion extra via idempotencia (mismo fingerprint) y salga limpio — no hay corrupcion de estado, solo una iteracion gastada de mas.

## Notas de diseno

- **Por que leer verdict de labels y no del stdout del Skill tool**: el Skill tool devuelve la salida del skill al orquestador, pero esa salida es markdown libre. Los labels `validated:*` / `plan-validated:*` son el contrato persistido en GitHub que `/che-validate` aplica antes de retornar — leerlos es deterministico y resiliente a cambios de formato del reporte.
- **Por que hashear bodies (no IDs) para idempotencia**: `/che-validate` postea comments nuevos en cada iteracion, asi que los IDs cambian aunque el feedback semantico sea identico. Comparar IDs nunca dispararia. Hashear el body de los comments accionables (excluyendo los `## Review:` del propio loop) detecta el caso real: el reviewer dejo los mismos comentarios accionables y el commit no los resolvio. False positives son extremadamente raros (colision de SHA-1).
- **Por que no soportar `KIND=issue` sin verdict ciclo**: para issues, `/che-iterate` reescribe el body — el siguiente `/che-validate` valida el body nuevo y si los reviewers dejaron mas comments, los aplica. El ciclo es identico al de PR conceptualmente; solo cambia el medio (body vs codigo).
- **Por que `--max=3` y no `--max=5`**: 3 cubre el caso "primera review pidio cambios → fix → segunda review aprueba" con un buffer extra. 5 ya es zona donde la idempotencia o el needs-human son mas probables — preferimos cortar antes y que el humano decida. Override explicito si la tarea lo necesita.
- **Por que existe en lite y no en che-cli**: che-cli es un CLI con humans-in-the-loop deliberado entre cada step (`che validate`, `che iterate` corren en shells humanas). El profile lite corre dentro de una sesion Claude Code donde el agente humano ya esta presente y dispuesto a delegar tareas batch. El loop tiene sentido aca porque el costo marginal de una iteracion es bajo (subagent paralelo) y el costo de espera humana entre `validate→iterate→validate` es alto (context switch). En CLI, "loop" = humano corriendo dos comandos, no hace falta automatizar; en lite, "loop" = humano esperando subagentes, automatizar paga. Por eso `che:looping` no existe en `che-cli/internal/labels/labels.go` y este skill no lo introduce — control flow del orquestador, no estado del issue/PR.
- **Estado final post-loop (segun exit reason)**:
  - `EXIT_REASON=approved` → state final `che:validated` (el ultimo paso fue `/che-validate` que aplico approve y no se invoco iterate).
  - `EXIT_REASON=needs-human` → state final `che:validated` con verdict `needs-human` (igual: ultimo paso fue validate).
  - `EXIT_REASON=no-actionable` → state final `che:validated` (ultimo paso fue iterate con 0 aplicados → rollback).
  - `EXIT_REASON=idempotent` → state final `che:validated` (idempotencia se detecta antes de invocar iterate).
  - `EXIT_REASON=max-iter` → state final depende: si la ultima iteracion del while fue iterate exitoso, queda `che:executed` (PR) / `che:plan` (issue); si fue validate sin iterate, queda `che:validated`. **Reportar el state final explicito en el Paso 5 leyendo el label actual antes de salir.**
  - `EXIT_REASON=iterate-persist-failed` / `iterate-failed` / `validate-failed` / `validate-no-verdict` → state final segun el rollback que el skill interno haya aplicado (tipicamente `che:validated`).

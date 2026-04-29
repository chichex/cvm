Aplica comments/reviews de un PR o issue lanzando un agente Opus con el contexto consolidado. `$ARGUMENTS` puede ser un numero (`/che-iterate 42`), una URL (`/che-iterate https://github.com/owner/repo/pull/42`), o vacio (`/che-iterate` usa la branch actual). El skill NO commitea — deja los cambios en el working tree para que el humano los revise. Aplica las transitions de la state machine `che:*` de che-cli en modo lenient (ver "Tagging" abajo).

## Tagging (state machine de che-cli)

Aplica las transitions de `che-cli/internal/labels/labels.go`:

| Target | Pre (lock) | Success | Rollback |
|---|---|---|---|
| PR (`che:validated`) | `validated→executing` | `executing→executed` | `executing→validated` |
| issue (`che:validated`) | `validated→planning` | `planning→plan` | `planning→validated` |

**Stateref**: para PR, si NO tiene label `che:*`, leer `closingIssuesReferences` y usar el `che:*` del issue linkeado. Si tampoco hay → `CURRENT_STATE=""` (lenient, warnear).

**Lenient**: si `CURRENT_STATE` no es `che:validated`, warn al usuario y aplicar el lock igual (limpiar `che:*` previos, aplicar `che:executing` o `che:planning`).

## Proceso

### Paso 1: Parsear `$ARGUMENTS` y resolver target

**Precheck de auth** (antes de cualquier `gh` que haga red): correr `gh auth status` una sola vez. Si falla, abortar inmediatamente con: "gh no esta autenticado o no hay red — corre `gh auth login` y reintenta.". Esto evita confundir un "not found" real con un fallo de auth/red en los pasos siguientes.

Detectar el formato:

1. **Vacio** → usar la branch actual. Resolver el PR asociado:
   ```bash
   gh pr view --json number,headRefName,author,title,url,baseRefName 2>/dev/null
   ```
   Si falla, abortar: "No hay PR asociado a la branch actual. Pasa `/che-iterate <numero>` o una URL.".

2. **URL** (`https://github.com/<owner>/<repo>/(pull|issues)/<N>`) → extraer `owner`, `repo`, `N`, y `kind` (`pull` o `issues`). Hacerlo con parseo local (split por `/`), NO interpolar la URL en un comando shell.

3. **Numero puro** (`^[0-9]+$`) → `N`. **Validar este regex ANTES de pasar `$N` a cualquier comando shell**; si no matchea, caer al caso 4. Una vez validado, detectar `kind` en este orden:
   ```bash
   gh pr view "$N" --json number 2>/dev/null
   ```
   Si retorna JSON valido → PR. Si falla:
   ```bash
   gh issue view "$N" --json number 2>/dev/null
   ```
   Si retorna JSON valido → issue. Si ambos fallan, abortar: "No encontre PR ni issue #N en este repo.". Con el precheck de auth al tope del Paso 1 asumimos que este fallo es "not found" real y no auth/red; si el mensaje confunde al usuario, pedirle que re-corra `gh auth status` manualmente.

4. **Cualquier otro input** → abortar con mensaje de uso: "Uso: `/che-iterate [N | URL | (vacio para usar la branch actual)]`".

Resolver `owner` y `repo` si vienen de URL usando los valores parseados; en el resto de los casos usar:
```bash
gh repo view --json owner,name --jq '"\(.owner.login)/\(.name)"' 2>/dev/null
```

Guardar en variables: `OWNER`, `REPO`, `N`, `KIND` (`pr` o `issue`). Para `KIND=pr` tambien guardar `HEAD_REF` (de `headRefName`) — se usa en Paso 4.6 (branch check) y Paso 5.5 (push).

### Paso 2: Fetch de comments

**Metadata** (siempre):

- Si `KIND=pr`:
  ```bash
  gh pr view "$N" --json number,title,author,headRefName,baseRefName,url,state,body 2>/dev/null
  ```
- Si `KIND=issue`:
  ```bash
  gh issue view "$N" --json number,title,author,url,state,body 2>/dev/null
  ```

Guardar el `author.login` como `AUTHOR` para el filtrado posterior.

**Comments** (los paths son literales, `$OWNER`/`$REPO`/`$N` son variables shell sanitizadas en Paso 1):

- Issue comments (aplica a PR y a issue — GitHub trata el PR como issue para este endpoint):
  ```bash
  gh api --paginate "repos/$OWNER/$REPO/issues/$N/comments"
  ```

- **Solo si `KIND=pr`** tambien fetch:
  - Review comments (inline, con `path`/`line`/`diff_hunk`):
    ```bash
    gh api --paginate "repos/$OWNER/$REPO/pulls/$N/comments"
    ```
  - Reviews (los review bodies top-level, separados de los review comments):
    ```bash
    gh api --paginate "repos/$OWNER/$REPO/pulls/$N/reviews"
    ```

Parsear el JSON resultante. Para cada comment / review, quedarse con: `id`, `user.login`, `created_at`, `body`, y (solo para review comments) `path`, `line` (o `original_line` si `line` es null), `commit_id`, `diff_hunk`. Para reviews: `state` (`APPROVED` / `CHANGES_REQUESTED` / `COMMENTED`) y `body`.

### Paso 3: Filtrado barato

Descartar un comment / review si:

- **Es del autor del PR/issue**: `user.login == AUTHOR`.
- **Body puramente reaccional**, match case-insensitive contra el regex **despues de hacer strip de todo `\s+` (whitespace horizontal + newlines + tabs) al inicio y al final**:
  `^(lgtm|\+1|-1|👍|👎|🎉|🚀|thanks?|ty|nice|great|:\+1:|:-1:|:shipit:|:tada:|:rocket:)[.!]*$`
- **Review con `state == APPROVED` y `body` vacio o solo whitespace**.

Mantener todo lo demas, incluyendo comments cortos con contenido accionable (p.ej. "rename to X"). Contar cuantos se filtraron por cada categoria para reportarlo al final.

### Paso 4: Armar archivo de contexto

Escribir `/tmp/cvm-iterate-context.md` via Write tool. NUNCA interpolar bodies de comments en shell. Estructura:

```markdown
# Iteracion sobre <PR|Issue> #<N>

## Metadata
- Repo: <owner>/<repo>
- Titulo: <titulo>
- Autor: @<author>
- URL: <url>
- Estado: <state>
- Branch (solo PR): <headRefName> → <baseRefName>

## Body original
````markdown
<body del PR/issue — puede estar vacio>
````

## Diff (solo si PR)
<si el diff <= 2000 lineas: pegarlo inline dentro de un bloque `diff`>
<si > 2000 lineas: NO pegarlo — en su lugar escribir la nota de abajo>

> Diff con <N> lineas — truncado. Disponible completo en `/tmp/cvm-iterate-diff.txt`. Leer bajo demanda.

## Comments (<total> tras filtrado; <descartados> descartados)

### 1. @<user> — <created_at> — <tipo: issue comment | review comment | review>
<si es review comment:>
- Path: <path>:<line>
- Commit: <commit_id>
- Diff hunk:
  \`\`\`diff
  <diff_hunk>
  \`\`\`
<si es review:>
- State: <state>

````markdown
<body>
````

---

### 2. ...
```

Numerar los comments en orden cronologico (por `created_at`). Si no quedo ningun comment tras el filtrado, escribir una seccion `## Comments (0 tras filtrado)` con un aviso: "No hay comments accionables.".

**Nota sobre el fence de bodies**: usar fence de **4 backticks** (` ```` `) alrededor de los bodies del PR/issue y de los comments. Esto evita que headings `##` o bloques ` ``` ` dentro del body rompan la estructura del contexto. Si un body contiene literalmente 4 backticks consecutivos, escalar a 5 backticks — es extremadamente raro en la practica.

Para el diff del PR, **siempre** dumpear a archivo (nunca interpolar via shell) y luego decidir si inlinearlo:
```bash
gh pr diff "$N" > /tmp/cvm-iterate-diff.txt 2>/dev/null
wc -l /tmp/cvm-iterate-diff.txt
```

- Si `wc -l` ≤ **2000**: leer con `Read` y pegar dentro del bloque `diff` en el contexto.
- Si > **2000**: NO inlinearlo. Dejar la nota "truncado — disponible en `/tmp/cvm-iterate-diff.txt`" y el agente lo lee bajo demanda con `Read` (offset/limit) cuando necesita inspeccionar un cambio puntual. Esto evita saturar el context window del agente Opus en PRs grandes.

### Paso 4.5: Pre-transition (lock)

Antes de lanzar el agente, aplicar el lock segun `KIND`:

1. Resolver `CURRENT_STATE` (stateref):
   - Buscar `che:*` en los `labels` del fetch del Paso 2.
   - Si `KIND=pr` y el PR no tiene `che:*`: hacer fetch extra `gh pr view "$N" --json closingIssuesReferences` y leer labels de cada issue linkeado. Usar el primer `che:*` encontrado.
   - Si nadie tiene `che:*` → `CURRENT_STATE=""`.

2. Determinar transition:
   - `KIND=pr`: target lock es `che:executing`. Esperado `from=che:validated`.
   - `KIND=issue`: target lock es `che:planning`. Esperado `from=che:validated`.

3. Si `CURRENT_STATE == che:validated` (path normal):
   ```bash
   gh label create "<target_lock>" --force 2>/dev/null
   gh api -X DELETE "repos/$OWNER/$REPO/issues/$N/labels/che:validated" 2>/dev/null
   gh api -X POST "repos/$OWNER/$REPO/issues/$N/labels" -f "labels[]=<target_lock>"
   ```

4. Si `CURRENT_STATE != che:validated` (lenient):
   - Warn: "Target #<N> no esta en che:validated (esta en `<CURRENT_STATE>` o sin che:*). Aplicando lock igual."
   - Remover TODOS los `che:*` que tenga (loop sobre los 9 estados, tolerando 404).
   - Aplicar `<target_lock>`.

5. Guardar `PREVIOUS_STATE=$CURRENT_STATE` (para rollback).

### Paso 4.6: Verificar branch (solo `KIND=pr`)

Si `KIND=pr`, el subagent va a editar archivos y al final el orquestador hace commit + push. Para que el commit termine en la branch correcta, la branch local actual debe coincidir con `headRefName` del PR:

```bash
CURRENT_BRANCH=$(git branch --show-current)
if [[ "$CURRENT_BRANCH" != "$HEAD_REF" ]]; then
  echo "Branch local '$CURRENT_BRANCH' != headRefName del PR '$HEAD_REF'."
  echo "Hace 'git checkout $HEAD_REF' (o 'gh pr checkout $N') antes de /che-iterate."
  # rollback del lock
  exit 1
fi

# Working tree limpio para no mezclar cambios pre-existentes con los del subagent
if [[ -n "$(git status --porcelain)" ]]; then
  echo "Working tree no limpio. Stashea o commitea los cambios actuales antes de /che-iterate."
  # rollback del lock
  exit 1
fi
```

Si alguno falla, hacer rollback del lock (Paso 6.b) y abortar.

### Paso 5: Despachar al agente

Si quedaron 0 comments tras el filtrado, NO lanzar el agente. Hacer rollback del lock (Paso 6.b) y reportar al usuario que no hay nada accionable, salir.

En caso contrario, lanzar:

```
Agent(
  subagent_type: "general-purpose",
  model: "opus",
  description: "Aplicar comments de PR/Issue #<N>",
  prompt: <ver abajo>
)
```

El prompt del agente difiere segun `KIND`:

**Si `KIND=pr`** (sustituyendo `<N>`):

```
Tenes acceso al filesystem del worktree actual. El contexto completo esta en /tmp/cvm-iterate-context.md — leelo primero.

Tarea: evaluar los comments/reviews del PR #<N> y aplicar al codigo los cambios que sean accionables.

Criterios:
1. Para cada comment, decidi si es accionable (pide un cambio concreto), informativo (solo comenta sin pedir accion), o ruido residual que el filtrado no atrapo.
2. Aplicar los cambios accionables editando archivos directamente (Edit/Write).
3. Si dos comments se contradicen, priorizar el mas reciente y anotar el conflicto.
4. Si un comment es ambiguo, NO inventar interpretacion — marcarlo como "no aplicado, requiere aclaracion".

Restricciones:
- NO hagas commits ni push — eso lo hace el orquestador despues.
- NO crees archivos nuevos salvo que un comment lo pida explicitamente.
- NO respondas a los comments en GitHub. Solo tocas codigo local.
- NO delegues a otros agentes.

Output:
Reporte estructurado con:
- ## Aplicados — lista de comments procesados y que archivos tocaste
- ## Ignorados — comments no aplicados y la razon (ruido, ambiguo, fuera de scope, contradictorio)
- ## Archivos modificados — paths tocados

Termina tu respuesta con una seccion `## Key Learnings:` listando descubrimientos no-obvios sobre el codebase o el feedback que puedan ser utiles en futuras iteraciones.
```

**Si `KIND=issue`** (plan iteration — sustituyendo `<N>`):

```
Tenes acceso al filesystem del worktree actual. El contexto completo esta en /tmp/cvm-iterate-context.md — leelo primero.

Tarea: el issue #<N> es un plan que recibio comments/reviews pidiendo cambios. Re-escribi el body del issue incorporando el feedback accionable. NO toques archivos del repo — el resultado es un nuevo body para el issue.

Criterios:
1. Mantener la estructura del body original (mismas secciones: Idea/Contexto/Criterios/Notas/Clasificacion si aplica).
2. Para cada comment accionable, integrarlo en la seccion correspondiente del body. Si pide ajustar criterios, criterios. Si pide aclarar contexto, contexto. Etc.
3. Comments contradictorios: priorizar el mas reciente y anotar el conflicto en "Notas".
4. Comments ambiguos: anotarlos en "Notas" como "pendiente de aclaracion" — NO inventar interpretacion.

Output del subagent (ESTRICTO):
1. Escribir el nuevo body completo (markdown) a `/tmp/cvm-iterate-new-body.md` con Write tool.
2. En tu respuesta, devolver un reporte con:
   - ## Aplicados — comments incorporados al body y donde
   - ## Ignorados — comments no aplicados y la razon
   - ## Body actualizado — confirmar que escribiste a `/tmp/cvm-iterate-new-body.md`

Restricciones:
- NO modifiques archivos del repo.
- NO commitees, NO pushees.
- NO respondas a los comments en GitHub.
- NO delegues a otros agentes.

Termina tu respuesta con una seccion `## Key Learnings:`.
```

### Paso 5.5: Persistir cambios

**Si `KIND=pr`** y el agente reporto ≥1 cambio aplicado (`## Aplicados` no vacio):

```bash
# El check de Paso 4.6 garantiza que estamos en HEAD_REF y el WT estaba limpio antes.
# Los unicos cambios ahora son los que aplico el subagent.
git add -A
git commit -m "iterate(#$N): apply review feedback"
git push origin "HEAD:$HEAD_REF"
```

Capturar exit code de cada paso:
- Si `git commit` falla (raro, ej: pre-commit hook bloquea): rollback del lock (Paso 6.b) + warn al humano que tiene los cambios stageados sin commitear.
- Si `git push` falla (non-fast-forward, conflicto, network): rollback del lock + warn al humano que el commit local existe pero no se pudo pushear. El humano resuelve el conflicto a mano y reintenta.

Si el agente reporto 0 aplicados → saltar este paso (no hay nada que commitear); ir directo a rollback en Paso 6.b.

**Si `KIND=issue`** y el agente escribio `/tmp/cvm-iterate-new-body.md`:

```bash
# Verificar que el archivo existe y no esta vacio
[[ -s /tmp/cvm-iterate-new-body.md ]] || { echo "Subagent no genero nuevo body"; exit 1; }
gh issue edit "$N" --repo "$OWNER/$REPO" --body-file /tmp/cvm-iterate-new-body.md
```

Si `gh issue edit` falla → rollback del lock + warnear.

Si el agente NO escribio el archivo → saltar (no hay cambio que persistir); ir directo a rollback.

### Paso 6: Post-transition (success o rollback)

**6.a Success** — si el agente reporto al menos 1 comment aplicado (no vacio en `## Aplicados`):
- `KIND=pr`: aplicar `che:executing→che:executed`.
- `KIND=issue`: aplicar `che:planning→che:plan`.

```bash
gh label create "<target_success>" --force 2>/dev/null
gh api -X DELETE "repos/$OWNER/$REPO/issues/$N/labels/<target_lock>" 2>/dev/null
gh api -X POST "repos/$OWNER/$REPO/issues/$N/labels" -f "labels[]=<target_success>"
```

**6.b Rollback** — si el agente reporto 0 aplicados, todos ignorados, o el agente fallo:
- `KIND=pr`: aplicar `che:executing→che:validated` (volver a `PREVIOUS_STATE` solo si era `che:validated`; sino, aplicar `PREVIOUS_STATE` o limpiar `che:*` si era vacio).
- `KIND=issue`: aplicar `che:planning→che:validated` (mismas reglas).

### Paso 7: Reportar y persistir aprendizajes

Mostrar al usuario el reporte del agente tal cual, seguido de un resumen corto:

```
Iteracion sobre <PR|Issue> #<N> completada.
- Comments totales: <X>
- Filtrados (ruido/autor/APPROVED-vacio): <Y>
- Procesados por el agente: <Z>
- Archivos modificados (PR) / body actualizado (issue): <lista o "body de issue #<N>">
- Persistencia: <commit + push <hash> a <branch> | gh issue edit OK | sin cambios>
- Estado final: <che:executed|che:plan|che:validated (rollback)>
```

Si hubo failure de commit/push o `gh issue edit`, dejar explicito que el lock se rolleo y que el humano debe reintentar a mano (con el detalle del error).

Luego, el **skill** (no el subagent) invoca `/r` usando el Skill tool para persistir aprendizajes de la sesion. Esto no contradice la restriccion "NO delegues a otros agentes" del prompt del agente: la restriccion aplica al subagent Opus despachado en el Paso 5; el orquestador (este skill) si puede invocar `/r`.

## MUST DO
- Parsear `$ARGUMENTS` localmente (sin interpolar strings del usuario en shell).
- **Validar que `N` matchea `^[0-9]+$` ANTES de pasarlo a cualquier comando shell** (`gh pr view "$N"`, `gh api .../issues/$N/...`, etc). Sin esa validacion, un input tipo `42;rm -rf ~` rompe la garantia de no-interpolacion.
- Detectar PR vs issue con fallback `gh pr view` → `gh issue view`.
- Usar `gh api --paginate` para los tres endpoints cuando `KIND=pr`, solo `issues/<N>/comments` cuando `KIND=issue`.
- Filtrar comments del autor, reacciones puras (regex case-insensitive, **tras strip de `\s+` incluyendo newlines**), y reviews APPROVED vacios antes de pasar al agente.
- Escribir el contexto a `/tmp/cvm-iterate-context.md` con Write tool; el prompt del agente referencia el path.
- Dumpear el diff a `/tmp/cvm-iterate-diff.txt` **siempre**; inlinearlo en el contexto solo si tiene ≤ 2000 lineas. Si es mas grande, dejar solo el puntero al archivo.
- Resolver `CURRENT_STATE` via stateref: PR labels primero, fallback a labels del issue linkeado (`closingIssuesReferences`).
- Aplicar pre-transition (lock) ANTES del subagent. Lenient: si state actual no es `che:validated`, warn + limpiar `che:*` previos + aplicar el lock.
- Para `KIND=pr`: verificar branch local == `headRefName` y working tree limpio ANTES del subagent. Si no, rollback + abortar.
- Para `KIND=pr`: despues del subagent, si reporto ≥1 cambio aplicado: `git add -A && git commit -m "iterate(#$N): apply review feedback" && git push origin HEAD:$HEAD_REF`.
- Para `KIND=issue`: el subagent escribe el nuevo body a `/tmp/cvm-iterate-new-body.md`; despues hacer `gh issue edit "$N" --body-file <path>`.
- Si commit/push o `gh issue edit` falla → rollback del lock + reportar error explicito al humano.
- Aplicar post-transition (success o rollback) DESPUES de persistir cambios.
- Usar `gh api` REST para labels (NO `gh issue edit --add-label` — REST evita scope `read:org`).
- Lanzar `Agent(subagent_type='general-purpose', model='opus')` con prompt KIND-aware (PR edita archivos; issue escribe body al archivo temp).
- Despues del reporte del agente, invocar `/r` via Skill tool.

## MUST NOT DO (el skill)
- No interpolar bodies de comments, titulos, o `$ARGUMENTS` crudos en double-quoted shell commands. Todo lo de github va via `gh ... --json` / `gh api` y se parsea localmente.
- No delegar a `/go` — el skill lanza `Agent` directo.
- No commitear si el agente reporto 0 aplicados. No hacer `git push --force`. No saltarse hooks (`--no-verify`).
- No hacer checkout automatico a `headRefName` ni stashear cambios del usuario — abortar y pedirle al humano que prepare el WT.
- No comentar de vuelta en GitHub (replies a comments).
- No soportar GitLab / Bitbucket / otras plataformas en esta iteracion.
- No resolver review threads — el skill solo modifica codigo local + persiste.
- No lanzar el agente si no quedaron comments accionables tras el filtrado.

Nota: los archivos `/tmp/cvm-iterate-context.md` y `/tmp/cvm-iterate-diff.txt` los crea el **skill** como orquestacion (no cuentan como "archivos nuevos"). La restriccion "no crear archivos nuevos" aplica al **agente** despachado (ver prompt en Paso 5).

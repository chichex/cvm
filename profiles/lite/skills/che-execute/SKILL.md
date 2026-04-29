Toma un issue (numero/URL) o un input libre y ejecuta la implementacion en un worktree aislado, abriendo un PR draft contra la base. `$ARGUMENTS` puede ser un numero (`/che-execute 42`), una URL del repo actual (`/che-execute https://github.com/owner/repo/issues/42`), o texto libre describiendo la tarea (`/che-execute agregar endpoint /health`). Flags `--codex` / `--gemini` / `--opus` (default) eligen el agente. Aplica las transitions de la state machine `che:*` solo si `KIND=issue` (modo lenient).

Inspirado en `che-cli/internal/flow/execute/execute.go`, simplificado para el profile lite (sin `internal/plan` parser; usa el body crudo del issue como contexto).

## Tagging (state machine de che-cli)

Solo aplica cuando `KIND=issue`. `<from>` se detecta del fetch (prioridad `validated` > `plan` > `idea`).

| Target | Pre (lock) | Success | Rollback |
|---|---|---|---|
| issue (`che:idea\|plan\|validated`) | `<from>→executing` | `executing→executed` | `executing→<from>` |

**Lenient**: si el issue no esta en `che:idea\|plan\|validated`, warn y aplicar el lock igual (limpiar `che:*` previos, aplicar `che:executing`). En rollback, `<from>` cae a `che:plan` por default.

`KIND=freeform`: no hay issue, no se aplica state machine. El PR no lleva `Closes #N`.

## Proceso

### Paso 0: Parsear flag de provider

Inspeccionar `$ARGUMENTS` y detectar el primer flag de provider:

- `--codex` → rama Codex
- `--gemini` → rama Gemini
- `--opus` o sin flag → rama Opus (default)

Solo un provider por invocacion; si hay mas de uno, abortar con: "Solo un flag de provider por invocacion (`--opus` | `--codex` | `--gemini`).".

Remover el flag de `$ARGUMENTS` antes del Paso 1.

### Paso 1: Parsear input y resolver KIND

**Precheck de auth** (antes de cualquier `gh`): correr `gh auth status` una sola vez. Si falla, abortar inmediatamente con: "gh no esta autenticado o no hay red — corre `gh auth login` y reintenta.".

Detectar formato del `$ARGUMENTS` (sin flag, trim de whitespace):

1. **Vacio** → abortar con uso: "Uso: `/che-execute [N | URL | <descripcion libre>] [--codex|--gemini]`".

2. **Numero puro** (`^[0-9]+$`) → `KIND=issue`, `N=$ARGUMENTS`. **Validar el regex ANTES de pasar `$N` a cualquier comando shell.**

3. **URL** (`^https://github\.com/<owner>/<repo>/issues/<N>$`) → parsear via split por `/`. NUNCA interpolar la URL en un comando shell. `KIND=issue`. Si la URL es de `pull/` (PR), abortar: "/che-execute opera sobre issues, no PRs. Usa `/che-iterate` para PRs.".

4. **Cualquier otro texto** → `KIND=freeform`. Guardar el texto crudo en `FREEFORM_INPUT` para el Paso 4.

Resolver repo current:
```bash
gh repo view --json owner,name --jq '"\(.owner.login)/\(.name)"' 2>/dev/null
```
Guardar como `OWNER/REPO`. Si `KIND=issue` por URL y el `OWNER/REPO` parseado no matchea el current, abortar: "El issue es de `<owner>/<repo>` pero estas en `<current>`. Cambiate de repo o usa N puro.".

### Paso 2: Fetch del issue (solo `KIND=issue`)

```bash
gh issue view "$N" --json number,title,body,labels,url,state
```

Validar:
- `state == OPEN`. Si no, abortar.
- Capturar `TITLE`, `BODY`, `URL`, `LABELS`.

Detectar `FROM_STATE` (prioridad validated > plan > idea):
- Si `LABELS` contiene `che:validated` → `FROM_STATE=che:validated`
- Else si contiene `che:plan` → `FROM_STATE=che:plan`
- Else si contiene `che:idea` → `FROM_STATE=che:idea`
- Else → `FROM_STATE=""` (lenient, warn)

Bloqueos hard (abortar):
- `che:executing` presente → "Otro run en curso o quedo colgado; quita `che:executing` a mano si es lo segundo."
- `che:executed | che:closing | che:closed` → "Issue ya avanzo; execute no aplica."
- `che:locked` → "Otro flow lo tiene agarrado o quedo colgado. Si es lo segundo: `gh api -X DELETE \"repos/$OWNER/$REPO/issues/$N/labels/che:locked\"`."

### Paso 3: Prechecks de repo + base branch

- `git rev-parse --show-toplevel` → `REPO_ROOT`. Si falla, abortar.
- `git remote get-url origin` debe contener `github.com`. Si no, abortar.
- Detectar base branch:
  ```bash
  gh repo view --json defaultBranchRef --jq '.defaultBranchRef.name' 2>/dev/null
  ```
  Guardar como `BASE_BRANCH` (default `main` si falla).
- Verificar que `BASE_BRANCH` existe localmente actualizado: `git fetch origin "$BASE_BRANCH"`.

### Paso 4: Determinar slug, branch y worktree

**Slug** (lowercase, `[a-z0-9-]`, max 40 chars):
- Si `KIND=issue`: del `TITLE` del issue.
- Si `KIND=freeform`: de la primera linea de `FREEFORM_INPUT` (max 80 chars antes de slugify).

Generar el slug localmente (NO via shell con `$TITLE`/`$FREEFORM_INPUT`): pasarlo por replace de no-alfanumericos a `-`, lowercase, trim.

**Branch + worktree**:
- `KIND=issue`: `BRANCH=exec/$N-$SLUG`, `WT_PATH=$REPO_ROOT/.worktrees/issue-$N`
- `KIND=freeform`: `BRANCH=exec/$SLUG-$(date +%s)`, `WT_PATH=$REPO_ROOT/.worktrees/exec-$SLUG`

Si `WT_PATH` ya existe, abortar: "El worktree `$WT_PATH` ya existe — limpia con `git worktree remove --force $WT_PATH` o reintenta cuando termine el run anterior.".

### Paso 4.5: Pre-transition (lock) — solo `KIND=issue`

```bash
gh label create "che:executing" --force 2>/dev/null
# Limpiar che:* previos (loop sobre los 9 estados; tolerar 404)
for st in che:idea che:planning che:plan che:validating che:validated che:executing che:executed che:closing che:closed; do
  gh api -X DELETE "repos/$OWNER/$REPO/issues/$N/labels/$st" 2>/dev/null
done
gh api -X POST "repos/$OWNER/$REPO/issues/$N/labels" -f "labels[]=che:executing"
```

Si el `FROM_STATE` no era `che:idea|plan|validated`, warn antes: "Issue #$N no esta en che:idea/plan/validated (esta en `<FROM_STATE>` o sin che:*). Aplicando lock igual.".

### Paso 5: Crear worktree

```bash
git worktree add "$WT_PATH" -b "$BRANCH" "origin/$BASE_BRANCH"
```

Si falla: rollback del lock (Paso 8.b) + abortar.

### Paso 6: Armar prompt y archivo de contexto

Escribir `/tmp/cvm-execute-context.md` via Write tool. NUNCA interpolar `BODY`/`TITLE`/`FREEFORM_INPUT` en shell.

Estructura del contexto:

```markdown
# Tarea de implementacion

## Worktree
<WT_PATH absoluto>

[Si KIND=issue:]
## Issue
- Numero: #<N>
- Titulo: <TITLE>
- URL: <URL>
- State labels: <list de che:*>

## Body original
````markdown
<BODY>
````

[Si KIND=freeform:]
## Descripcion (input libre)
````markdown
<FREEFORM_INPUT>
````
```

Usar fence de 4 backticks alrededor del body para evitar romper la estructura si el body contiene `## ` o ` ``` `.

### Paso 7: Despachar al agente (depende de la rama)

Restricciones comunes en el prompt:
- Editar archivos SOLO dentro de `<WT_PATH>` (paths absolutos).
- NO commitear, NO push, NO crear PR — eso lo hace el orquestador.
- Si quedan cosas pendientes (info incompleta, dependencia bloqueada), dejar `<WT_PATH>/EXEC_NOTES.md` con lo pendiente — va al PR body.
- NO delegar a otros agentes.

Output esperado del agente (estricto):
- `## Implementado` — lista de cambios y archivos tocados.
- `## Pendiente` — qué quedó sin hacer y por qué (vacío si todo OK).
- `## Archivos modificados` — paths absolutos tocados.

#### Rama Opus (default)

```
Agent(
  subagent_type: "general-purpose",
  model: "opus",
  description: "Implementar <issue #N | tarea libre>",
  prompt: <contenido descrito abajo>
)
```

El prompt incluye:
1. Header con las restricciones de arriba.
2. Path del contexto: "Lee primero `/tmp/cvm-execute-context.md` para los detalles completos.".
3. La tarea reformulada (issue title + objetivo, o el FREEFORM_INPUT resumido).
4. Cierre: "Termina tu respuesta con `## Key Learnings:` listando descubrimientos no-obvios sobre el codebase o la tarea.".

#### Rama Codex

Verificar disponibilidad:
```bash
codex exec "echo ok" 2>/dev/null
```
Si falla: rollback del lock + worktree, sugerir omitir `--codex` para fallback a Opus.

Escribir el prompt completo a `/tmp/cvm-execute-prompt.txt` via Write tool. Lanzar:
```bash
(cd "$WT_PATH" && codex exec "$(cat /tmp/cvm-execute-prompt.txt)") 2>&1
```

#### Rama Gemini

Verificar disponibilidad en este orden:
1. `~/.cvm/available-tools.json` y `gemini.available == true`.
2. Fallback: `which gemini 2>/dev/null`.
3. Si ambos fallan: rollback del lock + worktree, sugerir omitir `--gemini`.

Lanzar:
```bash
(cd "$WT_PATH" && gemini -p "$(cat /tmp/cvm-execute-prompt.txt)") 2>&1
```

Sin timeout en ninguna rama. Esperar a que termine.

### Paso 8: Verificar cambios + persistir

Chequear que el worktree tenga cambios:
```bash
git -C "$WT_PATH" status --porcelain
```

#### 8.a — sin cambios

Si el output esta vacio:
- Avisar al usuario: "El agente no genero cambios en `$WT_PATH`. Nada que pushear.".
- Cleanup: `git worktree remove --force "$WT_PATH"` + `git branch -D "$BRANCH"` (tolerar errores; warn si quedan).
- Rollback del lock (Paso 8.b si `KIND=issue`).
- Salir.

#### 8.b — con cambios → commit + push + PR

```bash
git -C "$WT_PATH" add -A

# Commit message
COMMIT_MSG_FILE=$(mktemp)
# Si KIND=issue:
echo "feat(#$N): $TITLE" > "$COMMIT_MSG_FILE"
echo "" >> "$COMMIT_MSG_FILE"
echo "Closes #$N" >> "$COMMIT_MSG_FILE"
# Si KIND=freeform: solo titulo (primera linea de FREEFORM_INPUT, slugificado a lenguaje natural).
# Generar el contenido del archivo via Write tool (NO interpolar TITLE/FREEFORM_INPUT en shell).

git -C "$WT_PATH" commit -F "$COMMIT_MSG_FILE"
rm -f "$COMMIT_MSG_FILE"

git -C "$WT_PATH" push --set-upstream origin "$BRANCH"
```

Si `git commit` o `git push` falla → rollback del lock + warn al humano (worktree y commit local quedan para retry).

Crear PR draft. Construir body via Write tool a `/tmp/cvm-execute-pr-body.md`:

```markdown
[Si KIND=issue:]
Implementa #<N>.

Closes #<N>

[Si KIND=freeform:]
<FREEFORM_INPUT>

[Si existe <WT_PATH>/EXEC_NOTES.md, append:]
## Notas del agente
<contenido de EXEC_NOTES.md>
```

```bash
(cd "$WT_PATH" && gh pr create \
  --draft \
  --base "$BASE_BRANCH" \
  --head "$BRANCH" \
  --title "<title>" \
  --body-file /tmp/cvm-execute-pr-body.md)
```

Title: `feat(#$N): $TITLE` para issue, primera linea de `FREEFORM_INPUT` (max 70 chars) para freeform. NO interpolar via shell — pasar `--title` con un argumento ya construido en variable bash que el harness sabe que es safe (sale de fetch JSON parseado o del input ya validado).

Capturar la URL del PR en `PR_URL`.

### Paso 8.5: Post-transition (solo `KIND=issue`)

**Success** (PR creado):
```bash
gh api -X DELETE "repos/$OWNER/$REPO/issues/$N/labels/che:executing" 2>/dev/null
gh label create "che:executed" --force 2>/dev/null
gh api -X POST "repos/$OWNER/$REPO/issues/$N/labels" -f "labels[]=che:executed"
```

**Rollback** (cualquier falla post-lock pre-PR):
```bash
gh api -X DELETE "repos/$OWNER/$REPO/issues/$N/labels/che:executing" 2>/dev/null
# Re-aplicar FROM_STATE (default che:plan si era vacio)
gh label create "$FROM_STATE" --force 2>/dev/null
gh api -X POST "repos/$OWNER/$REPO/issues/$N/labels" -f "labels[]=$FROM_STATE"
```

**Caso intermedio** (PR creado pero post-transition falla): warn al humano que el label quedo en `che:executing` y el PR existe; pedirle que lo arregle a mano. NO hacer rollback del PR.

### Paso 9: Comment al issue (solo `KIND=issue`)

Best-effort (no fatal):
```bash
gh issue comment "$N" --body-file /tmp/cvm-execute-issue-comment.md
```

Body (escribir via Write tool a `/tmp/cvm-execute-issue-comment.md`):

```markdown
<!-- claude-cli: skill=execute role=pr-link -->
## Ejecucion completada

Se abrio un PR draft con los cambios:

- PR: <PR_URL>

El issue quedo en `che:executed`. Revisa el PR + CI; si queres validacion automatica antes de mergear, corre `/che-validate <PR_URL>`.
```

### Paso 10: Reportar y persistir aprendizajes

Mostrar al usuario:

```
Execute <issue #<N> | tarea libre> completada.
- Worktree: <WT_PATH>  (queda intacto para iteraciones; remove con `git worktree remove --force`)
- Branch: <BRANCH>
- PR: <PR_URL>
- Estado del issue: <che:executed | n/a (freeform)>
- Pendiente (EXEC_NOTES.md): <inline si existe, "ninguno" si no>
```

Solo rama Opus: invocar `/r` via Skill tool al final para persistir aprendizajes. Codex/Gemini no.

## MUST DO
- Parsear flag de provider (Paso 0) ANTES de procesar el input.
- Parsear `$ARGUMENTS` localmente. **Validar `^[0-9]+$` ANTES de pasar `$N` a shell.** Validar URL con split (no regex en shell con interpolacion).
- Detectar `KIND=issue` vs `freeform` con fallback claro.
- Solo `KIND=issue`: aplicar lock `che:executing` ANTES del worktree + agente. Lenient: si no esta en idea/plan/validated, warn + aplicar.
- Crear worktree aislado en `.worktrees/issue-N` (issue) o `.worktrees/exec-<slug>` (freeform).
- Pasar al agente el `WT_PATH` absoluto + el contexto via `/tmp/cvm-execute-context.md` con Write tool. NUNCA interpolar bodies en shell.
- Rama Opus: `Agent(subagent_type='general-purpose', model='opus')`. Codex/Gemini: verificar disponibilidad + Bash con `cd $WT_PATH`.
- Verificar `git status --porcelain` en el worktree post-agente. Si vacio: cleanup + rollback + abortar.
- Commit con `Closes #N` (issue) o sin (freeform). Push con `--set-upstream`.
- Crear PR draft con `gh pr create --draft --base $BASE_BRANCH --head $BRANCH --title <safe> --body-file /tmp/...`.
- Solo `KIND=issue`: post-transition `che:executing→che:executed`. Si falla post-PR: warn (no rollback del PR).
- Solo `KIND=issue`: comment al issue con link al PR (best-effort).
- Usar `gh api` REST para labels (NO `gh issue edit --add-label` — REST evita scope `read:org`).
- Solo rama Opus: invocar `/r` via Skill tool al final.

## MUST NOT DO
- No interpolar `TITLE`/`BODY`/`FREEFORM_INPUT`/`$ARGUMENTS` crudos en double-quoted shell commands. Todo via `gh ... --json` parseado o Write tool a archivos temp.
- No procesar URLs cross-repo (abortar si OWNER/REPO de la URL != current).
- No procesar URLs de `pull/` (redirigir a `/che-iterate`).
- No commitear si el agente no genero cambios. No `git push --force`. No `--no-verify`.
- No hacer rollback del PR si la post-transition falla — el PR remoto es prioridad.
- No reusar `WT_PATH` existente sin pedir cleanup explicito al usuario.
- No correr la rama Codex/Gemini si el CLI no esta disponible.
- No aplicar state machine si `KIND=freeform`.
- No agregar timeout al agente.
- No delegar a `/go` ni a otros skills (excepto `/r` al final en rama Opus).

Nota: los archivos en `/tmp/cvm-execute-*.md` los crea el **skill** como orquestacion. La restriccion "no crear archivos fuera del worktree" aplica al **agente** despachado en el Paso 7.

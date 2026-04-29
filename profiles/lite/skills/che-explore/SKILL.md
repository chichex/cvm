Toma un issue (numero o URL) y lo enriquece con un analisis estructurado: postea un comment con paths/preguntas/riesgos + prepende un plan consolidado al body. `$ARGUMENTS` puede ser un numero (`/che-explore 42`) o una URL (`/che-explore https://github.com/owner/repo/issues/42`). Flags `--codex` / `--gemini` / `--opus` (default) eligen el agente. Aplica las transitions de la state machine `che:*` de che-cli en modo lenient.

Inspirado en `che-cli/internal/flow/explore/explore.go`, simplificado para el profile lite (output del agente en markdown, no JSON parseado).

## Tagging (state machine de che-cli)

`<from>` se detecta del fetch (esperado: `che:idea`). Lenient: si no hay `che:*`, `<from>=che:idea` por default.

| Target | Pre (lock) | Success | Rollback |
|---|---|---|---|
| issue (`che:idea`) | `<from>→planning` | `planning→plan` | `planning→<from>` |

Si `<from>` no es `che:idea`, warn al usuario antes de aplicar el lock — aplicar igual.

## Proceso

### Paso 0: Parsear flag de provider

Inspeccionar `$ARGUMENTS` y detectar el primer flag de provider:

- `--codex` → rama Codex
- `--gemini` → rama Gemini
- `--opus` o sin flag → rama Opus (default)

Solo un provider por invocacion; si hay mas de uno, abortar con: "Solo un flag de provider por invocacion (`--opus` | `--codex` | `--gemini`).".

Remover el flag de `$ARGUMENTS` antes del Paso 1.

### Paso 1: Parsear input y resolver issue

**Precheck de auth**: correr `gh auth status` una sola vez. Si falla, abortar: "gh no esta autenticado o no hay red — corre `gh auth login` y reintenta.".

Detectar formato del `$ARGUMENTS` (sin flag, trim de whitespace):

1. **Vacio** → abortar: "Uso: `/che-explore [N | URL] [--codex|--gemini]`".

2. **Numero puro** (`^[0-9]+$`) → `N=$ARGUMENTS`. **Validar el regex ANTES de pasar `$N` a cualquier comando shell.**

3. **URL** (`^https://github\.com/<owner>/<repo>/issues/<N>$`) → parsear via split por `/`. NUNCA interpolar la URL en shell. Si la URL es de `pull/`, abortar: "/che-explore opera sobre issues, no PRs.".

4. **Cualquier otro input** → abortar (no acepta freeform; el output requiere un issue concreto que actualizar).

Resolver repo current:
```bash
gh repo view --json owner,name --jq '"\(.owner.login)/\(.name)"' 2>/dev/null
```
Guardar como `OWNER/REPO`. Si la URL es cross-repo, abortar.

### Paso 2: Fetch del issue

```bash
gh issue view "$N" --json number,title,body,labels,url,state
```

Validar:
- `state == OPEN`. Si no, abortar.
- Capturar `TITLE`, `BODY`, `URL`, `LABELS`.

Detectar `FROM_STATE` (prioridad idea > "" lenient default):
- Si `LABELS` contiene `che:idea` → `FROM_STATE=che:idea`
- Else → `FROM_STATE=""` (lenient, warn)

Detectar `HAS_CT_PLAN`:
- `LABELS` contiene `ct:plan` → `HAS_CT_PLAN=true`. El agente NO necesita clasificar.
- Sin `ct:plan` → `HAS_CT_PLAN=false`. El agente debe devolver seccion `## Clasificacion` con `type:` y `size:`.

Bloqueos hard (abortar):
- `che:planning | che:plan | che:executing | che:executed | che:validating | che:validated | che:closing | che:closed` presente → "Issue ya avanzo en el pipeline; explore no aplica.".
- `che:locked` → "Otro flow lo tiene agarrado o quedo colgado.".

### Paso 3: Pre-transition (lock)

```bash
gh label create "che:planning" --force 2>/dev/null
# Limpiar che:* previos (loop sobre los 9 estados; tolerar 404)
for st in che:idea che:planning che:plan che:validating che:validated che:executing che:executed che:closing che:closed; do
  gh api -X DELETE "repos/$OWNER/$REPO/issues/$N/labels/$st" 2>/dev/null
done
gh api -X POST "repos/$OWNER/$REPO/issues/$N/labels" -f "labels[]=che:planning"
```

Si `FROM_STATE` no era `che:idea`, warn antes: "Issue #$N no esta en che:idea (esta en `<FROM_STATE>` o sin che:*). Aplicando lock igual.".

### Paso 4: Armar archivo de contexto

Escribir `/tmp/cvm-explore-context.md` via Write tool. NUNCA interpolar `BODY`/`TITLE` en shell.

Estructura:

```markdown
# Issue a explorar

## Metadata
- Repo: <OWNER>/<REPO>
- Numero: #<N>
- Titulo: <TITLE>
- URL: <URL>
- Labels actuales: <list>
- Necesita clasificacion: <true|false>  # = !HAS_CT_PLAN

## Body original
````markdown
<BODY>
````
```

Usar fence de 4 backticks alrededor del body para evitar romper la estructura.

### Paso 5: Despachar al agente

Restricciones comunes en el prompt:
- Tenes acceso de LECTURA al codebase (Read/Grep/Glob). NO edites archivos.
- NO hagas commits, NO push, NO crees archivos en el repo.
- NO comentes en GitHub directamente — solo devuelve el markdown.
- NO delegues a otros agentes.

Output esperado (markdown estricto, headers H2 exactos):

```markdown
## Analisis
<2-4 lineas con el contexto del issue>

## Paths relevantes
- `path/al/archivo.go:line` — <por que importa>
- ...

## Preguntas abiertas
- <pregunta 1 — algo que el dueño debe responder antes de ejecutar>
- ...

## Riesgos
- <riesgo 1 — que puede salir mal>
- ...

## Suposiciones
- <decision tecnica que el ejecutor tomaria sin pedir voto>
- ...

## Clasificacion
<solo si "Necesita clasificacion: true">
type: feature | bug | chore | docs
size: xs | s | m | l | xl

## Plan consolidado
### Goal
<objetivo en 1-2 lineas>

### Approach
<enfoque tecnico, 1-3 lineas>

### Pasos
1. <paso accionable>
2. ...

### Criterios de aceptacion
- <criterio observable>
- ...

### Fuera de alcance
- <cosa que NO se toca>
- ...

## Proximo paso
<una linea: ej "correr /che-execute <N>", "esperar respuesta a preguntas", etc>
```

Si `HAS_CT_PLAN=true`, **omitir** la seccion `## Clasificacion` del prompt esperado y avisar al agente que no la genere.

#### Rama Opus (default)

```
Agent(
  subagent_type: "general-purpose",
  model: "opus",
  description: "Explorar issue #<N>",
  prompt: <ver abajo>
)
```

El prompt incluye:
1. Header con las restricciones de arriba.
2. "Lee primero `/tmp/cvm-explore-context.md` para los detalles completos.".
3. Instrucciones de output (las secciones markdown listadas arriba).
4. Cierre: "Termina tu respuesta con `## Key Learnings:` listando descubrimientos no-obvios sobre el codebase o el issue.".

#### Rama Codex

Verificar disponibilidad:
```bash
codex exec "echo ok" 2>/dev/null
```
Si falla: rollback del lock, sugerir omitir `--codex` para fallback a Opus.

Escribir el prompt completo a `/tmp/cvm-explore-prompt.txt` via Write tool. Lanzar:
```bash
codex exec "$(cat /tmp/cvm-explore-prompt.txt)" 2>&1
```
(El `cwd` queda en el repo current; codex tiene acceso al filesystem.)

#### Rama Gemini

Verificar disponibilidad en este orden:
1. `~/.cvm/available-tools.json` y `gemini.available == true`.
2. Fallback: `which gemini 2>/dev/null`.
3. Si ambos fallan: rollback del lock, sugerir omitir `--gemini`.

Lanzar:
```bash
gemini -p "$(cat /tmp/cvm-explore-prompt.txt)" 2>&1
```

Sin timeout en ninguna rama. Esperar a que termine.

### Paso 6: Validar output y splittear

Capturar el output completo del agente como `AGENT_OUTPUT`.

**Validacion**:
- Tiene que existir un header literal `## Plan consolidado` (linea que arranque con `## ` y matchee). Si no existe → rollback del lock + abortar: "El agente no devolvio `## Plan consolidado`; reintenta o cambia de provider.".
- Si `HAS_CT_PLAN=false`: tiene que existir `## Clasificacion` con `type:` y `size:` parseables. Si no, lenient: warnear y caer al default (`type:feature`, `size:m`).

**Split** (parsear localmente con Read tool sobre el output guardado a `/tmp/cvm-explore-output.md`):
- `PLAN_BLOCK` = todo lo que va desde `## Plan consolidado` hasta el siguiente header H2 (`^## ` que no sea sub-header `### `) o EOF, EXCLUYENDO el header siguiente.
- `COMMENT_BODY` = el `AGENT_OUTPUT` completo, sin filtrar.

Guardar `AGENT_OUTPUT` a `/tmp/cvm-explore-output.md` apenas se recibe (Write tool), antes del split, para tener trazabilidad si algo falla despues.

### Paso 7: Aplicar clasificacion (solo si `HAS_CT_PLAN=false`)

Parsear `## Clasificacion`. Extraer:
- `TYPE` ∈ `{feature, bug, chore, docs}`. Default: `feature`.
- `SIZE` ∈ `{xs, s, m, l, xl}`. Default: `m`.

Aplicar labels (atomico, mejor esfuerzo):
```bash
gh label create "ct:plan" --force 2>/dev/null
gh label create "type:$TYPE" --force 2>/dev/null
gh label create "size:$SIZE" --force 2>/dev/null

gh api -X POST "repos/$OWNER/$REPO/issues/$N/labels" \
  -f "labels[]=ct:plan" -f "labels[]=type:$TYPE" -f "labels[]=size:$SIZE"
```

Si la clasificacion fallo (no aplicaron labels): warn pero seguir — el plan consolidado en el body es lo importante.

### Paso 8: Postear comment

Usar el `AGENT_OUTPUT` completo como body del comment, agregando un header de trazabilidad:

```bash
# Construir el body via Write tool a /tmp/cvm-explore-comment.md
# Contenido:
#   <!-- claude-cli: skill=explore agent=<provider> -->
#   ## Exploracion del issue
#
#   <AGENT_OUTPUT>
gh issue comment "$N" --body-file /tmp/cvm-explore-comment.md
```

Capturar la URL del comment como `COMMENT_URL`.

Si falla: rollback del lock + abortar (no editamos body si no se posteo el comment — preferimos consistencia).

### Paso 9: Editar el body del issue

Construir el nuevo body: prepender `PLAN_BLOCK` al `BODY` original.

Estructura (escribir via Write tool a `/tmp/cvm-explore-new-body.md`):

```markdown
## Plan consolidado
<contenido de PLAN_BLOCK, sin el header "## Plan consolidado" porque ya lo pusimos arriba>

---

<BODY original>
```

NOTA: el `PLAN_BLOCK` extraido en Paso 6 incluye su header `## Plan consolidado` — al construir el nuevo body, escribir el header una sola vez. Dos opciones:
- Variante A (mas simple): pegar `PLAN_BLOCK` tal cual (incluye el header) + `\n---\n` + `BODY`. Saltarse el `## Plan consolidado` "manual" arriba.
- Variante B: strippear el header del `PLAN_BLOCK` y escribirlo a mano.

Elegir Variante A en la implementacion. El resultado debe tener un solo `## Plan consolidado`.

**Idempotencia**: si el `BODY` original ya contiene `## Plan consolidado`, NO duplicar — strippear el bloque viejo (de `## Plan consolidado` hasta el siguiente `---` o `## ` H2) antes de prepender el nuevo. Esto permite re-correr `/che-explore` sobre un issue sin acumular planes.

```bash
gh issue edit "$N" --body-file /tmp/cvm-explore-new-body.md
```

Si falla: warn al humano que el comment ya quedo posteado pero el body no se actualizo (`<COMMENT_URL>`). Rollback del lock. Salir con error.

### Paso 10: Post-transition

**Success** (comment + body OK):
```bash
gh api -X DELETE "repos/$OWNER/$REPO/issues/$N/labels/che:planning" 2>/dev/null
gh label create "che:plan" --force 2>/dev/null
gh api -X POST "repos/$OWNER/$REPO/issues/$N/labels" -f "labels[]=che:plan"
```

**Rollback** (cualquier falla post-lock pre-comment, o post-comment si decides abortar — ver Paso 8/9):
```bash
gh api -X DELETE "repos/$OWNER/$REPO/issues/$N/labels/che:planning" 2>/dev/null
# Re-aplicar FROM_STATE (default che:idea si era vacio)
TARGET_FROM="${FROM_STATE:-che:idea}"
gh label create "$TARGET_FROM" --force 2>/dev/null
gh api -X POST "repos/$OWNER/$REPO/issues/$N/labels" -f "labels[]=$TARGET_FROM"
```

### Paso 11: Reportar y persistir aprendizajes

Mostrar al usuario:

```
Explore issue #<N> completado.
- Provider: <opus|codex|gemini>
- Comment: <COMMENT_URL>
- Body actualizado: si (plan consolidado prependido)
- Clasificacion aplicada: <type:<TYPE> + size:<SIZE> | n/a (ya tenia ct:plan)>
- Estado del issue: che:plan
- Proximo paso (sugerido por el agente): <linea de "## Proximo paso">
```

Solo rama Opus: invocar `/r` via Skill tool al final para persistir aprendizajes. Codex/Gemini no.

## MUST DO
- Parsear flag de provider (Paso 0) ANTES de procesar el input.
- Parsear `$ARGUMENTS` localmente. **Validar `^[0-9]+$` ANTES de pasar `$N` a shell.**
- Aceptar SOLO `N` o URL. NO freeform — explore necesita un issue concreto que actualizar.
- Detectar `FROM_STATE` y `HAS_CT_PLAN` del fetch.
- Aplicar lock `<from>→che:planning` ANTES del agente. Lenient: si no esta en `che:idea`, warn + aplicar.
- Pasar al agente el contexto via `/tmp/cvm-explore-context.md` con Write tool. NUNCA interpolar `BODY`/`TITLE` en shell.
- Rama Opus: `Agent(subagent_type='general-purpose', model='opus')`. Codex/Gemini: verificar disponibilidad + Bash con `codex exec`/`gemini -p`.
- Validar que el output del agente contenga `## Plan consolidado`. Si no, rollback + abortar.
- Splittear el output en COMMENT_BODY (todo) y PLAN_BLOCK (solo la seccion del plan + sus subsecciones `### `).
- Si `HAS_CT_PLAN=false` y el agente devolvio `## Clasificacion`: aplicar `ct:plan` + `type:*` + `size:*` (defaults `feature`/`m` si parsing falla).
- Postear comment con el AGENT_OUTPUT completo + header de trazabilidad.
- Editar el body del issue prependiendo `PLAN_BLOCK` (deduplicar si ya existia un `## Plan consolidado`).
- Aplicar post-transition `che:planning→che:plan` solo si comment + body editaron OK.
- Usar `gh api` REST para labels (NO `gh issue edit --add-label` — REST evita scope `read:org`).
- Solo rama Opus: invocar `/r` via Skill tool al final.

## MUST NOT DO
- No interpolar `TITLE`/`BODY`/`$ARGUMENTS` crudos en double-quoted shell commands. Todo via `gh ... --json` parseado o Write tool a archivos temp.
- No procesar URLs cross-repo (abortar si OWNER/REPO de la URL != current).
- No procesar URLs de `pull/` (explore solo opera sobre issues).
- No editar archivos del repo. NO commitear, NO push.
- No reescribir el body desde cero — solo prependear el `PLAN_BLOCK` al body original.
- No duplicar `## Plan consolidado` si ya existia uno (strippear el viejo).
- No avanzar a `che:plan` si el comment o el body update fallaron — rollback al `<from>`.
- No correr la rama Codex/Gemini si el CLI no esta disponible.
- No agregar timeout al agente.
- No delegar a otros skills (excepto `/r` al final en rama Opus).

Nota: los archivos en `/tmp/cvm-explore-*.md` los crea el **skill** como orquestacion. El **agente** despachado en el Paso 5 NO debe crear archivos en el repo; solo devuelve el markdown como respuesta.

Revisar un PR o issue de GitHub lanzando agentes seleccionados en paralelo (opus/codex/gemini), posteando cada review como comment separado, y aplicando las transitions de la state machine `che:*` de che-cli en modo lenient. $ARGUMENTS es el PR/issue a revisar: numero (`24`), URL completa, o `pr 24` / `issue 24`.

## Contract

Outputs observables que otros skills (especialmente `/che-loop`) componen y dependen de su estabilidad. Cambiar estos strings/comportamientos es breaking change — sincronizar con consumers.

- **Modo non-interactive (default Opus)**: el Paso 4 muestra el prompt `Que agentes? [O/c/g/a] (default: O):`. Cuando se invoca desde un orquestador (ej. `/che-loop`) sin respuesta humana, la entrada vacia se trata como aceptacion del default → `AGENTS=[opus]`. Ese contrato permite componer `/che-validate` en cadena sin que el prompt bloquee. Si se necesita otro agente desde un orquestador, hay que extender la API del skill (ej. flag explicito `--agents O,c`) y forwardearlo desde el caller; mientras tanto, default-Opus es el unico modo non-interactive estable.
- **Header canonico de cada review**: cada comment posteado en el Paso 7 arranca **literalmente** con `## Review: <agente> (<perspectiva>)` (ver Paso 7 step 1). Ese prefijo es contract estable — `/che-iterate` Paso 3 lo usa como excepcion al filtro de autor (no descarta reviews machine-generated del propio bot), y `/che-loop` Paso 4.6 lo usa para excluir reviews del fingerprint de idempotencia. Cambiar ese formato rompe ambos consumers silenciosamente.
- **Verdict label persistido**: el Paso 7.6.a aplica `<VERDICT_NS>:<verdict>` (`validated:approve` / `validated:changes-requested` / `validated:needs-human` para PR; `plan-validated:*` para issue) ANTES de retornar al caller. Leer ese label es la forma deterministica de obtener el verdict (no parsear stdout). En rollback (todos los agentes fallaron) NO se aplica verdict label — el consumer debe distinguir "sin verdict label" como senal de fallo.

## Tagging (state machine de che-cli)

Aplica las transitions de `che-cli/internal/labels/labels.go`:

| Target | Pre (lock) | Success | Rollback |
|---|---|---|---|
| PR (`che:executed`) | `executed→validating` | `validating→validated` + `validated:<verdict>` | `validating→executed` |
| issue (`che:plan`) | `plan→validating` | `validating→validated` + `plan-validated:<verdict>` | `validating→plan` |

**Verdict** consolidado de los subagentes (cada uno emite `## Verdict: approve|changes-requested|needs-human` como ultima linea):
- Si algun subagent emite `needs-human` → consolidado = `needs-human`.
- Else si algun subagent emite `changes-requested` → consolidado = `changes-requested`.
- Else (todos `approve`) → `approve`.

Antes de aplicar el verdict label, remover los otros 2 verdicts del namespace (`validated:*` o `plan-validated:*`) — son mutuamente excluyentes.

**Stateref**: para PR, si NO tiene `che:*`, leer `closingIssuesReferences` y usar el `che:*` del issue linkeado.

**Lenient**: si current state no calza con el `from` esperado, warn + limpiar `che:*` previos + aplicar lock igual.

## Proceso

### Paso 1: Parsear input

Extraer numero, tipo explicito y (si viene URL) owner/repo desde $ARGUMENTS:

- `24` → numero pelado, tipo a detectar, repo actual
- `#24` → idem
- `pr 24` / `PR 24` → tipo forzado a PR, repo actual
- `issue 24` → tipo forzado a issue, repo actual
- `https://github.com/<org>/<repo>/pull/24` → PR 24 en `<org>/<repo>`
- `https://github.com/<org>/<repo>/issues/24` → issue 24 en `<org>/<repo>`

Si no se puede extraer un numero, abortar: "No pude extraer PR/issue desde '<input>'. Usa: numero, `pr N`, `issue N`, o URL completa."

Guardar:
- `NUM`: numero parseado
- `FORCED_KIND`: `pr`, `issue`, o vacio si solo vino numero
- `OWNER_REPO`: `<owner>/<repo>` si vino URL, vacio si no
- `REPO_FLAG`: `--repo "$OWNER_REPO"` si `OWNER_REPO` no esta vacio, sino vacio

`REPO_FLAG` (cuando aplique) debe usarse en TODOS los `gh pr|issue view|diff|comment` y como contexto en `gh api` (ver Paso 7). Nunca asumas el repo actual cuando vino una URL.

### Paso 2: Detectar tipo

Si `FORCED_KIND` ya esta seteado, usarlo. Sino, separar verificacion de asignacion (no mezclar stdout):

```bash
if gh pr view "$NUM" $REPO_FLAG --json number >/dev/null 2>&1; then
  KIND=pr
elif gh issue view "$NUM" $REPO_FLAG --json number >/dev/null 2>&1; then
  KIND=issue
else
  echo "No existe PR ni issue #$NUM en ${OWNER_REPO:-este repo}." >&2
  exit 1
fi
```

Guardar `KIND` = `pr` o `issue`.

### Paso 3: Fetchear contexto

Para PRs:

```bash
gh pr view "$NUM" $REPO_FLAG --json number,title,body,author,labels,files,baseRefName,headRefName
gh pr diff "$NUM" $REPO_FLAG
```

Para issues:

```bash
gh issue view "$NUM" $REPO_FLAG --json number,title,body,author,labels,comments
```

Escribir el contexto con `Write` tool (no heredoc) a `/tmp/cvm-validate-<NUM>-context.md` con este formato:

```markdown
# <KIND> #<NUM>: <title>

**Repo**: <owner>/<repo> (o "actual" si no vino URL)
**Author**: <author>
**Labels**: <labels>
**Files changed** (solo PR): <lista>

## Body
<body>

## Diff (solo PR)
<diff truncado si >200KB>
```

Si el diff supera 200KB, truncarlo al ultimo archivo completo que quepa y anotar `[... diff truncado: N archivos omitidos ...]`.

### Paso 4: Preguntar agentes

Verificar disponibilidad antes del prompt:

**Codex**: `codex exec "echo ok" 2>/dev/null`

**Gemini**:
1. Leer `~/.cvm/available-tools.json` y verificar `gemini.available == true`
2. Fallback: `which gemini 2>/dev/null`

Mostrar prompt con default explicito, marcando no disponibles:

```
Revisar <PR|issue> #<NUM>: <title>

Agentes: O=opus, c=codex, g=gemini, a=all
Que agentes? [O/c/g/a] (default: O):
```

Parsear respuesta:
- vacio o `O` / `o` → solo opus
- letras combinadas (`Oc`, `og`, `cg`, `Ocg`) → cada letra = un agente
- `a` / `all` → opus + codex + gemini (los disponibles)
- cualquier otra cosa → repetir la pregunta una vez, y si vuelve a ser invalida, abortar

Filtrar los que no esten disponibles y avisar. Si queda 0 agentes, abortar: "Ningun agente disponible."

Guardar `AGENTS` como la lista final (ej: `[opus, codex]`).

### Paso 4.5: Pre-transition (lock → `che:validating`)

1. Resolver `CURRENT_STATE` (stateref):
   - Buscar `che:*` en los `labels` del fetch del Paso 3.
   - Si `KIND=pr` y no hay `che:*` en el PR: `gh pr view "$NUM" $REPO_FLAG --json closingIssuesReferences` y leer labels de cada issue linkeado.
   - Si nadie tiene `che:*` → `CURRENT_STATE=""`.

2. Determinar `EXPECTED_FROM`:
   - `KIND=pr` → `che:executed`.
   - `KIND=issue` → `che:plan`.

3. Asegurar `che:validating` existe en el repo y aplicar:
   ```bash
   gh label create "che:validating" --force 2>/dev/null
   ```
   - Si `CURRENT_STATE == EXPECTED_FROM` (path normal):
     ```bash
     gh api -X DELETE "repos/$TARGET_REPO/issues/$NUM/labels/$EXPECTED_FROM" 2>/dev/null
     gh api -X POST "repos/$TARGET_REPO/issues/$NUM/labels" -f "labels[]=che:validating"
     ```
   - Else (lenient): warnear + remover los 9 `che:*` (DELETE con tolerancia a 404) + aplicar `che:validating`.

4. Guardar `PREVIOUS_STATE=$CURRENT_STATE` para rollback.

### Paso 5: Armar prompts diferenciados

Cada agente tiene una perspectiva fija:

- **opus** → arquitectura / diseno / trade-offs
- **codex** → correctness / bugs / edge cases
- **gemini** → legibilidad / claridad / mantenibilidad

Para cada agente armar un prompt que:

1. Referencia `/tmp/cvm-validate-<NUM>-context.md` como input (NUNCA contenido inline)
2. Declara la perspectiva del agente
3. Pide output en markdown listo para postear como comment de GitHub
4. Incluye seccion `## Key Learnings:` con descubrimientos no-obvios
5. **Termina con la linea exacta `## Verdict: approve|changes-requested|needs-human`** (uno solo, parseable)

Plantilla (ajustar por perspectiva):

```
Revisa el <KIND> #<NUM> de <owner>/<repo o "este repo">. El contexto completo (title, body, files, diff) esta en:

    /tmp/cvm-validate-<NUM>-context.md

Tu perspectiva es **<perspectiva>**. Enfocate en eso y evita comentar cosas fuera de tu angulo.

Output: markdown listo para postear como comment en GitHub. Estructura sugerida:
- Resumen de 1-2 lineas
- Hallazgos concretos (con referencias a archivo:linea cuando aplique)
- Recomendaciones accionables
- Riesgos si no se atienden

Despues de la review incluir:
1. Una seccion `## Key Learnings:` listando descubrimientos no-obvios.
2. Como **ultima linea del output**, exactamente uno de:
   - `## Verdict: approve` — la review esta lista para mergear/aprobar desde tu perspectiva.
   - `## Verdict: changes-requested` — hay cambios concretos requeridos.
   - `## Verdict: needs-human` — hay ambigüedad o trade-off que requiere decision humana.

La linea de Verdict tiene que ser la ULTIMA del output, sin texto despues. El orquestador la parsea para consolidar y aplicar el label.
```

Para codex y gemini, escribir el prompt en un archivo temporal con `Write` tool:

```
/tmp/cvm-validate-<NUM>-prompt-codex.txt
/tmp/cvm-validate-<NUM>-prompt-gemini.txt
```

### Paso 6: Lanzar agentes en paralelo

Todos los agentes en el MISMO mensaje, en paralelo:

- **opus**: `Agent(subagent_type: "general-purpose", model: "opus", description: "check #<NUM> arq", prompt: <prompt>)`
- **codex**: Bash `codex exec - < /tmp/cvm-validate-<NUM>-prompt-codex.txt 2>&1`
- **gemini**: Bash `gemini -p "" < /tmp/cvm-validate-<NUM>-prompt-gemini.txt 2>&1`

Pasar el prompt por stdin (nunca via `$(cat file)` dentro de double quotes): si el prompt llega a contener `$var`, backticks o `$(...)`, el shell los evaluaria antes de llegar al CLI. La redireccion pasa el archivo tal cual.

Timeout por agente externo: envolver cada invocacion con `gtimeout 600` (macOS via coreutils) o `timeout 600` (Linux) para que un CLI colgado no bloquee la sesion. Si el binario `gtimeout`/`timeout` no esta disponible, seguir sin timeout y anotarlo en el reporte final.

Si un agente falla (error, exit != 0, output vacio, timeout alcanzado), registrarlo en `FAILED` con el motivo (`timeout`, `exit=<code>`, `empty output`) y continuar con los demas.

### Paso 7: Postear cada review como comment

Para cada agente que completo:

1. Escribir el cuerpo del comment con `Write` tool a `/tmp/cvm-validate-<NUM>-<agente>.md` con este header:

```markdown
## Review: <agente> (<perspectiva>)

<contenido del agente>
```

2. Verificar tamano del archivo. Si supera 60KB, truncar el contenido del agente y agregar al final:
   `\n\n_[review truncada: supero 60KB, contenido completo disponible localmente]_`

3. Postear via `gh api` para obtener `html_url` confiable (los PR/issue comments comparten el endpoint `/repos/.../issues/<num>/comments`):

```bash
# Resolver owner/repo: si vino URL, usar OWNER_REPO; si no, derivarlo del repo actual.
TARGET_REPO="${OWNER_REPO:-$(gh repo view --json nameWithOwner -q .nameWithOwner)}"

URL=$(gh api \
  --method POST \
  "repos/$TARGET_REPO/issues/$NUM/comments" \
  -F body=@/tmp/cvm-validate-<NUM>-<agente>.md \
  --jq .html_url)
```

`gh api ... --jq .html_url` devuelve solo la URL del comment recien creado (es la respuesta de la API, no parsing de stdout informal). Funciona igual para PR y para issue porque ambos comparten el endpoint de issue comments.

Si el `gh api` falla (exit != 0 o `URL` vacio), registrar en `FAILED_POST` y continuar.

### Paso 7.5: Parsear verdicts y consolidar

Para cada agente que completo (no fallo), parsear la ultima linea no vacia de su output. Buscar match exacto:
- `## Verdict: approve` → verdict = `approve`
- `## Verdict: changes-requested` → verdict = `changes-requested`
- `## Verdict: needs-human` → verdict = `needs-human`
- Cualquier otra cosa (no matchea) → verdict = `needs-human` (default conservador) y warn al usuario: "Subagent <agente> no emitio verdict parseable, asumiendo needs-human".

Consolidar:
- Si algun verdict es `needs-human` → `CONSOLIDATED=needs-human`.
- Else si algun verdict es `changes-requested` → `CONSOLIDATED=changes-requested`.
- Else (todos `approve`) → `CONSOLIDATED=approve`.

Si TODOS los agentes fallaron (no hay verdicts), `CONSOLIDATED=""` → ir directo a rollback en Paso 7.6.b.

### Paso 7.6: Post-transition (success o rollback)

Determinar `VERDICT_NS` segun `KIND`:
- `KIND=pr` → namespace `validated:*`. Labels: `validated:approve`, `validated:changes-requested`, `validated:needs-human`.
- `KIND=issue` → namespace `plan-validated:*`. Labels: `plan-validated:approve`, `plan-validated:changes-requested`, `plan-validated:needs-human`.

**7.6.a Success** — si `CONSOLIDATED` no esta vacio:

1. Aplicar transition `che:validating→che:validated`:
   ```bash
   gh label create "che:validated" --force 2>/dev/null
   gh api -X DELETE "repos/$TARGET_REPO/issues/$NUM/labels/che:validating" 2>/dev/null
   gh api -X POST "repos/$TARGET_REPO/issues/$NUM/labels" -f "labels[]=che:validated"
   ```

2. Aplicar verdict label (los 3 son mutuamente excluyentes — remover los otros 2 antes):
   ```bash
   TARGET_VERDICT="$VERDICT_NS:$CONSOLIDATED"
   gh label create "$TARGET_VERDICT" --force 2>/dev/null
   for v in approve changes-requested needs-human; do
     [[ "$v" == "$CONSOLIDATED" ]] && continue
     gh api -X DELETE "repos/$TARGET_REPO/issues/$NUM/labels/$VERDICT_NS:$v" 2>/dev/null
   done
   gh api -X POST "repos/$TARGET_REPO/issues/$NUM/labels" -f "labels[]=$TARGET_VERDICT"
   ```

**7.6.b Rollback** — si `CONSOLIDATED` esta vacio (todos los agentes fallaron):

- `KIND=pr`: aplicar `che:validating→che:executed` (o `PREVIOUS_STATE` si era distinto y no vacio; lenient si era vacio: solo remover `che:validating`).
- `KIND=issue`: aplicar `che:validating→che:plan` (mismas reglas).

NO aplicar verdict label en rollback (no hubo verdict valido).

### Paso 8: Reporte final

Mostrar al usuario:

```
Review de <KIND> #<NUM> completa.

Verdicts:
- <agente1>: <verdict>
- <agente2>: <verdict>
Consolidado: <CONSOLIDATED>

Comments posteados:
- <agente1> (<perspectiva>): <url>
- <agente2> (<perspectiva>): <url>

Estado final:
- che:* → <che:validated | che:executed (rollback) | che:plan (rollback)>
- verdict label → <validated:<x> | plan-validated:<x> | "(no aplicado: rollback)">

Agentes que fallaron (si hubo): <lista con motivo breve>
Posts que fallaron (si hubo): <lista>
```

Si todos fallaron, dejarlo explicito: "Ninguna review se posteo. Rollback aplicado a <state previo>."

No ejecutar `/r` automaticamente — las reviews viven en GitHub, no en auto-memory.

## MUST DO
- Parsear input aceptando numero pelado, `pr N`, `issue N` y URLs completas de PR o issue, conservando `owner/repo` cuando viene URL
- Pasar `--repo $OWNER/$REPO` (o equivalente) en TODOS los `gh pr|issue view|diff|comment` cuando vino URL
- Detectar tipo automaticamente con fallback (`gh pr view` primero, luego `gh issue view`) usando `>/dev/null 2>&1` y asignando `KIND` solo segun exit code (nunca capturando stdout)
- Fetchear contexto con `gh <pr|issue> view --json` + `gh pr diff` (solo PR) y escribirlo con `Write` a `/tmp/cvm-validate-<NUM>-context.md`
- Preguntar agentes con prompt `[O/c/g/a]` y default explicito (Opus)
- Verificar disponibilidad de codex y gemini antes de ofrecerlos
- Armar prompts diferenciados por perspectiva (opus=arq, codex=correctness, gemini=legibilidad)
- Pasar el path del contexto a cada agente — NUNCA inline
- Lanzar todos los agentes en paralelo en el mismo mensaje
- Postear cada review como comment separado con header `## Review: <agente> (<perspectiva>)`
- Postear via `gh api ... -F body=@<file> --jq .html_url` para obtener la URL desde la respuesta de la API (no desde stdout no documentado de `gh pr|issue comment`)
- Truncar reviews >60KB y anotar la truncacion
- Reportar URLs de comments posteados y fallos al final
- Seguir sin abortar si un agente falla: reportar el fallo al final
- Resolver `CURRENT_STATE` via stateref (PR labels primero, fallback issue linkeado).
- Aplicar pre-transition `→che:validating` ANTES de lanzar agentes. Lenient si current no es el `from` esperado.
- Pedir verdict explicito como ultima linea del output de cada subagent (`## Verdict: approve|changes-requested|needs-human`).
- Consolidar verdicts con regla: needs-human > changes-requested > approve.
- Aplicar `che:validating→che:validated` + `<ns>:<verdict>` (removiendo los otros 2 verdicts) si hubo al menos 1 verdict.
- Rollback `che:validating→<previous>` si todos los agentes fallaron.
- Usar `gh api` REST para labels (NO `gh issue edit --add-label`).

## MUST NOT DO
- No pasar contenido del diff inline en comandos shell ni en prompts
- No interpolar $ARGUMENTS ni texto del usuario en comandos con double quotes
- No pasar el prompt como `$(cat prompt.txt)` dentro de double quotes (expande `$var`, backticks y `$(...)` del contenido). Usar redireccion por stdin
- No descartar `owner/repo` cuando vino una URL — operar siempre sobre el repo correcto
- No usar `gh pr comment` / `gh issue comment` para capturar URLs (su stdout no es contrato estable). Usar `gh api` con `--jq .html_url`
- No mezclar verificacion (`gh ... --json number`) con asignacion en el mismo pipeline (`&& echo "pr"`); separar exit code de output
- No lanzar agentes secuencialmente — siempre paralelo
- No sintetizar las reviews en un unico comment (fuera de alcance)
- No persistir reviews en auto-memory (viven en GitHub)
- No ejecutar `/r` al final
- No soportar GitLab/Bitbucket — solo GitHub via `gh`
- No ofrecer agentes no disponibles
- No correr codex/gemini sin timeout cuando `gtimeout`/`timeout` este disponible — un CLI colgado bloquea la sesion completa

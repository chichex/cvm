Orquestar el pipeline completo `/portable-spec` → `/portable-plan` → `/portable-code-loop` desde un unico punto de entrada. Acepta como input un prompt libre, un numero/URL de issue, o un issue ya etiquetado con `entity:spec`. Detecta el tipo de input automaticamente, evalua el "tamano" del cambio post-spec y post-plan contra thresholds heuristicos, emite advertencias bloqueantes cuando el cambio supera esos limites, y al final clasifica el resultado en una de cuatro categorias de fit. `$ARGUMENTS` es el input (prompt, numero de issue o URL). Si viene vacio se pide.

Skill **exclusivo para Claude Code** (depende de los subagents `portable-code-executor` y `portable-code-validator`). El orquestador (Claude principal) maneja la composicion de skills via Skill tool; si el runtime no soporta Skills anidados, cae a fallback de prosa.

## Thresholds

| Metrica | Threshold (advertencia si supera) |
|---------|-----------------------------------|
| Asunciones validadas (post-spec) | > 10 |
| Pasos del plan (post-plan) | > 8 |
| Archivos afectados (post-plan) | > 15 |
| Asunciones sin refinar (post-spec) | > 15 |

Estos valores viven aqui y son legibles por el usuario cuando ve la advertencia. No hay archivo de config externo.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar:
```
No hay un repo GitHub configurado en este directorio. /portable-auto necesita un repo GitHub para operar.

Configura el remote (`gh repo create` o `gh repo set-default`) y volve a correr.
```

### 2. Validar working tree limpio
```bash
git status --porcelain
```
Si hay cambios sin commitear, abortar:
```
Working tree no esta limpio. /portable-auto delega a /portable-plan que crea branches; commiteá o stashé los cambios pendientes antes de correr.
```

### 3. Parsear `$ARGUMENTS` y detectar tipo de input

Si `$ARGUMENTS` esta vacio, pedir al usuario:
```
Pasame el input: puede ser un prompt libre, un numero de issue (ej: `42` o `#42`) o una URL de issue (ej: https://github.com/owner/repo/issues/42).
```
Esperar respuesta. **No** continuar hasta tenerla.

Tipo de input (evaluar en orden):
1. **Numero de issue**: si matchea `^#?[0-9]+$` — extraer el numero. Guardar como `INPUT_ISSUE`.
2. **URL de issue**: si matchea `github\.com/.+/issues/[0-9]+` — extraer el numero del path. Guardar como `INPUT_ISSUE`.
3. **Prompt libre**: cualquier otra cosa. Guardar como `INPUT_PROMPT`.

Si el input puede interpretarse de mas de una forma (ej: un numero que podria ser un issue o parte de un prompt), preguntar al usuario en formato multiple-choice antes de continuar:
```
Tu input puede interpretarse de mas de una forma:

1) Como numero de issue (#<N>)
2) Como prompt libre: "<input>"
3) Otra interpretacion (especificame)

Cual es?
```
Esperar respuesta. **No** continuar hasta tenerla.

## Fase 1 — Clasificacion del issue (si aplica) o arranque desde prompt

### Rama A: input es un issue (`INPUT_ISSUE` definido)

Cargar el issue:
```bash
gh issue view "$INPUT_ISSUE" --json title,body,labels,state --jq '{title:.title, state:.state, labels:[.labels[].name]}'
```

Si el issue no existe o da error, abortar con el error de `gh`.
Si `state == "CLOSED"`, avisar y pedir confirmacion:
```
El issue #<INPUT_ISSUE> esta cerrado. Continuar igual? (si/no)
```
Si dice no, abortar.

**Detectar si tiene `entity:spec`:**
```bash
gh issue view "$INPUT_ISSUE" --json labels --jq '.labels[].name' | grep -Fxq "entity:spec"
```
- Si tiene `entity:spec`: saltar directamente a **Fase 3** (arrancar desde `/portable-plan`). Guardar `SPEC_ISSUE = INPUT_ISSUE`. Anotar `skipped_spec = true`.
- Si NO tiene `entity:spec`: usar el body del issue como historia de usuario. Guardar `INPUT_PROMPT = body`. Continuar a **Fase 2** (arrancar desde `/portable-spec`). Anotar `skipped_spec = false`.

### Rama B: input es prompt libre (`INPUT_PROMPT` definido)

Continuar directamente a **Fase 2**. Anotar `skipped_spec = false`.

## Fase 2 — Invocar `/portable-spec`

Anunciar al usuario:
```
--- Fase 1/3: /portable-spec ---
```

**Invocar el skill `/portable-spec`** con `INPUT_PROMPT` como argumento.

Si el runtime de Claude Code soporta Skills anidados: usar Skill tool con `/portable-spec`.
**Fallback** (si Skills anidados no estan disponibles): pausar y pedirle al usuario:
```
Skills anidados no disponibles en este entorno. Por favor:
1. Corre `/portable-spec <historia>` manualmente.
2. Una vez creado el issue, volvé a /portable-auto con el numero de issue resultante.
```
Y abortar esta invocacion.

Esperar el resultado de `/portable-spec`. Parsear el `## Result` del output:
```bash
# Extraer el numero de issue del URL
SPEC_URL=$(echo "$spec_result" | grep -E '^- url:' | sed 's/^- url: //')
SPEC_ISSUE=$(echo "$SPEC_URL" | grep -oE '[0-9]+$')
ASSUMPTIONS_TOTAL=$(echo "$spec_result" | grep -E '^- assumptions_total:' | grep -oE '[0-9]+')
ASSUMPTIONS_REFINED=$(echo "$spec_result" | grep -E '^- assumptions_refined:' | grep -oE '[0-9]+')
```

Guardar `skipped_spec = false`.

Mostrar al usuario:
```
Spec creada: <SPEC_URL>
Asunciones: <ASSUMPTIONS_TOTAL> totales, <ASSUMPTIONS_REFINED> refinadas.
```

### Chequeo heuristico post-spec

Cargar el body del issue de spec para contar asunciones:
```bash
gh issue view "$SPEC_ISSUE" --json body --jq '.body'
```

Contar asunciones validadas (lineas `^[0-9]+\.` dentro de la seccion `## Asunciones validadas`):
- Extraer el bloque entre `## Asunciones validadas` y el siguiente `##`.
- Contar las lineas que arrancan con `<N>.` (numero seguido de punto).
- Guardar como `COUNT_ASSUMPTIONS`.

Contar asunciones sin refinar: `COUNT_UNREFINED = ASSUMPTIONS_TOTAL - ASSUMPTIONS_REFINED` (si los valores no estan disponibles, usar `COUNT_ASSUMPTIONS` como estimacion conservadora).

**Evaluar thresholds:**
- Si `COUNT_ASSUMPTIONS > 10` O `COUNT_UNREFINED > 15`: anotar `size_warning_spec = true` y ejecutar **Advertencia bloqueante** (ver mas abajo).
- Si no supera ningun threshold: continuar a Fase 3.

## Fase 3 — Invocar `/portable-plan`

Anunciar al usuario:
```
--- Fase 2/3: /portable-plan ---
```

**Invocar el skill `/portable-plan`** con `SPEC_ISSUE` como argumento.

Si el runtime no soporta Skills anidados: pausar y pedirle al usuario que corra `/portable-plan <SPEC_ISSUE>` manualmente y vuelva con el numero de PR resultante.

Esperar el resultado de `/portable-plan`. Parsear el `## Result` del output:
```bash
PLAN_PR=$(echo "$plan_result" | grep -E '^- pr:' | grep -oE '[0-9]+')
PLAN_URL=$(echo "$plan_result" | grep -E '^- url:' | sed 's/^- url: //')
PLAN_FILE=$(echo "$plan_result" | grep -E '^- plan_file:' | sed 's/^- plan_file: //')
```

Mostrar al usuario:
```
Plan creado: <PLAN_URL>
```

### Chequeo heuristico post-plan

Leer el archivo `.portable/plans/<N>-<slug>.md` del PR recien creado:
```bash
gh pr diff "$PLAN_PR" --name-only | grep '\.portable/plans/'
# Leer el archivo via Read tool
```

Contar pasos (lineas `^- \[ \]` dentro de la seccion `## Pasos`):
- Extraer el bloque entre `## Pasos` y el siguiente `##`.
- Contar las lineas que arrancan con `- [ ]`.
- Guardar como `COUNT_STEPS`.

Contar archivos afectados (lineas que arrancan con `` - ` `` dentro de la seccion `## Archivos afectados`):
- Extraer el bloque entre `## Archivos afectados` y el siguiente `##`.
- Contar las lineas que arrancan con `` - ` ``.
- Guardar como `COUNT_FILES`.

**Evaluar thresholds:**
- Si `COUNT_STEPS > 8` O `COUNT_FILES > 15`: anotar `size_warning_plan = true` y ejecutar **Advertencia bloqueante** (ver mas abajo).
- Si no supera ningun threshold: continuar a Fase 4.

## Advertencia bloqueante

Cuando un threshold se supera, mostrar al usuario:

```
--- Advertencia de tamano ---

El cambio supera los thresholds configurados:
<listar las metricas que superaron, ej:>
  - Asunciones: <COUNT_ASSUMPTIONS> (threshold: 10)
  - Pasos: <COUNT_STEPS> (threshold: 8)

Este tipo de cambio es grande para el pipeline automatico y puede generar friction o necesitar mas iteraciones.

Como queres continuar?

1) continuar — seguir con el pipeline igual
2) abortar   — detener aqui; los artefactos ya creados quedan en GitHub para que los uses manualmente
3) ajustar   — te sugiero alternativas para reducir el scope

Cual elegis?
```

Esperar respuesta del usuario. **No** continuar hasta tenerla.

- Si elige `1` (continuar): registrar la advertencia en estado de conversacion y continuar.
- Si elige `2` (abortar): mostrar los URLs de los artefactos ya creados y abortar:
  ```
  Detenido. Artefactos creados hasta ahora:
  <si existe SPEC_URL:>  - Spec: <SPEC_URL>
  <si existe PLAN_URL:>  - Plan: <PLAN_URL>
  Podes retomar manualmente usando /portable-plan o /portable-code-loop.
  ```
- Si elige `3` (ajustar): proponer alternativas en prosa libre segun contexto. Ejemplos tipicos:
  - "Podes dividir la historia en dos specs: una para X y otra para Y."
  - "Podes acotar el scope a solo la parte de Z y dejar el resto para un follow-up."
  - "Podes refinar mas asunciones con /portable-spec antes de generar el plan."
  Despues de sugerir, preguntar:
  ```
  Querés ajustar el scope y volver a correr, o preferís continuar igual?
  1) Ajusto y vuelvo a correr
  2) Continuar igual
  ```
  Si elige `1`: abortar con instrucciones de como retomar. Si elige `2`: continuar.

## Fase 4 — Invocar `/portable-code-loop`

Anunciar al usuario:
```
--- Fase 3/3: /portable-code-loop ---
```

**Invocar el skill `/portable-code-loop`** con `PLAN_PR` como argumento.

Si el runtime no soporta Skills anidados: pausar y pedirle al usuario que corra `/portable-code-loop <PLAN_PR>` manualmente.

Esperar el resultado de `/portable-code-loop`. Parsear el `## Result` del output:
```bash
LOOP_VERDICT=$(echo "$loop_result" | grep -E '^- verdict:' | sed 's/^- verdict: //')
LOOP_ITERATIONS=$(echo "$loop_result" | grep -E '^- iterations_used:' | sed 's/^- iterations_used: //')
# Forma: "N/MAX_ITER" — extraer N
ITER_USED=$(echo "$LOOP_ITERATIONS" | cut -d'/' -f1)
ITER_MAX=$(echo "$LOOP_ITERATIONS" | cut -d'/' -f2)
```

### Detectar si hubo `code:failed` intermedio

```bash
OWNER_REPO=$(gh repo view --json nameWithOwner --jq '.nameWithOwner')
gh api "repos/$OWNER_REPO/issues/$PLAN_PR/timeline" --paginate \
  --jq '.[] | select(.event == "labeled" and .label.name == "code:failed") | .label.name' \
  | grep -c "code:failed"
```

Si el count es `> 0`: anotar `had_code_failed = true`. Si es `0`: `had_code_failed = false`.

## Fase 5 — Veredicto final de fit

Calcular la categoria de fit basada en las senales acumuladas en la conversacion:

| Categoria | Criterios |
|-----------|-----------|
| **encajo limpio** | `LOOP_VERDICT == PASS` Y `ITER_USED <= 1` Y `had_code_failed == false` Y `size_warning_spec == false` Y `size_warning_plan == false` |
| **encajo con friccion** | `LOOP_VERDICT == PASS` Y (`ITER_USED > 1` O `had_code_failed == true`) Y `size_warning_spec == false` Y `size_warning_plan == false` |
| **encajo con riesgo residual** | `LOOP_VERDICT == PASS` Y (`size_warning_spec == true` O `size_warning_plan == true`) |
| **no encajo** | `LOOP_VERDICT != PASS` (loop agoto iteraciones sin PASS) |

Nota: si aplican multiples categorias, priorizar la mas "grave" en orden: `no encajo` > `encajo con riesgo residual` > `encajo con friccion` > `encajo limpio`.

Mostrar el veredicto final:

```
## Resultado /portable-auto

- spec: <SPEC_URL o "skipped (input era entity:spec)">
- plan: <PLAN_URL>
- pr: <PR_URL del loop>
- iteraciones: <ITER_USED>/<ITER_MAX>
- verdict loop: <PASS|FAIL>
- fit: <categoria>

### Detalle del fit

<Categoria elegida con explicacion concisa de por que:>

**encajo limpio** — El pipeline corrio sin friction: una iteracion, sin fallos intermedios, dentro de los thresholds de tamano. El PR esta listo para review.

— o —

**encajo con friccion** — El pipeline termino en PASS pero requirio <ITER_USED> iteraciones <y/o hubo code:failed intermedios>. El resultado es correcto pero el cambio genero trabajo extra en el loop.

— o —

**encajo con riesgo residual** — El pipeline termino en PASS pero el cambio supero thresholds de tamano (<listar cuales>). El codigo esta validado pero el scope amplio implica mayor superficie de error no cubierta por el loop automatico. Se recomienda revision humana cuidadosa.

— o —

**no encajo** — El loop agoto sus iteraciones sin alcanzar PASS. El PR quedo en su ultimo estado. Revisa el feedback del ultimo validate y decide si retomar manualmente o cerrar.

---
<si PASS:>
PR listo para review/merge: <PR_URL>
<si FAIL:>
Estado final del PR: <PR_URL> — label: code:failed
```

## MUST DO

- Validar `gh repo view` y working tree limpio ANTES de procesar el input.
- Detectar el tipo de input en orden: numero > URL > prompt libre.
- Pedir desambiguacion en multiple-choice cuando el input es ambiguo.
- Verificar label `entity:spec` en el issue antes de decidir si saltar `/portable-spec`.
- Ejecutar chequeo heuristico post-spec (antes de `/portable-plan`) y post-plan (antes de `/portable-code-loop`).
- Mostrar advertencia bloqueante con opciones `continuar` / `abortar` / `ajustar` cuando se supera un threshold.
- Anotar en estado de conversacion si se disparo advertencia de tamano en alguna fase (`size_warning_spec`, `size_warning_plan`).
- Detectar `code:failed` intermedio via timeline API de GitHub.
- Calcular y mostrar el veredicto final de fit con la categoria correcta.
- Usar `--json + --jq` en todas las llamadas a `gh` (nunca scraping de texto).
- Pasar contenido de usuario via archivo temporal (Write tool) en lugar de interpolacion en shell.

## MUST NOT DO

- No invocar `/portable-code-loop` sin haber completado `/portable-plan` primero.
- No saltear el chequeo heuristico de tamano aunque parezca obvio que el cambio es chico.
- No persistir estado en disco — todo vive en la conversacion.
- No aplicar labels propios — los labels los aplican los skills hijos.
- No tocar `~/.claude/CLAUDE.md` en runtime.
- No agregar lo que no se pidio (flags extra, labels extras, etc.).
- No continuar despues de advertencia bloqueante sin esperar respuesta del usuario.
- No persistir nada en auto-memory.
- No delegar a subagent directo — la orquestacion vive en el orquestador principal.
- No avanzar de fase sin tener el resultado parseado de la fase anterior.

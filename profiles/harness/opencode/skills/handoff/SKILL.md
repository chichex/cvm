---
name: handoff
description: Construye un prompt para iniciar una nueva sesion (en otra ventana, otro modelo, u otro humano), a partir de un foco explicito o de la conversacion actual. El prompt es el artefacto principal y se copia al clipboard; el .md de handoff con state es opcional, solo se genera si el foco lo requiere. Usar para derivar una tarea concreta (ej. "ejecuta el plan X") a una sesion limpia o pasarle el laburo a otro agente.
---

Construye un prompt para iniciar una nueva sesion (en otra ventana, otro modelo, u otro humano), a partir de un foco explicito o de la conversacion actual. El prompt es el artefacto principal y se copia al clipboard; el handoff `.md` con state es opcional, solo se genera si el foco lo requiere. Usar cuando queres pasarle el laburo a otro agente, derivar una tarea concreta (ej. "ejecuta el plan X") a una sesion limpia, o resetear context preservando lo necesario.

Skill **one-shot**: corre una vez, genera el prompt (y opcionalmente un `.md` en path temporal), reporta. No persiste estado en el repo.

## Argumentos

```text
/handoff [<foco de la proxima sesion>]
```

- Si los argumentos estan vacios, el foco se infiere de la conversacion en curso.
- Si hay argumentos, son el foco explicito ("ejecuta el plan `.harness/plans/3-foo.md`", "seguir con el fix de auth", "abri el PR del plan", etc.).

## Pre-flight

### 1. Determinar el modo

Dos modos posibles:

- **A. Continuacion de sesion**: el proximo agente sigue la tarea en curso desde donde quedo. State grande (decisiones, en curso, bloqueos) que no vive en repo/PR/issue. Output: `.md` de handoff + prompt `"Lee <file> y continua..."`.
- **B. Tarea derivada**: el proximo agente arranca una tarea concreta (ej. ejecutar un plan, abrir un PR, correr `/hs-auto` sobre un issue). El context util ya vive en repo/PR/issue/plan. Output: prompt autocontenido apuntando a esos refs, **sin** `.md` intermediario salvo que haya state extra.

Inferir el modo:

- Sin argumentos o con hint tipo "seguir/continuar/retomar" => modo A.
- Con argumentos que nombran una accion concreta + ref (path/PR/issue) => modo B.
- Ambiguo (no esta claro si quiere continuar todo o solo derivar la tarea) => preguntar con multiple choice (regla del profile: opciones numeradas + "otra"). Si el foco es vago de origen ("hace algo con el plan"), lanzar `/clarify` con los argumentos como input antes de seguir.

### 2. Detectar el directorio temporal (solo si modo A o modo B con state extra)

```bash
TMPDIR="${TMPDIR:-/tmp}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
HANDOFF_FILE="$TMPDIR/cvm-handoff-$TS.md"
```

Nunca escribir el handoff dentro del repo; es efimero y no debe commitearse.

### 3. Detectar repo, branch y cwd (si aplica)

```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
git rev-parse --abbrev-ref HEAD 2>/dev/null
pwd
```

Capturar `REPO`, `BRANCH`, `CWD`. Si no aplica, marcar como `null`; el skill funciona igual fuera de repo.

## Ejecutar

### Modo A: continuacion de sesion

1. **Identificar referencias externas** de la conversacion (sin re-leer todo, solo lo que recordas del thread):
   - PRs abiertos o mencionados.
   - Issues abiertos o mencionados.
   - Paths a `.harness/plans/<N>-<slug>.md` si aplica.
   - URLs externas relevantes (docs, ADRs, dashboards).
   - Archivos del repo modificados en la sesion.

   Cada referencia se cita por URL o path absoluto; no se copia su contenido al handoff.

2. **Redactar el handoff** al path `$HANDOFF_FILE`:

   ```markdown
   # Handoff - <TS UTC>

   ## Foco de la proxima sesion

   <una linea: que va a hacer el proximo agente.>

   ## Contexto

   <2 a 4 parrafos: que estamos haciendo y por que. NO transcribir la conversacion mensaje a mensaje. Resumir.>

   ## Decisiones tomadas

   - <decision 1>: <una linea de motivacion>
   - <decision 2>: ...

   ## En curso

   - <que esta a medio hacer y donde quedo>
   - <que quedo bloqueado y por que>

   ## Proximos pasos

   1. <accion concreta>
   2. ...

   ## Referencias

   - Repo: <REPO o "(fuera de repo)">
   - Branch: <BRANCH o "(N/A)">
   - CWD: <CWD>
   - PR / Issue en juego: <URL o "(ninguno)">
   - Plan: <path relativo en el repo o "(ninguno)">
   - Spec: <URL o "(ninguna)">
   - Archivos relevantes:
     - <path 1>
     - <path 2>

   ## Skills sugeridos para el proximo agente

   - `/<skill>`: <por que conviene usarlo en esta tarea>
   - ...

   ## Generado

   - Fecha: <TS UTC>
   - Origen: handoff desde sesion OpenCode
   ```

3. **Prompt de continuacion**:

   ```text
   Lee <HANDOFF_FILE> y continua desde donde quedo la sesion anterior.
   ```

### Modo B: tarea derivada

1. **Identificar los refs minimos** que el proximo agente necesita: path del plan, URL del PR/issue, branch, cwd. Lo que viva en repo/PR/issue/plan **no se duplica**, se referencia.

2. **Construir el prompt autocontenido** tailoreado al foco. Estructura sugerida (no rigida; adaptar al caso):

   ```text
   <objetivo concreto en imperativo, derivado de los argumentos>

   Refs:
   - Repo: <REPO>
   - Branch: <BRANCH>
   - CWD: <CWD>
   - Plan / PR / issue: <path o URL>
   - Otros: <si aplica>

   Skills sugeridos: /<skill1>, /<skill2>

   <si hay restricciones o decisiones no obvias de la sesion actual que aplican: 1-3 lineas.>
   ```

   Ejemplos de prompt segun el foco:

   - `/handoff ejecuta el plan` (con plan armado en la sesion):
     ```text
     Ejecuta el plan paso a paso, validando build/tests antes de cada push.

     Refs:
     - Repo: anthropics/cvm
     - Branch: feat/foo
     - CWD: /Users/.../cvm
     - Plan: .harness/plans/3-foo.md

     Skills sugeridos: /hs-auto
     ```

   - `/handoff abri PR del plan`:
     ```text
     Abri el PR del plan listo en `.harness/plans/3-foo.md`. Spec: <URL>. Aplica labels entity:plan.
     ```

3. **`.md` opcional**: solo escribir `$HANDOFF_FILE` si hay state de la sesion que **no** vive en repo/PR/issue y que el proximo agente necesita (ej. decisiones tomadas verbalmente, asumpciones, alternativas descartadas). Si no hay tal state, **no generar `.md`**.

### Reglas de redaccion (ambos modos)

- **No duplicar**: si algo ya esta en un PR, issue, plan o commit, referenciarlo por URL/path en lugar de copiarlo.
- **Sintetizar, no transcribir**: el handoff es un resumen, no un log.
- **Redactar info sensible**: API keys, passwords, tokens, PII; no copiarlos al prompt ni al `.md` aunque aparezcan en la conversacion. Marcar `[redactado]` donde haga falta.
- **Sugerir 1 a 3 skills** del profile activo que el proximo agente va a necesitar. Mirar `ls profiles/harness/opencode/skills/` (o el profile que aplique) y elegir los relevantes a la tarea.

### Copiar prompt al clipboard

```bash
if command -v wl-copy >/dev/null 2>&1; then
  printf '%s' "$PROMPT" | wl-copy
  CLIPBOARD_OK=1
elif command -v xclip >/dev/null 2>&1; then
  printf '%s' "$PROMPT" | xclip -selection clipboard
  CLIPBOARD_OK=1
elif command -v pbcopy >/dev/null 2>&1; then
  printf '%s' "$PROMPT" | pbcopy
  CLIPBOARD_OK=1
else
  CLIPBOARD_OK=0
fi
```

Si ninguna herramienta esta disponible, no fallar; solo omitir el copy y reportarlo abajo.

## Reporte

```text
## /handoff report

- modo: <A continuacion | B tarea derivada>
- file: <HANDOFF_FILE o "(no generado)">
- referenced:
  - PRs: <N>
  - issues: <N>
  - paths: <N>
- skills sugeridos: <lista corta>

### Prompt para la proxima sesion

<imprimir el prompt completo aca, tal como quedo en el clipboard>

### Para continuar

<si CLIPBOARD_OK=1:>
Prompt copiado al clipboard. Hace `/clear` y pega (Ctrl+Shift+V), o abri una nueva terminal y pega.

<si CLIPBOARD_OK=0:>
Clipboard no disponible (instalar `wl-copy`, `xclip` o `pbcopy`). Copiar a mano una de las dos opciones:

- Opcion A, mismo terminal: hacer `/clear` y pegar el prompt de arriba.
- Opcion B, nueva terminal: `opencode "<prompt>"`.
```

A diferencia del comportamiento previo, el prompt **si** se imprime entero en el chat (es el artefacto principal del skill). El `.md` (cuando existe) sigue siendo path-only.

## MUST DO

- Determinar el modo (A o B) antes de generar nada. Si es ambiguo, preguntar con multiple choice o lanzar `/clarify`.
- En modo A, escribir el handoff a `$TMPDIR/cvm-handoff-<TS>.md`, fuera del repo.
- En modo B, generar `.md` **solo** si hay state que no vive ya en repo/PR/issue.
- Referenciar PR/issue/plan/commits por URL o path; no copiar su contenido.
- Redactar info sensible (keys, passwords, tokens, PII).
- Sugerir skills concretos del profile actual que aplican a la tarea siguiente.
- Imprimir el prompt completo en el reporte.

## MUST NOT DO

- No escribir el handoff dentro del repo. Es efimero.
- No generar `.md` en modo B si los refs alcanzan; eso duplica.
- No transcribir la conversacion mensaje a mensaje; sintetizar.
- No copiar API keys, passwords, tokens o PII aunque aparezcan en la conversacion.
- No persistir nada en auto-memory.
- No actuar sobre un foco ambiguo sin clarificar primero.

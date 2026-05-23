---
name: handoff
description: Compacta la conversacion actual en un documento de handoff para que otro agente la continue; resume contexto, decisiones, trabajo en curso y proximos pasos, referenciando PRs/issues/plans sin duplicar su contenido. Usar cuando la sesion se va a cerrar a mitad de tarea o queres pasarle el laburo a otro agente.
---

Compacta la conversacion actual en un documento de handoff para que otro agente (en otra sesion, otro modelo, u otro humano) la continue. Resume contexto, decisiones tomadas, trabajo en curso y proximos pasos. Sin duplicar contenido que ya vive en PRs/issues/commits; referenciarlos por URL o path. Usar cuando la sesion se va a cerrar a mitad de tarea, queres pasarle el laburo a otro agente, o la conversacion crecio mucho y necesitas un reset preservando el state.

Skill **one-shot**: corre una vez, genera el doc en un path temporal del sistema, reporta el path. No persiste estado en el repo.

## Argumentos

```text
/handoff [<foco del proximo agente>]
```

- Si los argumentos estan vacios, generar un handoff generico (continuar la tarea en curso).
- Si hay argumentos, son una hint sobre que va a hacer la proxima sesion ("seguir con el fix de auth", "implementar la fase 2 del plan", etc.). Tailorear el doc en consecuencia.

## Pre-flight

### 1. Detectar el directorio temporal

```bash
TMPDIR="${TMPDIR:-/tmp}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
HANDOFF_FILE="$TMPDIR/cvm-handoff-$TS.md"
```

Nunca escribir el handoff dentro del repo; es efimero y no debe commitearse.

### 2. Detectar repo y branch (si aplica)

```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
git rev-parse --abbrev-ref HEAD 2>/dev/null
```

Capturar `REPO` y `BRANCH` si estan disponibles. Si no, marcar como `null`; el skill funciona igual fuera de repo.

## Ejecutar

### 1. Identificar referencias externas

Recopilar de la conversacion (sin re-leer todo, solo lo que recordas del thread):

- PRs abiertos o mencionados.
- Issues abiertos o mencionados.
- Paths a `.harness/plans/<N>-<slug>.md` si aplica.
- URLs externas relevantes (docs, ADRs, dashboards).
- Archivos del repo modificados en la sesion.

Cada referencia se cita por URL o path absoluto; no se copia su contenido al handoff.

### 2. Redactar el handoff

Estructura del doc:

```markdown
# Handoff - <TS UTC>

## Foco de la proxima sesion

<una linea: que va a hacer el proximo agente. Si hubo argumentos, usarlos como base.>

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

### 3. Reglas de redaccion

- **No duplicar**: si algo ya esta en un PR, issue, plan o commit, referenciarlo por URL/path en lugar de copiarlo.
- **Sintetizar, no transcribir**: el handoff es un resumen, no un log.
- **Redactar info sensible**: API keys, passwords, tokens, PII; no copiarlos al handoff aunque aparezcan en la conversacion. Marcar `[redactado]` donde haga falta.
- **Sugerir 1 a 3 skills** del profile activo que el proximo agente va a necesitar. Mirar `ls profiles/harness/opencode/skills/` (o el profile que aplique) y elegir los relevantes a la tarea.

Escribir al path `$HANDOFF_FILE`.

### 4. Copiar prompt de continuacion al clipboard

Despues de escribir el handoff, copiar al clipboard del sistema el prompt que la proxima sesion va a recibir, para que el usuario solo tenga que pegar:

```bash
PROMPT="Lee $HANDOFF_FILE y continua desde donde quedo la sesion anterior."
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

- file: <HANDOFF_FILE>
- lines: <linea count>
- referenced:
  - PRs: <N>
  - issues: <N>
  - paths: <N>
- skills sugeridos: <lista corta>

### Para continuar

<si CLIPBOARD_OK=1:>
Prompt copiado al clipboard. Hace `/clear` y pega (Ctrl+Shift+V).

<si CLIPBOARD_OK=0:>
Clipboard no disponible (instalar `wl-copy`, `xclip` o `pbcopy`). Copiar a mano una de las dos opciones:

- Opcion A, mismo terminal: hacer `/clear` y pegar:

      Lee <HANDOFF_FILE> y continua desde donde quedo la sesion anterior.

- Opcion B, nueva terminal:

      opencode "Lee <HANDOFF_FILE> y continua desde donde quedo la sesion anterior."
```

Sustituir `<HANDOFF_FILE>` por el path real en el reporte. No imprimir el contenido completo del handoff en el chat; solo el path y el resumen del reporte.

## MUST DO

- Escribir el handoff a `$TMPDIR/cvm-handoff-<TS>.md`, fuera del repo (es efimero, no se commitea).
- Referenciar PR/issue/plan/commits por URL o path; no copiar su contenido al handoff.
- Redactar info sensible (keys, passwords, tokens, PII) antes de escribir.
- Sugerir skills concretos del profile actual que aplican a la tarea siguiente.

## MUST NOT DO

- No escribir el handoff dentro del repo. Es efimero.
- No transcribir la conversacion mensaje a mensaje; sintetizar.
- No copiar API keys, passwords, tokens o PII aunque aparezcan en la conversacion.
- No persistir nada en auto-memory.
- No imprimir el handoff completo en el chat; solo el path y el reporte.

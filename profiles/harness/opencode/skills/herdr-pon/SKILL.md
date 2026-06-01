---
name: herdr-pon
description: Arma un loop ping-pong automatico entre el pane actual y un pane validador en herdr. Usar para rondas de validacion paralelas que avisan con accionables hasta verde o hasta un cap.
---

Arma un loop ping-pong automatico entre el pane actual (A, caller) y un pane validador (B) corriendo en `herdr`: A le pasa a B una tarea de validacion, B corre y avisa de vuelta con un accionable, A arregla y le contesta a B, y siguen hasta verde o hasta un cap de rondas.

Construye sobre `/herdr-detach`: reusa su pre-flight, lanzamiento del agente, manejo de dialogos y limpieza de panes stray. Lo nuevo es el protocolo callback: `state.json`, `output.md` y un helper `ping.sh` generado en runtime.

## Argumentos

```text
/herdr-pon [--here|--new] [--max-rounds N] <agente> <prompt>
```

- `<agente>`: `claude` u `opencode`. `codex` no esta soportado.
- `<prompt>`: tarea de validacion para B. Todo lo que sigue al agente se concatena.
- `--max-rounds N`: default `5`, entero mayor o igual a 1.
- `--here`: split del pane origen, default.
- `--new`: workspace nuevo dedicado.

El pane validador siempre queda abierto.

## Modelo Mental

```text
pane A (caller)                         pane B (validador)
  |--- lanza B + contrato ronda 1 ------>|
  |                                      | corre validacion
  |<-- ping accionable si falla ---------|
  | arregla                              |
  |--- ping "revalida" ---------------->|
  |<-- VERDE o CAP ----------------------|
```

El skill solo hace setup y kickoff. Despues el loop se auto-sostiene: cada ping es un turno nuevo en la sesion persistente del otro lado. A debe volver al usuario inmediatamente con el reporte inicial; no espera activamente el resultado de B y tampoco sigue trabajando por su cuenta. Queda idle hasta que B pinguee.

## Pre-Flight

Seguir `/herdr-detach` para validar `herdr`, server, binario del agente, integracion, `HERDR_PANE_ID`, workspace y cwd. Diferencias:

- Parsear `--max-rounds N` y validar entero `>= 1`.
- Validar agente contra `{claude, opencode}`; abortar con `codex`.
- Capturar handle estable de A con `A_TERM=$(herdr pane get "$HERDR_PANE_ID" | jq -r .result.pane.terminal_id)`. Usar terminal id, no nombre de agente ni pane id.

## Estado Compartido

Crear estado bajo el cwd base del caller:

```bash
STATE_DIR="$CWD_BASE/.herdr-pon/run-<TS>"
mkdir -p "$STATE_DIR"
```

Definir rutas absolutas:

- `STATE_FILE=$STATE_DIR/state.json`
- `OUTPUT_FILE=$STATE_DIR/output.md`
- `PING_SH=$STATE_DIR/ping.sh`

Escribir `state.json` inicial despues de lanzar B:

```json
{
  "round": 1,
  "max_rounds": <N>,
  "pane_a": "<A_TERM>",
  "pane_b": "<B_TERM>",
  "status": "running"
}
```

Escribir `ping.sh` y hacerlo ejecutable:

```bash
#!/usr/bin/env bash
set -uo pipefail
PEER="${1:?falta peer handle}"
MSG_FILE="${2:?falta msg file}"
[ -f "$MSG_FILE" ] || { echo "msg file inexistente: $MSG_FILE" >&2; exit 1; }

herdr agent send "$PEER" "$(cat "$MSG_FILE")"
sleep 0.5
for try in 1 2; do
  PANE=$(herdr agent get "$PEER" | jq -r .result.agent.pane_id)
  herdr pane send-keys "$PANE" Enter
  sleep 1
  ST=$(herdr agent get "$PEER" | jq -r .result.agent.agent_status)
  [ "$ST" = "working" ] && exit 0
done
echo "warn: no se confirmo transicion a working para $PEER" >&2
```

## Lanzar B

Usar el flujo de `/herdr-detach` con `B_NAME="pon-<AGENT>-<TS>"`:

- `--here`: `herdr agent start "$B_NAME" --workspace <WORKSPACE_ID> --tab <TAB_ID> --cwd "$CWD_BASE" --split right --no-focus -- <AGENT>`.
- `--new`: `herdr pane run "$ROOT_PANE_ID" "<AGENT>"`, esperar deteccion sobre `ROOT_TERM`, y renombrar a `B_NAME`.
- Cerrar shells pelados creados por el launch, operando por `terminal_id`.
- Resolver `B_TERM=$(herdr agent get "$B_NAME" | jq -r .result.agent.terminal_id)`.

## Kickoff

Enviar a B un contrato self-describing con estas partes resueltas a valores absolutos:

```text
Sos el agente VALIDADOR (pane B) en un loop ping-pong AUTOMATICO con el pane A. Segui este contrato al pie.

ESTADO COMPARTIDO:
- state : <STATE_FILE>
- output: <OUTPUT_FILE>
- ping  : <PING_SH>

HANDLES:
- Tu peer es A: <A_TERM>. Para avisarle: escribi tu mensaje a un archivo y corre `bash <PING_SH> <A_TERM> <archivo>`.
- Tu propio handle es B: <B_TERM>. A te va a contestar escribiendo en tu input.

TAREA DE VALIDACION:
<USER_PROMPT>

LOOP:
1. Lee state y toma round/max_rounds.
2. Corre la validacion de verdad. Escribi resultado completo en output, encabezado `## Ronda <round>`.
3. Si todo pasa: status="green", ping a A con VERDE, termina.
4. Si falla: incrementa round. Si round > max_rounds: status="capped", ping a A con CAP, termina. Si no, ping accionable a A con resumen y ruta de output. Queda idle esperando revalidacion.
```

Enviar el contrato con el mismo mecanismo robusto de `/herdr-detach`: temp file, `agent send`, `sleep 0.5`, `Enter`, reintento maximo 2.

Despues del envio, NO esperar activamente el resultado de B. No llamar `herdr agent wait`, no leer/pollear el pane B, no bloquear el turno del skill. Devolver el reporte inicial y terminar; B va a escribirle a A mediante `ping.sh` cuando tenga un resultado accionable.

Importante: tras devolver el reporte inicial, A no debe avanzar con mas trabajo relacionado ni empezar nuevas rondas por iniciativa propia. El estado correcto es idle: esperar a que B mande el siguiente ping y recien ahi actuar. Esto evita acumular trabajo mientras el validador todavia esta corriendo.

## Comportamiento De A

Cuando B pinguea:

- Ping accionable: leer `OUTPUT_FILE`, aplicar arreglos, escribir mensaje corto a temp file y revalidar con `bash <PING_SH> <B_TERM> <archivo>`.
- Ping VERDE: reportar cierre exitoso, re-resolver el pane actual de B con `herdr agent get <B_TERM> | jq -r .result.agent.pane_id`, cerrar ese pane con `herdr pane close <pane>`, y no volver a pinguear.
- Ping CAP: reportar corte por cap y apuntar a `OUTPUT_FILE`.
- Si queda ambiguo, leer `STATE_FILE` antes de actuar.
- Si no llego ping de B, no hacer nada mas en este loop.

## Reporte Inicial

```text
## /herdr-pon iniciado

- validador (B): <AGENT> @ <B_NAME> (term <B_TERM>), pane <B_PANE>
- caller (A): term <A_TERM>
- modo: <here | new>
- max_rounds: <N>
- state: <STATE_FILE>
- output: <OUTPUT_FILE>
- ronda 1 disparada en B.

B valida y pinponea conmigo solo hasta verde o hasta <N> rondas.

Inspeccionar B: `herdr agent read <B_TERM> --source visible`
Frenar a mano: `herdr pane close <B_PANE>`
```

## MUST DO

- Validar agente contra `{claude, opencode}`.
- Reusar el flujo seguro de `/herdr-detach` para launch, dialogos y cleanup.
- Direccionar por terminal id (`A_TERM`, `B_TERM`), no por nombre ni pane id.
- Generar `state.json`, `output.md` y `ping.sh` en `.herdr-pon/run-<TS>/`.
- Usar rutas absolutas en el contrato.
- Respetar `max_rounds` siempre.
- Tras disparar la ronda 1, responder el reporte inicial y quedar idle hasta el ping de B.
- Cuando B devuelve VERDE, cerrar el pane validador que dio el verde.

## MUST NOT DO

- No soportar `codex`.
- No bloquear con `--wait` ni pollear a B.
- No quedarse esperando el resultado de B despues del kickoff; el resultado llega por callback/ping.
- No seguir trabajando ni acumular tareas despues del kickoff; solo actuar cuando B pinguea.
- No correr el loop sin tope.
- No mandar el contrato en cada ronda.
- No focusear ni cerrar B por default; excepcion: cerrar B cuando devuelve VERDE.
- No agregar `.herdr-pon/` a `.gitignore` salvo pedido explicito.
- No persistir nada en memoria.

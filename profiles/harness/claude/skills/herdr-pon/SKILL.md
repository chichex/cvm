Arma un loop ping-pong automatico entre el pane actual (A, el que invoca) y un pane validador (B) corriendo en `herdr`: A le pasa a B una tarea de validacion, B la corre y le **avisa de vuelta** a A con un accionable, A arregla y le contesta a B en su propio pane, y siguen rebotando solos hasta verde o hasta un cap de rondas. Usar cuando queres delegar rondas de validacion (tests, lint, review) a una sesion paralela visible que te pinguee cuando hay algo que hacer, en vez de pollear vos.

Skill **exclusivo para Claude Code** (depende del tool `Bash` para hablar con la CLI `herdr`). Construye **sobre `herdr-detach`**: reusa todo su pre-flight, lanzamiento del agente, manejo de dialogos de pre-arranque y limpieza de panes stray. Lo nuevo de `herdr-pon` es la **capa de protocolo callback**: un contrato self-describing embebido en cada ping + un helper `ping.sh` generado en runtime, que convierten el detach pull (A pollea con `--wait`) en un loop push bidireccional (A y B se escriben solos).

Asume que la sesion actual ya corre dentro de `herdr` y se ancla al pane origen via `HERDR_PANE_ID` (invariante al focus).

## Argumentos

```text
/herdr-pon [--here|--new] [--max-rounds N] <agente> <prompt>
```

- `<agente>`: `claude` | `opencode`. **`codex` NO esta soportado** (su dialogo de update y su modelo de turnos rompen el loop) — abortar si lo pasan.
- `<prompt>`: texto libre con la tarea de validacion que corre B. Todo lo que sigue al `<agente>` se concatena.
- `--max-rounds N`: tope de rondas del loop. **Default `5`**. Al alcanzarlo, el lado que toca corta y avisa al usuario en vez de seguir.
- `--here`: split del pane que **origino** la invocacion (anclado via `HERDR_PANE_ID`, no al focused). Split a la derecha. **Default**.
- `--new`: workspace nuevo dedicado.

El pane validador (B) siempre se deja abierto.

## Modelo mental

```
pane A (vos, el caller)                      pane B (validador)
  |  /herdr-pon "corre tests"                  |
  |------- lanza B + contrato + ronda 1 ------>|
  |                                            |  corre validacion
  |                                            |  escribe output -> OUTPUT_FILE
  |<------ ping accionable (push, +Enter) -----|  (si falla)
  |  arregla                                   |  ...idle, esperando...
  |------- ping "revalida" (push, +Enter) ---->|  vuelve a validar
  |             ...repite hasta...             |
  |<------ ping "VERDE, loop cerrado" ---------|  (si pasa, o si cap)
  |  reporta al usuario, fin                   |  fin
```

El skill **solo hace setup + kickoff**. Despues el loop se auto-sostiene: cada ping es un turno nuevo en la sesion persistente del otro lado, que recuerda el contrato. **A y B son sesiones persistentes** — el contrato se establece una vez en el prompt inicial de B; los pings siguientes pueden ser cortos.

Ownership del veredicto: **B decide** verde/falla (es quien corre la validacion) e incrementa la ronda. A solo arregla y revalida.

## Pre-flight

Identico a `herdr-detach` §1–§6, con tres diferencias. **No reimplementar** — seguir `herdr-detach` para binario/server/integracion/resolucion de workspace/pane base. Diferencias:

### P1. Parseo de `$ARGUMENTS`
- Extraer flags `--here`/`--new` (excluyentes, default `--here`) y `--max-rounds N` (default `5`; validar que `N` sea entero ≥ 1) antes del primer token no-flag.
- Primer token no-flag = `AGENT`. Validar contra **`{claude, opencode}`** — `codex` y cualquier otro valor cortan el flujo:

  ```text
  /herdr-pon soporta validador claude u opencode (codex no). Pasame uno de esos.
  ```
- El resto concatenado = `USER_PROMPT` (la tarea de validacion). Si esta vacio, pedirlo; si responde vacio, abortar.

### P2. Capturar el handle estable de A (caller) — paso NUEVO
B necesita poder escribirle a A. El handle de direccionamiento del loop es el **`terminal_id`** del pane origen: es **único y estable** (el `pane_id` se renumera al cerrar panes, y el *nombre* de agente NO es único — varias sesiones comparten el nombre `claude`/`opencode` por default, así que `agent send claude` sería ambiguo). herdr acepta terminal ids como target en `agent get/send/read/wait`.

Resolver desde `HERDR_PANE_ID` (ojo el path: `.result.pane.*`, no `.result.*`):

```bash
A_TERM=$(herdr pane get "$HERDR_PANE_ID" | jq -r .result.pane.terminal_id)
```

`A_TERM` es el handle de A (`A_HANDLE`) para todo el loop. **No** renombrar el pane de A (es el del usuario; el terminal_id ya alcanza). `<TS>` = `date +%s`.

### P3. Resto del pre-flight
Igual que `herdr-detach`: validar `herdr` en PATH + server `running`, `command -v <AGENT>`, auto-instalar la integracion de `<AGENT>` si esta `not installed`/`outdated`, resolver `WORKSPACE_ID`/`TAB_ID`/`CWD_BASE` (y `ROOT_PANE_ID`/`ROOT_TERM` en `--new`).

## Setup del loop (estado compartido)

Crear el directorio de estado en el cwd del caller (A y B comparten cwd → la ruta relativa resuelve al mismo absoluto, pero **usar siempre rutas absolutas** en el contrato). `<TS>` ya definido arriba:

```bash
STATE_DIR="$CWD_BASE/.herdr-pon/run-<TS>"
mkdir -p "$STATE_DIR"
```

Resolver a absoluto y definir:
- `STATE_FILE = $STATE_DIR/state.json`
- `OUTPUT_FILE = $STATE_DIR/output.md`
- `PING_SH = $STATE_DIR/ping.sh`

`.herdr-pon/` es transiente y seguro de borrar; si el repo lo trackea, mencionarlo en el reporte (no agregarlo a `.gitignore` salvo que el usuario lo pida).

### state.json inicial
Escribir via Write (no heredoc):

```json
{
  "round": 1,
  "max_rounds": <N>,
  "pane_a": "<A_TERM>",
  "pane_b": "<B_TERM>",
  "status": "running"
}
```

`<B_TERM>` (terminal_id de B) se completa tras lanzar B (abajo) — escribir el archivo despues de tenerlo, o reescribir el campo.

### ping.sh (helper de envio robusto)
Centraliza el "send + enter" fragil (el race del TTY y el caso pasted-text que `herdr-detach` §3 documenta) en un solo lugar, asi ni A ni B lo reimplementan. **Dos gotchas reales con validador `opencode`** (verificados — con `claude` no se notan): (1) la tecla de submit es **`enter` MINUSCULA**; `Enter` capitalizada inserta un newline en el input de opencode y NO submitea, asi que el ping queda sin disparar. (2) opencode puede quedar en **"shell mode"** (footer `esc exit shell mode`): ahi el texto pegado corre como **comando zsh** en vez de prompt de chat (`command not found: ...`). Un `esc` previo lo saca a modo chat — pero **`esc` NO se manda a claude** (ahi cancela/interrumpe el input). El helper detecta el tipo de peer (`.result.agent.agent`) y solo aplica el esc-guard a opencode. Escribir via Write y `chmod +x`:

```bash
#!/usr/bin/env bash
# herdr-pon: escribe <msg_file> en el input del agente <peer> y lo submitea.
# Confirma el envio esperando la transicion a "working" (hasta 3 enters).
# Uso: ping.sh <peer_handle> <msg_file>
set -uo pipefail
PEER="${1:?falta peer handle}"
MSG_FILE="${2:?falta msg file}"
[ -f "$MSG_FILE" ] || { echo "msg file inexistente: $MSG_FILE" >&2; exit 1; }

AGENT=$(herdr agent get "$PEER" | jq -r .result.agent.agent)
PANE=$(herdr agent get "$PEER" | jq -r .result.agent.pane_id)

# opencode puede quedar en "shell mode" (footer "esc exit shell mode"): ahi el texto corre
# como comando zsh en vez de prompt. Un esc previo garantiza modo chat. NO mandar esc a claude.
[ "$AGENT" = "opencode" ] && { herdr pane send-keys "$PANE" esc; sleep 0.3; }

herdr agent send "$PEER" "$(cat "$MSG_FILE")"
sleep 0.5

# opencode: si el paste cayo igual en shell mode, salir y reenviar una vez
if [ "$AGENT" = "opencode" ] && herdr agent read "$PEER" --source visible 2>/dev/null | grep -q "exit shell mode"; then
  herdr pane send-keys "$PANE" esc; sleep 0.3
  herdr agent send "$PEER" "$(cat "$MSG_FILE")"; sleep 0.5
fi

# submit con 'enter' MINUSCULA (la 'Enter' capitalizada con opencode inserta newline, no submitea)
for try in 1 2 3; do
  PANE=$(herdr agent get "$PEER" | jq -r .result.agent.pane_id)
  herdr pane send-keys "$PANE" enter
  sleep 1
  ST=$(herdr agent get "$PEER" | jq -r .result.agent.agent_status)
  [ "$ST" = "working" ] && exit 0
done
# best-effort: el texto quedo en el input aunque no detectamos "working"
echo "warn: no se confirmo transicion a working para $PEER" >&2
```

## Lanzar B + kickoff

### L1. Lanzar el validador
Igual que `herdr-detach` §1 (+ §1b cleanup de stray panes). `AGENT_NAME` = `B_NAME` = `pon-<AGENT>-<TS>`.
- `--here`: snapshot `TERMS_BEFORE`, luego `herdr agent start "$B_NAME" --workspace ... --tab ... --cwd "$CWD_BASE" --split right --no-focus -- <AGENT>`.
- `--new`: lanzar dentro del `root_pane` con `herdr pane run "$ROOT_PANE_ID" "<AGENT>"`, esperar deteccion sobre `ROOT_TERM`, `herdr agent rename "$ROOT_TERM" "$B_NAME"`.
- §1b: cerrar shells pelados (`agent: null`) por `terminal_id`, re-resolver el `pane_id` de B despues.

Capturar `B_TERM=$(herdr agent get "$B_NAME" | jq -r .result.agent.terminal_id)` y completar `pane_b` en `state.json`. De acá en más, el loop direcciona a B por `B_TERM` (terminal_id, único+estable); `B_NAME` ya cumplió su rol en el launch/diálogos.

### L2. Dialogos de pre-arranque
Igual que `herdr-detach` §2: loop hasta 4 iteraciones, trust-folder `Enter`, Bypass `Down`+`Enter`. (No hay rama codex porque `codex` no esta soportado.) Confirmar "agente listo" con las tres condiciones combinadas (status `idle` + prompt char al inicio de la ultima linea + ausencia de patrones de menu).

### L3. Enviar el contrato + disparar ronda 1
Construir el prompt inicial de B = **contrato de protocolo** (template abajo, con los placeholders resueltos a valores reales/absolutos). Escribirlo a un temp file y enviarlo con el mecanismo de `herdr-detach` §3 (`agent send` via `"$(cat tmp)"` → `sleep 0.5` → re-resolver `pane_id` → `enter` minúscula, reintento). **Con validador `opencode`** aplican los dos gotchas del helper: submit con `enter` minúscula (no `Enter`) y, si el footer muestra `esc exit shell mode`, mandar `esc` antes y reenviar (no a claude). Reusar `ping.sh` para esto en vez de rearmarlo a mano.

#### Contrato (prompt inicial de B)

```text
Sos el agente VALIDADOR (pane B) en un loop ping-pong AUTOMATICO con el pane A, que te lanzo. Segui este contrato al pie y no lo rompas.

ESTADO COMPARTIDO (rutas absolutas):
- state : <STATE_FILE>      (JSON: round, max_rounds, status)
- output: <OUTPUT_FILE>     (donde escribis el resultado de cada ronda)
- ping  : <PING_SH>         (helper para avisarle al otro pane)

HANDLES:
- Tu peer es A, handle (terminal_id) "<A_TERM>". Para avisarle: escribi tu mensaje a un archivo temporal (mktemp) y corre:  bash <PING_SH> <A_TERM> <ese_archivo>
- Tu propio handle es "<B_TERM>". A te va a contestar escribiendo en TU input (un turno nuevo): cuando eso pase, volves al paso 1.

TAREA DE VALIDACION:
<USER_PROMPT>

LOOP — empeza la RONDA 1 AHORA:
1. Lee <STATE_FILE> y tomá `round` y `max_rounds`.
2. Corre la validacion. Escribi el resultado COMPLETO de esta ronda en <OUTPUT_FILE>, encabezado con "## Ronda <round>" (sobreescribi el archivo, no acumules rondas viejas).
3. SI TODO PASA (verde, nada que arreglar):
   - Actualiza <STATE_FILE>: status="green".
   - Mensaje a A: "VERDE en ronda <round>: la validacion paso, no hay nada que arreglar. Loop cerrado." -> bash <PING_SH> <A_TERM> <msg>.
   - NO esperes respuesta. TERMINA el loop.
4. SI HAY FALLAS:
   a. Incrementa `round` en <STATE_FILE> (round = round + 1).
   b. Si el nuevo `round` > `max_rounds`:
      - Actualiza status="capped".
      - Mensaje a A: "CAP de rondas (<max_rounds>) alcanzado sin pasar. Corto el loop. Ultimo resultado en <OUTPUT_FILE>." -> bash <PING_SH> <A_TERM> <msg>.
      - TERMINA.
   c. Si no llegaste al cap, mensaje ACCIONABLE a A:
      "Ronda <round_anterior> fallo. Resultado completo en <OUTPUT_FILE>. Resumen: <que fallo y que hay que arreglar, concreto>. Cuando termines de arreglar, revalidame: escribi un mensaje corto (ej. 'arreglado X, revalida') a un archivo y corre  bash <PING_SH> <B_TERM> <archivo>."
      -> bash <PING_SH> <A_TERM> <msg>.
   d. TERMINA TU TURNO y quedate idle. Cuando A te revalide, recibis un prompt nuevo en tu input -> volves al paso 1.

REGLAS:
- No cierres tu pane. No toques el pane de A salvo via <PING_SH>.
- Nunca reportes un resultado de validacion que no corriste de verdad.
- Mensajes a A: cortos y accionables; el detalle va en <OUTPUT_FILE>.
```

### L4. Fire-and-forget
No bloquear (el loop es push). Reportar al usuario y devolver el control.

## Comportamiento de A (responder) — cuando B te pinguea

Tras el kickoff, el skill termina pero **vos (A) seguis en el loop**. Cada ping de B llega como un prompt nuevo en tu input. El ping es self-describing, pero el protocolo es:

- **Ping accionable** ("Ronda N fallo… arregla… revalidame con `bash <PING_SH> <B_TERM> <archivo>`"):
  1. Lee `<OUTPUT_FILE>` para el detalle.
  2. Aplica los arreglos.
  3. Escribi un mensaje corto a un temp file y revalida a B: `bash <PING_SH> <B_TERM> <archivo>`.
  4. Volves a quedar disponible; el proximo ping de B reanuda el ciclo.
- **Ping VERDE** ("VERDE en ronda N… loop cerrado"): el loop termino OK. Reportar al usuario el cierre y la ronda. No volver a pinguear a B.
- **Ping CAP** ("CAP de rondas alcanzado…"): el loop se corto sin pasar. Reportar al usuario, apuntar a `<OUTPUT_FILE>`, sugerir revisar a mano o re-lanzar con `--max-rounds` mas alto. No volver a pinguear a B.

Si un ping queda ambiguo, leer `<STATE_FILE>` (`status`/`round`) para resolver el estado antes de actuar.

## Reporte (post-kickoff)

Re-resolver el `pane_id` de B fresh antes del reporte:

```text
## /herdr-pon iniciado

- validador (B): <AGENT> @ <B_NAME> (term <B_TERM>), pane <B_PANE>
- caller (A): term <A_TERM>
- modo: <here | new>
- max_rounds: <N>
- state: <STATE_FILE>
- output: <OUTPUT_FILE>
- ronda 1 disparada en B.

B valida y pinponea conmigo solo hasta verde o hasta <N> rondas. Cuando me avise, sigo el accionable (arreglo -> revalido) y te aviso cuando el loop cierre.

Inspeccionar B: `herdr agent read <B_TERM> --source visible`
Frenar a mano: `herdr pane close <B_PANE>` (re-resolver el pane_id si hubo cierres en el medio)
```

## MUST DO

- Validar `<AGENT>` contra **`{claude, opencode}`** — abortar con `codex` y cualquier otro valor.
- Reusar el pre-flight/launch/dialogos/cleanup de `herdr-detach` (binario, server, integracion, anclaje a `HERDR_PANE_ID`, `agent start --split right` en `--here` con snapshot `TERMS_BEFORE`, `pane run` sobre el `root_pane` en `--new`, §1b de stray panes). No reimplementar.
- Direccionar el loop por `terminal_id` (`A_TERM`/`B_TERM`), no por nombre: los nombres de agente NO son únicos (varias sesiones comparten `claude`/`opencode`), así que `agent send claude` es ambiguo. El terminal_id es único y estable. `A_TERM` sale de `herdr pane get "$HERDR_PANE_ID" | jq -r .result.pane.terminal_id` (path `.result.pane.*`). No renombrar el pane de A.
- Generar `state.json`, `output.md` y `ping.sh` en `$CWD_BASE/.herdr-pon/run-<TS>/`, con **rutas absolutas** en el contrato (A y B comparten cwd, pero anclar a absoluto evita sorpresas si alguno cambia de dir).
- `ping.sh` es la unica via de envio entre panes: `agent send` + `enter` **minúscula** con confirmacion por transicion a `working` (hasta 3 intentos). Hacerlo ejecutable. **Validador `opencode`:** la `Enter` capitalizada NO submitea (inserta newline) y un paste puede caer en "shell mode" (corre como zsh) → el helper usa `enter` minúscula y un esc-guard condicionado al tipo de peer (`.result.agent.agent == opencode`); a claude NO se le manda `esc`.
- Embeber el contrato completo y self-describing en el prompt inicial de B, con todos los placeholders resueltos a valores reales. B es persistente: el contrato se manda una vez.
- Default `--max-rounds 5`. El cap se chequea en B (quien incrementa `round`); al superarlo, B avisa a A y corta — el loop nunca corre infinito.
- Fire-and-forget tras el kickoff (loop push, sin `--wait`).
- Como A: ante cada ping de B, seguir la rama correspondiente (accionable → arreglar+revalidar; VERDE/CAP → reportar y cerrar). Consultar `state.json` si el ping queda ambiguo.
- Re-resolver el `pane_id` desde el handle (`agent get <terminal_id> | jq -r .result.agent.pane_id`) justo antes de cualquier `pane send-keys`/`pane close` (herdr renumera `pane_id`s al cerrar panes). `ping.sh` ya lo hace internamente.

## MUST NOT DO

- No soportar `codex` — su dialogo de update (`npm install -g`) y su reporte de status en estados intermedios rompen el loop automatico.
- No bloquear con `--wait` ni pollear a B — el modelo es push; B te escribe.
- No correr el loop sin tope — siempre respetar `max_rounds`; sin esa guarda dos agentes pueden rebotar infinito quemando tokens.
- No reimplementar el send+Enter en el contrato — los agentes usan `ping.sh`, no arman el `agent send`/`pane send-keys` a mano.
- No mandar el contrato en cada ronda — B lo recuerda (sesion persistente); los pings de revalidacion son cortos.
- No focusear el pane de B (`--no-focus`) ni cerrarlo al terminar — la inspeccion manual queda como side-channel.
- No guardar el `pane_id` devuelto por `agent start`/`workspace create` y reusarlo sin re-resolver — usar el handle (`A_TERM`/`B_TERM`, terminal_id) como fuente de verdad.
- No reportar (como B) un veredicto de validacion sin haber corrido la validacion de verdad.
- No agregar `.herdr-pon/` a `.gitignore` ni borrar `STATE_DIR` automaticamente sin que el usuario lo pida.
- No persistir nada en auto-memory.
- No agregar flags que el usuario no pidio.

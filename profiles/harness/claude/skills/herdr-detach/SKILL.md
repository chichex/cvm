Deriva un prompt a otro agente CLI (`claude`, `opencode`, `codex`) corriendo en un pane de `herdr`, opcionalmente bloqueando hasta que el agente termine y devuelve la respuesta. Usar cuando queres delegar trabajo a una sesion paralela visible en otro pane y, segun el caso, seguir trabajando vos o esperar la respuesta antes de continuar.

Skill **exclusivo para Claude Code** (depende del tool `Bash` para hablar con la CLI `herdr` por su socket API). Asume que la sesion actual ya esta corriendo dentro de `herdr` — el skill se ancla al pane que originó la invocacion via la env var `HERDR_PANE_ID` (invariante al focus state, asi que el usuario puede moverse a otro pane mientras el skill se resuelve sin desviar el origen).

## Argumentos

```text
/herdr-detach [--wait] [--here|--new] <agente> <prompt>
```

- `<agente>`: `claude` | `opencode` | `codex`. Otros valores cortan el flujo.
- `<prompt>`: texto libre que se le envia al agente derivado. Todo lo que sigue al `<agente>` se concatena en un solo string.
- `--wait`: bloquea hasta que el agente derivado termine de responder (status `idle` o `done`) y devuelve la respuesta inline. Default: fire-and-forget — devuelve `pane_id` y sigue.
- `--here`: split del pane que **originó** la invocacion (no del pane actualmente focused — se ancla via `HERDR_PANE_ID`). Split a la derecha. **Default**.
- `--new`: crea un workspace nuevo dedicado.

El pane derivado siempre se deja abierto, aun en modo `--wait`, para que puedas inspeccionarlo o seguir la conversacion a mano.

## Pre-flight

### 1. Verificar binario `herdr`

```bash
command -v herdr
```

Si no esta en PATH, abortar:

```text
herdr no esta instalado o no esta en PATH. Ver https://herdr.dev.
```

### 2. Verificar server corriendo

```bash
herdr status 2>&1
```

Parsear el JSON o texto: si `server.status` no es `running`, abortar pidiendo al usuario que abra `herdr` (la sesion necesita estar adentro de la app para que el split o el workspace nuevo aparezca a la vista).

### 3. Parsear `$ARGUMENTS`

- Trim whitespace.
- Extraer flags `--wait`, `--here`, `--new` (en cualquier orden) antes del primer token no-flag.
- `--here` y `--new` son excluyentes — si aparecen ambos, abortar. Si no aparece ninguno, default `--here`.
- El primer token no-flag es `AGENT`. Validar contra `{claude, opencode, codex}`. Si no matchea, abortar con la lista valida.
- El resto concatenado con espacios es `PROMPT`. Si esta vacio, pedir al usuario:

```text
Pasame el prompt para mandarle a <AGENT> (texto libre, puede ser multilinea):
```

Si responde vacio, abortar.

### 4. Verificar binario del agente

```bash
command -v <AGENT>
```

Si no esta en PATH, abortar con un mensaje especifico al agente (no remediar — la instalacion la hace el usuario).

### 5. Auto-instalar integracion de herdr si falta

```bash
herdr integration status 2>&1
```

Parsear la linea correspondiente a `<AGENT>`. Si dice `not installed` o `outdated`, instalarla sin preguntar:

```bash
herdr integration install <AGENT>
```

La integracion hookea el reporting de estado (`idle|working|blocked|done`) para que `agent wait` funcione confiablemente. Es side-effect en `~/.<agent>/` (hook script + settings) pero es la pieza que hace todo lo demas posible — instalar es lo correcto. Si la instalacion falla, reportar stderr completo y abortar.

### 6. Resolver workspace / pane base

Para `--here`, **anclar al pane que originó la invocacion** — no al focused state, porque el usuario puede moverse de pane mientras el skill se resuelve. Herdr inyecta una env var en cada pane que maneja, que identifica de forma estable a ese pane:

```bash
[ "${HERDR_ENV:-}" = "1" ] || abort "no estoy corriendo dentro de un pane herdr (HERDR_ENV no es 1)"
[ -n "${HERDR_PANE_ID:-}" ] || abort "HERDR_PANE_ID vacio — la sesion no esta managed por herdr"
```

La API acepta `HERDR_PANE_ID` (formato `p_N`) como identifier de pane. Resolver `cwd`, `workspace_id` y `tab_id` con un solo call:

```bash
herdr pane get "$HERDR_PANE_ID" 2>&1
```

Parsear el JSON. Capturar `PANE_ID_BASE` (= `pane_id` real del response, ej `w652897bef8d7d2-1`), `WORKSPACE_ID`, `TAB_ID`, `CWD_BASE`. **No usar `focused: true`** — el campo viene en el response pero no es lo que nos interesa: lo que importa es el pane del shell que invoca, no el que tenga focus en el momento.

Para `--new`:

```bash
herdr workspace create --cwd <CWD_ACTUAL> --label "detach-<AGENT>" --no-focus 2>&1
```

Capturar `WORKSPACE_ID`, `TAB_ID` y — clave — el `root_pane` que herdr crea con cada workspace nuevo: su `pane_id` (`ROOT_PANE_ID`, ej `w652...-1`) y su `terminal_id` (`ROOT_TERM`, estable). El `root_pane` es un shell pelado, pero **no se descarta**: en §1 se lanza el agente *adentro* de el con `pane run` (en vez de `agent start`, que spawnearia un pane nuevo y dejaria el root como stray). Asi `--new` queda en 1 workspace = 1 pane, sin necesidad de cleanup. Para `<CWD_ACTUAL>` usar el `cwd` del pane origen (resuelto desde `HERDR_PANE_ID` como arriba), no el `pwd` del shell — son lo mismo en el caso comun, pero anclar al pane origen es la fuente de verdad si el shell hubiera cambiado de directorio.

## Ejecutar

### 1. Lanzar el agente

`<TS>` = timestamp corto (`date +%s`) para no chocar con nombres existentes. Definir `AGENT_NAME = "detach-<AGENT>-<TS>"` y usarlo como **handle estable** en todo lo que sigue.

Para `--here` — un solo comando. `agent start --split right` crea pane + arranca el agente atomicamente; NO hacer `pane split` antes (deja un shell pelado extra al costado). **Antes** de lanzar, snapshotear los `terminal_id` existentes del workspace (no `pane_id`: el `pane_id` se renumera en cada cierre; el `terminal_id` es estable) para poder atribuir cualquier shell pelado al launch en §1b sin tocar panes preexistentes del usuario:

```bash
TERMS_BEFORE=$(herdr pane list --workspace <WORKSPACE_ID> | jq -r '.result.panes[].terminal_id' | sort)
herdr agent start "$AGENT_NAME" \
  --workspace <WORKSPACE_ID> --tab <TAB_ID> \
  --cwd <CWD_BASE> --split right --no-focus \
  -- <AGENT>
```

Para `--new` — **reusar el `root_pane`** del workspace recien creado en vez de spawnear un pane nuevo. `agent start` (sin `--split`) crea un pane nuevo (`-2`) y deja el `root_pane` (`-1`) como shell pelado: esa es la causa del "pane al pedo". En su lugar, lanzar el agente *dentro* del `root_pane` con `pane run` (verificado en herdr 0.6.2: reusa el pane existente, no crea uno nuevo, y la integracion detecta al agente en ~2s):

```bash
herdr pane run "$ROOT_PANE_ID" "<AGENT>"
```

`pane run` escribe el comando + Enter en el shell pelado del `root_pane`, que arranca pristino en su prompt. Como no pasamos por `agent start`, el agente queda sin `AGENT_NAME`; hay que (a) esperar a que herdr **detecte** al agente sobre `ROOT_TERM` y (b) renombrarlo a `AGENT_NAME` para que todo el flujo de abajo (§2-§5) lo targetee igual que en `--here`:

```bash
for i in 1 2 3 4 5 6; do
  LABEL=$(herdr agent get "$ROOT_TERM" 2>/dev/null | jq -r '.result.agent.agent // empty')
  [ -n "$LABEL" ] && break
  sleep 2
done
herdr agent rename "$ROOT_TERM" "$AGENT_NAME"
```

`ROOT_TERM` (el `terminal_id` del `root_pane`) es estable y nunca se renumera, asi que es el target seguro para la deteccion/rename. Tras el rename, `--new` y `--here` comparten el resto del flujo via `AGENT_NAME`.

### 1b. Cerrar panes stray (shells pelados) — backstop post-launch

Con el flujo correcto **no deberia quedar ningun shell pelado**: en `--here` `agent start --split right` es atomico (1 pane), y en `--new` el agente reusa el `root_pane` via `pane run` (no se crea un pane extra). Este paso es un backstop por si algo se desincroniza (tipicamente un split de mas en `--here`). Un shell pelado se identifica sin ambiguedad porque tiene **`agent: null`** y **`agent_status: "unknown"`** en `pane list`.

Regla clave: **identificar y cerrar por `terminal_id`, nunca por `pane_id`**. herdr renumera los `pane_id` en cada cierre, asi que una lista de `pane_id` capturada de antemano se corrompe apenas cerras el primero (cerras el equivocado). El `terminal_id` es estable. El patron en ambos modos es: resolver el `terminal_id` del agente (para protegerlo), buscar shells pelados a cerrar, y cerrar de a uno re-resolviendo el `pane_id` actual desde su `terminal_id` justo antes de cada `pane close`.

```bash
AGENT_TERM=$(herdr agent get "$AGENT_NAME" | jq -r .result.agent.terminal_id)
```

**`--new`** — el agente ocupa el `root_pane` reusado, asi que normalmente **no hay nada que cerrar** (pane_count = 1). Como backstop, si quedo algun shell pelado (`agent: null`, ≠ el del agente), cerrarlo de a uno re-resolviendo el `pane_id` desde el `terminal_id`. Cap de 5:

```bash
for i in 1 2 3 4 5; do
  TERM=$(herdr pane list --workspace <WORKSPACE_ID> \
    | jq -r --arg at "$AGENT_TERM" \
      '[.result.panes[] | select(.agent == null and .terminal_id != $at) | .terminal_id][0] // empty')
  [ -n "$TERM" ] || break
  PID=$(herdr pane list --workspace <WORKSPACE_ID> \
    | jq -r --arg t "$TERM" '.result.panes[] | select(.terminal_id == $t) | .pane_id')
  herdr pane close "$PID"
done
```

**`--here`** — el split cae en un tab que puede tener panes del usuario, asi que **no** cerrar todo lo que tenga `agent: null`. Cerrar solo los shells pelados aparecidos en este launch = los `terminal_id` con `agent: null` que **no** estaban en `TERMS_BEFORE` (snapshot del §1). Tipicamente 0; 1 si hubo un split de mas:

```bash
NEW_BARE_TERMS=$(comm -13 <(echo "$TERMS_BEFORE") \
  <(herdr pane list --workspace <WORKSPACE_ID> | jq -r '.result.panes[] | select(.agent == null) | .terminal_id' | sort))
for t in $NEW_BARE_TERMS; do
  [ "$t" = "$AGENT_TERM" ] && continue
  PID=$(herdr pane list --workspace <WORKSPACE_ID> | jq -r --arg t "$t" '.result.panes[] | select(.terminal_id == $t) | .pane_id')
  [ -n "$PID" ] && herdr pane close "$PID"
done
```

Tras cerrar, **re-resolver el `pane_id` del agente** (`herdr agent get "$AGENT_NAME" | jq -r .result.agent.pane_id`) antes de cualquier `pane send-keys`/`pane read` posterior: la renumeracion del cierre ya cambio el id.

### Tracking del pane

herdr **renumera `pane_id`s al cerrar panes** (ej. cerras `-3`, lo que era `-4` pasa a ser `-3`). Eso significa que el `pane_id` devuelto por `agent start` *no es estable*:

- En `--new` el agente vive en el `root_pane` reusado (`-1`); su `pane_id` es estable salvo que un cierre concurrente lo renumere.
- En `--here` con multiples detaches concurrentes se puede invalidar si el usuario u otro skill cierra un pane intermedio.

Solucion: usar `AGENT_NAME` como source of truth. Los comandos `agent send`/`agent read`/`agent wait` aceptan `AGENT_NAME` directamente y no necesitan re-resolucion. Para los comandos `pane *` (`pane send-keys`, `pane close`, `pane read`), **re-resolver** el `pane_id` actual justo antes de la llamada:

```bash
PANE_ID=$(herdr agent get "$AGENT_NAME" | jq -r .result.agent.pane_id)
```

### 2. Handle de dialogos de pre-arranque

`claude` y `codex` pueden mostrar dialogos antes de aceptar input. Pueden aparecer en secuencia (ej. update → trust → bypass), asi que hay que loop-checkar hasta que no quede ninguno. Dialogos conocidos:

| Dialogo | Trigger | Default seleccionada | Como llegar a la opcion correcta |
|---|---|---|---|
| Trust this directory | Primera vez que `claude`/`codex` abre un cwd | "Yes, I trust" / "Yes, continue" (opcion 1, **correcta**) | `Enter` |
| Bypass Permissions warning | `claude` con `permissions.defaultMode=bypassPermissions` en settings.json | "No, exit" (opcion 1, **incorrecta**) | `Down` + `Enter` |
| Codex update available | `codex` cuando hay version nueva publicada | "Update now" (opcion 1, **peligrosa** — corre `npm install -g`) | `Down` + `Enter` (= "Skip") |

Flujo (hasta N=4 iteraciones — varios dialogs pueden encadenarse, ademas update puede tardar en renderizar):

```bash
sleep 3
PANE_ID=$(herdr agent get "$AGENT_NAME" | jq -r .result.agent.pane_id)
herdr agent read "$AGENT_NAME" --source visible --lines 40 --format text
```

Sobre el texto leido (case-insensitive):

- Si contiene `Update available` junto con opciones tipo `1. Update now` y `2. Skip` (codex), mandar `Down` + `Enter` para elegir "Skip" — **NUNCA** mandar Enter solo (la default lanza un `npm install -g` no pedido):

  ```bash
  herdr pane send-keys "$PANE_ID" Down
  herdr pane send-keys "$PANE_ID" Enter
  ```

- Si contiene `Bypass Permissions mode` (o equivalente), mandar `Down` + `Enter` para saltar de "No, exit" a "Yes, I accept" antes de confirmar:

  ```bash
  herdr pane send-keys "$PANE_ID" Down
  herdr pane send-keys "$PANE_ID" Enter
  ```

- Si contiene `trust this folder`, `trust the contents`, `Yes, I trust` o `Yes, continue`, mandar `Enter` directo (la default ya es la correcta):

  ```bash
  herdr pane send-keys "$PANE_ID" Enter
  ```

- Si **no** matchea ningun dialogo conocido, verificar que el agente este realmente listo (no parado en un menu desconocido) con **todas** estas condiciones a la vez:
  1. Status del agente es `idle` (`herdr agent get "$AGENT_NAME"` → `.result.agent.agent_status == "idle"`).
  2. El char de prompt (`❯` para claude, `›` para codex, `┃` para opencode) aparece **al inicio (con whitespace permitido) de la ultima linea no vacia** del buffer visible — no en el medio, no en un menu.
  3. El buffer **no** contiene patrones de menu: `^\s*[0-9]+\.\s` (opcion numerada), `Press enter to continue`, `select an option`, ni `Update available`.

  Si las tres se cumplen, salir del loop. Si alguna falla, seguir iterando.

Entre iteraciones, dormir 2 segundos. Si despues de 4 iteraciones sigue habiendo un estado no resuelto, abortar y avisar al usuario que confirme manualmente en el pane (no adivinar opciones desconocidas — el costo de equivocarse es cerrar la sesion o lanzar un `npm install -g` indeseado).

### 3. Enviar el prompt

Escribir `PROMPT` a un temp file para evitar problemas de shell-quoting con multilinea o comillas:

```bash
PROMPT_FILE=$(mktemp)
# Escribir $PROMPT al archivo via Write (no via heredoc en shell).
```

Mandar el texto al input del agente, **dormir 0.5s** para dar tiempo al TTY de procesar, re-resolver `PANE_ID`, y mandar Enter:

```bash
herdr agent send "$AGENT_NAME" "$(cat "$PROMPT_FILE")"
sleep 0.5
PANE_ID=$(herdr agent get "$AGENT_NAME" | jq -r .result.agent.pane_id)
herdr pane send-keys "$PANE_ID" Enter
```

`agent send` escribe texto literal sin ejecutar; el Enter posterior es lo que envia el prompt al agente (en los TUIs de claude/opencode/codex, Enter es el binding para "send"). El `sleep 0.5` evita un race observado en codex donde el primer Enter llega antes de que el TTY haya terminado de procesar el `agent send` y se pierde silenciosamente (el prompt queda escrito en el input sin haberse enviado).

**Caso prompt largo (pasted text)**: cuando `agent send` manda un prompt multilinea con muchas lineas (>20-30), el TUI de claude lo trata como "pasted text" y lo colapsa en el input box como `❯ [Pasted text #1 +25 lines][Pasted text #2 +19 lines]...`. En ese estado, **el primer Enter NO submitea** — solo confirma el paste / cambia el modo del input. Hace falta un segundo Enter para que el TUI mande el prompt al agente. Sintomas observados tras el primer Enter: el prompt sigue en el input box como `[Pasted text +N lines]`, el status sigue `idle` (sin spinner). Recien con el segundo Enter el input se limpia y aparece el spinner (`✢ Finagling…` o similar).

**Confirmar que el envio surtio efecto** (loop hasta N=2 reintentos de Enter — no mas, para no mandar Enters de mas que confundan al TUI): despues de cada Enter, leer el input box y chequear si el prompt todavia esta ahi:

```bash
herdr pane send-keys "$PANE_ID" Enter
sleep 0.8
herdr agent read "$AGENT_NAME" --source visible --lines 10 --format text
```

- Si el buffer visible todavia muestra `[Pasted text +N lines]` (o contenido del prompt persistiendo en el input area, con el agente aun `idle`), mandar Enter una segunda vez y re-chequear. No pasar de 2 Enters totales.
- Como confirmacion adicional, esperar hasta 5s a que el agente transicione a `working`. Si no transiciona ni el input se limpio, reintentar Enter (idempotente en los TUIs cuando el input box ya tiene el prompt encolado):

```bash
herdr agent wait "$AGENT_NAME" --status working --timeout 5000 \
  || herdr pane send-keys "$(herdr agent get "$AGENT_NAME" | jq -r .result.agent.pane_id)" Enter
```

Este check es ademas necesario para que el `agent wait --status idle` del paso §4 no resuelva con un false-positive: codex puede estar en `idle` *antes* de procesar el prompt, y `wait --status idle` resolveria inmediatamente sin haber esperado nada.

### 4. Esperar (solo si `--wait`)

```bash
herdr agent wait "$AGENT_NAME" --status idle --timeout 600000
```

Timeout de 10 minutos por default. Si el agente entra primero a `done` herdr resuelve igual (esos status estan en el mismo bucket).

**Precondicion**: este `wait` confia en que §3 ya confirmo transicion a `working`. Sin ese check, `wait --status idle` puede resolver inmediatamente con el `idle` previo al envio del prompt (visto en codex que reporta `idle`/`done` en estados intermedios). El check del §3 es la guarda contra eso.

Si el wait falla con timeout, no abortar el pane — solo reportar timeout y devolver `pane_id` para inspeccion manual.

Despues de resolver, dormir 1-2 segundos extra para que el TUI termine de renderizar la respuesta final antes de leer.

### 5. Leer respuesta (solo si `--wait`)

```bash
herdr agent read "$AGENT_NAME" --source visible --lines 80 --format text
```

El output viene con UI noise (banners, frames, status bar). No tratar de parsear quirurgicamente — entregar el bloque visible tal cual, y dejar que el usuario localice la respuesta. Como referencia, el patron tipico es:

- claude: la respuesta esta entre `⏺ ` y el siguiente prompt vacio `❯`.
- opencode: entre `┃` y el siguiente input frame.
- codex: entre `• ` y el siguiente `›`.

Si el agente respondio con texto corto, basta con incluirlo entero. Si es largo (> 60 lineas), incluir solo las ultimas 60 lineas del visible buffer y avisar.

## Reporte

### Modo fire-and-forget (sin `--wait`)

Re-resolver `PANE_ID` fresh antes de armar el reporte (`herdr agent get "$AGENT_NAME" | jq -r .result.agent.pane_id`):

```text
## /herdr-detach report

- modo: fire-and-forget
- agente: <AGENT>
- ubicacion: <here | new>
- workspace_id: <WORKSPACE_ID>
- agent_name: <AGENT_NAME>
- pane_id: <PANE_ID>
- prompt enviado (primeras 80 chars): <PROMPT[:80]>...

Para inspeccionar: `herdr agent read <AGENT_NAME> --source visible`
Para volver focused: `herdr agent focus <AGENT_NAME>`
Para cerrar: `herdr pane close <PANE_ID>` (re-resolver el pane_id si pasaron cierres de panes en el medio)
```

### Modo blocking (con `--wait`)

Re-resolver `PANE_ID` fresh antes del reporte:

```text
## /herdr-detach report

- modo: wait
- agente: <AGENT>
- ubicacion: <here | new>
- agent_name: <AGENT_NAME>
- pane_id: <PANE_ID>
- status final: <idle | done | timeout>

### Respuesta de <AGENT>

```
<bloque visible del pane, sin trim agresivo>
```

Pane sigue abierto. Cerrar con `herdr pane close <PANE_ID>` (re-resolver si pasaron cierres en el medio).
```

## MUST DO

- Validar que `herdr` esta en PATH y que su server esta `running` antes de cualquier accion.
- Validar `<AGENT>` contra `{claude, opencode, codex}` — abortar si no matchea, no asumir default.
- Auto-instalar la integracion de `herdr` del agente si esta `not installed` o `outdated`, sin preguntar.
- Anclar el modo `--here` al pane origen via `HERDR_PANE_ID` (env var inyectada por herdr), **nunca** al focused state. Si `HERDR_ENV` no es `1` o `HERDR_PANE_ID` esta vacio, abortar — la sesion no esta managed por herdr.
- En `--here`: lanzar con `herdr agent start --split right`. **No** correr `pane split` antes (deja shell pelado extra). Snapshotear los `terminal_id` del workspace (`TERMS_BEFORE`) *antes* de lanzar.
- En `--new`: **reusar el `root_pane`** del workspace recien creado lanzando el agente con `herdr pane run "$ROOT_PANE_ID" "<AGENT>"` — NO `agent start` (sin `--split` crea un pane nuevo y deja el root como stray; esa era la causa de "1 workspace = 2 panes"). Capturar `ROOT_PANE_ID` y `ROOT_TERM` en el `workspace create`. Tras lanzar, esperar la deteccion del agente sobre `ROOT_TERM` y `herdr agent rename "$ROOT_TERM" "$AGENT_NAME"` para unificar el handle con `--here`.
- **Post-launch, verificar que no quedaron shells pelados (§1b) como backstop** — con `pane run` en `--new` y `agent start --split right` en `--here` no deberia quedar ninguno, pero igual chequear. Identificarlos por `agent: null` + `agent_status: "unknown"`, y operar **por `terminal_id` (estable), nunca por `pane_id` (se renumera en cada cierre)**. En `--new`: normalmente 0; si quedo alguno, cerrarlo de a uno re-resolviendo el `pane_id` desde el `terminal_id` (cap 5). En `--here`: cerrar solo los `terminal_id` con `agent: null` ausentes de `TERMS_BEFORE` (diff), nunca los preexistentes del usuario ni el del agente. Re-resolver el `pane_id` del agente despues de cerrar.
- Usar `AGENT_NAME` (= `detach-<AGENT>-<TS>`) como handle estable. herdr renumera `pane_id`s al cerrar panes, asi que el `pane_id` devuelto por `agent start` puede invalidarse. Para `agent send`/`read`/`wait`/`focus`, pasar `AGENT_NAME` directamente. Para `pane send-keys`/`pane close`/`pane read`, **re-resolver** el pane_id justo antes de cada llamada via `herdr agent get "$AGENT_NAME" | jq -r .result.agent.pane_id`.
- Manejar los dialogos de pre-arranque automaticamente: trust-folder con `Enter` (default correcta), Bypass Permissions con `Down`+`Enter` (default incorrecta), Codex update con `Down`+`Enter` (= "Skip"; la default "Update now" lanza un `npm install -g` no pedido). Loop-checkar hasta 4 iteraciones porque pueden encadenarse.
- Para detectar "agente listo" sin dialog visible: exigir las tres condiciones a la vez — `agent_status: idle` **+** prompt char (`❯`/`›`/`┃`) al inicio de la ultima linea no vacia **+** ausencia de patrones de menu (`^\s*[0-9]+\.\s`, `Press enter to continue`, `select an option`, `Update available`). El status `idle` solo no alcanza: codex reporta idle en menus de opciones.
- Pasar `PROMPT` via temp file + `"$(cat <tmp>)"` para evitar shell-injection y problemas con comillas/multilinea.
- Mandar `agent send` → `sleep 0.5` → `pane send-keys Enter`. El sleep evita race observado en codex donde el primer Enter llega antes del flush del TTY y se pierde silenciosamente. Si el agente no transiciona a `working` en 5s post-envio, reintentar Enter.
- Verificar via `agent read` que el prompt se submiteo despues del Enter. Si quedo como `[Pasted text +N lines]` en el input (con el agente aun `idle`), mandar Enter de nuevo. Claude TUI trata input largo (>20-30 lineas) como pasted text y requiere confirmacion adicional: el primer Enter solo confirma el paste, el segundo es el que envia. Loop hasta 2 Enters totales, no mas.
- En modo `--wait`, primero confirmar transicion a `working` (paso §3), despues `wait --status idle` con timeout ~10min. Sin la guarda de `working`, `wait --status idle` puede resolver con falso-positivo del idle previo al envio del prompt.
- Devolver `pane_id` (resuelto en el momento del reporte) en todos los modos para que el usuario pueda inspeccionar o cerrar a mano.

## MUST NOT DO

- No correr `/herdr-detach` si `herdr status` reporta server no-running — abortar, no intentar arrancarlo desde aca.
- No remediar la ausencia del binario del agente (`claude`/`opencode`/`codex`) — abortar y pedir al usuario que lo instale.
- No focus al pane derivado por default (`--no-focus` siempre) — el usuario sigue trabajando en el actual.
- No cerrar el pane derivado al terminar, ni en modo `--wait` — la inspeccion manual queda como side-channel valido. (Excepcion: el cierre de shells pelados en §1b — esos son `agent: null`, nunca el pane del agente, que se protege por su `terminal_id`.)
- No guardar el `pane_id` devuelto por `agent start`/`workspace create` y usarlo en pasos posteriores sin re-resolver — herdr renumera `pane_id`s al cerrar panes (un cierre concurrente del usuario o de otro skill, o el backstop de §1b). En `--new` usar `ROOT_TERM` (terminal_id, estable) como ancla para detectar/renombrar al agente.
- No correr `pane split` (ni ningun comando que cree un pane) por separado en `--here` — `agent start --split right` ya crea el pane. Un split de mas es la causa tipica de "panes al pedo".
- No cerrar a ciegas todos los `agent: null` del tab en `--here` — podes matar un shell que el usuario abrio a proposito. Cerrar solo el diff contra `TERMS_BEFORE`. (En `--new` si es seguro cerrar todos: el workspace es dedicado.)
- No cerrar panes por un `pane_id` capturado en una lista de antemano cuando vas a cerrar mas de uno — al cerrar el primero, herdr renumera y el resto de los `pane_id` apuntan a panes equivocados. Iterar por `terminal_id` (estable) y re-resolver el `pane_id` justo antes de cada `pane close`.
- No saltarse §1b ni asumir a ciegas que el launch quedo limpio — verificar via `pane list` en ambos modos y cerrar strays si aparecen (con `pane run` en `--new` no deberian, pero el check es barato).
- En `--new`, no usar `agent start` para lanzar el agente — crea un pane nuevo y deja el `root_pane` como stray. Usar `pane run "$ROOT_PANE_ID"` para reusar el root.
- No declarar "agente listo" solo porque ves `❯`/`›`/`┃` en el buffer ni solo porque `agent_status == idle`. Exigir las tres condiciones combinadas (ver MUST DO).
- No mandar Enter inmediatamente despues de `agent send` sin el `sleep 0.5` previo — race observado en codex.
- No mandar Enter solo en el dialog de update de codex — la default es "Update now" y dispara `npm install -g`. Mandar `Down` + `Enter` para "Skip".
- No interpolar `PROMPT` crudo en la linea de shell de `agent send` — usar temp file.
- No parsear/limpiar la respuesta del agente derivado mas alla de truncar a las ultimas 60 lineas si es muy larga.
- No persistir nada en auto-memory.
- No agregar flags que el usuario no pidio (`--model`, `--cwd`, `--timeout`, etc).

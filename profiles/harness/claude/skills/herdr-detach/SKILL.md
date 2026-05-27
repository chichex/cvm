Deriva un prompt a otro agente CLI (`claude`, `opencode`, `codex`) corriendo en un pane de `herdr`, opcionalmente bloqueando hasta que el agente termine y devuelve la respuesta. Usar cuando queres delegar trabajo a una sesion paralela visible en otro pane y, segun el caso, seguir trabajando vos o esperar la respuesta antes de continuar.

Skill **exclusivo para Claude Code** (depende del tool `Bash` para hablar con la CLI `herdr` por su socket API). Asume que la sesion actual ya esta corriendo dentro de `herdr` — el skill detecta el workspace/pane focused automaticamente para el modo `--here`.

## Argumentos

```text
/herdr-detach [--wait] [--here|--new] <agente> <prompt>
```

- `<agente>`: `claude` | `opencode` | `codex`. Otros valores cortan el flujo.
- `<prompt>`: texto libre que se le envia al agente derivado. Todo lo que sigue al `<agente>` se concatena en un solo string.
- `--wait`: bloquea hasta que el agente derivado termine de responder (status `idle` o `done`) y devuelve la respuesta inline. Default: fire-and-forget — devuelve `pane_id` y sigue.
- `--here`: split del pane focused actual (split a la derecha). **Default**.
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

Para `--here`:

```bash
herdr agent list 2>&1
```

Parsear JSON, encontrar el agente con `focused: true`. Capturar `WORKSPACE_ID`, `PANE_ID_BASE` (sera el pane a splittear), `CWD_BASE`. Si no hay ninguno focused (raro), abortar pidiendo focus manual.

Para `--new`:

```bash
herdr workspace create --cwd <CWD_ACTUAL> --label "detach-<AGENT>" --no-focus 2>&1
```

Capturar `WORKSPACE_ID` y `PANE_ID_BASE` del root_pane del workspace nuevo. Usar el `pwd` del shell actual como `CWD_ACTUAL`.

## Ejecutar

### 1. Lanzar el agente

Para `--here`, splittear primero a la derecha y luego arrancar el agente en el pane nuevo:

```bash
herdr pane split <PANE_ID_BASE> --direction right --cwd <CWD_BASE> --no-focus
```

Capturar el `pane_id` del split como `TARGET_PANE`.

Para `--new`, el `TARGET_PANE` es el root pane del workspace recien creado (no splittear).

En ambos modos, arrancar el agente nombrado en ese pane:

```bash
herdr agent start "detach-<AGENT>-<TS>" --workspace <WORKSPACE_ID> --tab <TAB_ID> --cwd <CWD_BASE> --no-focus -- <AGENT>
```

`<TS>` es un timestamp corto (`date +%s` o equivalente) para no chocar con nombres existentes. Capturar el `pane_id` devuelto como `TARGET_PANE` (sobreescribe el del split — herdr a veces reusa el slot o crea uno nuevo segun el caso).

> Nota: si `agent start` se invoca con `--workspace` pero sin `--tab`, herdr puede abrir el agente en un tab/pane distinto al del split. Capturar el `pane_id` del response del `agent start` y usar ese siempre como `TARGET_PANE`.

### 2. Handle de dialogos de pre-arranque

`claude` y `codex` pueden mostrar dialogos antes de aceptar input. Pueden aparecer en secuencia (ej. trust folder seguido de bypass permissions), asi que hay que loop-checkar hasta que no quede ninguno. Dialogos conocidos:

| Dialogo | Trigger | Default seleccionada | Como llegar a la opcion correcta |
|---|---|---|---|
| Trust this directory | Primera vez que `claude`/`codex` abre un cwd | "Yes, I trust" / "Yes, continue" (opcion 1, **correcta**) | `Enter` |
| Bypass Permissions warning | `claude` con `permissions.defaultMode=bypassPermissions` en settings.json | "No, exit" (opcion 1, **incorrecta**) | `Down` + `Enter` |

Flujo (hasta N=3 iteraciones):

```bash
sleep 3
herdr agent read <TARGET_PANE> --source visible --lines 30 --format text
```

Sobre el texto leido (case-insensitive):

- Si contiene `Bypass Permissions mode` (o equivalente), mandar `Down` + `Enter` para saltar de "No, exit" a "Yes, I accept" antes de confirmar:

  ```bash
  herdr pane send-keys <TARGET_PANE> Down
  herdr pane send-keys <TARGET_PANE> Enter
  ```

- Si contiene `trust this folder`, `trust the contents`, `Yes, I trust` o `Yes, continue`, mandar `Enter` directo (la default ya es la correcta):

  ```bash
  herdr pane send-keys <TARGET_PANE> Enter
  ```

- Si no matchea ningun dialogo conocido y el status es `idle` con un input prompt visible (`❯` para claude, `›` para codex), el agente esta listo — salir del loop.

Entre iteraciones, dormir 2 segundos. Si despues de 3 iteraciones sigue habiendo un dialogo no resuelto, abortar y avisar al usuario que confirme manualmente en el pane (no adivinar opciones desconocidas — el costo de equivocarse es cerrar la sesion).

### 3. Enviar el prompt

Escribir `PROMPT` a un temp file para evitar problemas de shell-quoting con multilinea o comillas:

```bash
PROMPT_FILE=$(mktemp)
# Escribir $PROMPT al archivo via Write (no via heredoc en shell).
```

Mandar el texto al input del agente y luego Enter:

```bash
herdr agent send <TARGET_PANE> "$(cat "$PROMPT_FILE")"
herdr pane send-keys <TARGET_PANE> Enter
```

`agent send` escribe texto literal sin ejecutar; el Enter posterior es lo que envia el prompt al agente (en los TUIs de claude/opencode/codex, Enter es el binding para "send").

### 4. Esperar (solo si `--wait`)

```bash
herdr agent wait <TARGET_PANE> --status idle --timeout 600000
```

Timeout de 10 minutos por default. Si el agente entra primero a `done` herdr resuelve igual (esos status estan en el mismo bucket).

Si el wait falla con timeout, no abortar el pane — solo reportar timeout y devolver `pane_id` para inspeccion manual.

Despues de resolver, dormir 1-2 segundos extra para que el TUI termine de renderizar la respuesta final antes de leer.

### 5. Leer respuesta (solo si `--wait`)

```bash
herdr agent read <TARGET_PANE> --source visible --lines 80 --format text
```

El output viene con UI noise (banners, frames, status bar). No tratar de parsear quirurgicamente — entregar el bloque visible tal cual, y dejar que el usuario localice la respuesta. Como referencia, el patron tipico es:

- claude: la respuesta esta entre `⏺ ` y el siguiente prompt vacio `❯`.
- opencode: entre `┃` y el siguiente input frame.
- codex: entre `• ` y el siguiente `›`.

Si el agente respondio con texto corto, basta con incluirlo entero. Si es largo (> 60 lineas), incluir solo las ultimas 60 lineas del visible buffer y avisar.

## Reporte

### Modo fire-and-forget (sin `--wait`)

```text
## /herdr-detach report

- modo: fire-and-forget
- agente: <AGENT>
- ubicacion: <here | new>
- workspace_id: <WORKSPACE_ID>
- pane_id: <TARGET_PANE>
- prompt enviado (primeras 80 chars): <PROMPT[:80]>...

Para inspeccionar: `herdr agent read <TARGET_PANE> --source visible`
Para volver focused: `herdr agent focus <TARGET_PANE>`
Para cerrar: `herdr pane close <TARGET_PANE>`
```

### Modo blocking (con `--wait`)

```text
## /herdr-detach report

- modo: wait
- agente: <AGENT>
- ubicacion: <here | new>
- pane_id: <TARGET_PANE>
- status final: <idle | done | timeout>

### Respuesta de <AGENT>

```
<bloque visible del pane, sin trim agresivo>
```

Pane sigue abierto en `<TARGET_PANE>`. Cerrar con `herdr pane close <TARGET_PANE>`.
```

## MUST DO

- Validar que `herdr` esta en PATH y que su server esta `running` antes de cualquier accion.
- Validar `<AGENT>` contra `{claude, opencode, codex}` — abortar si no matchea, no asumir default.
- Auto-instalar la integracion de `herdr` del agente si esta `not installed` o `outdated`, sin preguntar.
- Detectar el pane focused para `--here`; crear workspace nuevo solo si `--new`.
- Capturar el `pane_id` siempre del response de `agent start` (no del split, porque pueden diferir).
- Manejar los dialogos de pre-arranque automaticamente: trust-folder con `Enter` (default correcta), Bypass Permissions con `Down`+`Enter` (default incorrecta). Loop-checkar porque pueden aparecer en secuencia.
- Pasar `PROMPT` via temp file + `"$(cat <tmp>)"` para evitar shell-injection y problemas con comillas/multilinea.
- Mandar Enter despues de `agent send` — `agent send` solo escribe el texto, no lo envia.
- En modo `--wait`, esperar idle/done con timeout generoso (~10 min) y leer el visible buffer; dejar el pane abierto siempre.
- Devolver `pane_id` en todos los modos para que el usuario pueda inspeccionar o cerrar a mano.

## MUST NOT DO

- No correr `/herdr-detach` si `herdr status` reporta server no-running — abortar, no intentar arrancarlo desde aca.
- No remediar la ausencia del binario del agente (`claude`/`opencode`/`codex`) — abortar y pedir al usuario que lo instale.
- No focus al pane derivado por default (`--no-focus` siempre) — el usuario sigue trabajando en el actual.
- No cerrar el pane derivado al terminar, ni en modo `--wait` — la inspeccion manual queda como side-channel valido.
- No interpolar `PROMPT` crudo en la linea de shell de `agent send` — usar temp file.
- No parsear/limpiar la respuesta del agente derivado mas alla de truncar a las ultimas 60 lineas si es muy larga.
- No persistir nada en auto-memory.
- No agregar flags que el usuario no pidio (`--model`, `--cwd`, `--timeout`, etc).

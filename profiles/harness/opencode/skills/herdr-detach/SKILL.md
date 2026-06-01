---
name: herdr-detach
description: Deriva un prompt a otro agente CLI en un pane de herdr, opcionalmente esperando la respuesta. Usar cuando queres delegar trabajo a claude, opencode o codex en una sesion paralela visible.
---

Deriva un prompt a otro agente CLI (`claude`, `opencode`, `codex`) corriendo en un pane de `herdr`, opcionalmente bloqueando hasta que el agente termine y devuelve la respuesta. Usar cuando queres delegar trabajo a una sesion paralela visible en otro pane y, segun el caso, seguir trabajando o esperar la respuesta antes de continuar.

Asume que la sesion actual corre dentro de `herdr`. El modo `--here` se ancla al pane que originó la invocacion via `HERDR_PANE_ID`, no al pane enfocado.

## Argumentos

```text
/herdr-detach [--wait] [--here|--new] <agente> <prompt>
```

- `<agente>`: `claude`, `opencode` o `codex`. Otros valores abortan.
- `<prompt>`: texto libre enviado al agente derivado. Todo lo que sigue al agente se concatena.
- `--wait`: espera hasta status `idle` o `done` y devuelve el buffer visible. Default: fire-and-forget.
- `--here`: split a la derecha del pane origen. Default.
- `--new`: crea un workspace nuevo dedicado.

El pane derivado siempre queda abierto.

## Pre-Flight

1. Verificar `herdr` en PATH con `command -v herdr`. Si falta, abortar: `herdr no esta instalado o no esta en PATH. Ver https://herdr.dev.`
2. Verificar server con `herdr status 2>&1`. Si `server.status` no es `running`, abortar pidiendo al usuario abrir `herdr`.
3. Parsear argumentos: extraer `--wait`, `--here`, `--new`; `--here` y `--new` son excluyentes; default `--here`.
4. Validar `<agente>` contra `{claude, opencode, codex}`. Si el prompt queda vacio, pedirlo al usuario.
5. Verificar binario del agente con `command -v <AGENT>`. Si falta, abortar.
6. Verificar integracion: `herdr integration status 2>&1`. Si el agente esta `not installed` u `outdated`, correr `herdr integration install <AGENT>` sin preguntar. Si falla, reportar stderr y abortar.
7. Para `--here`, exigir `HERDR_ENV=1` y `HERDR_PANE_ID` no vacio. Resolver `pane_id`, `workspace_id`, `tab_id`, `cwd` con `herdr pane get "$HERDR_PANE_ID"`.
8. Para `--new`, crear workspace con cwd del pane origen: `herdr workspace create --cwd <CWD_BASE> --label "detach-<AGENT>" --no-focus` y capturar `root_pane` y `terminal_id`.

## Lanzar Agente

Definir `TS=$(date +%s)` y `AGENT_NAME="detach-<AGENT>-<TS>"`.

Para `--here`:

- Snapshotear terminales existentes: `TERMS_BEFORE=$(herdr pane list --workspace <WORKSPACE_ID> | jq -r '.result.panes[].terminal_id' | sort)`.
- Lanzar con `herdr agent start "$AGENT_NAME" --workspace <WORKSPACE_ID> --tab <TAB_ID> --cwd <CWD_BASE> --split right --no-focus -- <AGENT>`.
- No correr `pane split` por separado.

Para `--new`:

- Reusar el `root_pane` del workspace: `herdr pane run "$ROOT_PANE_ID" "<AGENT>"`.
- Esperar deteccion del agente sobre `ROOT_TERM` y renombrar: `herdr agent rename "$ROOT_TERM" "$AGENT_NAME"`.
- No usar `agent start` sin `--split`, porque crea un pane extra.

Post-launch, cerrar solo shells pelados (`agent: null`, `agent_status: unknown`) creados por este launch. Operar por `terminal_id`, nunca por `pane_id`, porque herdr renumera pane ids al cerrar. Re-resolver el pane id del agente despues de cualquier cierre.

## Dialogos De Pre-Arranque

Loop hasta 4 iteraciones leyendo `herdr agent read "$AGENT_NAME" --source visible --lines 40 --format text`.

- Trust directory: mandar `Enter`.
- Bypass permissions: mandar `Down` + `Enter`.
- Codex update available: mandar `Down` + `Enter` para `Skip`; nunca `Enter` directo.
- Si no hay dialogo conocido, declarar listo solo si se cumplen las 3 condiciones: `agent_status == idle`, prompt char (`❯`, `›`, `┃`) al inicio de la ultima linea no vacia, y ausencia de menus (`^[0-9]+.`, `Press enter to continue`, `select an option`, `Update available`).

Si no se resuelve, abortar pidiendo confirmacion manual en el pane.

## Enviar Prompt

Escribir el prompt a un temp file para evitar shell quoting. Enviar con:

```bash
herdr agent send "$AGENT_NAME" "$(cat "$PROMPT_FILE")"
sleep 0.5
PANE_ID=$(herdr agent get "$AGENT_NAME" | jq -r .result.agent.pane_id)
herdr pane send-keys "$PANE_ID" Enter
```

Confirmar que el input se envio: si queda como pasted text o el agente no transiciona a `working` en 5s, mandar Enter una segunda vez. Cap maximo: 2 Enters totales.

## Esperar Opcionalmente

Solo con `--wait`:

- Primero confirmar transicion a `working` para evitar falso positivo de idle previo.
- Esperar con `herdr agent wait "$AGENT_NAME" --status idle --timeout 600000`.
- Dormir 1-2s y leer `herdr agent read "$AGENT_NAME" --source visible --lines 80 --format text`.
- Si el buffer es largo, devolver solo las ultimas 60 lineas y avisar.

## Reporte

Fire-and-forget:

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
Para cerrar: `herdr pane close <PANE_ID>`
```

Con `--wait`, incluir status final y el bloque visible del pane. En ambos modos, re-resolver `pane_id` justo antes de reportar.

## MUST DO

- Validar `herdr`, server, agente e integracion antes de lanzar.
- Anclar `--here` a `HERDR_PANE_ID`, nunca al focus state.
- Usar `AGENT_NAME` como handle estable para `agent send/read/wait/focus`.
- Re-resolver `pane_id` antes de cualquier `pane send-keys` o `pane close`.
- Manejar dialogos conocidos de forma segura.
- Pasar prompt via temp file.
- Dejar el pane derivado abierto.

## MUST NOT DO

- No arrancar `herdr` desde el skill si el server no esta corriendo.
- No remediar ausencia de `claude`, `opencode` o `codex`.
- No focusear el pane derivado por default.
- No guardar y reutilizar `pane_id` sin re-resolver.
- No mandar Enter directo en el dialogo de update de codex.
- No interpolar el prompt crudo en shell.
- No persistir nada en memoria.

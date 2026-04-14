# S-016: CVM Dashboard — Realtime Observability Web Viewer

| Field | Value |
|-------|-------|
| **ID** | S-016 |
| **Version** | 0.1.0 |
| **Status** | draft |
| **Validation Strategy** | manual + browser testing |
| **Related Specs** | S-010 (sdd-mem), S-011 (realtime-capture), S-013 (sqlite-fts5), S-015 (tool-observation) |
| **Owner** | chiche |

---

## Objetivo

Exponer una interfaz web local que permita observar en tiempo real lo que CVM está capturando: entradas de KB siendo creadas, buffers de sesión activos, observaciones de herramientas, y estadísticas agregadas. El dashboard MUST ser iniciado por el usuario con `cvm dashboard`, no arrancar automáticamente. El dashboard es estrictamente read-only: MUST NOT modificar la KB bajo ninguna circunstancia.

---

## Alcance

### Incluido

| ID | Item | Descripción |
|----|------|-------------|
| B-001 | Subcomando `cvm dashboard` | Inicia un servidor HTTP local en el puerto configurado |
| B-002 | Timeline view | Feed cronológico de entradas de KB recientes, auto-refrescante |
| B-003 | Session view | Contenido del buffer de sesión activo (observaciones de tools, prompts de usuario) |
| B-004 | KB browser | Lista buscable de todas las entradas KB (global + local) con full-text search |
| B-005 | Stats view | Conteos, estimaciones de tokens, entradas stale, breakdown por tipo de tag |
| B-006 | Auto-refresh SSE | Server-Sent Events para actualizaciones en tiempo real sin polling activo del browser |
| B-007 | Embedded frontend | HTML/CSS/JS embebido en el binario Go via `//go:embed` — sin archivos externos |
| B-008 | API JSON endpoints | Endpoints REST que sirven datos de la KB como JSON |
| B-009 | Port configurable | Puerto configurable via flag `--port` y variable de entorno `CVM_DASHBOARD_PORT` |
| B-010 | Scope dual | El dashboard lee tanto la KB global como la KB local del directorio donde se invoca |

### Excluido

- Autenticación o autorización (el dashboard es local-only)
- Escritura a la KB desde el dashboard (read-only estricto)
- WebSocket (SSE es suficiente)
- Frameworks web externos (solo `net/http` stdlib)
- React, Vue, o cualquier framework JS (vanilla JS únicamente)
- Base de datos propia del dashboard
- Múltiples proyectos simultáneos (una sola instancia por directorio)
- Modo embebido como servidor de larga duración (el proceso termina con Ctrl-C)

---

## Contratos

### I-001: Subcomando CLI

```
cvm dashboard [--port <int>] [--project <path>]
```

- **I-001a**: `--port` (default: `3333`) MUST aceptar cualquier puerto 1024–65535. Puerto 0 MUST retornar error.
- **I-001b**: `--project` (default: directorio de trabajo actual) especifica el directorio del proyecto para la KB local.
- **I-001c**: Si el puerto está en uso, el proceso MUST fallar con mensaje `"port <N> already in use"` y exit code 1.
- **I-001d**: El proceso MUST imprimir `"Dashboard running at http://localhost:<port>"` a stdout al arrancar.
- **I-001e**: El proceso MUST detenerse limpiamente al recibir SIGINT o SIGTERM (Ctrl-C).
- **I-001f**: La variable de entorno `CVM_DASHBOARD_PORT` MUST sobreescribir el default pero MUST ser sobreescrita por `--port`.

### I-002: API Endpoints

Todos los endpoints retornan `Content-Type: application/json` y HTTP 200 en éxito. En error, retornan HTTP 4xx/5xx con body `{"error": "<mensaje>"}`.

#### GET /api/timeline

Parámetros de query:

| Param | Tipo | Default | Descripción |
|-------|------|---------|-------------|
| `days` | int | `7` | Número de días hacia atrás |
| `limit` | int | `50` | Máximo de entradas a retornar |
| `scope` | string | `"both"` | `"global"`, `"local"`, o `"both"` |

Respuesta:

```json
{
  "days": [
    {
      "date": "2026-04-13",
      "entries": [
        {
          "key": "session-summary-20260413-143022",
          "tags": ["session", "summary"],
          "scope": "global",
          "updated_at": "2026-04-13T14:30:22Z",
          "first_line": "Goal: implement S-016 | Accomplished: spec written",
          "token_estimate": 87
        }
      ]
    }
  ],
  "total": 12
}
```

- **I-002a**: Las entradas MUST estar ordenadas por `updated_at` descendente dentro de cada día.
- **I-002b**: `scope` MUST ser `"global"`, `"local"`, o `"both"`. Valor inválido MUST retornar HTTP 400.
- **I-002c**: `limit` MUST estar entre 1 y 500. Fuera de rango MUST retornar HTTP 400.
- **I-002d**: Si la KB local no existe (no `.cvm` en el proyecto), las entradas locales MUST retornar array vacío sin error.

#### GET /api/session

Parámetros de query:

| Param | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `id` | string | No | Session ID. Si se omite, retorna todos los buffers activos. |

Respuesta (con `id`):

```json
{
  "session_id": "abc123",
  "key": "session-buffer-abc123",
  "lines": [
    { "timestamp": "14:30", "type": "USER", "content": "implement the spec for S-016" },
    { "timestamp": "14:31", "type": "TOOL", "tool": "Write", "content": "wrote /tmp/foo.md" },
    { "timestamp": "14:32", "type": "TOOL", "tool": "Bash", "content": "go build ./..." }
  ],
  "line_count": 3,
  "found": true
}
```

Respuesta (sin `id`, listado de buffers activos):

```json
{
  "buffers": [
    {
      "session_id": "abc123",
      "key": "session-buffer-abc123",
      "line_count": 12,
      "updated_at": "2026-04-13T14:32:00Z"
    }
  ]
}
```

- **I-002e**: El campo `type` MUST ser `"USER"` o `"TOOL"`. Para líneas que no matcheen ningún patrón, `type` MUST ser `"RAW"` y `content` la línea completa.
- **I-002f**: El parsing de líneas MUST seguir el formato `[HH:MM] [TOOL:<name>] <content>` para tool observations y `[HH:MM] USER: <content>` para prompts de usuario (definido en S-011 y S-015).
- **I-002g**: Si `id` se provee pero no existe el buffer, `found` MUST ser `false` y `lines` MUST ser `[]`.

#### GET /api/entries

Parámetros de query:

| Param | Tipo | Default | Descripción |
|-------|------|---------|-------------|
| `q` | string | `""` | Query full-text search |
| `tag` | string | `""` | Filtrar por tag exacto |
| `scope` | string | `"both"` | `"global"`, `"local"`, o `"both"` |
| `limit` | int | `100` | Máximo de entradas |
| `offset` | int | `0` | Para paginación |

Respuesta:

```json
{
  "entries": [
    {
      "key": "gotcha-sqlite-wal-mode",
      "tags": ["gotcha", "sqlite"],
      "scope": "global",
      "enabled": true,
      "created_at": "2026-04-10T09:00:00Z",
      "updated_at": "2026-04-13T14:00:00Z",
      "body": "WAL mode required for concurrent reads during dashboard...",
      "token_estimate": 42
    }
  ],
  "total": 1,
  "offset": 0,
  "limit": 100
}
```

- **I-002h**: Si `q` está presente, el endpoint MUST usar `Backend.Search()` con FTS5. Si `q` está vacío, MUST usar `Backend.List()`.
- **I-002i**: `body` MUST estar presente en la respuesta y contener el body completo de la entrada (sin truncar).
- **I-002j**: `token_estimate` MUST ser `len(body) / 4` (estimación chars/4, consistente con I-004 de S-013).
- **I-002k**: `offset` y `limit` MUST aplicar post-fetch cuando el backend es flat (que no soporta paginación nativa).

#### GET /api/stats

Sin parámetros.

Respuesta:

```json
{
  "global": {
    "total": 42,
    "enabled": 40,
    "stale": 5,
    "total_tokens": 12400,
    "by_type": {
      "decision": 12,
      "learning": 15,
      "gotcha": 8
    },
    "by_topic": {
      "cvm": 5,
      "backend": 3
    }
  },
  "local": {
    "total": 8,
    "enabled": 8,
    "stale": 0,
    "total_tokens": 1200,
    "by_type": {
      "decision": 3
    },
    "by_topic": {
      "infra": 2
    }
  },
  "active_sessions": 1
}
```

- **I-002l**: `stale` MUST ser el count de entradas con `last_referenced` nulo o con más de 30 días sin referencia.
- **I-002m**: `by_type` MUST contener tags clasificados como tipo (`ClassifyTag() == "type"`). `by_topic` MUST contener tags clasificados como tema. Tags internos MUST ser excluidos. Ver S-019 para la clasificación.
- **I-002n**: `active_sessions` MUST ser el count de entradas con key que empieza por `"session-buffer-"` en la KB local.
- **I-002o**: Si la KB global no está inicializada, `global` MUST retornar conteos en cero sin error.

#### GET /api/events

Server-Sent Events stream para actualizaciones en tiempo real.

- **I-002p**: El endpoint MUST retornar `Content-Type: text/event-stream` y `Cache-Control: no-cache`.
- **I-002q**: El servidor MUST emitir un evento SSE cada 2 segundos con `event: tick` y `data: {"ts": "<ISO8601>"}`.
- **I-002r**: El servidor MUST emitir un evento SSE `event: entry_added` con `data: {"key": "<key>", "scope": "<scope>"}` cuando una nueva entrada es detectada (via polling interno al backend cada 2 segundos).
- **I-002s**: El servidor MUST emitir un evento SSE `event: session_updated` con `data: {"session_id": "<id>"}` cuando un buffer de sesión cambia.
- **I-002t**: El servidor MUST enviar comentarios SSE (`: keepalive`) cada 30 segundos para mantener la conexión viva.
- **I-002u**: Al detectar cambios, el servidor MUST comparar snapshots de `updated_at` entre polls, no diff completo de bodies. Esto previene overhead con KBs grandes.

#### GET / (root)

- **I-002v**: La raíz MUST servir el HTML embebido principal (`index.html`) con status 200.
- **I-002w**: Assets estáticos (CSS, JS) MUST estar embebidos y servidos desde rutas bajo `/static/`.

### I-003: Frontend — Estructura de Vistas

El frontend es una SPA mínima con 4 tabs navegables via hash URL:

| Tab | Hash | Descripción |
|-----|------|-------------|
| Timeline | `#timeline` | Feed cronológico de entradas recientes |
| Session | `#session` | Buffer de sesión activo (auto-detecta si hay uno) |
| Browser | `#browser` | Buscador de KB con filtros |
| Stats | `#stats` | Panel de estadísticas |

- **I-003a**: El tab activo MUST sincronizarse con el hash de URL (`window.location.hash`).
- **I-003b**: Al cargar la página sin hash, MUST mostrar `#timeline` por default.
- **I-003c**: La navegación entre tabs MUST ser sin recarga de página.

### I-004: Frontend — Timeline View

- **I-004a**: MUST mostrar entradas ordenadas por `updated_at` descendente, agrupadas por día.
- **I-004b**: MUST auto-refrescar cada 2 segundos via SSE (`event: entry_added` / `event: tick`).
- **I-004c**: Cada entrada MUST mostrar: `key`, `tags` (como badges), `scope` (global/local), tiempo relativo (e.g., "3 min ago"), y primera línea del body.
- **I-004d**: Al hacer click en una entrada, MUST expandir y mostrar el body completo inline (sin navegación a nueva página).
- **I-004e**: MUST tener un selector de scope: "Global", "Local", "Both".
- **I-004f**: El scroll MUST mantenerse en la posición actual cuando llegan nuevas entradas (no auto-scroll al top).
- **I-004g**: MUST mostrar un indicador de conexión SSE (verde = conectado, gris = desconectado).

### I-005: Frontend — Session View

- **I-005a**: MUST auto-detectar el session buffer activo más reciente llamando a `GET /api/session` sin `id`.
- **I-005b**: Si hay exactamente un buffer activo, MUST mostrarlo automáticamente.
- **I-005c**: Si hay múltiples buffers activos, MUST mostrar un selector dropdown para elegir cuál ver.
- **I-005d**: MUST mostrar cada línea del buffer con: timestamp, tipo (`USER` / `TOOL:<name>` / `RAW`), y contenido. Las líneas de tipo `TOOL` MUST mostrar el nombre del tool como badge de color.
- **I-005e**: MUST auto-refrescar cuando SSE emite `event: session_updated`.
- **I-005f**: Si no hay buffers activos, MUST mostrar mensaje "No active session buffer found".
- **I-005g**: El contenido MUST hacer scroll automático al final cuando llegan nuevas líneas (comportamiento de log tail).

### I-006: Frontend — KB Browser

- **I-006a**: MUST mostrar un campo de búsqueda de texto libre que llama a `GET /api/entries?q=<query>` con debounce de 300ms.
- **I-006b**: MUST mostrar un filtro de tag (dropdown o input) que llama a `GET /api/entries?tag=<tag>`.
- **I-006c**: MUST mostrar un selector de scope (Global / Local / Both).
- **I-006d**: Los resultados MUST mostrar: `key`, `tags`, `scope`, `updated_at`, y `token_estimate`.
- **I-006e**: Al hacer click en una entrada, MUST mostrar el body completo en un panel lateral o expandido.
- **I-006f**: MUST mostrar el total de entradas encontradas y el tiempo de respuesta del API en milisegundos.
- **I-006g**: La búsqueda vacía MUST retornar todas las entradas (hasta `limit` 100).

### I-007: Frontend — Stats View

- **I-007a**: MUST mostrar conteos para global y local: total, enabled, stale, total_tokens.
- **I-007b**: MUST mostrar `active_sessions` como un contador destacado.
- **I-007c**: MUST mostrar `by_type` y `by_topic` como secciones separadas ("Types" y "Topics"), cada una ordenada por count descendente. Tags internos MUST ser excluidos. Ver S-019.
- **I-007d**: MUST auto-refrescar cada 10 segundos (no via SSE — poll directo a `/api/stats`).
- **I-007e**: MUST mostrar la estimación total de tokens con el formato humano (e.g., "12.4k tokens").

### I-008: Embedded Assets

- **I-008a**: El binario Go MUST incluir todos los assets web via `//go:embed web/static/*` y `//go:embed web/index.html`.
- **I-008b**: El directorio `web/` MUST contener: `index.html`, `static/app.js`, `static/style.css`.
- **I-008c**: El CSS MUST usar solo propiedades estándar sin vendor prefixes. Paleta: fondo oscuro (#1e1e2e), texto claro (#cdd6f4), badges de colores por tag type.
- **I-008d**: El JS MUST ser vanilla ES2020+ sin transpilación ni bundler. MUST funcionar en Chrome 110+ y Firefox 115+.
- **I-008e**: El HTML MUST ser válido según HTML5 (sin errores en validator.w3.org). Un solo archivo `index.html`.

---

## Behaviors

### B-001: Arranque del servidor

**Given** el usuario ejecuta `cvm dashboard` en `/workspace/myproject`
**When** el puerto 3333 está disponible
**Then**:
- El servidor HTTP arranca y escucha en `0.0.0.0:3333`
- Se imprime `"Dashboard running at http://localhost:3333"` a stdout
- `GET http://localhost:3333/` retorna HTTP 200 con el HTML del dashboard
- El proceso bloquea hasta recibir SIGINT

### B-002: Puerto personalizado

**Given** el usuario ejecuta `cvm dashboard --port 8080`
**When** el puerto 8080 está disponible
**Then** el servidor escucha en 8080 y el mensaje de stdout dice `"http://localhost:8080"`.

### B-003: Puerto ya en uso

**Given** el usuario ejecuta `cvm dashboard` cuando el puerto 3333 está ocupado por otro proceso
**When** el servidor intenta hacer bind
**Then**:
- El proceso imprime `"port 3333 already in use"` a stderr
- Exit code 1
- No se inicia ningún servidor

### B-004: Timeline con datos

**Given** la KB global tiene 3 entradas con `updated_at` en los últimos 2 días y la KB local tiene 1 entrada
**When** el browser hace `GET /api/timeline?scope=both&limit=50`
**Then**:
- La respuesta contiene 2 días con las 4 entradas distribuidas correctamente
- Las entradas de hoy van en el primer objeto del array `days`
- Cada entrada tiene `scope` = `"global"` o `"local"` según corresponda

### B-005: Timeline con KB vacía

**Given** las KB global y local están vacías (inicializadas pero sin entradas)
**When** el browser hace `GET /api/timeline`
**Then**:
- La respuesta retorna `{"days": [], "total": 0}` con HTTP 200
- No se retorna error

### B-006: Session buffer activo

**Given** existe una entrada `session-buffer-abc123` en la KB local con contenido:
```
[14:30] USER: implement the spec for S-016
[14:31] [TOOL:Write] wrote /workspace/cvm/specs/dashboard.spec.md
[14:32] [TOOL:Bash] go build ./...
```
**When** el browser hace `GET /api/session?id=abc123`
**Then**:
- `found` es `true`
- `lines` tiene 3 elementos
- El elemento 0 tiene `type: "USER"`, `timestamp: "14:30"`, `content: "implement the spec for S-016"`
- El elemento 1 tiene `type: "TOOL"`, `tool: "Write"`, `content: "wrote /workspace/cvm/specs/dashboard.spec.md"`
- El elemento 2 tiene `type: "TOOL"`, `tool: "Bash"`, `content: "go build ./..."`

### B-007: Session buffer inexistente

**Given** no existe ninguna entrada con key `session-buffer-xyz999` en la KB local
**When** el browser hace `GET /api/session?id=xyz999`
**Then** la respuesta retorna `{"session_id": "xyz999", "key": "session-buffer-xyz999", "lines": [], "line_count": 0, "found": false}` con HTTP 200.

### B-008: Búsqueda full-text

**Given** la KB global tiene una entrada con key `gotcha-sqlite-wal` y body que contiene "WAL mode"
**When** el browser hace `GET /api/entries?q=WAL+mode&scope=global`
**Then**:
- La respuesta contiene esa entrada
- `total` es >= 1
- La entrada tiene `body` completo con el texto "WAL mode"

### B-009: Stats con sesión activa

**Given** existe `session-buffer-abc123` en la KB local y hay 42 entradas globales y 8 locales
**When** el browser hace `GET /api/stats`
**Then**:
- `global.total` es `42`
- `local.total` es `8`
- `active_sessions` es `1`

### B-010: SSE tick

**Given** el browser tiene una conexión abierta a `GET /api/events`
**When** pasan 2 segundos
**Then** el cliente recibe al menos un evento `event: tick` con `data` JSON válido que incluye `"ts"`.

### B-011: SSE nueva entrada

**Given** el browser tiene una conexión abierta a `GET /api/events`
**When** se agrega una nueva entrada a la KB (via `cvm kb put` en otra terminal)
**Then** dentro de los próximos 4 segundos el cliente recibe `event: entry_added` con el `key` de la nueva entrada.

### B-012: KB no inicializada

**Given** el usuario ejecuta `cvm dashboard` desde un directorio sin `.cvm/` local
**When** el browser hace `GET /api/timeline?scope=local`
**Then**:
- La respuesta retorna `{"days": [], "total": 0}` con HTTP 200
- No hay error 500
- La KB global sigue sirviendo normalmente

### B-013: KB muy grande

**Given** la KB global tiene 2000 entradas
**When** el browser hace `GET /api/timeline?limit=50`
**Then**:
- La respuesta retorna exactamente 50 entradas (las más recientes)
- El tiempo de respuesta MUST ser < 500ms

### B-014: Múltiples dashboards intentando arrancar

**Given** ya hay un proceso `cvm dashboard` corriendo en el puerto 3333
**When** el usuario ejecuta un segundo `cvm dashboard` en el mismo puerto
**Then** el segundo proceso falla con `"port 3333 already in use"` y exit code 1, sin afectar el primer proceso.

---

## Edge Cases

| ID | Scenario | Expected Behavior |
|----|----------|------------------|
| E-001 | Backend es flat-file (no SQLite) | El dashboard MUST funcionar. Las búsquedas FTS no estarán disponibles — `GET /api/entries?q=<query>` MUST retornar resultados filtrados por substring simple (case-insensitive) sobre el body. |
| E-002 | DB SQLite corrupta al arranque | El servidor MUST arrancar igualmente usando FlatBackend como fallback. MUST loguear `"warning: SQLite unavailable, using flat backend"` a stderr. |
| E-003 | Línea en session buffer con formato inesperado | La línea MUST ser parseada como `type: "RAW"` con `content` igual a la línea completa. No debe causar panic ni error en el endpoint. |
| E-004 | `limit` = 0 en `/api/entries` | MUST retornar HTTP 400 con `{"error": "limit must be between 1 and 500"}`. |
| E-005 | `scope` = `"invalid"` en cualquier endpoint | MUST retornar HTTP 400 con `{"error": "scope must be global, local, or both"}`. |
| E-006 | Browser desconecta el SSE stream | El servidor MUST detectar la desconexión via context cancellation y cerrar la goroutine del SSE handler. No debe haber goroutine leak. |
| E-007 | Body de entrada contiene markdown con caracteres especiales | El body MUST retornarse como string JSON válido. El JSON MUST estar correctamente escaped. |
| E-008 | KB tiene entradas con tags malformadas (no JSON array) | El servidor MUST normalizar tags malformadas a `[]` sin retornar error. |

---

## Invariantes

| ID | Invariante |
|----|------------|
| I-INV-001 | El servidor NEVER modifica la KB. Ningún handler HTTP tiene permiso de write. Todas las operaciones sobre el Backend usan métodos read-only: `List`, `Search`, `Timeline`, `Stats`, `Get`, `Compact`. |
| I-INV-002 | El servidor NEVER carga el body completo de todas las entradas en memoria para servir `/api/timeline`. Solo carga `CompactEntry` (key, tags, first_line, updated_at) y el body completo solo cuando se pide una entrada individual. |
| I-INV-003 | El servidor NEVER emite datos cross-project. La KB local que se lee MUST corresponder al `--project` con el que arrancó el proceso. |
| I-INV-004 | El SSE handler MUST usar una goroutine por conexión cliente. El número de goroutines activas MUST ser acotado por el número de clientes SSE conectados simultáneamente. |
| I-INV-005 | Los assets HTML/CSS/JS MUST estar embebidos en el binario. El servidor MUST arrancar correctamente aunque el directorio `web/` no exista en el filesystem en runtime. |

---

## Errores

| Código HTTP | Situación | Formato body |
|-------------|-----------|--------------|
| 400 | Parámetro inválido (limit, scope, port) | `{"error": "<descripción>"}` |
| 404 | Ruta no encontrada | `{"error": "not found"}` |
| 500 | Error interno (panic de backend, I/O failure) | `{"error": "internal server error"}` |
| N/A (arranque) | Puerto en uso | stderr: `"port <N> already in use"`, exit code 1 |
| N/A (arranque) | Puerto inválido | stderr: `"port must be between 1024 and 65535"`, exit code 1 |

---

## Restricciones No Funcionales

| ID | Restricción |
|----|-------------|
| NF-001 | Tiempo de arranque MUST ser < 200ms desde `cvm dashboard` hasta que el servidor acepta conexiones. |
| NF-002 | Respuesta de `/api/timeline?limit=50` MUST ser < 500ms en una KB con hasta 2000 entradas. |
| NF-003 | Respuesta de `/api/stats` MUST ser < 300ms en una KB con hasta 2000 entradas. |
| NF-004 | El binario Go MUST buildar correctamente en macOS ARM64 y AMD64 con `go build ./...`. No se aceptan CGO dependencies (usar `modernc.org/sqlite` pure-Go). |
| NF-005 | El frontend MUST cargar y ser usable en < 1 segundo en localhost (sin throttling). Sin assets externos (no CDN, no Google Fonts). |
| NF-006 | El servidor MUST manejar al menos 5 clientes SSE simultáneos sin degradación perceptible. |
| NF-007 | El proceso MUST liberar el puerto al terminar. No debe dejar el puerto en TIME_WAIT prolongado. |

---

## Estructura de Archivos

```
cmd/dashboard/         # Subcomando CLI
  main.go              # Entry point: parse flags, start server
internal/dashboard/    # Lógica del servidor
  server.go            # HTTP server, routes, SSE
  api.go               # Handlers para /api/* endpoints
  parser.go            # Parsing de líneas de session buffer
  watcher.go           # Polling interno para detectar cambios de KB
web/                   # Assets embebidos
  index.html           # SPA principal (todo el shell HTML)
  static/
    app.js             # Lógica de frontend (vanilla JS)
    style.css          # Estilos
```

---

## Plan de Implementación

### Wave 1 — Scaffolding y servidor básico

- Crear estructura de directorios `cmd/dashboard/`, `internal/dashboard/`, `web/`
- Implementar `cmd/dashboard/main.go`: parse de flags `--port` y `--project`, validación, arranque
- Implementar `internal/dashboard/server.go`: `net/http` server, shutdown graceful con SIGINT/SIGTERM
- Crear `web/index.html` mínimo (placeholder "CVM Dashboard") y `web/static/app.js`, `web/static/style.css` vacíos
- Wiring de `//go:embed` en el servidor
- Registrar `dashboard` como subcomando en el CLI principal de CVM
- Gate: `cvm dashboard` arranca, sirve el placeholder HTML, y para limpiamente con Ctrl-C

### Wave 2 — API endpoints (sin SSE)

- Implementar `internal/dashboard/api.go` con los 4 endpoints:
  - `GET /api/timeline` usando `Backend.Timeline()` + `Backend.Compact()`
  - `GET /api/session` usando `Backend.Get()` + `Backend.List(tag="session-buffer")`
  - `GET /api/entries` usando `Backend.Search()` o `Backend.List()`
  - `GET /api/stats` usando `Backend.Stats()` + conteo de `session-buffer-*` entries
- Implementar `internal/dashboard/parser.go`: parsing de líneas de session buffer
- Implementar validación de parámetros (scope, limit, offset)
- Gate: todos los endpoints retornan JSON correcto consultados con `curl`

### Wave 3 — SSE y watcher

- Implementar `internal/dashboard/watcher.go`: goroutine que pollea la KB cada 2 segundos y compara snapshots de `updated_at`
- Implementar `GET /api/events` en `server.go`: SSE handler con tick cada 2s, eventos `entry_added` y `session_updated`
- Manejar desconexión del cliente via context cancellation
- Gate: `curl -N http://localhost:3333/api/events` muestra eventos tick y entry_added cuando se agrega una entrada

### Wave 4 — Frontend completo

- Implementar `web/static/app.js`: tab navigation, fetch de APIs, rendering de Timeline, Session, Browser, Stats
- Implementar `web/static/style.css`: paleta oscura, badges de tags, layout responsive básico
- Conectar SSE desde el frontend para auto-refresh
- Gate: abrir el dashboard en Chrome muestra las 4 vistas funcionales con datos reales de la KB

### Wave 5 — Edge cases y pulido

- Manejar KB no inicializada (local), DB corrupta (flat fallback), backend flat-file (búsqueda substring)
- Manejar formatos de línea inesperados en parser.go (fallback a RAW)
- Validar tags malformadas
- Asegurar que no hay goroutine leaks (SSE disconnect)
- Gate: todos los edge cases E-001 a E-008 pasan verificación manual

---

## Especificaciones Relacionadas

- **S-010** (sdd-mem): Define el formato de entradas KB, tipos válidos, y estructura de índice
- **S-011** (realtime-capture): Define el formato de session buffer (`session-buffer-<id>`) y el lifecycle de captura
- **S-013** (sqlite-fts5): Define la interfaz `Backend` que el dashboard usa para leer datos
- **S-015** (tool-observation): Define el formato de líneas `[HH:MM] [TOOL:<name>] <content>` que el parser debe entender

---

## Dependencias

- Go stdlib: `net/http`, `embed`, `encoding/json`, `os/signal`, `context`
- `github.com/chichex/cvm/internal/kb` — Backend interface y factory
- `github.com/chichex/cvm/internal/config` — Scope y paths de KB
- `modernc.org/sqlite` — Ya presente en el módulo (pure-Go SQLite para build sin CGO)
- Sin nuevas dependencias externas

---

## Changelog

| Version | Fecha | Cambio |
|---------|-------|--------|
| 0.1.0 | 2026-04-13 | Draft inicial |

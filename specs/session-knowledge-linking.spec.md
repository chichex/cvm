# S-020: Dashboard — Clickable Session-Knowledge Linking

| Field | Value |
|-------|-------|
| **ID** | S-020 |
| **Version** | 0.2.0 |
| **Status** | verified |
| **Validation Strategy** | tests post-impl + manual browser testing |
| **Related Specs** | S-016 (dashboard), S-017 (session-system), S-018 (nested-retro-sessions) |
| **Owner** | chiche |
| **GitHub Issue** | #14 |

---

## Objetivo

Hacer que las knowledge entries mostradas en session cards sean clickeables y naveguen al Knowledge tab, que el Knowledge tab muestre de qué sesión viene cada entry, y que las sesiones activas muestren knowledge siendo capturada en realtime.

---

## Alcance

### Incluido

| ID | Item | Descripción |
|----|------|-------------|
| B-001 | Clickable knowledge pills | Las knowledge pills en session cards MUST ser clickeables y navegar al Knowledge tab |
| B-002 | Knowledge tab highlight | Al navegar desde una session card, la entry objetivo MUST resaltarse visualmente |
| B-003 | Session origin badge | El Knowledge tab MUST mostrar de qué sesión viene cada entry que tiene `session_id` |
| B-004 | Realtime knowledge in active sessions | Las session cards activas MUST mostrar knowledge entries nuevas sin recarga manual (via full session list reload en SSE `entry_added`) |
| B-005 | API: session_id en entries | El endpoint `/api/entries` MUST incluir `session_id` en la respuesta cuando exista |
| B-006 | kb.Entry: SessionID field | `kb.Entry` MUST exponer `SessionID` para que los reads del Backend lo devuelvan |

### Excluido

- Navegación inversa: click en session badge dentro de Knowledge tab → volver a Sessions tab (future)
- Filtro por session_id en Knowledge tab (future)
- Cambios en `/api/sessions` (ya devuelve knowledge entries correlacionadas)

---

## Contratos

### C-001: Cambio en `kb.Entry` (kb.go)

```go
type Entry struct {
    Key            string    `json:"key"`
    Tags           []string  `json:"tags"`
    Enabled        bool      `json:"enabled"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
    LastReferenced time.Time `json:"last_referenced,omitempty"`
    SessionID      string    `json:"session_id,omitempty"` // NEW — S-020
}
```

- **C-001a**: `SessionID` MUST ser `omitempty` — solo presente cuando la entry tiene session linkada.
- **C-001b**: La columna `session_id` ya existe en SQLite (migración de S-017). No se requiere nueva migración.
- **C-001c**: `scanDocument` y `scanEntries` en `sqlite_backend.go` MUST incluir `session_id` en el SELECT y poblar `Entry.SessionID`.
- **C-001d**: `parseEntry` MUST aceptar `sessionID string` y asignarlo a `Entry.SessionID`.
- **C-001e**: Los métodos `Get()`, `List()`, `Search()`, `Timeline()`, y `LoadDocuments()` MUST devolver `SessionID` poblado cuando existe en SQLite.
- **C-001f**: `FlatBackend` no persiste `session_id`. Los reads MUST devolver zero-value (`""`), que `omitempty` omite del JSON. Sin cambios en flat.go.

### C-002: Cambio en `entryJSON` (api.go)

```go
type entryJSON struct {
    Key           string   `json:"key"`
    Tags          []string `json:"tags"`
    Scope         string   `json:"scope"`
    Enabled       bool     `json:"enabled"`
    CreatedAt     string   `json:"created_at"`
    UpdatedAt     string   `json:"updated_at"`
    Body          string   `json:"body"`
    TokenEstimate int      `json:"token_estimate"`
    SessionID     string   `json:"session_id,omitempty"` // NEW — S-020
}
```

- **C-002a**: `session_id` MUST ser `omitempty` — solo presente cuando la entry tiene session linkada.
- **C-002b**: El valor MUST ser exact string match con el `id` del session card correspondiente en `/api/sessions`.
- **C-002c**: `handleEntries` MUST poblar `SessionID` desde `doc.Entry.SessionID` (disponible por C-001).

### C-003: Navegación cross-tab (app.js)

```
navigateToKnowledgeEntry(key: string): void
```

- **C-003a**: MUST cambiar el tab activo a `knowledge`.
- **C-003b**: MUST resetear filtros del Knowledge tab (q vacío, tag vacío, scope both) y recargar la lista. Justificación: la entry objetivo podría no aparecer con los filtros previos del usuario.
- **C-003c**: MUST seleccionar la entry con el key dado en la lista via exact string match en `entry.key === key`.
- **C-003d**: MUST hacer scroll a la entry en la lista via `scrollIntoView()` y aplicar clase CSS `.highlight-entry`.
- **C-003e**: MUST mostrar el detalle de la entry en el panel derecho.
- **C-003f**: La clase `.highlight-entry` MUST removerse después de 2 segundos. La remoción usa un JS `setTimeout`; la transición visual (fade out) usa CSS `transition` en la clase.
- **C-003g**: El key se pasa como argumento JS string (no via URL/hash). No requiere URL encoding.

### C-004: Session origin badge (app.js)

```
renderSessionBadge(sessionId: string): HTMLElement
```

- **C-004a**: Si `sessionId.length > 8`, MUST mostrar los primeros 8 caracteres + "…". Si `<= 8`, MUST mostrar el ID completo sin ellipsis.
- **C-004b**: MUST usar la clase CSS `.badge--session`.
- **C-004c**: MUST tener `title` attribute con el session ID completo (siempre, truncado o no).

---

## Behaviors

### B-001: Click en knowledge pill navega al Knowledge tab

**Given** el usuario está en el Sessions tab y una session card tiene 2 knowledge entries visibles
**When** el usuario hace click en la knowledge pill con key `gotcha-sqlite-wal`
**Then**:
- El tab activo cambia a Knowledge
- El hash de URL cambia a `#knowledge`
- La entry `gotcha-sqlite-wal` aparece seleccionada en la lista
- La entry tiene un highlight visual (borde/fondo) que se desvanece en 2 segundos
- El panel de detalle muestra el body completo de `gotcha-sqlite-wal`

### B-002: Knowledge tab muestra session origin

**Given** la KB tiene una entry `learning-sse-keepalive` con `session_id = "778a7b24-509f-4f79-a99e-cd01e631ef82"`
**When** el usuario está en el Knowledge tab y ve la lista de entries
**Then**:
- La entry card muestra un badge `.badge--session` con texto `778a7b24…`
- El badge tiene `title = "778a7b24-509f-4f79-a99e-cd01e631ef82"`
- Entries sin `session_id` NO muestran badge de sesión

### B-003: Active session muestra knowledge en realtime

**Given** el usuario está en el Sessions tab viendo una session card activa con 0 knowledge entries
**When** SSE emite `entry_added` con key `gotcha-new-finding` y scope `global`
**Then**:
- La session list completa se recarga (mecanismo existente: `loadSessions()` en handler de `entry_added`)
- Si la nueva entry tiene `session_id` que matchea la session card, la entry aparece en la sección de knowledge
- El contador del toggle se actualiza (e.g., "1 linked knowledge entry")

### B-004: Navegación resetea filtros del Knowledge tab

**Given** el usuario está en el Knowledge tab con búsqueda activa `q=sqlite` y tag filter `gotcha`
**When** el usuario navega al Sessions tab, luego hace click en una knowledge pill con key `learning-wal-mode`
**Then**:
- Los filtros del Knowledge tab se resetean (q vacío, tag vacío, scope both)
- La lista se recarga sin filtros
- La entry `learning-wal-mode` se muestra seleccionada y con highlight

### B-005: API devuelve session_id en entries

**Given** la KB tiene entry `learning-sse-keepalive` con `session_id = "778a7b24-509f-4f79-a99e-cd01e631ef82"` y entry `gotcha-sqlite-wal` sin session_id
**When** el browser hace `GET /api/entries?scope=both`
**Then**:
- La entry `learning-sse-keepalive` tiene `"session_id": "778a7b24-509f-4f79-a99e-cd01e631ef82"` en el JSON
- La entry `gotcha-sqlite-wal` NO tiene campo `session_id` en el JSON (omitempty)

### B-006: kb.Entry expone SessionID en reads

**Given** la tabla `entries` tiene un row con `key = "learning-sse-keepalive"` y `session_id = "778a7b24-509f-4f79-a99e-cd01e631ef82"`
**When** se llama a `backend.Get("learning-sse-keepalive")`
**Then**:
- `doc.Entry.SessionID` es `"778a7b24-509f-4f79-a99e-cd01e631ef82"`
- Lo mismo aplica para `List()`, `Search()`, `Timeline()`, y `LoadDocuments()`

---

## Edge Cases

| ID | Scenario | Expected Behavior |
|----|----------|------------------|
| E-001 | Click en knowledge pill pero la entry fue eliminada entre el load de sessions y la navegación | El Knowledge tab MUST cargar normalmente. Si la entry no existe en la lista, MUST mostrar el panel de detalle vacío con mensaje "Entry not found". No error. |
| E-002 | Knowledge tab ya tiene la entry cargada cuando se navega desde session card | MUST resetear filtros y recargar (C-003b). La entry se selecciona post-recarga. |
| E-003 | Session ID es UUID completo (36 chars) | El badge MUST truncar a los primeros 8 caracteres + "…". `title` attribute MUST contener el ID completo. |
| E-004 | Flat backend (no SQLite) — entries no tienen session_id | `Entry.SessionID` es zero-value (`""`). `omitempty` lo omite del JSON. Session badges no se renderizan. Comportamiento degradado graceful. |
| E-005 | Multiple rapid clicks en knowledge pills | Leading-edge throttle: el primer click ejecuta inmediatamente; clicks subsecuentes dentro de 300ms son ignorados. |

---

## Invariantes

| ID | Invariante |
|----|------------|
| I-001 | El dashboard NEVER modifica la KB. Los clicks en knowledge pills son navegación read-only. |
| I-002 | La navegación cross-tab MUST ser idempotente: navegar a la misma entry dos veces produce el mismo resultado visual. |
| I-003 | El SSE handler existente MUST seguir funcionando sin cambios. B-003 se satisface con el full session list reload que ya ocurre en `entry_added`. |
| I-004 | Las knowledge pills MUST seguir funcionando como antes (expandir/colapsar lista) cuando no se hace click en una pill individual. El click en el toggle abre la lista; el click en una pill individual navega. |
| I-005 | La interfaz `Backend` no cambia su signature. Solo cambia el contenido de `Entry` (nuevo campo). Todos los callers existentes siguen funcionando sin modificación. |

---

## Errores

No se introducen nuevos errores HTTP. El único cambio backend es agregar un campo a `entryJSON` y poblar `SessionID` en `kb.Entry` reads.

---

## Restricciones No Funcionales

| ID | Restricción |
|----|-------------|
| NF-001 | La navegación cross-tab MUST completarse en < 300ms (percepción instantánea). Validación: manual browser testing. |
| NF-002 | El highlight animation MUST usar CSS transitions para el fade visual. El `setTimeout` de 2s para remover la clase `.highlight-entry` es aceptable. |
| NF-003 | Cero dependencias nuevas. Vanilla JS + CSS. |

---

## Dependencias

### Backend (kb package)
- `kb.go`: agregar `SessionID` a `Entry` struct (C-001)
- `sqlite_backend.go`: actualizar `scanDocument`, `scanEntries`, `parseEntry`, y queries de `Get`, `List`, `Search`, `Timeline`, `LoadDocuments` (C-001c–e)
- `flat.go`: sin cambios (C-001f)

### Dashboard API
- `api.go`: agregar `SessionID` a `entryJSON`, poblar desde `doc.Entry.SessionID` (C-002)

### Dashboard Frontend
- `app.js`: `navigateToKnowledgeEntry()`, click handler en knowledge pills, `renderSessionBadge()`, highlight CSS class (C-003, C-004)
- `style.css`: `.highlight-entry`, `.badge--session` (C-003d, C-004b)

### Sin cambios
- `/api/sessions` — ya devuelve knowledge entries correlacionadas
- SSE handler — ya emite `entry_added` y `session_updated`
- Hash routing — ya implementado (`switchTab()`)

---

## Nota: Drift de S-016

S-016 (dashboard, v0.1.0, draft) describe 4 tabs (`#timeline`, `#session`, `#browser`, `#stats`) pero la implementación actual tiene 3 tabs (`#sessions`, `#knowledge`, `#stats`). S-020 usa los nombres de la implementación actual. S-016 requiere actualización independiente.

---

## Changelog

| Version | Fecha | Cambio |
|---------|-------|--------|
| 0.1.0 | 2026-04-14 | Draft inicial |
| 0.2.0 | 2026-04-14 | Post-review Opus: agregar C-001 (kb.Entry.SessionID), B-006, UUIDs reales en test data, fix truncation rules, clarificar throttle vs debounce, documentar filter reset rationale, nota drift S-016 |

# S-021 — Retro Summary Observability

- **ID**: S-021
- **Version**: 0.2.1
- **Status**: draft
- **Parent**: S-018 (nested-retro-sessions)
- **Validation**: Tests post-impl + manual browser testing

## Objetivo

Cuando el retro de una sesión termina, el dashboard muestra "✓ Retro complete" pero no indica
qué hizo el retro ni por qué no hay learnings. El usuario no tiene forma de distinguir entre
"el retro corrió y no encontró nada nuevo" vs "el retro falló" vs "el retro encontró 3 learnings".

Este spec agrega un summary estructurado a la sesión padre que el dashboard renderiza
inline, dando observabilidad completa del resultado del retro.

## Alcance

- Schema: agregar columna `retro_summary TEXT` a tabla `sessions` (en el row del parent)
- Backend: `generateRetro()` construye summary JSON y lo persiste en la sesión padre
- API: incluir `retro_summary` parseado en `sessionCardJSON`
- Frontend: renderizar stats del retro inline en la card del padre

**Fuera de alcance:**
- Endpoint separado para raw output (raw_output va inline, capped a 10K)
- Cambios a la retro child session (S-018 no cambia)

## Contratos

### C-001 — Schema migration

```go
// In openGlobalDB(), after existing migrations:
var hasRetroSummaryCol int
if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('sessions') WHERE name='retro_summary'").Scan(&hasRetroSummaryCol); err == nil && hasRetroSummaryCol == 0 {
    db.Exec("ALTER TABLE sessions ADD COLUMN retro_summary TEXT")
}
```

La columna almacena JSON stringificado. NULL para sesiones sin retro o pre-S-021.

### C-002 — RetroSummary struct

```go
// In internal/session/session.go
type RetroSummary struct {
    EntriesFound     int    `json:"entries_found"`      // entries en el JSON array parseado de claude
    EntriesPersisted int    `json:"entries_persisted"`   // guardadas exitosamente via kb.Put
    EntriesSkipped   int    `json:"entries_skipped"`     // empty key || kb.Put retornó error
    Error            string `json:"error,omitempty"`     // mensaje de error si claude o parse falló
    RawOutput        string `json:"raw_output"`          // output de claude -p, truncado a 10000 runes
}
```

**Definición de `entries_skipped`:** incrementar por cada entry del JSON array parseado donde:
- `entry.Key == ""` (empty key), O
- `kb.Put()` retornó error (write failure)

`entries_found = entries_persisted + entries_skipped` MUST ser invariante.

### C-003 — generateRetro persiste summary en parent

`generateRetro()` MUST:
1. Capturar el raw output de `claude -p` y aplicar `TrimSpace` (eliminar whitespace leading/trailing)
2. Truncar raw output a 10000 runes. Si truncado, appendear `" [truncated]"` (suffix NO cuenta en el límite)
3. Construir un `RetroSummary` con los stats del resultado
4. Persistir en la sesión **padre**: `UPDATE sessions SET retro_summary = ? WHERE id = ?` usando `sessionID` (primer parámetro de `generateRetro`)
5. Si claude falla (exit != 0): construir summary con `Error` y `RawOutput` vacío, persistir igualmente
6. Si el parse del JSON falla: construir summary con `Error` y `RawOutput` con el output que no se pudo parsear
7. Si el UPDATE falla: log warning a stderr, no bloquear End()

La firma de `generateRetro` NO cambia.

### C-004 — API response

```go
type sessionCardJSON struct {
    // ... existing fields ...
    RetroSummary *retroSummaryJSON `json:"retro_summary,omitempty"` // NEW
}

type retroSummaryJSON struct {
    EntriesFound     int    `json:"entries_found"`
    EntriesPersisted int    `json:"entries_persisted"`
    EntriesSkipped   int    `json:"entries_skipped"`
    Error            string `json:"error,omitempty"`
    RawOutput        string `json:"raw_output"`
}
```

`GET /api/sessions` MUST:
- Parsear `retro_summary` de la columna TEXT de cada sesión
- Incluir como `retro_summary` en el JSON response (campo omitido si NULL)
- `raw_output` se incluye inline (max 10K runes + posible suffix)

### C-005 — Frontend rendering

Dentro de la retro indicator section (app.js, retro badge area):

**Retro activo** (sin cambios):
- `retro_session.status === "active"`: `"⟳ Summarizing…"`

**Retro terminado con summary:**
- `retro_summary.error` vacío/ausente AND `entries_found > 0`:
  `"✓ Retro: N found, M persisted"` (N = entries_found, M = entries_persisted)
- `retro_summary.error` vacío/ausente AND `entries_found === 0`:
  `"✓ Retro: no new learnings"`
- `retro_summary.error` presente:
  `"⚠ Retro failed"` — el error message se muestra inline expandible debajo del badge (click en el badge togglea visibilidad)

**Retro terminado sin summary** (backward compat pre-S-021):
- `"✓ Retro complete"` (comportamiento actual, sin stats)

**raw_output expandible:**
- Debajo de los stats del retro, un toggle `"Show raw output"` / `"Hide raw output"`
- Click muestra el `raw_output` en un `<pre>` element con `overflow-x: auto`
- Solo visible si `raw_output` no está vacío

## Behaviors

### B-001 — Retro exitoso persiste summary en parent

**Given** una sesión `abc-123` con 10 eventos
**And** claude -p retorna `[{"key":"k1","body":"b1","tags":["learning"]},{"key":"k2","body":"b2","tags":["decision"]},{"key":"","body":"b3","tags":["gotcha"]}]`
**And** kb.Put para `k1` y `k2` sucede exitosamente
**When** `generateRetro("abc-123", events, "haiku")` termina
**Then** la sesión `abc-123` MUST tener `retro_summary` con:
  - `entries_found: 3`
  - `entries_persisted: 2`
  - `entries_skipped: 1` (key vacío)
  - `error: ""` (omitido en JSON)
  - `raw_output: <output completo>`

### B-002 — Retro sin learnings nuevos (output `[]`)

**Given** una sesión `abc-123` donde todo ya fue capturado
**And** claude -p retorna `[]`
**When** `generateRetro` termina
**Then** la sesión `abc-123` MUST tener `retro_summary` con:
  - `entries_found: 0`, `entries_persisted: 0`, `entries_skipped: 0`
  - `error: ""` (omitido), `raw_output: "[]"`

### B-003 — Retro sin learnings nuevos (stdout vacío)

**Given** una sesión `abc-123`
**And** claude -p retorna stdout vacío (`""`)
**When** `generateRetro` termina
**Then** la sesión `abc-123` MUST tener `retro_summary` con:
  - `entries_found: 0`, `entries_persisted: 0`, `entries_skipped: 0`
  - `error: ""` (omitido), `raw_output: ""`

### B-004 — Retro con error de invocación

**Given** una sesión `abc-123`
**And** `claude -p` falla (exit code != 0)
**When** `generateRetro` maneja el error
**Then** la sesión `abc-123` MUST tener `retro_summary` con:
  - `entries_found: 0`, `entries_persisted: 0`, `entries_skipped: 0`
  - `error: "claude invocation failed: <mensaje>"`
  - `raw_output: ""`

### B-005 — Retro con error de parse

**Given** una sesión `abc-123`
**And** claude -p retorna `"Sorry, I can't help with that"` (no es JSON válido)
**When** `generateRetro` maneja el error de parse
**Then** la sesión `abc-123` MUST tener `retro_summary` con:
  - `entries_found: 0`, `entries_persisted: 0`, `entries_skipped: 0`
  - `error: "parsing retro output: invalid character 'S' looking for beginning of value"`
  - `raw_output: "Sorry, I can't help with that"`

### B-006 — Dashboard muestra stats del retro

**Given** sesión `abc-123` con `retro_summary` JSON: `{"entries_found":3,"entries_persisted":2,"entries_skipped":1,"raw_output":"[...]"}`
**When** `GET /api/sessions` es llamado
**Then** `abc-123.retro_summary` MUST contener `{entries_found: 3, entries_persisted: 2, entries_skipped: 1, raw_output: "[...]"}`

### B-007 — Frontend renderiza stats inline

**Given** un session card con `retro_summary.entries_found: 3` y `entries_persisted: 2`
**When** el card se renderiza
**Then** el retro indicator MUST mostrar `"✓ Retro: 3 found, 2 persisted"`

### B-008 — Frontend renderiza retro sin learnings

**Given** un session card con `retro_summary.entries_found: 0` y sin `error`
**When** el card se renderiza
**Then** el retro indicator MUST mostrar `"✓ Retro: no new learnings"`

### B-009 — Frontend renderiza retro con error

**Given** un session card con `retro_summary.error: "claude invocation failed: timeout"`
**When** el card se renderiza
**Then** el retro indicator MUST mostrar `"⚠ Retro failed"`
**And** debajo MUST haber un elemento expandible con el mensaje de error completo

### B-010 — Frontend renderiza raw output expandible

**Given** un session card con `retro_summary.raw_output: "[{\"key\":\"k1\",...}]"`
**When** el usuario clickea "Show raw output"
**Then** el raw output MUST mostrarse en un `<pre>` element
**And** un segundo click MUST ocultarlo

## Edge Cases

### E-001 — kb.Put falla para una entry

**Given** claude -p retorna 3 entries
**And** kb.Put falla para la segunda entry con `"disk full"`
**When** `generateRetro` termina
**Then** `entries_found: 3`, `entries_persisted: 2`, `entries_skipped: 1`
**And** `error` MUST estar vacío (kb.Put failures son skips, no errors globales)

### E-002 — raw_output excede 10000 runes

**Given** claude -p retorna un output de 15000 runes
**When** se construye el summary
**Then** `raw_output` MUST contener las primeras 10000 runes + `" [truncated]"`
**And** `len([]rune(raw_output))` MUST ser 10012 (10000 + len(" [truncated]"))

### E-003 — raw_output exactamente 10000 runes

**Given** claude -p retorna un output de exactamente 10000 runes
**When** se construye el summary
**Then** `raw_output` MUST contener el output completo SIN suffix

### E-004 — Migration sobre DB existente

**Given** tabla `sessions` sin columna `retro_summary`
**When** `openGlobalDB()` corre
**Then** la migración MUST agregar la columna via PRAGMA check (como las migraciones existentes)
**And** sesiones existentes MUST tener `retro_summary = NULL`
**And** una segunda ejecución MUST ser no-op

### E-005 — Backward compat: sesiones sin retro_summary

**Given** sesiones creadas antes de S-021 (retro_summary = NULL)
**When** el dashboard las renderiza
**Then** si tienen `retro_session`: MUST mostrar `"✓ Retro complete"` (sin stats)
**And** si no tienen `retro_session`: sin cambios

### E-006 — Summary persistence falla

**Given** `generateRetro` completa exitosamente (entries persistidas en KB)
**And** el UPDATE de `retro_summary` falla (DB locked, etc.)
**When** se maneja el error
**Then** MUST logear `"warning: failed to persist retro summary: <error>"` a stderr
**And** MUST NOT afectar el return value de `generateRetro` (best-effort)
**And** el dashboard MUST mostrar `"✓ Retro complete"` (E-005 fallback)

### E-007 — Multibyte Unicode en raw_output

**Given** claude -p retorna output con emojis/CJK (multibyte UTF-8)
**When** se aplica truncation
**Then** MUST truncar a 10000 **runes** (no bytes), sin cortar caracteres por la mitad

## Invariantes

### I-001
`retro_summary` MUST ser NULL o JSON válido que deserializa a `RetroSummary`. Nunca texto libre.

### I-002
`entries_found == entries_persisted + entries_skipped` MUST ser true siempre que `error` esté vacío.

### I-003
La persistencia del summary MUST ser best-effort — un fallo al guardar el summary MUST NOT causar que `generateRetro` retorne un error diferente al que ya retornaría.

### I-004
`raw_output` almacenado MUST tener ≤ 10000 runes + posible suffix `" [truncated]"` (12 chars).

## Errores

| Condition | Behavior |
|-----------|----------|
| UPDATE de retro_summary falla | Log warning, no-op — E-005 fallback en UI |
| JSON marshal del summary falla | Log warning, no-op |
| claude -p exit != 0 | Construir summary con Error, persistir |
| JSON parse del output falla | Construir summary con Error + RawOutput, persistir |
| kb.Put falla para entry individual | Incrementar entries_skipped, log warning, continuar |
| raw_output > 10000 runes | Truncar + suffix |

## Restricciones no funcionales

- La migración MUST ser idempotente (PRAGMA check pattern)
- El summary se persiste con una sola query UPDATE — no N+1
- `raw_output` capped a ~10K runes para mantener payload del API razonable

## Specs relacionadas

- S-018 (nested-retro-sessions): define la child session y el parent link (no cambia)
- S-017 (session-system): lifecycle, generateRetro(), schema base
- S-020 (session-knowledge-linking): knowledge pills que el summary complementa

## Dependencias

- `generateRetro()` en `internal/session/session.go` (S-017)
- `sessionCardJSON` en `internal/dashboard/api.go` (S-018)
- `openGlobalDB()` migrations en `internal/session/session.go` (S-017)
- Frontend retro indicator en `internal/dashboard/web/static/app.js` (S-018)

## Changelog

| Version | Cambio |
|---------|--------|
| 0.1.0 | Initial draft |
| 0.2.0 | Feedback Codex+Gemini: summary en parent (no child) elimina race conditions; raw_output truncado antes de persistir resuelve contradicción; drop endpoint separado; migration usa PRAGMA check; agregar B-003 (stdout vacío), E-006 (persistence failure), E-007 (multibyte); definir entries_skipped precisamente; especificar rendering inline expandible |
| 0.2.1 | C-003 step 1: aclarar que TrimSpace se aplica al raw output antes de truncar (verificación Codex) |

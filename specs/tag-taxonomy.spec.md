# S-019: KB Tag Taxonomy + Dashboard Tag Visualization

| Field | Value |
|-------|-------|
| **ID** | S-019 |
| **Version** | 0.3.1 |
| **Status** | implemented |
| **Validation Strategy** | tests post-impl + manual browser testing |
| **Related Specs** | S-010 (sdd-mem, tag system), S-016 (dashboard) |
| **Issues** | #12 (tag taxonomy), #13 (dashboard tag visualization) |
| **Owner** | chiche |

---

## Objetivo

Clasificar los tags existentes de la KB en dos categorías (tipos y temas) usando una lista fija conocida, mejorar la visualización del dashboard para mostrarlos separados, ocultar tags internos/ruido, y limpiar entries huérfanas sin tipo.

---

## Contexto

Hoy los tags se guardan como strings planos sin distinción. Los tags que representan "qué es" (`learning`, `gotcha`, `decision`) se mezclan con tags que representan "de qué trata" (`cvm`, `backend`, `infra`) y con tags internos/ruido (`auto-captured`, `s013`, `session-buffer`).

El mecanismo `--type` con prefijo `type:` existe en el código pero nunca se usó en producción. Todos los hooks, retros, y skills escriben tags bare con `--tag`. Esta spec no cambia eso — clasifica en base a una lista conocida.

---

## Alcance

### Incluido

| ID | Item | Descripción |
|----|------|-------------|
| B-001 | Lista de type tags conocidos | Variable exportada con los tags que representan tipo |
| B-002 | `IsInternalTag()` | Función que identifica tags de ruido a ocultar del dashboard |
| B-003 | `ClassifyTag()` | Función que clasifica un tag en `type`, `topic`, o `internal` |
| B-004 | Stats API restructurada | `/api/stats` retorna `by_type` y `by_topic` en vez de `by_tag` flat |
| B-005 | Frontend: tags agrupados | Stats view muestra tipos y temas en secciones separadas |
| B-006 | Frontend: click-to-filter | Click en un tag en Stats navega a Browser tab con ese tag pre-filtrado |
| B-007 | Comando `cvm kb migrate-tags` | Elimina entries sin tag de tipo + limpia tags internos |
| B-008 | Cleanup dead code | Remover código muerto relacionado al mecanismo `type:` prefix no usado |

### Excluido

- Cambios al formato de almacenamiento de tags (siguen siendo `[]string` bare)
- Nuevos tipos (se mantienen los 5 existentes)
- Validación forzada al escribir (no se bloquea un `kb put` sin tipo)
- Cambios a `cvm kb search` (ya funciona con `--tag`)

---

## Contratos

### C-001: Clasificación de tags

```go
// Spec: S-019 | Req: B-001
var TypeTags = []string{"decision", "learning", "gotcha", "discovery", "session"}

// Spec: S-019 | Req: B-002
var InternalTags = []string{"auto-captured", "session-buffer"}

// Spec: S-019 | Req: B-003
func ClassifyTag(tag string) string // returns "type", "topic", or "internal"
```

- **C-001a**: `ClassifyTag("learning")` MUST retornar `"type"`.
- **C-001b**: `ClassifyTag("auto-captured")` MUST retornar `"internal"`.
- **C-001c**: `ClassifyTag("session-buffer")` MUST retornar `"internal"`.
- **C-001d**: `ClassifyTag("cvm")` MUST retornar `"topic"`.
- **C-001e**: `ClassifyTag("s013")` MUST retornar `"internal"` (tags que matchean `^s\d{3}$`).
- **C-001f**: Tags con prefijo `type:` MUST ser clasificados como `"internal"` (son residuo del mecanismo no usado).
- **C-001g**: Tags con prefijo `session-buffer` MUST ser clasificados como `"internal"`.

### C-002: Stats API restructurada

```
GET /api/stats
```

Respuesta (cambio respecto a S-016):

```json
{
  "global": {
    "total": 42,
    "enabled": 40,
    "stale": 5,
    "total_tokens": 12400,
    "by_type": {
      "learning": 15,
      "gotcha": 8,
      "decision": 12
    },
    "by_topic": {
      "cvm": 5,
      "backend": 3,
      "infra": 2
    }
  },
  "local": { ... },
  "active_sessions": 1
}
```

- **C-002a**: `by_type` MUST contener solo tags donde `ClassifyTag() == "type"`.
- **C-002b**: `by_topic` MUST contener solo tags donde `ClassifyTag() == "topic"`.
- **C-002c**: Tags internos (`ClassifyTag() == "internal"`) MUST ser excluidos.
- **C-002d**: El field `by_tag` MUST ser removido (reemplazado por `by_type` y `by_topic`).

### C-003: Frontend Stats View

- **C-003a**: La sección de tags MUST mostrar dos grupos: "Types" y "Topics".
- **C-003b**: Cada grupo MUST estar ordenado por count descendente.
- **C-003c**: Si un grupo no tiene entries, MUST no mostrarse.
- **C-003d**: Los badges de tipo MUST usar los colores existentes de `TAG_COLORS` (`badge-learning`, `badge-gotcha`, `badge-decision`).
- **C-003e**: Los badges de topic MUST usar `badge-default`.

### C-004: Click-to-filter

- **C-004a**: Click en un tag (tipo o topic) en Stats MUST navegar a `#browser` con el tag como filtro.
- **C-004b**: El Browser tab MUST leer el tag de la URL al cargar y pre-filtrar la búsqueda.

### C-005: Migration command

```
cvm kb migrate-tags [--dry-run] [--local]
```

- **C-005a**: MUST eliminar toda entry que no tenga al menos un tag de tipo (donde `ClassifyTag(tag) == "type"`).
- **C-005b**: Entries con key que empieza por `session-buffer-` MUST ser eliminadas (son residuo del sistema viejo S-011/S-015, superseded por S-017).
- **C-005c**: `--dry-run` MUST imprimir los cambios propuestos sin aplicarlos.
- **C-005d**: `--local` MUST operar sobre la KB local en vez de la global.
- **C-005e**: MUST imprimir un resumen: `"Deleted N untyped entries, removed M session-buffer entries"`.
- **C-005f**: Si no hay cambios, MUST imprimir `"No changes needed"`.

### C-006: Cleanup dead code

**Go — `internal/kb/kb.go`:**
- **C-006a**: `PutWithOptions()` (line 337) MUST ser removida. Es dead code — nadie la llama en producción. El CLI reimplementa la misma lógica inline en `cmd/kb.go`.
- **C-006b**: Los tests que usan `PutWithOptions` (`kb_test.go:63 TestPutWithOptions_TypeTag`, `kb_test.go:90 TestPutWithOptions_InvalidType`, y los usos en `kb_e2e_test.go`) MUST ser actualizados para usar `Put()` directo o el path de CLI.

**Go — `cmd/kb.go`:**
- **C-006c**: `--type` en `cvm kb put` (line 50) MUST almacenar el tag bare. Cambiar `tags = append(tags, "type:"+typeTag)` a `tags = append(tags, typeTag)`.

**Go — `internal/kb/sqlite_backend.go`:**
- **C-006d**: `SearchOptions.TypeTag` filter (lines 319-321, 413-415) MUST buscar el tag bare. Cambiar `"type:"+opts.TypeTag` a `opts.TypeTag`.

**Go — `internal/kb/flat.go`:**
- **C-006e**: `TypeTag` filter (lines 155-160) MUST buscar el tag bare. Cambiar `"type:"+opts.TypeTag` a `opts.TypeTag`.

**JS — `internal/dashboard/web/static/app.js`:**
- **C-006f**: `tag.replace(/^type:/, '')` en `badgeClass()` (line 74) MUST ser removido. Nunca hay tags con prefijo `type:` en producción — es un no-op.
- **C-006g**: `TAG_COLORS` entries `summary` y `spec-gap` (lines 69-70) MUST ser removidas. Esos tipos no existen.
- **C-006h**: Los CSS classes `badge-summary` y `badge-spec-gap` en `style.css` MUST ser removidos.

**Go — `internal/dashboard/api.go`:**
- **C-006i**: La exclusión de `session-buffer-` en el listado de sesiones (line 827) MUST ser removida. Las session-buffer entries son residuo del sistema viejo (S-011/S-015) y la migration las elimina. Post-migration no hay entries con ese patrón.

---

## Behaviors

### B-001: Stats API con tags mixtos

**Given** la KB global tiene:
- Entry A con tags `["learning", "cvm"]`
- Entry B con tags `["gotcha", "infra"]`
- Entry C con tags `["learning", "auto-captured", "s017"]`
**When** el browser hace `GET /api/stats`
**Then**:
- `global.by_type` = `{"learning": 2, "gotcha": 1}`
- `global.by_topic` = `{"cvm": 1, "infra": 1}`
- `auto-captured` y `s017` NO aparecen

### B-002: Frontend muestra tags agrupados

**Given** el dashboard muestra `by_type: {learning: 5, gotcha: 3}` y `by_topic: {backend: 2, cvm: 4}`
**When** el usuario navega al tab Stats
**Then**:
- Ve una sección "Types" con badges `learning (5)`, `gotcha (3)`
- Ve una sección "Topics" con badges `cvm (4)`, `backend (2)`

### B-003: Click tag filtra Browser

**Given** Stats muestra `learning (5)` en la sección Types
**When** el usuario clickea el badge `learning`
**Then**:
- El tab activo cambia a Browser
- El filtro de tag se pre-llena con `learning`
- Los resultados muestran solo entries con tag `learning`

### B-004: Migration limpia entries sin tipo

**Given** la KB tiene:
- Entry X con tags `["learning", "cvm"]` (tiene tipo)
- Entry Y con tags `["infra", "auto-captured"]` (sin tipo)
- Entry Z con key `"session-buffer-abc123"` y tags `["session-buffer"]`
- Entry W con tags `["decision", "backend"]` (tiene tipo)
**When** el usuario ejecuta `cvm kb migrate-tags`
**Then**:
- Entry X: sin cambios
- Entry Y: eliminada (no tiene tag de tipo)
- Entry Z: eliminada (session-buffer residuo)
- Entry W: sin cambios
- Output: `"Deleted 1 untyped entries, removed 1 session-buffer entries"`

### B-005: Dry run

**Given** la KB tiene entries para limpiar
**When** el usuario ejecuta `cvm kb migrate-tags --dry-run`
**Then**:
- Imprime las entries que serían eliminadas
- No elimina nada
- Output incluye `"[dry-run]"` prefix

### B-006: --type flag corregido

**Given** el usuario ejecuta `cvm kb put "my-key" --body "content" --type learning --tag "cvm"`
**When** la entry se guarda
**Then**:
- Tags resultantes: `["learning", "cvm"]`
- NO `["type:learning", "cvm"]`

---

## Edge Cases

| ID | Scenario | Expected Behavior |
|----|----------|------------------|
| E-001 | Entry tiene tag `"type:learning"` (residuo viejo) | `ClassifyTag` lo clasifica como `"internal"`. Migration la elimina si no tiene otro tag de tipo bare. |
| E-002 | `cvm kb migrate-tags` en KB vacía | MUST imprimir `"No changes needed"` y exit 0. |
| E-003 | Stats con 0 entries en un scope | `by_type` y `by_topic` MUST ser `{}` (empty map), no `null`. |
| E-004 | Browser tab cargado con tag filter que no existe | MUST mostrar 0 resultados sin error. |
| E-005 | Entry tiene tag `"session"` (es un ValidType) | `ClassifyTag("session")` retorna `"type"`. La entry se mantiene. |

---

## Invariantes

| ID | Invariante |
|----|------------|
| I-001 | Tags internos NEVER aparecen en `by_type` o `by_topic` del stats response. |
| I-002 | `cvm kb search --tag learning` MUST seguir funcionando idéntico (no breaking change). |
| I-003 | El dashboard MUST seguir siendo read-only. La migration es un comando CLI separado. |
| I-004 | Después de migration, toda entry (excepto residuos aún no limpiados en local) MUST tener al menos un tag de tipo. |

---

## Errores

| Situación | Comportamiento |
|-----------|----------------|
| `--type` con tipo inválido (e.g., `--type foo`) | CLI error: `"invalid type \"foo\": must be one of decision, learning, gotcha, discovery, session"` |
| `migrate-tags` falla al eliminar una entry | Loguear warning y continuar con las demás. Reportar failures en el resumen. |

---

## Restricciones No Funcionales

| ID | Restricción |
|----|-------------|
| NF-001 | `GET /api/stats` MUST seguir respondiendo en < 300ms con 2000 entries. |
| NF-002 | `migrate-tags` MUST completar en < 5 segundos para 1000 entries. |

---

## Plan de Implementación

### Wave 1 — Clasificación de tags + cleanup Go (Go)

**Archivos:** `internal/kb/kb.go`, `internal/kb/sqlite_backend.go`, `internal/kb/flat.go`, `cmd/kb.go`, `internal/kb/kb_test.go`, `internal/kb/kb_e2e_test.go`

1. Agregar `TypeTags`, `InternalTags` variables exportadas
2. Agregar `ClassifyTag()` function
3. Remover `PutWithOptions()` (dead code) — C-006a
4. Actualizar tests que usaban `PutWithOptions` — C-006b
5. Fix `--type` en `cvm kb put`: almacenar bare tag — C-006c
6. Fix `TypeTag` filter en sqlite_backend.go: buscar bare — C-006d
7. Fix `TypeTag` filter en flat.go: buscar bare — C-006e
8. Tests unitarios para `ClassifyTag` y para el flag `--type` corregido

**Gate:** `go test ./...` pasa. `cvm kb put --type learning` guarda `"learning"` (no `"type:learning"`).

### Wave 3 — Migration command (Go)

**Archivos:** `cmd/kb.go`

1. Implementar subcommand `cvm kb migrate-tags`
2. Flags: `--dry-run`, `--local`
3. Lógica: eliminar entries sin tag de tipo + session-buffer residuos
4. Tests

**Gate:** `cvm kb migrate-tags --dry-run` lista entries a eliminar.

### Wave 4 — Dashboard API (Go)

**Archivos:** `internal/dashboard/api.go`

1. Cambiar `scopeStatsJSON`: reemplazar `ByTag` con `ByType` y `ByTopic`
2. En `buildScopeStats()`: usar `ClassifyTag()` para clasificar
3. Remover exclusión de `session-buffer-` en listado de sesiones (line 827) — C-006i
4. Tests

**Gate:** `curl /api/stats` retorna respuesta restructurada.

### Wave 5 — Frontend (JS/CSS)

**Archivos:** `internal/dashboard/web/static/app.js`, `internal/dashboard/web/static/style.css`

1. Actualizar Stats rendering para dos secciones (Types, Topics)
2. Implementar click-to-filter: click badge → navegar a `#browser` con tag filter
3. Actualizar Browser tab para leer tag de URL al cargar
4. Remover `tag.replace(/^type:/, '')` de `badgeClass()` — C-006f
5. Remover `summary` y `spec-gap` de `TAG_COLORS` — C-006g
6. Remover `badge-summary` y `badge-spec-gap` de CSS — C-006h

**Gate:** Dashboard muestra tags agrupados, click filtra correctamente.

---

## Specs Relacionadas

- **S-010** (sdd-mem): Define `ValidTypes` — esta spec reutiliza la misma lista como `TypeTags`
- **S-016** (dashboard): Define `/api/stats` — esta spec reemplaza `by_tag` con `by_type` + `by_topic`

---

## Dependencias

- `internal/kb` — Entry struct, Backend interface (no schema change)
- `internal/dashboard` — API handlers, frontend assets
- Sin nuevas dependencias externas

---

## Changelog

| Version | Fecha | Cambio |
|---------|-------|--------|
| 0.1.0 | 2026-04-14 | Draft inicial |
| 0.2.0 | 2026-04-14 | Removida dimensión area. Sin tipos nuevos. Agregado cleanup. |
| 0.3.0 | 2026-04-14 | Reescrita desde cero basada en auditoría del código real. Sin prefijo `type:`. Clasificación por lista conocida. |
| 0.3.1 | 2026-04-14 | Inventario completo de dead code con file:line exacto (C-006a a C-006i). |

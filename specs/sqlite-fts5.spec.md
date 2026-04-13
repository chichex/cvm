# S-013: SQLite + FTS5 Backend for KB

| Field | Value |
|-------|-------|
| **ID** | S-013 |
| **Version** | 0.1.0 |
| **Status** | draft |
| **Validation Strategy** | TDD |
| **Related Specs** | S-010 (sdd-mem), B-015 |
| **Owner** | chiche |

---

## Objetivo

Reemplazar el backend de flat files (`.index.json` + `entries/*.md`) con SQLite + FTS5 para la Knowledge Base de cvm. El nuevo backend MUST proveer búsqueda full-text con ranking, acceso concurrente seguro, y migración automática desde flat files. El backend flat file MUST seguir funcionando como fallback de recuperación ante corrupción.

---

## Alcance

### Incluido
- Abstracción `Backend` interface que unifica flat-file y SQLite
- Schema SQLite: tabla `entries` + tabla virtual FTS5
- Migración automática desde flat files en el primer uso de SQLite
- Config via env var `CVM_KB_BACKEND` (valores: `sqlite`, `flat`; default: `sqlite`)
- FTS5 search con ranking BM25 y stemming porter
- WAL mode para acceso concurrente seguro
- Detección de DB corrupta y fallback a flat files con warning en stderr
- Todas las funciones públicas de `internal/kb/kb.go` sin cambio de firma

### Excluido
- Vector search / embeddings
- Sincronización entre máquinas
- Web UI
- Migración de SQLite → flat (la dirección es one-way)

---

## Contratos

### I-001: Backend Interface

```go
// Spec: S-013 | Req: I-001
type Backend interface {
    Put(key, body string, tags []string, now time.Time) error
    Get(key string) (Document, error)
    List(tag string) ([]Entry, error)
    Remove(key string) error
    Search(query string, opts SearchOptions) ([]SearchResult, error)
    Timeline(days int) ([]TimelineDay, error)
    Stats() (StatsResult, error)
    Compact() ([]CompactEntry, error)
    SetEnabled(key string, enabled bool) error
    LoadDocuments() ([]Document, error)
    SaveDocument(doc Document) error
    Close() error
}
```

**Reglas del contrato:**
- I-001a: La interface MUST ser satisfecha por ambos backends (`FlatBackend` y `SQLiteBackend`)
- I-001b: El campo `now time.Time` en `Put` es inyectable para facilitar tests deterministas
- I-001c: `Close()` MUST ser idempotente (llamar dos veces no debe retornar error)
- I-001d: Ninguna función pública existente en `kb.go` cambia su firma

### I-002: Factory Function

```go
// Spec: S-013 | Req: I-002
func NewBackend(scope config.Scope, projectPath string) (Backend, error)
```

- I-002a: Lee `CVM_KB_BACKEND` env var
- I-002b: Si el valor es `"flat"`, retorna `FlatBackend`
- I-002c: Si el valor es `"sqlite"` o está vacío (default), retorna `SQLiteBackend`
- I-002d: Si `SQLiteBackend` falla al inicializar (DB corrupta, permisos), MUST loguear warning a stderr y retornar `FlatBackend` como fallback
- I-002e: Valor inválido en `CVM_KB_BACKEND` MUST retornar error inmediatamente (sin fallback)

---

## Schema SQLite

### I-003: Tabla `entries`

```sql
-- Spec: S-013 | Req: I-003
CREATE TABLE IF NOT EXISTS entries (
    key         TEXT    PRIMARY KEY,
    body        TEXT    NOT NULL DEFAULT '',
    tags        TEXT    NOT NULL DEFAULT '[]',  -- JSON array
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT    NOT NULL,               -- RFC3339
    updated_at  TEXT    NOT NULL,               -- RFC3339
    last_referenced TEXT                        -- RFC3339, nullable
);
```

### I-004: Tabla virtual FTS5

```sql
-- Spec: S-013 | Req: I-004
CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
    key,
    body,
    tags,
    content='entries',
    content_rowid='rowid',
    tokenize='porter ascii'
);
```

- I-004a: `content='entries'` configura FTS5 como content table para evitar duplicación de datos
- I-004b: `tokenize='porter ascii'` habilita stemming básico (run/running/ran → run)
- I-004c: Los triggers MUST mantener FTS5 sincronizado con `entries`

### I-005: Triggers de sincronización FTS5

```sql
-- Spec: S-013 | Req: I-005
CREATE TRIGGER IF NOT EXISTS entries_ai AFTER INSERT ON entries BEGIN
    INSERT INTO entries_fts(rowid, key, body, tags)
    VALUES (new.rowid, new.key, new.body, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS entries_ad AFTER DELETE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid, key, body, tags)
    VALUES ('delete', old.rowid, old.key, old.body, old.tags);
END;

CREATE TRIGGER IF NOT EXISTS entries_au AFTER UPDATE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid, key, body, tags)
    VALUES ('delete', old.rowid, old.key, old.body, old.tags);
    INSERT INTO entries_fts(rowid, key, body, tags)
    VALUES (new.rowid, new.key, new.body, new.tags);
END;
```

### I-006: WAL Mode y configuración

```sql
-- Spec: S-013 | Req: I-006
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;
PRAGMA busy_timeout=5000;
```

- I-006a: WAL mode MUST activarse al abrir la conexión, antes de cualquier operación
- I-006b: `busy_timeout=5000` (ms) previene errores inmediatos bajo contención
- I-006c: La DB MUST crearse con permisos `0600` (lectura/escritura solo para el owner)

---

## Localización del Archivo SQLite

### I-007: DB Path

- I-007a: Para scope global: `{GlobalKBDir}/kb.db`
- I-007b: Para scope local: `{LocalKBDir}/kb.db`
- I-007c: El directorio padre MUST crearse con `os.MkdirAll` si no existe (permisos `0755`)

---

## Migración

### B-001: Detección de migración

**Given** que `CVM_KB_BACKEND=sqlite` (o vacío)
**And** que el archivo `kb.db` NO existe en el KB dir
**And** que existe `.index.json` con al menos una entrada
**When** `NewBackend` es invocado
**Then** MUST ejecutar la migración automática antes de retornar el backend
**And** MUST imprimir a stderr: `"[cvm] migrating KB to SQLite..."`
**And** MUST imprimir a stderr al finalizar: `"[cvm] migration complete: N entries imported"`

### B-002: Migración con 0 entradas

**Given** que `.index.json` existe pero contiene `{"entries":[]}`
**When** se ejecuta la migración
**Then** MUST crear la DB SQLite vacía sin error
**And** MUST NO imprimir el mensaje de migración (no hay nada que migrar)

### B-003: Migración con entradas existentes

**Given** que `.index.json` contiene 3 entradas (keys: `"foo"`, `"bar"`, `"baz"`)
**And** que los archivos `entries/foo.md`, `entries/bar.md`, `entries/baz.md` existen
**When** se ejecuta la migración
**Then** MUST insertar las 3 entradas en `entries` con todos sus campos (key, body, tags, enabled, created_at, updated_at, last_referenced)
**And** FTS5 MUST indexar las 3 entradas via triggers
**And** los archivos flat file originales MUST permanecer intactos (no se eliminan)

### B-004: Migración con clave que contiene caracteres especiales

**Given** que existe una entrada con key `"session-2026/04/13"` (contiene `/`)
**When** se ejecuta la migración
**Then** MUST insertarse correctamente en SQLite usando la key literal como TEXT
**And** MUST NO usar la key como path de archivo en el backend SQLite

### B-005: Lectura concurrente durante migración

**Given** que la migración está en curso (transacción abierta en WAL mode)
**And** que otro proceso intenta `Get(key)` en la misma DB
**Then** el lector MUST bloquear hasta máximo `busy_timeout` (5000ms)
**And** si supera el timeout MUST retornar error descriptivo

### B-006: DB file no existe, no hay flat files tampoco

**Given** que `kb.db` NO existe
**And** que `.index.json` NO existe
**When** `NewBackend` es invocado
**Then** MUST crear una DB SQLite vacía con el schema correcto
**And** MUST retornar el backend sin error

---

## Comportamiento de Funciones Públicas con SQLite Backend

### B-007: Put — inserción y actualización

**Given** backend SQLite activo
**When** `Put(scope, projectPath, "my-key", "body text", []string{"tag1"})` es invocado
**And** `"my-key"` no existe en la DB
**Then** MUST insertar un registro en `entries` con `enabled=1`, `created_at=now`, `updated_at=now`
**And** FTS5 MUST indexar la entrada via el trigger `entries_ai`

**Given** que `"my-key"` ya existe
**When** `Put` es invocado con el mismo key y body nuevo
**Then** MUST hacer `UPDATE` preservando `created_at` original
**And** MUST actualizar `updated_at=now`
**And** FTS5 MUST re-indexar via los triggers `entries_au`

### B-008: Search — FTS5 con ranking

**Given** backend SQLite activo con entradas indexadas
**When** `Search(scope, projectPath, "golang")` es invocado
**Then** MUST usar FTS5 con la query `"golang"` usando sintaxis `entries_fts MATCH ?`
**And** MUST retornar resultados ordenados por rank BM25 (rank ASC, porque rank es negativo en FTS5)
**And** el campo `Rank` de `SearchResult` MUST mapearse como: rank=0 para key exacta, rank=1 para key parcial, rank=2 para match en body/tags

**Given** la query `"running"` y una entrada con body `"I was running tests"`
**When** `Search` es invocado
**Then** MUST retornar la entrada (stemming: `running` → `run`)

### B-009: SearchWithOptions — filtros sobre FTS5

**Given** `opts.Tag = "learning"` y `opts.Since = 24h`
**When** `SearchWithOptions` es invocado
**Then** MUST aplicar filtro de tag en SQL: `WHERE json_each(tags) = 'learning'`
**And** MUST aplicar filtro de tiempo: `WHERE updated_at >= ?`
**And** MUST combinar filtros via JOIN entre `entries_fts` y `entries`

### B-010: Remove

**Given** backend SQLite activo
**When** `Remove(scope, projectPath, "my-key")` es invocado
**And** la entry existe
**Then** MUST ejecutar `DELETE FROM entries WHERE key = ?`
**And** FTS5 MUST des-indexar via trigger `entries_ad`
**And** MUST retornar nil

**When** la entry NO existe
**Then** MUST retornar `fmt.Errorf("entry %q not found", key)`

### B-011: Show — actualiza LastReferenced

**Given** backend SQLite activo
**When** `Show(scope, projectPath, "my-key")` es invocado
**Then** MUST retornar el body completo (sin frontmatter — la DB almacena body puro)
**And** MUST ejecutar `UPDATE entries SET last_referenced = ? WHERE key = ?` con `time.Now()`

### B-012: Stats y StatsDetailed

**Given** backend SQLite
**When** `StatsDetailed` es invocado
**Then** MUST calcular `total`, `enabled`, `stale` via SQL COUNT queries
**And** token estimation MUST seguir siendo `len(body)/4` (chars/4, I-004 de S-010)
**And** entry es stale si: `last_referenced IS NOT NULL AND last_referenced < now-30d` OR `last_referenced IS NULL AND created_at < now-30d`

### B-013: Timeline

**Given** backend SQLite con N entradas en los últimos `days` días
**When** `Timeline(scope, projectPath, days)` es invocado
**Then** MUST retornar entradas agrupadas por día (formato `"2006-01-02"`) usando `DATE(updated_at)`
**And** días MUST estar ordenados descendente
**And** entradas dentro de cada día MUST estar ordenadas por `updated_at DESC`

### B-014: Compact

**Given** backend SQLite
**When** `Compact(scope, projectPath)` es invocado
**Then** MUST retornar `[]CompactEntry` con `key`, `tags`, `firstLine` (primeros 80 chars del body), `UpdatedAt`
**And** resultado MUST estar ordenado por `updated_at DESC`

### B-015: Clean

**Given** backend SQLite
**When** `Clean(scope, projectPath)` es invocado
**Then** MUST ejecutar `DELETE FROM entries` (que cascade-limpia FTS5 via triggers)
**And** MUST retornar el count de entradas eliminadas

### B-016: SetEnabled

**Given** backend SQLite
**When** `SetEnabled(scope, projectPath, "my-key", false)` es invocado
**Then** MUST ejecutar `UPDATE entries SET enabled = 0, updated_at = ? WHERE key = ?`
**And** si key no existe MUST retornar `fmt.Errorf("entry %q not found", key)`

### B-017: LoadDocuments y SaveDocument

**Given** backend SQLite
**When** `LoadDocuments` es invocado
**Then** MUST retornar todos los `Document` (Entry + body) desde la DB via `SELECT * FROM entries`

**When** `SaveDocument(doc)` es invocado
**Then** MUST hacer upsert: si key existe → UPDATE, si no → INSERT
**And** MUST mantener `created_at` original en UPDATE

### B-018: PutWithDedup — dedup con hash en SQLite

**Given** backend SQLite
**When** `PutWithDedup` es invocado con body ya existente (mismo SHA256) y mismo key
**Then** MUST retornar `skipped=true` sin escribir a la DB (comportamiento idéntico a flat)

**When** mismo hash pero diferente key
**Then** MUST imprimir warning a stderr y proceder con el insert

---

## Edge Cases y Manejo de Errores

### E-001: DB corrupta detectada en apertura

**Given** que `kb.db` existe pero su contenido está corrupto (magic bytes inválidos, o `PRAGMA integrity_check` falla)
**When** `NewBackend` intenta abrir la DB
**Then** MUST loguear a stderr: `"[cvm] warning: SQLite DB corrupt, falling back to flat files"`
**And** MUST retornar `FlatBackend` (no error fatal)
**And** el archivo `kb.db` corrupto MUST ser renombrado a `kb.db.corrupt.<timestamp>` para diagnóstico

### E-002: Permisos insuficientes en el archivo DB

**Given** que `kb.db` existe con permisos `0000` (no readable/writable)
**When** `NewBackend` intenta abrir la DB
**Then** MUST loguear a stderr: `"[cvm] warning: cannot open SQLite DB (permission denied), falling back to flat files"`
**And** MUST retornar `FlatBackend`

### E-003: Disco lleno durante escritura

**Given** backend SQLite activo
**When** un `Put` o `SaveDocument` falla con `SQLITE_FULL` (disk full)
**Then** MUST retornar error inmediatamente: `"kb: disk full"`
**And** MUST NO dejar la transacción abierta (rollback implícito de SQLite)
**And** MUST NO intentar fallback a flat files (el error es del sistema, no de la DB)

### E-004: Búsqueda con query vacía

**Given** backend SQLite
**When** `Search(scope, projectPath, "")` es invocado
**Then** MUST retornar todos los entries (equivalente a `List("")`), ordenados por `updated_at DESC`
**And** MUST NO ejecutar FTS5 MATCH con query vacía (causa panic en SQLite)

### E-005: Key con caracteres especiales en FTS5

**Given** key `"session-2026/04-insights"` (contiene `/` y `-`)
**When** `Search` es invocado con query `"session-2026"`
**Then** FTS5 MUST tratar el key como texto opaco (no parsearlo como ruta)
**And** MUST retornar la entry con snippet del key

### E-006: Concurrencia — múltiples escrituras simultáneas

**Given** WAL mode activo
**When** dos goroutines ejecutan `Put` simultáneamente sobre distintas keys
**Then** MUST serializar via `busy_timeout` sin retornar error
**And** ambas escrituras MUST completarse exitosamente

### E-007: Migración con entry cuyo archivo .md no existe

**Given** que `.index.json` referencia la key `"missing-entry"`
**And** que `entries/missing-entry.md` NO existe
**When** se ejecuta la migración
**Then** MUST migrar la entry con body vacío (`""`) y loguear warning: `"[cvm] migration warning: body not found for key missing-entry, importing with empty body"`
**And** MUST continuar con el resto de entradas (no abortar)

---

## Invariantes

| ID | Invariante |
|----|-----------|
| I-008 | FTS5 MUST estar sincronizado con `entries` en todo momento (garantizado por triggers) |
| I-009 | `created_at` de una entry MUST ser inmutable después del primer insert |
| I-010 | Una key MUST ser única dentro de un scope (PRIMARY KEY constraint) |
| I-011 | El backend activo MUST ser transparente para todas las funciones públicas de `kb.go` |
| I-012 | La migración MUST ser idempotente: si `kb.db` ya existe, no re-migrar |
| I-013 | Los archivos flat file originales MUST preservarse post-migración |

---

## Restricciones No Funcionales

| ID | Restricción |
|----|------------|
| NF-001 | MUST usar `modernc.org/sqlite` (pure Go, no CGO) |
| NF-002 | `go build` MUST completar sin CGO (`CGO_ENABLED=0`) |
| NF-003 | Latencia de `Put` en SQLite MUST ser ≤ 50ms bajo carga normal |
| NF-004 | Latencia de `Search` para 10K entries MUST ser ≤ 100ms |
| NF-005 | La abstracción Backend MUST NO romper la API pública de `kb.go` (compatibilidad backward completa) |
| NF-006 | WAL mode MUST estar activo; journal mode DELETE está prohibido |
| NF-007 | La DB MUST crearse con permisos `0600` |

---

## Behaviors: Configuración

### B-019: Variable de entorno `CVM_KB_BACKEND`

**Given** `CVM_KB_BACKEND=flat`
**When** `NewBackend` es invocado
**Then** MUST retornar `FlatBackend` sin intentar abrir ningún archivo `.db`

**Given** `CVM_KB_BACKEND=sqlite` (o vacío)
**When** `NewBackend` es invocado
**Then** MUST intentar abrir/crear `kb.db` y retornar `SQLiteBackend`

**Given** `CVM_KB_BACKEND=postgres`
**When** `NewBackend` es invocado
**Then** MUST retornar `error: unknown backend "postgres"` (sin fallback)

---

## Plan de Implementación

### Wave 1: Abstracción Backend + FlatBackend wrapper (no-op refactor)
- Definir `Backend` interface en `internal/kb/backend.go`
- Implementar `FlatBackend` wrapeando la lógica existente de `kb.go`
- Implementar `NewBackend` con dispatch por `CVM_KB_BACKEND`
- Refactorizar todas las funciones públicas de `kb.go` para usar `NewBackend` internamente
- **Gate**: todos los tests existentes en `kb_test.go` y `kb_e2e_test.go` MUST pasar sin modificación

### Wave 2: Schema SQLite + operaciones CRUD
- Añadir dependencia `modernc.org/sqlite` a `go.mod`
- Implementar `SQLiteBackend` en `internal/kb/sqlite_backend.go`
- Schema: tabla `entries` + tabla virtual FTS5 + triggers
- WAL mode + `busy_timeout`
- Implementar: `Put`, `Get`, `Remove`, `Show`, `List`, `SetEnabled`, `LoadDocuments`, `SaveDocument`
- **Gate**: nuevos unit tests para `SQLiteBackend` (TDD: tests primero)

### Wave 3: FTS5 Search
- Implementar `Search` y `SearchWithOptions` sobre FTS5
- Ranking BM25, stemming porter
- Filtros por tag y since via JOIN SQL
- Manejo de query vacía (E-004)
- **Gate**: tests de búsqueda con stemming y ranking

### Wave 4: Migración automática
- Implementar `migrateFromFlat(src Backend, dbPath string)` en `internal/kb/migration.go`
- Detección: `kb.db` no existe + `.index.json` existe
- Transacción atómica: importar todas las entries o rollback
- Manejo de entries con archivos faltantes (E-007)
- **Gate**: tests de migración incluyendo edge cases B-002, B-003, B-004, B-007

### Wave 5: Manejo de errores y fallback
- Detección de DB corrupta (E-001): `PRAGMA integrity_check` al abrir
- Fallback a FlatBackend con warning
- Renombrado de DB corrupta a `.corrupt.<timestamp>`
- Manejo de permisos (E-002)
- **Gate**: tests de error recovery

---

## Tests a Generar (TDD)

Los siguientes tests MUST ser escritos antes de la implementación de cada wave:

```
// Wave 1
TestNewBackend_DefaultIsSQLite
TestNewBackend_EnvVarFlat
TestNewBackend_EnvVarInvalid

// Wave 2
TestSQLiteBackend_Put_Insert
TestSQLiteBackend_Put_Update_PreservesCreatedAt
TestSQLiteBackend_Get_NotFound
TestSQLiteBackend_Remove_NotFound
TestSQLiteBackend_SetEnabled
TestSQLiteBackend_Close_Idempotent

// Wave 3
TestSQLiteBackend_Search_FTS5_Basic
TestSQLiteBackend_Search_Stemming
TestSQLiteBackend_Search_EmptyQuery_ReturnsAll
TestSQLiteBackend_SearchWithOptions_FilterByTag
TestSQLiteBackend_SearchWithOptions_FilterBySince
TestSQLiteBackend_Search_RankingBM25

// Wave 4
TestMigration_ZeroEntries
TestMigration_MultipleEntries
TestMigration_SpecialCharsInKey
TestMigration_MissingBodyFile
TestMigration_Idempotent

// Wave 5
TestNewBackend_CorruptDB_FallbackToFlat
TestNewBackend_PermissionDenied_FallbackToFlat
TestSQLiteBackend_Put_DiskFull
```

---

## Changelog

| Version | Fecha | Cambio |
|---------|-------|--------|
| 0.1.0 | 2026-04-13 | Spec inicial — expande B-015 de S-010 |

---

## Specs Relacionadas

- **S-010** (sdd-mem): Define el contrato original de KB y todos los tipos públicos
- **B-015**: Item del roadmap Phase 3 que origina esta spec

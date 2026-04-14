# S-018 — Nested Retro Sessions

- **ID**: S-018
- **Version**: 0.1.0
- **Status**: draft
- **Parent**: S-017 (session-system)
- **Validation**: Tests post-impl + manual browser testing

## Objetivo

Cuando `generateRetro()` invoca `claude -p`, esa invocación dispara el hook `SessionStart`
y crea una sesión "hija" que aparece como card independiente en el dashboard. El usuario quiere:

1. Trazabilidad: saber que esa sesión hija existe y está ligada a la sesión padre
2. Anidamiento: la sesión hija NO MUST aparecer como card top-level en el dashboard
3. Visibilidad: dentro de la card del padre, mostrar la sesión de retro y los learnings que produjo

## Alcance

- Schema: agregar `parent_session_id` a tabla `sessions`
- Backend: propagar parent_session_id desde `generateRetro()` → env var → hook → CLI → `Start()`
- API: filtrar sesiones hijas del listado top-level; incluirlas como metadata del padre
- Frontend: renderizar sesión de retro + learnings anidados dentro de la card del padre

## Contratos

### C-001 — Schema migration

```sql
-- Idempotent migration in sessionsSchema and openGlobalDB
ALTER TABLE sessions ADD COLUMN parent_session_id TEXT;
CREATE INDEX IF NOT EXISTS idx_sessions_parent ON sessions(parent_session_id);
```

### C-002 — CLI flag

```
cvm session start --session-id <uuid> --project <path> --profile <name> --parent-session-id <uuid>
```

New optional flag `--parent-session-id`. When provided, `Start()` MUST persist it in the `parent_session_id` column.

### C-003 — Start() signature

```go
func Start(sessionID, project, profileName, parentSessionID string) error
```

New parameter `parentSessionID`. Empty string means no parent (top-level session).

### C-004 — Environment variable propagation

`generateRetro()` MUST set `CVM_PARENT_SESSION_ID=<sessionID>` in the env of the `claude -p` subprocess.

### C-005 — Hook propagation

`session-start.sh` MUST read `$CVM_PARENT_SESSION_ID` and pass `--parent-session-id` to `cvm session start` if set.

### C-006 — API response

```typescript
interface SessionCard {
  // ... existing fields ...
  retro_session?: {
    id: string;
    status: string;      // "active" | "ended"
    started_at: string;
    ended_at?: string;
  };
  knowledge: KnowledgeEntry[];  // populated, not empty slice
}
```

`GET /api/sessions` MUST:
- Exclude sessions where `parent_session_id IS NOT NULL` from the top-level array
- For each top-level session, query child sessions and include as `retro_session`
- Populate `knowledge` with actual KB entries linked via `session_id`

### C-007 — Frontend rendering

Within the parent session card, after the summary preview section:
- If `retro_session` exists: show a retro indicator with status
- If `knowledge` has entries: show expandable knowledge pills (existing infrastructure)

## Behaviors

### B-001 — Retro creates child session with parent link

**Given** a session `abc-123` is being ended via `cvm session end abc-123`
**When** `generateRetro()` invokes `claude -p --model haiku`
**Then** the subprocess env MUST contain `CVM_PARENT_SESSION_ID=abc-123`
**And** the `SessionStart` hook MUST create the child session with `parent_session_id = abc-123`

### B-002 — Dashboard hides child sessions from top-level

**Given** session `abc-123` (parent) and session `def-456` (child, parent_session_id = abc-123) exist
**When** `GET /api/sessions` is called
**Then** the response MUST contain `abc-123` as a top-level card
**And** the response MUST NOT contain `def-456` as a top-level card
**And** `abc-123`'s `retro_session` field MUST contain `{id: "def-456", status: "ended", ...}`

### B-003 — Knowledge entries populated in card

**Given** session `abc-123` has 2 KB entries linked via `session_id`
**When** `GET /api/sessions` is called
**Then** `abc-123`'s `knowledge` array MUST contain 2 entries with `key`, `tags`, `body`

### B-004 — Frontend shows retro indicator

**Given** a session card for `abc-123` has `retro_session` data
**When** the card is rendered
**Then** a retro indicator MUST be visible showing the retro session status
**And** if `retro_session.status` is `"active"`: show "Summarizing..."
**And** if `retro_session.status` is `"ended"`: show "Retro complete"

### B-005 — Frontend shows learnings

**Given** a session card for `abc-123` has `knowledge` entries
**When** the card is rendered
**Then** the knowledge section MUST be visible with expandable pills
**And** each pill MUST show `key`, `tags`, and first line of `body`

## Edge Cases

### E-001 — Session with no retro

**Given** a session ended with `CVM_SESSION_RETRO_ENABLED=false` or < 3 events
**When** the card is rendered
**Then** `retro_session` MUST be null/omitted
**And** `knowledge` MUST be empty array

### E-002 — Retro session still active (in progress)

**Given** a retro `claude -p` is still running (child session status = active)
**When** the card is rendered
**Then** `retro_session.status` MUST be `"active"`
**And** frontend MUST show "Summarizing..." indicator

### E-003 — Multiple child sessions for same parent

**Given** a session was ended, retro ran, then the session was resumed and ended again
**When** `GET /api/sessions` is called
**Then** `retro_session` MUST reflect the most recent child session (ORDER BY started_at DESC LIMIT 1)

### E-004 — Migration on existing database

**Given** an existing `sessions` table without `parent_session_id` column
**When** `openGlobalDB()` is called
**Then** the migration MUST add the column idempotently (ALTER TABLE IF NOT EXISTS pattern)
**And** existing sessions MUST have `parent_session_id = NULL` (top-level by default)

### E-005 — Parent session deleted by GC

**Given** a child session `def-456` with `parent_session_id = abc-123`
**And** `abc-123` has been deleted by `cvm session gc`
**When** `GET /api/sessions` is called
**Then** `def-456` MUST NOT appear as top-level card (still filtered by parent_session_id IS NOT NULL)

## Invariantes

### I-001
Top-level sessions (`parent_session_id IS NULL`) MUST be the only sessions visible as cards in the dashboard.

### I-002
A child session MUST always have a non-empty `parent_session_id`.

### I-003
The `knowledge` field MUST be populated from the entries table via `session_id` foreign key, never hardcoded empty.

## Errores

| Condition | Behavior |
|-----------|----------|
| `--parent-session-id` references non-existent session | Store anyway (no FK constraint) |
| `CVM_PARENT_SESSION_ID` env var empty or missing | Start() creates top-level session (backward compat) |
| Migration fails (ALTER TABLE) | Log warning, continue without parent tracking |

## Restricciones no funcionales

- Migration MUST be idempotent (safe to run multiple times)
- API query MUST NOT introduce N+1 — batch child sessions and knowledge entries
- Frontend MUST handle missing `retro_session` and empty `knowledge` gracefully

## Specs relacionadas

- S-017 (session-system): parent spec, schema and lifecycle
- S-016 (dashboard): frontend rendering

## Dependencias

- SQLite `sessions` table (S-017)
- SQLite `entries` table with `session_id` column (S-017)
- `claude -p` CLI invocation in `generateRetro()` (S-017)
- `session-start.sh` hook (S-017)

## Changelog

| Version | Cambio |
|---------|--------|
| 0.1.0 | Initial draft |

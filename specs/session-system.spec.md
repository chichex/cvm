# S-017: CVM Session System

- **ID**: S-017
- **Version**: 0.6.0
- **Status**: implemented
- **Supersedes**: S-011 (realtime-capture), S-015 (tool-observation)
- **Modifies**: S-016 (dashboard — reads from sessions table instead of JSONL+PID inference), S-010 (sdd-mem — removes /learn, /decide, /gotcha skills)
- **Validation Strategy**: TDD (session CRUD, SQLite schema, retro integration) + manual (hooks, dashboard)

## Objective

Redesign the session system to use a SQLite `sessions` table as the single source of truth for session state, eliminating PID-based inference. Link all KB entries produced during a session to their originating session_id. Replace the session-end summary with a final retrospective pass that captures missed learnings. Consolidate /learn, /decide, /gotcha into /retro as the sole knowledge-capture mechanism.

## Scope

### In scope
- New `sessions` table in the existing KB SQLite database (`~/.cvm/global/kb.db`)
- Add `session_id` column to KB `entries` table
- Remove PID checking (`sessionIsPIDAlive`, `cleanOrphans`) entirely
- Session state derived from SQLite `status` column, not from JSONL+PID inference
- JSONL files remain as append-only event log (unchanged hot path)
- Session end runs `/retro` instead of generating a summary via `claude -p`
- Remove `/learn`, `/decide`, `/gotcha` skills (consolidated into `/retro`)
- Learning pulse (self-check) triggers `/retro` instead of individual skills
- Dashboard reads session list from SQLite `sessions` table
- Stale session handling deferred (out of scope for this version)

### Out of scope
- Stale/orphan session recovery (deferred — address separately)
- Migration of existing JSONL-only sessions
- Response capture
- Session search/filter CLI

## Contracts

### C-001: Sessions Table Schema

```sql
CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,   -- UUID from Claude Code
    status      TEXT NOT NULL DEFAULT 'active',  -- 'active' | 'ended'
    project     TEXT NOT NULL,      -- absolute path to project dir
    profile     TEXT NOT NULL DEFAULT '',
    started_at  TEXT NOT NULL,      -- RFC3339
    ended_at    TEXT,               -- RFC3339, NULL while active
    jsonl_path  TEXT NOT NULL,      -- path to ~/.cvm/sessions/<uuid>.jsonl
    event_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
```

Status values: `active`, `ended`. No `orphan`, no `stale` — those are deferred.

### C-002: KB Entries Table Migration

Add `session_id` column to existing `entries` table. No foreign key constraint — validation is done in application code to allow graceful degradation (B-015).

```sql
ALTER TABLE entries ADD COLUMN session_id TEXT;
CREATE INDEX IF NOT EXISTS idx_entries_session_id ON entries(session_id);
```

- `session_id` is nullable (existing entries and entries created outside a session have NULL)
- No FK constraint: allows `cvm kb put --session-id <uuid>` to succeed even if the session doesn't exist in the table (e.g., legacy sessions, race conditions). Validation is advisory.
- FTS5 triggers are NOT modified — `session_id` is not full-text indexed

### C-002a: Migration Mechanism

The migration MUST run on database open, in this order:

1. `CREATE TABLE IF NOT EXISTS sessions (...)` — safe if table already exists
2. Check if `session_id` column exists in `entries`: `SELECT 1 FROM pragma_table_info('entries') WHERE name='session_id'`
3. If missing: `ALTER TABLE entries ADD COLUMN session_id TEXT` + create index

This runs inside `NewSQLiteBackend()` alongside the existing schema initialization. No versioned migration system required — the check is idempotent.

### C-003: Session Event (JSONL line) — unchanged

All events share these base fields. JSONL format unchanged from v0.4.0.

```go
type SessionEvent struct {
    Type      string `json:"type"`                // "start" | "prompt" | "tool" | "agent" | "end"
    Timestamp string `json:"ts"`                  // RFC3339
    Content   string `json:"content,omitempty"`    // truncated payload
    Tool      string `json:"tool,omitempty"`       // tool name (only for type=tool)
    AgentType string `json:"agent_type,omitempty"` // agent type (only for type=agent)
}
```

### C-004: Session Start Event (first line of .jsonl) — PID removed

```go
type SessionStartEvent struct {
    Type      string            `json:"type"`        // always "start"
    Timestamp string            `json:"ts"`          // RFC3339
    SessionID string            `json:"session_id"`  // UUID from Claude Code
    Project   string            `json:"project"`     // absolute path to project dir
    Profile   string            `json:"profile"`     // active cvm profile name
    Tools     map[string]bool   `json:"tools"`       // detected tools
}
```

`PID` field removed — no longer used for state inference.

### C-005: Session End Event (last line of .jsonl) — summary_key removed

```go
type SessionEndEvent struct {
    Type      string `json:"type"`        // always "end"
    Timestamp string `json:"ts"`          // RFC3339
    Reason    string `json:"reason"`      // "normal" | "error"
}
```

`SummaryKey` removed — session end no longer generates a summary. The final /retro produces KB entries linked via `session_id` instead.

### C-006: Truncation Limits — unchanged

| Event type | Max content length | Behavior when exceeded |
|------------|-------------------|----------------------|
| prompt     | 300 runes         | Truncate, append "…" |
| tool       | 200 runes         | Truncate, append "…" |
| agent      | 300 runes         | Truncate, append "…" |

### C-007: CLI Interface — updated

```
cvm session start    [--session-id <uuid>] [--project <path>] [--profile <name>]
cvm session append   <uuid> --type <prompt|tool|agent> [--content <string>] [--tool <name>] [--agent-type <type>]
cvm session end      <uuid>
cvm session status
cvm session ls       [--limit <n>]   # default limit: 20
cvm session show     <uuid>
cvm session gc       [--older-than <duration>]  # default: 30d
```

`--pid` flag removed from `cvm session start`.

### C-008: Dashboard API — simplified

`GET /api/sessions` reads from SQLite `sessions` table:

```sql
SELECT s.*, COUNT(e.key) as kb_entries
FROM sessions s
LEFT JOIN entries e ON e.session_id = s.id
GROUP BY s.id
ORDER BY s.started_at DESC;
```

Response includes KB entries linked to each session.

`GET /api/stats` field `active_sessions`: `SELECT COUNT(*) FROM sessions WHERE status = 'active'`.

No PID checking, no JSONL parsing for state, no process name validation.

### C-009: Retro — Two Execution Contexts

`/retro` operates in two distinct contexts:

**A. Mid-session retro** (inside Claude conversation):
- Triggered by learning pulse (B-014) or manually by the user
- Runs as a Claude Code skill within the active conversation
- Has access to conversation context
- Persists entries via `cvm kb put --session-id <uuid>`
- Tags entries with their type: `learning`, `decision`, `gotcha`

**B. End-session retro** (Go CLI via `claude -p`):
- Triggered by `cvm session end` (B-005)
- Runs as a standalone `claude -p --model haiku` invocation (same pattern as the old `generateSummary()`)
- Input: JSONL events + list of KB entries already linked to this session_id
- Output: JSON array of missing insights, each with key, body, tags
- `cvm session end` parses the output and calls `kb.Put()` for each entry with `session_id` set
- Cheap and fast (haiku)

### C-009a: End-Session Retro Prompt Template

```
Analyze this coding session's events and identify learnings, decisions, or gotchas
that were NOT already captured in the existing KB entries listed below.

Output ONLY a JSON array. Each element: {"key": "...", "body": "...", "tags": ["learning"|"decision"|"gotcha"]}
If nothing new to capture, output: []

<events>
{events_text}
</events>

<already_captured>
{existing_kb_entries_for_this_session}
</already_captured>
```

Where `{events_text}` is the JSONL content (compacted to last 1000 events if needed) and `{existing_kb_entries_for_this_session}` is the result of `SELECT key, body, tags FROM entries WHERE session_id = ?`.

### C-010: KB Put Session Linking

`cvm kb put` gains a new optional flag:

```
cvm kb put <key> --body "..." --tag "a,b" [--session-id <uuid>]
```

When `--session-id` is provided, the entry's `session_id` column is set. This links the entry to the originating session.

**Backend interface change**: The `Backend.Put()` signature gains an optional `sessionID string` parameter. `FlatBackend` MUST accept the parameter but silently ignore it (flat files have no session_id column). The public `kb.Put()` function gains a corresponding parameter.

```go
// Before:
Put(key, body string, tags []string, now time.Time) error
// After:
Put(key, body string, tags []string, now time.Time, sessionID string) error
```

Empty string `""` means no session link. Callers that don't care about session linking pass `""`.

## Behaviors

### B-001: Session Start
- **Given** a Claude Code session starts with session_id `778a7b24-509f-4f79-a99e-cd01e631ef82` in project `/Users/me/workspace/cvm` with profile `sdd-mem`
- **When** the SessionStart hook runs `cvm session start --session-id 778a7b24-... --project /Users/me/workspace/cvm --profile sdd-mem`
- **Then** a file `~/.cvm/sessions/778a7b24-....jsonl` MUST be created with a start event (C-004)
- **And** a row MUST be inserted into the `sessions` table with `status='active'`, `started_at=now`, `ended_at=NULL`
- **And** the command MUST print the session UUID to stdout
- **And** the command MUST NOT run any orphan cleanup or PID checking

### B-002: Append Prompt Event — unchanged
- **Given** an active session `778a7b24`
- **When** `cvm session append 778a7b24-... --type prompt --content "..."`
- **Then** a JSON line MUST be appended to the JSONL file
- **And** the `event_count` in the sessions table MUST be incremented

### B-003: Append Tool Event — unchanged
- **Given** an active session `778a7b24`
- **When** `cvm session append 778a7b24-... --type tool --tool Bash --content "ls -la"`
- **Then** a JSON line with `type: "tool"` MUST be appended
- **And** `event_count` MUST be incremented

### B-004: Append Agent Event — unchanged
- **Given** an active session `778a7b24`
- **When** `cvm session append 778a7b24-... --type agent --agent-type haiku --content "Research complete..."`
- **Then** a JSON line with `type: "agent"` MUST be appended
- **And** `event_count` MUST be incremented

### B-005: Session End with Final Retro
- **Given** an active session `778a7b24` with 50 events
- **When** `cvm session end 778a7b24-...` runs
- **Then** the session file MUST be read (compacted to last 1000 events if needed)
- **And** existing KB entries for this session MUST be queried: `SELECT key, body, tags FROM entries WHERE session_id = ?`
- **And** `claude -p --model haiku` MUST be invoked with the retro prompt (C-009a), passing events + existing entries
- **And** the JSON array output MUST be parsed; for each element, `kb.Put()` MUST be called with `session_id` set
- **And** a final `{"type":"end","ts":"...","reason":"normal"}` line MUST be appended to JSONL
- **And** the sessions table MUST be updated: `status='ended'`, `ended_at=now`
- **And** `~/.cvm/learning-pulse` MUST be deleted if it exists
- **And** automation integration MUST be preserved
- **Note**: The retro prompt is sent to haiku for cost efficiency. The model is configurable via `CVM_SESSION_RETRO_MODEL` (default: `haiku`)

### B-006: Session End with Retro Disabled
- **Given** `CVM_SESSION_RETRO_ENABLED=false`
- **When** `cvm session end <uuid>` runs
- **Then** it MUST skip the retro pass entirely
- **And** it MUST append an "end" event with `reason: "normal"`
- **And** it MUST update the sessions table: `status='ended'`, `ended_at=now`
- **And** it MUST still perform automation integration and learning-pulse cleanup

### B-007: Session Status — reads from SQLite
- **Given** 2 sessions in the table: one `active`, one `ended`
- **When** `cvm session status` runs
- **Then** it MUST query `SELECT * FROM sessions WHERE status = 'active'`
- **And** it MUST show: session_id, project, profile, started_at, event_count

### B-008: Session List — reads from SQLite
- **Given** 5 sessions in the table
- **When** `cvm session ls` runs
- **Then** it MUST query `SELECT * FROM sessions ORDER BY started_at DESC LIMIT 20`
- **And** each MUST show: session_id (8 chars), status, project, started_at, event_count

### B-009: Session Show — unchanged
- **Given** a session `778a7b24` with 20 events
- **When** `cvm session show 778a7b24-...` runs
- **Then** it MUST print all JSONL events formatted as human-readable lines
- **And** it MUST also list KB entries linked to this session_id

### B-010: Session GC — updated
- **Given** 10 ended sessions, 3 with `ended_at` older than 30 days
- **When** `cvm session gc` runs
- **Then** for each session to delete: first `UPDATE entries SET session_id = NULL WHERE session_id = ?`, then `DELETE FROM sessions WHERE id = ?`, then remove JSONL file
- **And** age is measured by `ended_at` from SQLite (not file mtime)
- **And** it MUST NOT delete active sessions regardless of age

### B-011: Dashboard Sessions — reads from SQLite
- **Given** 1 active session and 3 ended sessions in the table
- **When** `GET /api/sessions` is called
- **Then** the response MUST come from the sessions table (C-008 query)
- **And** active sessions MUST have `status: "active"`
- **And** ended sessions MUST have `status: "ended"`
- **And** each session MUST include the count of linked KB entries

### B-012: Dashboard Stats — reads from SQLite
- **Given** 2 active sessions in the table
- **When** `GET /api/stats` is called
- **Then** `active_sessions` MUST be `2` (simple COUNT query)

### B-013: SSE Session Updates — unchanged
- **Given** the dashboard watcher is running and polling `~/.cvm/sessions/` at 2s intervals
- **When** a session JSONL file is modified (mtime changes)
- **Then** an SSE event `session_updated` MUST be emitted
- **Note**: The watcher polls JSONL file mtimes only, not SQLite. This is sufficient because every session state change (start, append, end) also modifies the JSONL file

### B-014: Learning Pulse Triggers Retro
- **Given** the learning pulse fires (15+ min since last check)
- **When** the `learning-decorator.sh` hook injects the self-check
- **Then** the injected protocol MUST instruct Claude to run `/retro` with scope `mid-session`
- **And** `/retro` MUST persist entries with the current `session_id`
- **And** `/retro` MUST NOT reference /learn, /decide, or /gotcha (those skills no longer exist)

### B-015: KB Put with Session Linking
- **Given** an active session `778a7b24`
- **When** `cvm kb put my-key --body "..." --tag "learning" --session-id 778a7b24-...`
- **Then** the KB entry MUST be created with `session_id = '778a7b24-...'` in the entries table
- **And** if the session_id doesn't exist in the sessions table, the command MUST warn but still create the entry (graceful degradation)

### B-016: SubagentStop Captures with Session Linking
- **Given** an active session `778a7b24` and a subagent that outputs `## Key Learnings:`
- **When** `subagent-stop.sh` extracts and persists learnings
- **Then** each `cvm kb put` call MUST include `--session-id 778a7b24-...`
- **And** the entries MUST be queryable by session_id

## Edge Cases

### E-001: Session File Does Not Exist on Append — unchanged
- **Given** `cvm session append <uuid>` is called but JSONL file does not exist
- **Then** it MUST exit with code 0 (no-op) and log warning to stderr

### E-002: Session Already Ended — unchanged
- **Given** session `778a7b24` has status `ended` in SQLite
- **When** `cvm session append 778a7b24-...` is called
- **Then** it MUST exit with code 0 (no-op) and log warning

### E-003: Session End on Empty/Short Session
- **Given** a session with < 3 events
- **When** `cvm session end` runs
- **Then** it MUST skip the retro pass (nothing meaningful to analyze)
- **And** it MUST update SQLite: `status='ended'`, `ended_at=now`
- **And** it MUST append end event to JSONL

### E-004: Session End with Retro Failure
- **Given** the retro pass fails (LLM error, timeout, etc.)
- **When** `cvm session end` runs
- **Then** it MUST still append end event with `reason: "error"`
- **And** it MUST still update SQLite: `status='ended'`
- **And** it MUST log the error to stderr and exit with code 0

### E-005: Empty session_id — unchanged
- **Given** `cvm session append ""` is called
- **Then** it MUST exit with code 1 with `error: session_id is required`

### E-006: Concurrent Appends — unchanged
- **Given** multiple hooks fire simultaneously
- **Then** JSONL writes use `syscall.Flock` (LOCK_EX); SQLite handles its own concurrency

### E-007: UUID Prefix Matching — unchanged
- **Given** `cvm session show 778a7b24` with unique match
- **Then** resolve to full UUID from SQLite

### E-008: Concurrent Session End — unchanged
- **Given** `cvm session end` called while another is in progress
- **Then** second call detects `status='ended'` in SQLite and exits with code 0

### E-009: Dashboard Backward Compatibility
- **Given** existing `session-summary-*` entries in global KB from pre-v0.5.0 sessions
- **When** `GET /api/sessions` is called
- **Then** the dashboard MUST show both: sessions from SQLite table AND legacy summaries from KB (key pattern match)
- **And** legacy summaries MUST be displayed with `status: "legacy"` to distinguish them

### E-010: SQLite Migration on Existing Install
- **Given** an existing CVM install with KB database but no `sessions` table
- **When** any `cvm session` command runs
- **Then** the migration MUST create the `sessions` table and add `session_id` column to `entries`
- **And** existing KB entries MUST be unaffected (`session_id = NULL`)

## Invariants

### I-001: Append Latency — unchanged
- `cvm session append` MUST target < 50ms completion

### I-002: Storage Location — unchanged
- Session files in `~/.cvm/sessions/`, session state in `~/.cvm/global/kb.db`

### I-003: File Format — unchanged
- Each fully-written JSONL line MUST be valid JSON
- First line MUST be start event, last line of closed session MUST be end event

### I-004: Cross-Project Visibility — simplified
- Dashboard reads from SQLite `sessions` table which is global
- No dependency on local KB or working directory

### I-005: No PID Checking
- The session system MUST NOT check process IDs, process names, or use `syscall.Kill(pid, 0)` for any purpose
- Session state is determined exclusively by the `status` column in SQLite

### I-006: No Local KB Dependency — unchanged
- Session system MUST NOT use local KB for session data

### I-007: File Locking — unchanged for JSONL
- JSONL writes use `syscall.Flock` with `LOCK_EX`
- SQLite handles its own concurrency via WAL mode

### I-008: Graceful Degradation — unchanged
- If `~/.cvm/sessions/` does not exist, create it
- If session file is corrupted, skip invalid lines and warn

### I-009: Retro Config
- `CVM_SESSION_RETRO_ENABLED=true|false` controls whether session end runs retro (default: `true`)
- `CVM_SESSION_RETRO_MODEL` controls which model to use for end-session retro (default: `haiku`)
- Replaces `CVM_AUTOSUMMARY_ENABLED` and `CVM_AUTOSUMMARY_MODEL`

### I-010: Session-KB Linkage
- Every KB entry created during a session SHOULD carry the `session_id`
- The link is advisory: entries function independently if session is GC'd (`session_id` set to NULL on GC)
- Query pattern: `SELECT * FROM entries WHERE session_id = ?` returns all knowledge from a session

### I-011: Append is Always O(1) — unchanged
- JSONL append: lock, check end event, write line, release lock
- SQLite `event_count` increment: single UPDATE (best-effort, advisory — if SQLite write fails, JSONL write still succeeds and event_count may drift; the JSONL file is authoritative for actual event count)

### I-012: Skills Consolidation
- `/learn`, `/decide`, `/gotcha` skills MUST be removed
- `/retro` is the sole knowledge-capture skill
- The learning-decorator hook MUST NOT reference removed skills

## Errors

| Condition | Exit code | Stderr message |
|-----------|-----------|----------------|
| Empty session_id | 1 | `error: session_id is required` |
| Session not found (for show/end) | 1 | `error: session <uuid> not found` |
| Session not found (for append) | 0 | `warning: session <uuid> not found, skipping append` |
| Invalid --type value | 1 | `error: type must be one of: prompt, tool, agent` |
| --type tool without --tool | 1 | `error: --tool is required when --type is tool` |
| --type agent without --agent-type | 1 | `error: --agent-type is required when --type is agent` |
| Retro pass failure | 0 | `warning: retro pass failed: <reason>` |
| Ambiguous UUID prefix | 1 | `error: ambiguous prefix <prefix>, matches: <list>` |
| Invalid --session-id on kb put | 0 | `warning: session <uuid> not found, entry created without link` |

## Non-functional Requirements

### NF-001: Performance
- `cvm session start`: < 100ms (SQLite INSERT + JSONL create)
- `cvm session append`: < 50ms target (JSONL append + SQLite UPDATE event_count)
- `cvm session end` (without retro): < 100ms
- `cvm session end` (with retro): < 60s (LLM-bound)
- `cvm session status/ls`: < 50ms (SQLite query, no file scanning)

### NF-002: Storage
- Sessions table: negligible overhead (one row per session, ~200 bytes)
- JSONL files: unchanged (100-500 events typical)
- Entries.session_id: one TEXT column, nullable

### NF-003: Compatibility — unchanged
- macOS (primary), Linux
- No GNU-only flags
- Go 1.21+

## Dead Code Removal

In addition to removals from v0.4.0:

| File/Component | Reason |
|----------------|--------|
| `sessionIsPIDAlive()` in api.go | PID checking eliminated (I-005) |
| `cleanOrphans()` in session.go | Orphan cleanup eliminated |
| `execPsComm()` in api.go | PID process name check eliminated |
| `profiles/sdd-mem/skills/learn.md` | Consolidated into /retro (I-012) |
| `profiles/sdd-mem/skills/decide.md` | Consolidated into /retro (I-012) |
| `profiles/sdd-mem/skills/gotcha.md` | Consolidated into /retro (I-012) |
| `generateSummary()` in session.go | Replaced by `generateRetro()` — same `claude -p` pattern but with retro prompt (C-009a) |
| `CVM_AUTOSUMMARY_ENABLED` env var | Replaced by `CVM_SESSION_RETRO_ENABLED` |
| `CVM_AUTOSUMMARY_MODEL` env var | Replaced by `CVM_SESSION_RETRO_MODEL` |

## Related Specs

- **S-016** (dashboard): MUST update to read from `sessions` table
- **S-013** (sqlite-fts5): Migration adds `session_id` to entries table and creates `sessions` table
- **S-010** (sdd-mem): Remove /learn, /decide, /gotcha skill references; update learning protocol

## Dependencies

- Existing SQLite database (`~/.cvm/global/kb.db`) via `modernc.org/sqlite`
- `claude -p --model haiku` for end-session retro (existing dependency, same as old summary generation)
- `/retro` skill (must be updated to support `mid-session` scope + session_id linking)
- Go `syscall.Flock` for JSONL file locking
- `Backend.Put()` interface change: add `sessionID string` parameter

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| 0.1.0 | 2026-04-13 | Initial draft — CVM-owned session system |
| 0.2.0 | 2026-04-13 | Triple review round 1: fix contradictions, lazy compaction, PID reuse detection |
| 0.3.0 | 2026-04-13 | Triple review round 2: process name validation, scope I-003, flag validation |
| 0.4.0 | 2026-04-13 | Triple review round 3: session_id identity gap, summary key collision, automation |
| 0.5.0 | 2026-04-13 | Major redesign: SQLite sessions table, remove PID checking, KB entries linked to session_id, session end runs /retro instead of summary, consolidate /learn /decide /gotcha into /retro, dashboard reads from SQLite |
| 0.6.0 | 2026-04-13 | Dual review fixes (Opus A+B): clarify retro runs via `claude -p --model haiku` (not as skill invocation), remove FK constraint on entries.session_id (use application-level validation for graceful degradation), specify Backend.Put() interface change with sessionID param, add migration mechanism (C-002a), add indexes on sessions.status and entries.session_id, mark event_count as advisory, GC uses ended_at not file mtime, clarify SSE watches JSONL not SQLite, add CVM_SESSION_RETRO_MODEL config |

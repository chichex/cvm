# S-017: CVM Session System

- **ID**: S-017
- **Version**: 0.4.0
- **Status**: approved
- **Supersedes**: S-011 (realtime-capture), S-015 (tool-observation)
- **Modifies**: S-016 (dashboard — session source changes from local KB to session files)
- **Validation Strategy**: TDD (CLI commands, storage, parsing) + manual (hooks, dashboard integration)

## Objective

Replace the current session lifecycle system (piggybacks on `~/.claude/sessions/`, uses local KB `session-buffer-*` entries, and `cvm lifecycle` commands) with a CVM-owned session system that stores structured events in `~/.cvm/sessions/`, provides unified CLI commands, and enables cross-project session visibility in the dashboard.

## Scope

### In scope
- New `cvm session` CLI command tree (start, append, end, status, ls, show, gc)
- JSONL-based session storage in `~/.cvm/sessions/<uuid>.jsonl`
- Centralized truncation logic in Go
- Dashboard reads sessions from `~/.cvm/sessions/` instead of local KB
- Hook updates (sdd-mem profile): replace `cvm kb put session-buffer` with `cvm session append`
- Removal of dead code: `cmd/lifecycle.go`, `internal/lifecycle/`, `hooks/tool-capture.sh`, `hooks/session-summary.sh`, `internal/dashboard/parser.go`
- Remove `sdd` profile (deprecated — `sdd-mem` is the sole profile)
- Preserve automation integration from `lifecycle.End()` in `cvm session end`: `saveActiveProfiles`, `automation.RecordSessionEnd`, `queueAutomationRunner`
- `cvm session start` prints KB stats and detected tools (same as current `lifecycle.Start()`) but does NOT mutate automation state
- Deprecate `CVM_AUTOSUMMARY_MIN_TOOLS` env var (S-011 legacy, replaced by hardcoded threshold in E-003)

### Out of scope
- Response capture (future enhancement, not in v0.1.0)
- Session search/filter CLI
- Session replay
- Migration of existing session-buffer KB entries

## Contracts

### C-001: Session Event (JSONL line)

All events share these base fields. Additional fields vary by type.

```go
// SessionEvent is the base for all JSONL lines. Concrete types embed extra fields.
// Discriminated by Type field. Implementation MAY use a single struct with optional
// fields or separate structs — as long as serialization matches the contracts below.
type SessionEvent struct {
    Type      string `json:"type"`                // "start" | "prompt" | "tool" | "agent" | "end"
    Timestamp string `json:"ts"`                  // RFC3339
    Content   string `json:"content,omitempty"`    // truncated payload
    Tool      string `json:"tool,omitempty"`       // tool name (only for type=tool)
    AgentType string `json:"agent_type,omitempty"` // agent type (only for type=agent)
}
```

### C-002: Session Start Event (first line of .jsonl)

```go
type SessionStartEvent struct {
    Type      string            `json:"type"`        // always "start"
    Timestamp string            `json:"ts"`          // RFC3339
    SessionID string            `json:"session_id"`  // UUID from Claude Code
    Project   string            `json:"project"`     // absolute path to project dir
    Profile   string            `json:"profile"`     // active cvm profile name
    PID       int               `json:"pid"`         // process ID of claude (derived via os.Getppid() in Go)
    Tools     map[string]bool   `json:"tools"`       // detected tools (auto-detected by cvm session start)
}
```

Tools are auto-detected by `cvm session start` (checks PATH for: claude, codex, gemini, gh, docker, node, npm, go). No `--tools` flag needed.

`--pid` is optional; if omitted, `cvm session start` uses `os.Getppid()` to get the parent process PID (the Claude Code process that invoked the hook).

### C-003: Session End Event (last line of .jsonl)

```go
type SessionEndEvent struct {
    Type       string `json:"type"`                // always "end"
    Timestamp  string `json:"ts"`                  // RFC3339
    SummaryKey string `json:"summary_key"`         // global KB key where summary was stored (empty if skipped/failed)
    Reason     string `json:"reason,omitempty"`    // "normal" | "orphan" | "error" — why the session ended
}
```

### C-004: Truncation Limits

"chars" means UTF-8 runes (Go `[]rune` length).

| Event type | Max content length | Behavior when exceeded |
|------------|-------------------|----------------------|
| prompt     | 300 runes         | Truncate, append "…" |
| tool       | 200 runes         | Truncate, append "…" |
| agent      | 300 runes         | Truncate, append "…" |

### C-005: Session File Cap

- Max events per session: **unlimited during active session** (append is always O(1))
- Compaction happens **only at session end** (`cvm session end`): if event count > 1000, keep start event + last 999 events before generating summary
- File location: `~/.cvm/sessions/<uuid>.jsonl`
- UUID: the `session_id` provided by Claude Code via stdin JSON

### C-006: CLI Interface

```
cvm session start    [--session-id <uuid>] [--project <path>] [--profile <name>] [--pid <int>]
cvm session append   <uuid> --type <prompt|tool|agent> [--content <string>] [--tool <name>] [--agent-type <type>]
cvm session end      <uuid>
cvm session status
cvm session ls       [--limit <n>]   # default limit: 20
cvm session show     <uuid>
cvm session gc       [--older-than <duration>]  # default: 30d
```

If `--session-id` is omitted from `cvm session start`, the command MUST generate a UUID v4. However, when called from a Claude Code hook, the hook script MUST extract `session_id` from stdin JSON and pass it via `--session-id` to ensure identity continuity with subsequent `append` calls from other hooks (which also receive `session_id` via stdin). The settings.json hook command MUST be a shell script that parses stdin, NOT a bare `cvm session start` invocation.

### C-007: Dashboard API Changes

`GET /api/stats` response field `active_sessions` MUST count open session files (no "end" event) in `~/.cvm/sessions/` with a validated live PID (per I-010: PID alive + process name contains "claude").

`GET /api/sessions` MUST return both:
- Active sessions from `~/.cvm/sessions/*.jsonl` (open, live PID)
- Completed summaries from global KB (`session-summary-*`)

### C-008: Summary Prompt Template

```
Summarize this coding session from the captured events.
Generate JSON: {"request": "...", "accomplished": "...", "discovered": "...", "next_steps": "..."}
Max 1-2 sentences per field. Output ONLY the JSON.

<events>
{events_text}
</events>
```

Where `{events_text}` is the JSONL content (after optional compaction to 1000 events).

## Behaviors

### B-001: Session Start
- **Given** a Claude Code session starts with session_id `778a7b24-509f-4f79-a99e-cd01e631ef82` in project `/Users/me/workspace/cvm` with profile `sdd-mem`
- **When** the SessionStart hook script extracts `session_id` from stdin JSON and runs `cvm session start --session-id 778a7b24-509f-4f79-a99e-cd01e631ef82 --project /Users/me/workspace/cvm --profile sdd-mem --pid 40558`
- **Then** a file `~/.cvm/sessions/778a7b24-509f-4f79-a99e-cd01e631ef82.jsonl` MUST be created
- **And** the first line MUST be a valid JSON object with `type: "start"`, the session_id, project, profile, pid (via `os.Getppid()` if not provided), and auto-detected tools
- **And** the command MUST print the session UUID to stdout
- **And** the command MUST run orphan cleanup (E-007) before creating the new session
- **And** the command MUST exit with code 0
- **Note**: the SessionStart hook in settings.json MUST call a shell script (e.g., `session-start.sh`) that reads stdin, extracts `session_id` via `python3 -c "..."`, and passes it to `cvm session start --session-id <id>`. This is the SAME pattern used by all other hooks (B-014). A bare `cvm session start` (without `--session-id`) would generate a new UUID that does NOT match the `session_id` that Claude Code passes to subsequent hooks, breaking identity continuity.

### B-002: Append Prompt Event
- **Given** an active session `778a7b24`
- **When** `cvm session append 778a7b24-509f-4f79-a99e-cd01e631ef82 --type prompt --content "hola claude por que me aparece una sesion activa"`
- **Then** a JSON line `{"type":"prompt","ts":"2026-04-13T17:02:03-03:00","content":"hola claude por que me aparece una sesion activa"}` MUST be appended to the session file
- **And** the command MUST complete in < 50ms (benchmark target, not hard CI assertion)

### B-003: Append Tool Event
- **Given** an active session `778a7b24`
- **When** `cvm session append 778a7b24-509f-4f79-a99e-cd01e631ef82 --type tool --tool Bash --content "ls -la ~/.claude/sessions/"`
- **Then** a JSON line with `type: "tool"`, `tool: "Bash"`, and the content MUST be appended
- **And** the command MUST complete in < 50ms (benchmark target)

### B-004: Append Agent Event
- **Given** an active session `778a7b24`
- **When** `cvm session append 778a7b24-509f-4f79-a99e-cd01e631ef82 --type agent --agent-type haiku --content "Research complete. Found 3 files matching..."`
- **Then** a JSON line with `type: "agent"`, `agent_type: "haiku"`, and the content MUST be appended

### B-005: Session End with Summary
- **Given** an active session `778a7b24` with 50 events
- **When** `cvm session end 778a7b24-509f-4f79-a99e-cd01e631ef82` runs
- **Then** the session file MUST be read (snapshot) WITHOUT holding the file lock during LLM call
- **And** if event count > 1000, compact to start + last 999 events before summarizing
- **And** the content MUST be summarized via `claude -p --model $CVM_AUTOSUMMARY_MODEL` using the prompt template from C-008
- **And** the summary MUST be stored in global KB as `session-summary-<YYYYMMDD-HHMMSS>-<uuid8>` with tags `session,summary` (where `<uuid8>` is the first 8 chars of the session UUID, ensuring collision resistance)
- **And** a final `{"type":"end","ts":"...","summary_key":"session-summary-20260413-170500-778a7b24","reason":"normal"}` line MUST be appended (under lock)
- **And** the session file MUST NOT be deleted (archive)
- **And** `~/.cvm/learning-pulse` MUST be deleted if it exists
- **And** automation integration MUST be preserved: call `saveActiveProfiles()`, `automation.RecordSessionEnd()`, `queueAutomationRunner()` equivalents
- **Note**: events appended between the snapshot read and the end event write are preserved in the file but MAY be absent from the summary. This is acceptable — the session is ending and a few missed events in the summary are tolerable.

### B-006: Session End with Autosummary Disabled
- **Given** `CVM_AUTOSUMMARY_ENABLED=false`
- **When** `cvm session end <uuid>` runs
- **Then** it MUST skip LLM summary generation entirely
- **And** it MUST append an "end" event with empty `summary_key` and `reason: "normal"`
- **And** it MUST still perform automation integration and learning-pulse cleanup

### B-007: Session Status
- **Given** 2 session files exist: one open (no "end" event, PID alive and process is "claude"), one closed (has "end" event)
- **When** `cvm session status` runs
- **Then** it MUST list only the open session with verified live PID
- **And** it MUST show: session_id, project, profile, start time, event count

### B-008: Session List
- **Given** 5 session files exist (2 open, 3 closed)
- **When** `cvm session ls` runs
- **Then** it MUST list all sessions ordered by start time descending (default limit: 20)
- **And** each MUST show: session_id (truncated to 8 chars), status (active/closed), project, start time, event count

### B-009: Session Show
- **Given** a session `778a7b24` with 20 events
- **When** `cvm session show 778a7b24-509f-4f79-a99e-cd01e631ef82` runs
- **Then** it MUST print all events formatted as human-readable lines

### B-010: Session GC
- **Given** 10 closed session files, 3 with file mtime older than 30 days
- **When** `cvm session gc` runs
- **Then** it MUST delete the 3 closed session files whose file mtime exceeds the threshold
- **And** it MUST NOT delete active (open) sessions regardless of age
- **And** it MUST print: `deleted 3 session(s)`
- **Note**: age is determined by file mtime (last modification time), not by timestamps inside the JSONL

### B-011: Dashboard Active Sessions Count
- **Given** the dashboard is running from any directory
- **And** there are 2 open session files with live PIDs in `~/.cvm/sessions/`
- **When** `GET /api/stats` is called
- **Then** `active_sessions` MUST be `2`
- **And** it MUST NOT depend on local KB or the dashboard's working directory

### B-012: Dashboard Sessions List
- **Given** 1 active session in `~/.cvm/sessions/` and 3 completed summaries in global KB
- **When** `GET /api/sessions` is called
- **Then** the response MUST contain 4 session cards
- **And** the active session card MUST have `status: "active"` and include the project dir from the start event
- **And** completed sessions MUST have `status: "summarized"`

### B-013: SSE Session Updates
- **Given** the dashboard watcher is running and polling `~/.cvm/sessions/` at 2s intervals
- **When** a session file in `~/.cvm/sessions/` is modified (mtime changes)
- **Then** an SSE event `session_updated` MUST be emitted

### B-014: Stdin Passthrough for Hooks
- **Given** a hook receives session_id via stdin JSON (Claude Code convention)
- **When** `learning-decorator.sh` extracts `session_id` from stdin
- **Then** it MUST pass it to `cvm session append <session_id> --type prompt --content "..."`
- **And** the same pattern applies to `tool-observe.sh` and `subagent-stop.sh`
- **And** hooks MUST continue to filter tools via `CVM_OBSERVE_TOOLS` env var before calling append

## Edge Cases

### E-001: Session File Does Not Exist on Append
- **Given** `cvm session append <uuid>` is called but `~/.cvm/sessions/<uuid>.jsonl` does not exist
- **When** the command runs
- **Then** it MUST exit with code 0 (no-op, silently drop the event)
- **And** it MUST log a warning to stderr: `warning: session <uuid> not found, skipping append`
- **Note**: this preserves I-003 — files are only created by `cvm session start`

### E-002: Session Already Ended
- **Given** session `778a7b24` has an "end" event
- **When** `cvm session append 778a7b24-...` is called
- **Then** it MUST exit with code 0 (no-op)
- **And** it MUST log a warning to stderr: `warning: session <uuid> already ended, ignoring append`

### E-003: Session End on Empty/Short Session
- **Given** a session with < 3 events (only start + 1-2 appends)
- **When** `cvm session end` runs
- **Then** it MUST skip LLM summary generation
- **And** it MUST append an "end" event with empty `summary_key` and `reason: "normal"`
- **And** it MUST NOT store a summary in global KB

### E-004: Session End with Summary Failure
- **Given** `claude -p --model haiku` fails or times out
- **When** `cvm session end` runs
- **Then** it MUST still append an "end" event (with empty `summary_key`, `reason: "error"`)
- **And** it MUST log the error to stderr
- **And** it MUST exit with code 0 (graceful degradation)

### E-005: Empty session_id
- **Given** a hook calls `cvm session append "" --type prompt --content "..."`
- **When** the command runs
- **Then** it MUST exit with code 1
- **And** it MUST log: `error: session_id is required`

### E-006: Compaction at Session End
- **Given** a session has 2500 events when `cvm session end` runs
- **When** the file is read for summarization
- **Then** only the start event + last 999 events MUST be used for the LLM summary
- **And** the original file is NOT rewritten — compaction is read-time only for summary input
- **And** the end event is appended as line 2501 (file keeps growing during the session)

### E-007: Orphan Session Cleanup
- **Given** a session file exists with no "end" event and the PID is dead (or PID alive but process name is not "claude" — indicating PID reuse)
- **When** `cvm session start` runs (before creating the new session)
- **Then** it MUST acquire the file lock on the orphan's session file
- **And** it MUST verify no "end" event exists (check-under-lock to prevent race)
- **And** it MUST append an "end" event with `summary_key: ""` and `reason: "orphan"`
- **And** it MUST log: `warning: cleaned up orphan session <uuid> (PID <pid> dead)`

### E-008: Concurrent Appends
- **Given** multiple hooks fire simultaneously for the same session
- **When** they each call `cvm session append`
- **Then** each append MUST use Go `syscall.Flock` (LOCK_EX) on the session file to prevent corruption
- **And** all events MUST be written (no data loss) — append is always O(1), no file rewrite

### E-009: UUID Prefix Matching
- **Given** `cvm session show 778a7b24`
- **When** only one session file starts with `778a7b24`
- **Then** it MUST resolve to the full UUID and show that session
- **When** multiple sessions match the prefix
- **Then** it MUST list all matches and exit with code 1

### E-010: Concurrent Session End
- **Given** `cvm session end <uuid>` is called while another end is already in progress
- **When** the second call tries to append the end event
- **Then** it MUST detect the existing end event (under lock) and exit with code 0
- **And** it MUST log: `warning: session <uuid> already ended`

### E-011: Dashboard Reads During Append
- **Given** the dashboard is reading a session file while an append is in progress
- **When** the dashboard reads the file
- **Then** it MUST handle partial last lines gracefully (skip lines that fail JSON parsing)
- **Note**: advisory flock is not required for readers — readers tolerate partial state

## Invariants

### I-001: Append Latency
- `cvm session append` MUST target < 50ms completion (no network, no LLM calls)
- This is a benchmark target verified via `go test -bench`, not a hard CI assertion

### I-002: Storage Location
- All session files MUST live in `~/.cvm/sessions/`
- CVM MUST NOT write to `~/.claude/sessions/`

### I-003: File Format
- Each fully-written line in a session file MUST be valid JSON (one JSON object per newline-terminated line)
- During concurrent writes, the last line MAY be a partial/unterminated JSON fragment — readers MUST tolerate this (see E-011)
- The first line MUST always be a start event (only `cvm session start` creates files)
- The last fully-written line of a closed session MUST be an end event

### I-004: Cross-Project Visibility
- The dashboard MUST show active sessions from ALL projects regardless of its own working directory
- Session count MUST NOT depend on local KB

### I-005: Backward Compatibility (Summaries)
- Completed session summaries in global KB (`session-summary-*`) MUST remain unchanged in format
- The dashboard MUST continue to display existing summaries alongside new active sessions
- **Note**: existing KB summaries use key format `session-<ts>` (legacy) and `session-summary-<ts>` (new). Dashboard MUST recognize both patterns.

### I-006: No Local KB Dependency
- The session system MUST NOT use local KB for session data
- Local KB continues to exist for non-session entries (learnings, decisions, etc.)

### I-007: File Locking
- All writes to session files MUST use Go `syscall.Flock` with `LOCK_EX` (exclusive advisory lock)
- Readers (dashboard, show, ls, status) do NOT acquire locks — they tolerate partial reads (E-011)
- **Note**: `flock` refers to the BSD syscall via Go's `syscall.Flock`, NOT the `flock(1)` CLI utility (not available on macOS by default)

### I-008: Graceful Degradation
- If `~/.cvm/sessions/` does not exist, `cvm session start` MUST create it
- If a session file is corrupted (invalid JSON lines), `cvm session show` MUST skip invalid lines and warn

### I-009: Autosummary Config
- `CVM_AUTOSUMMARY_ENABLED=true|false` controls whether `cvm session end` generates an LLM summary (default: `true`)
- `CVM_AUTOSUMMARY_MODEL` controls which model to use (default: `haiku`)
- When disabled, `cvm session end` skips LLM call but still appends end event and runs automation (B-006)

### I-010: PID Validation
- A session is considered "active" only if: (1) no end event exists, AND (2) the PID is alive, AND (3) the process command name contains "claude"
- This prevents false positives from PID reuse by the OS (e.g., PID recycled to "vim" won't match)
- Process name check: on macOS use `ps -p <pid> -o comm=`, on Linux use `/proc/<pid>/comm`
- If the process name check fails (PID alive but not "claude"), the session is treated as orphan (E-007)

### I-011: Append is Always O(1)
- `cvm session append` MUST always be a file append: acquire lock, read last line to check for end event (E-002), write new line, release lock
- The end-event check reads only the last line (seek to near-EOF, scan backwards for newline) — this is O(1) regardless of file size
- No file rewriting, no read-modify-write of existing content, no compaction on the hot path
- Compaction is read-time only (E-006) and only during `cvm session end`

## Errors

| Condition | Exit code | Stderr message |
|-----------|-----------|----------------|
| Empty session_id | 1 | `error: session_id is required` |
| Session file not found (for show/end) | 1 | `error: session <uuid> not found` |
| Session file not found (for append) | 0 | `warning: session <uuid> not found, skipping append` |
| Invalid --type value | 1 | `error: type must be one of: prompt, tool, agent` |
| --type tool without --tool | 1 | `error: --tool is required when --type is tool` |
| --type agent without --agent-type | 1 | `error: --agent-type is required when --type is agent` |
| Summary generation failure | 0 | `warning: summary generation failed: <reason>` |
| Ambiguous UUID prefix | 1 | `error: ambiguous prefix <prefix>, matches: <list>` |

## Non-functional Requirements

### NF-001: Performance
- `cvm session start`: < 100ms
- `cvm session append`: < 50ms target (benchmark, not hard assertion)
- `cvm session end` (without summary): < 100ms
- `cvm session end` (with summary): < 60s (LLM-bound)
- `cvm session status/ls`: < 200ms (with up to 100 session files)

### NF-002: Storage
- Typical session: 100-500 events, 50-250KB JSONL file
- Long session: unlimited events during session, compacted at read-time for summary
- Closed sessions archived until `cvm session gc` (default retention: 30 days)

### NF-003: Compatibility
- macOS (primary), Linux
- No GNU-only flags (no `grep -P`, no `timeout`)
- Go 1.21+

## Dead Code Removal

The following MUST be deleted as part of this implementation:

| File/Component | Reason |
|----------------|--------|
| `cmd/lifecycle.go` | Replaced by `cmd/session.go` |
| `internal/lifecycle/lifecycle.go` | Replaced by `internal/session/` package. Reuse `detectTools()` and `taggedEntryCount()`. |
| `internal/lifecycle/lifecycle_test.go` | Replaced by new tests |
| `internal/dashboard/parser.go` | Text line parser; new system uses structured JSONL |
| `profiles/sdd-mem/hooks/tool-capture.sh` | Unified into `tool-observe.sh` (NotebookEdit capture moves to tool-observe.sh) |
| `settings.json` PostToolUse entry for `tool-capture.sh` | Remove hook entry; NotebookEdit matcher moves to tool-observe.sh |
| `profiles/sdd-mem/hooks/session-summary.sh` | Absorbed into `cvm session end` |
| `context-inject.sh` orphan cleanup (lines ~10-59) | Orphan cleanup moves to `cvm session start` (E-007) |
| `~/.cvm/session.json` runtime file | Metadata now in session JSONL start event |
| `sessionLineJSON`, `ParseSessionLines` in api.go | Replaced by JSONL event reading |
| `cmd/root.go` `lifecycleCmd` registration | Replaced by `sessionCmd` |

## Related Specs

- **S-016** (dashboard): MUST be updated to read from `~/.cvm/sessions/` instead of local KB for active sessions. Invariant I-002n changes.
- **S-011** (realtime-capture): SUPERSEDED — all session buffer behaviors move to this spec.
- **S-015** (tool-observation): SUPERSEDED — tool capture moves to `cvm session append --type tool`. `CVM_OBSERVE_TOOLS` filtering stays in hooks.
- **S-012** (multi-validator): Minor update — `cvm lifecycle start` references become `cvm session start`.
- **S-010** (sdd-mem): Minor update — lifecycle references become session references.

## Dependencies

- `claude -p --model haiku` for summary generation (existing dependency)
- Global KB backend for storing summaries (existing dependency)
- Go `syscall.Flock` for file locking (BSD syscall, available on macOS and Linux)

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| 0.1.0 | 2026-04-13 | Initial draft — CVM-owned session system |
| 0.2.0 | 2026-04-13 | Address triple review round 1 (Opus+Codex+Gemini): fix E-001 vs I-003 contradiction, lazy compaction (I-011), Reason field (C-003), summary prompt (C-008), PID reuse detection (I-010), flock clarification (I-007), autosummary disabled (B-006), session gc (B-010), concurrent end (E-010), partial reads (E-011), legacy key format (I-005), automation integration, --limit default |
| 0.3.0 | 2026-04-13 | Address triple review round 2: replace started_at with process name validation (I-010), scope I-003 to fully-written lines, document late-append window, add flag validation errors, define GC mtime, deprecate CVM_AUTOSUMMARY_MIN_TOOLS, clarify --pid/I-011 |
| 0.4.0 | 2026-04-13 | Address triple review round 3 (Codex): fix SessionStart identity gap — hook MUST extract session_id from stdin and pass via --session-id (same pattern as all other hooks), not bare CLI invocation. Fix summary key collision — append uuid8 suffix for uniqueness. Clarify start-side automation is print-only (no state mutation), all automation integration is in session end |

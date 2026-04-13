# S-015: Semi-auto Observation Capture via PostToolUse Hook

**Version:** 0.1.0
**Status:** implemented
**Validation:** manual + integration tests
**Origen:** Expands S-011 B-001 and S-010 B-017 (Phase 3). Delivers richer per-tool observation capture with a configurable tool allowlist, tighter latency budget, and an enriched capture format that remains fully compatible with the existing session buffer consumed by session-summary.sh.

---

## Objective

Introduce a dedicated PostToolUse hook (`tool-observe.sh`) that captures richer, structured observations for Bash, Write, and Edit tool uses and appends them to the existing session buffer (`session-buffer-<session_id>`) defined in S-011. The hook MUST replace the PostToolUse capture responsibility of `tool-capture.sh` for the Bash/Write/Edit subset, or coexist alongside it as a separate registration — the implementation plan determines which. The capture format MUST be compatible with `session-summary.sh` without any changes to that script.

---

## Design Principle

> **Observe at the action boundary**: capturing tool inputs at PostToolUse is the only reliable, format-stable way to record what Claude did. Richer summaries (command text, file path, edit snippet) make session summaries more useful without adding LLM cost during the session.

---

## Scope

### Included

| ID | Item | Description |
|----|------|-------------|
| B-001 | Hook: tool-observe.sh | New PostToolUse hook that appends structured `[TOOL:<name>]` observations to the session buffer |
| B-002 | Bash capture | Record the command, truncated to 200 chars |
| B-003 | Write capture | Record the file path |
| B-004 | Edit capture | Record the file path + first 50 chars of `old_string` as a snippet |
| B-005 | Configurable allowlist | `CVM_OBSERVE_TOOLS` env var controls which tools trigger capture (default: `Bash,Write,Edit`) |
| B-006 | Buffer compatibility | Append format MUST be parseable by `session-summary.sh` without modification |
| B-007 | Buffer lifecycle | Reuses the `session-buffer-<session_id>` lifecycle defined in S-011 B-007; no new buffer keys |

### Excluded

- Capture of Read, Grep, Glob, Agent, SubagentStop (too noisy, no state change)
- NotebookEdit (was in S-011 scope; not in the default CVM_OBSERVE_TOOLS list, MAY be added by user via env override)
- LLM calls during the hook (zero-cost requirement)
- Changes to `session-summary.sh`
- Changes to `learning-decorator.sh`
- Changes to `session-digest.sh` or any S-010 artifact

---

## Hook Input Schema

Claude Code passes the following JSON to stdin for every PostToolUse event:

```json
{
  "session_id": "<string>",
  "tool_name": "<string>",
  "tool_input": { ... },
  "tool_response": { ... }
}
```

Field semantics relevant to this spec:

| Field | Type | Notes |
|-------|------|-------|
| `session_id` | string | Unique session identifier. MUST be used as buffer key suffix. Empty → exit 0. |
| `tool_name` | string | Name of the tool that was called (e.g., `Bash`, `Write`, `Edit`). |
| `tool_input` | object | The input passed to the tool. Shape varies by tool (see per-tool sections). |
| `tool_response` | object | The tool's output. Not used for capture (input is sufficient). |

### tool_input shapes

**Bash:**
```json
{ "command": "<shell command string>" }
```

**Write:**
```json
{ "file_path": "<absolute or relative path>", "content": "<file content>" }
```

**Edit:**
```json
{ "file_path": "<path>", "old_string": "<text to find>", "new_string": "<replacement>" }
```

---

## Contracts

### B-001: PostToolUse hook — tool-observe.sh

```
Trigger: PostToolUse hook (registered in settings.json)
Matcher: "Bash|Write|Edit" (default; controlled by CVM_OBSERVE_TOOLS at runtime)
Input (stdin): JSON with session_id, tool_name, tool_input, tool_response

Filter:
  MUST capture only tools listed in CVM_OBSERVE_TOOLS (default: Bash,Write,Edit).
  MUST silently exit 0 for any tool NOT in CVM_OBSERVE_TOOLS.
  MUST silently exit 0 if session_id is missing or empty.
  MUST silently exit 0 if tool_input is missing or not a JSON object.

Output:
  Append one line to KB entry "session-buffer-<session_id>" (--local) with format:
    [HH:MM] [TOOL:<tool_name>] <summary>
  Where <summary> is tool-specific (see B-002, B-003, B-004).

Latency:
  MUST complete in < 50ms (I-001). PostToolUse fires on every tool call; budget is strict.
```

### B-002: Bash capture

```
Input field: tool_input.command (string)
Summary format: <command truncated to 200 chars>

Full line example:
  [14:35] [TOOL:Bash] npm test -- --grep "auth"

Truncation:
  If len(command) > 200: use command[:200] + "…" (ellipsis character U+2026)
  If command is empty string: use "(empty command)"
  If tool_input.command is absent: use "(no command)"
```

### B-003: Write capture

```
Input field: tool_input.file_path (string)
Summary format: wrote <file_path>

Full line example:
  [14:36] [TOOL:Write] wrote /Users/ayrtonmarini/workspace/cvm/specs/foo.spec.md

If file_path is absent or empty: use "wrote (unknown path)"
```

### B-004: Edit capture

```
Input fields: tool_input.file_path (string), tool_input.old_string (string)
Summary format: edited <file_path> — "<old_string_snippet>"

Where <old_string_snippet> is:
  - First 50 chars of old_string, with internal newlines replaced by ↵ (U+21B5)
  - If len(old_string) > 50: append "…"
  - If old_string is empty string: use "(empty)"
  - If old_string is absent: use "(no old_string)"

Full line example:
  [14:37] [TOOL:Edit] edited src/auth.ts — "const token = req.headers['author…"

If file_path is absent or empty: use "(unknown path)"
```

### B-005: Configurable allowlist

```
Env var: CVM_OBSERVE_TOOLS (string, comma-separated tool names)
Default: "Bash,Write,Edit"

Parsing rules:
  - Split on comma, trim whitespace from each token
  - Case-sensitive match against tool_name from stdin
  - If CVM_OBSERVE_TOOLS is empty string or unset: use default "Bash,Write,Edit"
  - Unknown tool names in the list are silently ignored (no error)

Example overrides:
  CVM_OBSERVE_TOOLS="Bash,Write,Edit,NotebookEdit"  → adds NotebookEdit
  CVM_OBSERVE_TOOLS="Bash"                          → Bash only
  CVM_OBSERVE_TOOLS=""                              → fallback to default
```

### B-006: Buffer append

```
Key: "session-buffer-<session_id>"
Tags: "session-buffer" (same as S-011 B-007)
Scope: --local (same as S-011 B-007)

Append strategy (same as S-011 B-007 — read-modify-write):
  1. cvm kb show <buffer_key> --local → strip frontmatter via sed '1,/^$/d'
  2. Append new_line
  3. If line_count >= 100: drop oldest lines to keep total at 99 before appending
  4. cvm kb put <buffer_key> --body <new_body> --tag "session-buffer" --local

If buffer does not exist yet (first tool use before first UserPromptSubmit):
  MUST create the entry with new_line as the sole body line.
  This is a valid state; session-summary.sh tolerates buffers with lines from tools only.

If cvm kb put fails:
  Log warning to stderr: "[tool-observe] warning: cvm kb put failed"
  Continue; MUST NOT exit with non-zero (hook failure blocks Claude Code).
```

### B-007: settings.json registration

```
The hook MUST be registered under PostToolUse in settings.json with a matcher
that limits invocations to the default tool set:

{
  "PostToolUse": [
    {
      "matcher": "Bash|Write|Edit",
      "hooks": [
        { "type": "command", "command": "bash ~/.claude/hooks/tool-observe.sh" }
      ]
    }
  ]
}

Note: the matcher provides a fast pre-filter by Claude Code before stdin is even
written. The env var CVM_OBSERVE_TOOLS provides a second runtime filter inside
the hook. Both MUST be consistent; if CVM_OBSERVE_TOOLS is extended to include
NotebookEdit, the matcher MUST also be updated.

Relationship to existing tool-capture.sh registration:
  S-011 already registers tool-capture.sh for "Bash|Write|Edit|NotebookEdit".
  Two options for implementation (chosen during execute phase):
    Option A: Replace tool-capture.sh with tool-observe.sh for the Bash/Write/Edit subset.
    Option B: Register tool-observe.sh as an additional hook alongside tool-capture.sh,
              accepting that both append to the same buffer.
  Option A is preferred (avoids duplicate lines). The implementation wave decides.
```

---

## Behaviors (Given/When/Then)

### B-001: Bash command captured

```
GIVEN a session is active with session_id "abc123"
AND CVM_OBSERVE_TOOLS is unset (default: Bash,Write,Edit)
AND Claude executes Bash with command "npm test -- --grep auth"
WHEN the PostToolUse hook fires for tool_name "Bash"
THEN the KB entry "session-buffer-abc123" MUST contain a line matching:
  [HH:MM] [TOOL:Bash] npm test -- --grep auth
AND the hook MUST complete in < 50ms
```

### B-002: Long bash command truncated

```
GIVEN a Bash tool_input.command is 350 characters long
WHEN the PostToolUse hook fires
THEN the captured line MUST contain the first 200 chars of the command followed by "…"
AND the total line length MUST be <= ~230 chars (timestamp + prefix + 200 + ellipsis)
```

### B-003: Write captured

```
GIVEN Claude writes to file "/Users/ayrtonmarini/workspace/cvm/specs/foo.spec.md"
WHEN the PostToolUse hook fires for tool_name "Write"
THEN the buffer MUST contain:
  [HH:MM] [TOOL:Write] wrote /Users/ayrtonmarini/workspace/cvm/specs/foo.spec.md
```

### B-004: Edit captured with snippet

```
GIVEN Claude edits file "src/auth.ts" with old_string "const token = req.headers['authorization']"
WHEN the PostToolUse hook fires for tool_name "Edit"
THEN the buffer MUST contain:
  [HH:MM] [TOOL:Edit] edited src/auth.ts — "const token = req.headers['authori…"
AND the snippet MUST be exactly 50 chars + "…" since old_string is > 50 chars
```

### B-005: Edit with multiline old_string

```
GIVEN tool_input.old_string is "function foo() {\n  return 1;\n}"
WHEN the PostToolUse hook fires for tool_name "Edit"
THEN the captured snippet MUST have newlines replaced with ↵:
  "function foo() {↵  return 1;↵}"
```

### B-006: Filtered tool — no capture

```
GIVEN Claude uses the Read tool to read a file
WHEN the PostToolUse hook fires for tool_name "Read"
THEN NO line MUST be appended to the buffer
AND the hook MUST exit 0 in < 10ms (matcher pre-filter handles this at Claude Code level)
```

### B-007: First tool use before any user prompt

```
GIVEN a session starts
AND the user has NOT yet submitted a prompt (no UserPromptSubmit has fired)
AND Claude immediately executes a Bash command (e.g., via a hook or auto-action)
WHEN the PostToolUse hook fires
THEN the buffer entry "session-buffer-<session_id>" MUST be created with the tool line as its first line
AND session-summary.sh MUST be able to read and summarize this buffer at SessionEnd
```

### B-008: Binary file path in Write

```
GIVEN tool_input.file_path contains non-ASCII bytes or unusual characters
  (e.g., "/tmp/résumé-données.txt" or "/path/with spaces/file.bin")
WHEN the PostToolUse hook fires for tool_name "Write"
THEN the file_path MUST be captured as-is (no encoding transformation)
AND the hook MUST NOT crash (exit non-zero)
```

### B-009: Empty tool_input

```
GIVEN PostToolUse fires with tool_input as an empty object {}
  OR tool_input is null
  OR tool_input is absent from the JSON
WHEN the hook processes the event
THEN the hook MUST NOT crash
AND IF tool_name is in the allowlist:
  MUST append a line with the appropriate "(no command)" / "(unknown path)" / "(no old_string)" fallback
AND the hook MUST exit 0
```

### B-010: Buffer cap enforcement

```
GIVEN the buffer already has 100 lines
WHEN the PostToolUse hook fires and would append a new line
THEN the oldest line MUST be dropped
AND the buffer MUST have exactly 100 lines after the append
AND no data loss beyond the dropped oldest line is acceptable
```

### B-011: CVM_OBSERVE_TOOLS customization

```
GIVEN CVM_OBSERVE_TOOLS="Bash" is set in the environment
AND Claude executes a Write tool use
WHEN the PostToolUse hook fires for tool_name "Write"
THEN NO line MUST be appended to the buffer
AND the hook MUST exit 0
```

### B-012: Concurrent write safety

```
GIVEN Claude fires multiple tool uses in rapid succession within a single session
  (e.g., 3 Bash commands in a loop)
WHEN the PostToolUse hook fires for each
THEN each hook invocation MUST read the current buffer, append, and write back
  (sequential read-modify-write; no interleaved writes under Claude Code's sequential hook model)
AND all 3 lines MUST appear in the buffer after all 3 invocations complete
NOTE: Claude Code guarantees sequential hook execution within a session.
  If this guarantee changes, a file lock (flock) MUST be added (tracked as future risk).
```

---

## Edge Cases

| ID | Condition | Expected Behavior |
|----|-----------|------------------|
| E-001 | Buffer does not exist (first tool use before UserPromptSubmit) | MUST create the buffer entry with new_line as the sole body line |
| E-002 | Bash command > 200 chars | Truncate to 200 chars + "…"; never crash on long input |
| E-003 | Binary or non-UTF8 file path in Write/Edit | Capture as-is; python3 handles bytes with errors='replace' if needed; hook MUST NOT exit non-zero |
| E-004 | tool_input missing or null | Use per-field fallbacks ("(no command)", "(unknown path)", "(no old_string)"); append the line, exit 0 |
| E-005 | Concurrent writes (rapid tool calls) | Sequential under Claude Code model; no lock needed now; document as future risk |
| E-006 | Buffer at 100 lines | Drop oldest line, append new; buffer stays at 100 lines |
| E-007 | session_id is empty string | Log warning to stderr, exit 0; do NOT create buffer with key "session-buffer-" |
| E-008 | CVM_OBSERVE_TOOLS contains whitespace | Trim each token; "Bash , Write" MUST parse as ["Bash", "Write"] |
| E-009 | old_string contains only whitespace or control chars | Replace control chars and newlines per B-004 rules; capture as-is otherwise |
| E-010 | cvm not in PATH | Log warning to stderr; hook MUST exit 0 (graceful degradation) |

---

## Invariants

| ID | Invariant |
|----|-----------|
| I-001 | `tool-observe.sh` MUST complete in < 50ms (no LLM, no network, no subprocess spawning beyond one python3 call + one cvm call) |
| I-002 | The hook MUST always exit 0; a non-zero exit blocks Claude Code tool execution |
| I-003 | Buffer key format MUST be `session-buffer-<session_id>` with `--local` scope (S-011 I-006 compatibility) |
| I-004 | Buffer line cap MUST be 100 lines maximum (S-011 I-004 compatibility) |
| I-005 | No LLM calls MUST be made during the hook (zero added cost per tool use) |
| I-006 | Capture format `[HH:MM] [TOOL:<name>] <summary>` MUST be valid plain text parseable by session-summary.sh without modification |
| I-007 | The hook MUST be idempotent-safe: duplicate lines in the buffer (from re-runs) are acceptable and do not break summary generation |
| I-008 | `tool_response` MUST NOT be read or logged (may contain large outputs; irrelevant to session observation) |

---

## Non-Functional Constraints

| Constraint | Value |
|------------|-------|
| Latency | < 50ms per invocation (stricter than S-011 I-001 of 500ms — PostToolUse fires much more frequently than UserPromptSubmit) |
| Added cost per session | $0 (no LLM calls in the hook) |
| Added network calls | 0 |
| Shell compatibility | bash (not zsh); macOS-compatible; no GNU-only flags |
| Dependencies | python3 (already required by S-011), cvm (already in PATH for sdd-mem profile) |
| Buffer storage | Existing cvm KB local (no new files, no SQLite, no temp files) |

---

## Errors

| Condition | Behavior |
|-----------|----------|
| stdin JSON malformed | Log warning to stderr; exit 0 |
| session_id missing or empty | Log "[tool-observe] warning: session_id missing, skipping" to stderr; exit 0 |
| tool_name missing | Log warning to stderr; exit 0 |
| tool not in CVM_OBSERVE_TOOLS | Silent exit 0 (not an error; normal filter path) |
| cvm kb show fails | Treat as empty buffer; proceed with new_line as sole body |
| cvm kb put fails | Log "[tool-observe] warning: cvm kb put failed" to stderr; exit 0 |
| python3 not available | Log warning to stderr; exit 0 (graceful degradation) |
| cvm not in PATH | Log warning to stderr; exit 0 (graceful degradation) |

---

## Relationship to S-011

S-015 extends, but does not replace, S-011:

| Aspect | S-011 (tool-capture.sh) | S-015 (tool-observe.sh) |
|--------|------------------------|------------------------|
| Capture format | `[HH:MM] Bash: <cmd>` | `[HH:MM] [TOOL:Bash] <cmd>` |
| Bash truncation | 120 chars | 200 chars |
| Edit detail | `edited <path>` | `edited <path> — "<old_snippet>"` |
| Latency target | < 500ms | < 50ms |
| Configurable tools | No | Yes (CVM_OBSERVE_TOOLS) |
| Buffer key | `session-buffer-<session_id>` | Same |
| session-summary.sh compat | Yes | Yes (no change needed) |

The implementation wave MUST decide whether to:
- **Option A (preferred)**: Replace tool-capture.sh with tool-observe.sh for Bash/Write/Edit. Update settings.json to remove the old registration and add the new one.
- **Option B (fallback)**: Add tool-observe.sh as an additional registration, accepting both formats in the buffer (both are valid text; session-summary.sh handles mixed formats gracefully).

If Option A: tool-capture.sh MUST still be kept for NotebookEdit capture (or NotebookEdit added to CVM_OBSERVE_TOOLS).

---

## Related Specs

| Spec | Relationship |
|------|-------------|
| S-011 (`realtime-capture.spec.md`) | S-015 extends B-001; shares buffer lifecycle (B-007), session-summary.sh (B-004), and all invariants |
| S-010 (`sdd-mem.spec.md`) | B-017 in S-010 originally described this feature as Phase 3 future work; S-015 is its formal spec |

---

## Implementation Plan

### Wave 1 — Write tool-observe.sh

1. Create `profiles/sdd-mem/hooks/tool-observe.sh`:
   - Read stdin once: `INPUT=$(cat)`
   - Single python3 call extracts: `session_id`, `tool_name`, and builds `[TOOL:<name>] <summary>` string
   - Respect `CVM_OBSERVE_TOOLS` env var with default fallback
   - Apply per-tool formatting (B-002, B-003, B-004)
   - Apply buffer read-modify-write with 100-line cap (B-006)
   - All error paths exit 0
2. Make executable: `chmod +x tool-observe.sh`

### Wave 2 — settings.json registration

1. Add PostToolUse entry for `tool-observe.sh` with matcher `"Bash|Write|Edit"`
2. Decide Option A vs Option B:
   - Option A: remove or update the `tool-capture.sh` entry to `NotebookEdit` only
   - Option B: add alongside existing entry (document duplicate-line risk)
3. Add `CVM_OBSERVE_TOOLS` env var to `settings.json` `env` block (optional, for explicitness)

### Wave 3 — Integration test

1. Start a session; run a Bash command, a Write, and an Edit
2. Verify buffer contains 3 lines with `[TOOL:Bash]`, `[TOOL:Write]`, `[TOOL:Edit]` prefixes
3. Verify Bash summary truncates at 200 chars for a long command
4. Verify Edit line contains the `— "<snippet>"` portion
5. End session; verify session-summary.sh still produces a valid summary from the buffer

### Wave 4 — Edge case validation

1. Test with buffer at 100 lines → verify cap enforcement
2. Test with `CVM_OBSERVE_TOOLS="Bash"` → verify Write and Edit are not captured
3. Test first tool use before UserPromptSubmit → verify buffer is created
4. Test empty `tool_input` → verify fallback strings appear, no crash

---

## Changelog

| Version | Date | Author | Change |
|---------|------|--------|--------|
| 0.1.0 | 2026-04-13 | specifier | Initial draft |

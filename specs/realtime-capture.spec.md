# S-011: Realtime Session Capture — hooks over transcripts

**Version:** 0.1.0  
**Status:** draft  
**Validación:** manual + integration tests  
**Origen:** Sesión de debugging sdd-mem auto-summary (2026-04-13). El approach de parsear transcripts JSONL internos de Claude Code es frágil: format changes, SIGPIPE, reverse-engineering del project dir naming.

---

## Objetivo

Reemplazar el sistema de session digest basado en transcript parsing (S-010 B-003) con captura en tiempo real via hooks de Claude Code. Los hooks (`PostToolUse`, `SubagentStop`, `UserPromptSubmit`) appendean observaciones a un buffer en la KB durante la sesión. `SessionEnd` lee ese buffer y genera el auto-summary con haiku. Eliminar toda dependencia de los archivos JSONL internos de Claude Code.

## Principio de diseño

> **Capture at source, summarize at end**: en vez de reverse-engineerear los internos de Claude Code al final, capturar los datos que nos interesan cuando ocurren via la API pública de hooks. El storage es la KB existente — sin SQLite adicional, sin archivos temporales.

---

## Alcance

### Incluido

| ID | Item | Descripción |
|----|------|-------------|
| B-001 | Hook: PostToolUse capture | Append observación por cada tool use significativo (Bash, Write, Edit) a una entry de KB temporal |
| B-002 | Hook: UserPromptSubmit capture | Append el prompt del usuario (truncado) a la misma entry de buffer |
| B-003 | Hook: SubagentStop capture | Append resumen del subagent a la entry de buffer |
| B-004 | Hook: SessionEnd summary | Leer la entry de buffer, generar summary con haiku, guardar como session summary, limpiar buffer |
| B-005 | Session ID from stdin | Usar `session_id` del JSON de stdin en vez de derivar el project dir del cwd |
| B-006 | Deprecar session-digest.sh | Eliminar el hook que parsea transcripts JSONL |
| B-007 | Buffer entry lifecycle | La entry de buffer se crea al primer evento y se elimina al generar el summary |

### Excluido

- Captura de Read/Grep/Glob (demasiado ruidoso, no aporta al summary)
- Observer agent per-tool-use (caro, O(n²) context growth — razón por la que descartamos claude-mem)
- Cambios al binario Go de cvm (todo en shell hooks)
- Cambios al context-inject (B-002 de S-010, sigue funcionando igual)

---

## Contratos

### B-001: PostToolUse Capture

```
Trigger: PostToolUse hook
Input (stdin): JSON con session_id, tool_name, tool_input, tool_response
Filter: MUST capturar solo Bash, Write, Edit, NotebookEdit
         MUST NOT capturar Read, Grep, Glob, Agent (ruido)
         All tools NOT in the allowlist (Bash, Write, Edit, NotebookEdit)
         MUST be silently ignored with exit 0.

Output: append a KB entry "session-buffer-<session_id>" con formato:
  [HH:MM] <tool_name>: <resumen de 1 línea, max 120 chars>

Resumen por tool:
  - Bash: el comando (truncado a 120 chars)
  - Write: "wrote <file_path>"
  - Edit: "edited <file_path>"
  - NotebookEdit: "edited notebook <file_path>"

Latencia: MUST completar en < 500ms (append, no rewrite)
```

### B-002: UserPromptSubmit Capture

```
Trigger: UserPromptSubmit hook (extender learning-decorator.sh existente)
Input (stdin): JSON con session_id, user_prompt (o content)
         Nota: el prompt puede venir como string o array de content blocks

Composición con learning-decorator.sh:
  El script existente NO lee stdin. La nueva lógica debe:
  (a) Leer stdin una sola vez al inicio: INPUT=$(cat)
  (b) Extraer session_id y user_prompt de INPUT
  (c) Appendear al buffer de KB
  (d) LUEGO ejecutar el protocolo de learning injection existente (que escribe a stdout)
  Ambos comportamientos coexisten en el mismo script. El orden importa: la
  inyección de learning escribe a stdout para que Claude Code la consuma.

Output: append a KB entry "session-buffer-<session_id>" con formato:
  [HH:MM] USER: <prompt truncado a 200 chars>

MUST filtrar prompts que son solo system-reminder (empiezan con <system)
MUST ser el primer append de la sesión (crea la entry si no existe)
```

### B-003: SubagentStop Capture

```
Trigger: SubagentStop hook (extender subagent-stop.sh existente)
Input (stdin): JSON con session_id, agent_type, last_assistant_message

Composición con subagent-stop.sh:
  El script existente ya lee stdin con INPUT=$(cat). La nueva lógica de
  captura comparte esa misma lectura. Extraer session_id del mismo INPUT
  variable. No releer stdin (ya fue consumido). El orden es:
  (a) INPUT=$(cat) — leer stdin una sola vez
  (b) Extraer session_id, agent_type, last_assistant_message de INPUT
  (c) Appendear al buffer de KB (nueva lógica)
  (d) Continuar con el comportamiento existente del hook

Output: append a KB entry "session-buffer-<session_id>" con formato:
  [HH:MM] AGENT(<agent_type>): <last_assistant_message truncado a 200 chars>
```

### B-004: SessionEnd Summary

```
Trigger: SessionEnd hook (reemplaza session-digest.sh + auto-summary.sh)
Input (stdin): JSON con session_id

Flujo:
1. Leer entry "session-buffer-<session_id>" de KB
2. Si no existe o tiene < 3 líneas → skip (sesión trivial)
3. Llamar claude -p --model haiku con el buffer como contexto
4. Parsear respuesta JSON: {request, accomplished, discovered, next_steps}
5. Guardar como "session-<YYYYMMDD-HHMMSS>" con tag "session,summary"
6. Eliminar entry "session-buffer-<session_id>" de KB

Prompt template:
"""
Summarize this coding session from the captured events.
Generate JSON: {"request": "...", "accomplished": "...", "discovered": "...", "next_steps": "..."}
Max 1-2 sentences per field. Output ONLY the JSON.

<events>
{buffer_content}
</events>
"""

Si claude -p falla: log warning, eliminar buffer (datos se pierden, aceptable —
el on-the-fly learning ya capturó los learnings importantes durante la sesión).
```

### B-005: Session ID from stdin

```
Claude Code pasa session_id en el JSON de stdin para TODOS los tipos de hook
(PostToolUse, UserPromptSubmit, SubagentStop, SessionEnd). Esto es parte de la
API pública confirmada de Claude Code.

Usar este ID como key para el buffer en vez de derivar el project dir.

Extracción:
  session_id=$(echo "$HOOK_INPUT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('session_id',''))")

Si session_id está vacío: log warning a stderr, exit 0.
No crear buffer con key inválida.
```

### B-007: Buffer Entry Lifecycle

```
Key format: "session-buffer-<session_id>"
Tags: ["session-buffer"]
Scope: local (--local), porque es específico del proyecto

Lifecycle:
  1. Creado: primer UserPromptSubmit de la sesión
  2. Updated: cada PostToolUse, SubagentStop, UserPromptSubmit posterior (append)
  3. Eliminado: SessionEnd después de generar summary
  4. Orphaned: si la sesión crashea, el buffer queda. Cleanup en próximo SessionStart.

Append strategy:
  - Leer body actual con cvm kb show
  - `cvm kb show` retorna frontmatter + body. Para extraer solo el body:
      cvm kb show <key> | sed '1,/^$/d'
    (saltea todo hasta la primera línea en blanco después del frontmatter).
    Alternativamente, leer el archivo directamente desde
    ~/.cvm/local/kb/<project>/entries/<key>.md y stripear el YAML frontmatter.
  - Appendear nueva línea
  - Rewrite con cvm kb put (overwrite)
  - Cap: max 100 líneas en el buffer. Si se excede, dropear las más viejas.

Nota sobre race conditions:
  Claude Code ejecuta hooks secuencialmente dentro de una sesión — la ejecución
  concurrente de hooks no ocurre. El ciclo read-modify-write es seguro bajo esta
  garantía. Si Claude Code cambiara este comportamiento en el futuro, se
  necesitaría un mecanismo de file-lock o append atómico.
```

---

## Behaviors (Given/When/Then)

### B-001: Tool capture

```
GIVEN a session is active with sdd-mem profile
AND the user runs a Bash command "npm test"
WHEN the PostToolUse hook fires
THEN the KB entry "session-buffer-<session_id>" MUST contain a line:
  [14:32] Bash: npm test
AND the hook MUST complete in < 500ms
```

### B-001-neg: Filtered tools

```
GIVEN a session is active
AND Claude reads a file with the Read tool
WHEN the PostToolUse hook fires
THEN NO line MUST be appended to the buffer
AND the hook MUST exit immediately (< 50ms)
```

### B-002: User prompt capture

```
GIVEN a session starts
AND the user types "che, arreglame el bug del login"
WHEN the UserPromptSubmit hook fires
THEN a KB entry "session-buffer-<session_id>" MUST be created (--local)
AND it MUST contain:
  [14:30] USER: che, arreglame el bug del login
```

### B-003: Subagent capture

```
GIVEN a session is active
AND a subagent of type "Explore" completes with message "Found 3 issues in auth module"
WHEN the SubagentStop hook fires
THEN the buffer MUST contain: [HH:MM] AGENT(Explore): Found 3 issues in auth module
```

### B-004: Summary generation

```
GIVEN a session ends
AND the buffer has 25 lines of captured events
WHEN the SessionEnd hook fires
THEN claude -p MUST be called with the buffer content
AND a KB entry "session-<YYYYMMDD-HHMMSS>" MUST be created with tag "session,summary"
AND the buffer entry "session-buffer-<session_id>" MUST be deleted
```

### E-001: Short session skip

```
GIVEN a session ends
AND the buffer has < 3 lines (or doesn't exist)
WHEN the SessionEnd hook fires
THEN NO LLM call MUST be made
AND the buffer MUST be cleaned up silently
```

### E-002: Orphaned buffer cleanup

```
GIVEN a new session starts
AND there are KB entries with tag "session-buffer" from previous sessions
WHEN the SessionStart hook fires (context-inject.sh, already a SessionStart hook)
THEN orphaned buffers MUST be logged as warning
AND entries with tag "session-buffer" older than 24h MUST be deleted

context-inject.sh adds a step at the top:
  1. Scan KB local for entries with tag "session-buffer"
  2. For each entry: if created_at < now - 24h → cvm kb rm <key> --local
  3. Log deleted entries as warning to stderr
  Then continue with existing context injection behavior.
```

### E-003: claude -p failure

```
GIVEN a session ends
AND the buffer exists
AND claude -p fails (timeout, not available, error)
WHEN the SessionEnd hook fires
THEN the buffer MUST be deleted (datos se pierden, aceptable — el on-the-fly
     learning ya capturó los learnings importantes durante la sesión)
AND a warning MUST be logged to stderr
AND session end MUST NOT be blocked
```

---

## Invariantes

| ID | Invariante |
|----|-----------|
| I-001 | PostToolUse hook MUST complete in < 500ms (no LLM, no network) |
| I-002 | Capture hooks MUST complete in < 500ms (same as I-001). This applies to all of B-001, B-002, B-003. |
| I-003 | Auto-summary MUST NOT block session end |
| I-004 | Buffer MUST NOT grow beyond 100 lines (~2500 tokens) |
| I-005 | All capture hooks MUST be safe to re-run (duplicate lines in buffer are acceptable and do not affect summary quality) |
| I-006 | session_id from stdin MUST be used as buffer key (no project dir derivation) |
| I-007 | Auto-summary cost MUST be < $0.01 per session |
| I-008 | Buffer storage MUST use existing cvm KB (no temp files, no SQLite) |

---

## Errores

| Condición | Comportamiento |
|-----------|---------------|
| stdin JSON malformed | Log warning, exit 0 |
| session_id missing | Log warning, exit 0 (no crear buffer con key inválida) |
| cvm kb put fails | Log warning, continue |
| cvm kb show fails (buffer read) | Treat as empty buffer, skip summary |
| claude -p not available | Log warning, eliminar buffer |
| claude -p returns invalid JSON | Store raw text as summary |
| Buffer exceeds 100 lines | Drop oldest lines on append |

---

## Migración desde S-010

| S-010 Component | Action |
|----------------|--------|
| session-digest.sh (B-003) | **DELETE** — replaced by B-001/B-002/B-003 realtime capture |
| auto-summary.sh (B-004) | **REPLACE** — new version reads KB buffer instead of digest file |
| context-inject.sh (B-002) | **MODIFY** — add orphan buffer cleanup step at top (E-002) |
| learning-decorator.sh | **EXTEND** — add UserPromptSubmit capture (B-002) |
| subagent-stop.sh | **EXTEND** — add SubagentStop capture (B-003) |

**settings.json migration:**
- `CVM_AUTOSUMMARY_MIN_TOOLS` env var becomes unused. Remove from settings.json.

**Reconciliación con S-010 B-017:**
Esta spec supercede S-010 B-017 (Semi-auto observation capture). B-017 describía
el mismo approach como un ítem futuro de Fase 3; S-011 lo implementa como
reemplazo de S-010 B-003/B-004.

---

## Estructura de archivos

```
profiles/sdd-mem/hooks/
├── context-inject.sh          # MODIFIED: add orphan buffer cleanup (E-002)
├── learning-decorator.sh      # MODIFIED: add prompt capture
├── subagent-stop.sh           # MODIFIED: add agent capture  
├── tool-capture.sh            # NEW: PostToolUse observation capture
├── session-summary.sh         # NEW: replaces session-digest.sh + auto-summary.sh
├── session-digest.sh          # DELETED
├── auto-summary.sh            # DELETED
├── (other sdd hooks)          # UNCHANGED
```

---

## Plan de implementación

### Wave 1 — Buffer infrastructure + tool capture
1. Crear `tool-capture.sh` (B-001) — PostToolUse hook que appenda a KB
2. Agregar PostToolUse hook entry en settings.json
3. Test: hacer sesión, verificar que `session-buffer-*` se crea en KB local

### Wave 2 — User prompt + subagent capture
1. Extender `learning-decorator.sh` para appendear prompt al buffer (B-002)
2. Extender `subagent-stop.sh` para appendear agent summary al buffer (B-003)
3. Test: verificar que el buffer tiene líneas USER y AGENT

### Wave 3 — SessionEnd summary + cleanup
1. Crear `session-summary.sh` (B-004) — lee buffer, llama haiku, guarda summary, limpia buffer
2. Agregar orphan cleanup en SessionStart (E-002)
3. Reemplazar session-digest.sh + auto-summary.sh en settings.json
4. Test end-to-end: sesión completa → summary en KB → buffer eliminado

### Wave 4 — Cleanup + commit
1. Eliminar session-digest.sh y auto-summary.sh del profile
2. Actualizar spec S-010 con referencia a S-011
3. Actualizar REGISTRY.md

---

## Estimación de costos

| Componente | Tokens | Costo |
|-----------|--------|-------|
| PostToolUse capture | 0 | $0 (shell append) |
| UserPromptSubmit capture | 0 | $0 (shell append) |
| SubagentStop capture | 0 | $0 (shell append) |
| SessionEnd summary (Haiku) | ~600 in + ~300 out | ~$0.0005 |
| **Total por sesión** | | **< $0.001** |

vs approach anterior: mismo costo, pero sin fragilidad de transcript parsing.

---

## Decisiones de diseño

1. **KB como buffer, no temp files**: usar `cvm kb put --local` para el buffer evita problemas de naming, permisos, y cleanup. La KB ya tiene el tooling de read/write/delete.

2. **Append via read+rewrite**: `cvm kb` no tiene un append nativo. El hook hace `show` → append → `put`. Para < 100 líneas esto es < 100ms.

3. **Solo Bash/Write/Edit**: capturar Read/Grep/Glob generaría ruido sin valor para el summary. Las tool uses que importan son las que cambian estado.

4. **session_id from stdin**: Claude Code pasa el session_id en el JSON de stdin de cada hook. Usar esto elimina toda la lógica de derivar project dir del cwd.

5. **Buffer en KB local (--local)**: el buffer es específico del proyecto. Usar `--local` para no contaminar la KB global con datos transitorios.

6. **Orphan cleanup lazy**: si una sesión crashea, el buffer queda en KB local. Se limpia en el próximo SessionStart (si tiene > 24h). No es urgente.

# S-010: Profile `sdd-mem` — SDD con memoria persistente mejorada

**Version:** 0.1.0  
**Status:** draft  
**Validación:** manual + integration tests  
**Origen:** Análisis comparativo cvm vs claude-mem (abril 2026)

---

## Objetivo

Crear un nuevo profile `sdd-mem` que extiende `sdd` con un sistema de memoria persistente mejorado, inspirado en las mejores ideas de [claude-mem](https://github.com/thedotmack/claude-mem) pero sin su overhead de infraestructura (sin daemon HTTP, sin ChromaDB, sin subprocess observer).

El profile mantiene el workflow SDD completo y agrega:
- Context injection automático al inicio de sesión
- Session summaries automáticos al cierre (via Haiku, ~$0.001/sesión)
- Búsqueda mejorada con progressive disclosure
- Token budget awareness en la KB

## Principio de diseño

> **Batch over streaming**: claude-mem gasta ~$0.03/sesión observando cada tool use en tiempo real. sdd-mem extrae el mismo valor por ~$0.001 procesando el transcript completo al final. 37x más barato, calidad "good" vs "best".

---

## Alcance

### Incluido

#### Phase 1: Quick Wins (profile + hooks + CLAUDE.md)
Nuevos archivos del profile, sin cambios al binario Go.

| ID | Item | Descripción |
|----|------|-------------|
| B-001 | Profile `sdd-mem` | Copia de `sdd` con las mejoras de memoria |
| B-002 | Hook: context injection | `SessionStart` inyecta compact index de KB entries recientes |
| B-003 | Hook: session digest | `SessionEnd` extrae smart digest del transcript JSONL |
| B-004 | Hook: auto session summary | `SessionEnd` post-digest: llama `claude -p --model haiku` para generar summary estructurado y lo guarda con `cvm kb put` |
| B-005 | CLAUDE.md: progressive disclosure | Instruir a Claude a usar search → show (2-step) en vez de leer todo |
| B-006 | Rule: token-awareness | No inyectar más de N tokens de KB context (configurable) |
| B-007 | Skill: `/session-summary` | Trigger manual del auto-summary (para sesiones donde el hook no corrió) |

#### Phase 2: KB CLI improvements (cambios al binario Go)
Mejoras al comando `cvm kb` que benefician a todos los profiles.

| ID | Item | Descripción |
|----|------|-------------|
| B-008 | `cvm kb put --type` | Flag `--type` con enum: decision, learning, gotcha, discovery, session. Persiste como tag `type:<valor>` |
| B-009 | `cvm kb search` ranking | Ordenar resultados: exact key match > key contains > body contains. Flag `--sort recent\|relevance` |
| B-010 | `cvm kb search` filters | Flags `--tag`, `--since`, `--type` para filtrar resultados |
| B-011 | `cvm kb timeline` | Nuevo subcommand: entries ordenadas cronológicamente, agrupadas por día. Flag `--days N` |
| B-012 | `cvm kb stats` tokens | Mostrar estimated tokens por entry (chars/4) y total. Warning si > 50K tokens |
| B-013 | Content-hash dedup | En `Put()`, calcular SHA256 del body. Si entry con mismo hash existe (distinto key), warn. Si mismo key y body idéntico, skip write |
| B-014 | `cvm kb compact` | Generar compact index: key, tags, first line, updated. Para context injection |

#### Phase 3: Mejoras estratégicas
Incluido en el roadmap como Wave 5. Se implementa después de Phase 1 y 2.

| ID | Item | Notas |
|----|------|-------|
| B-015 | SQLite + FTS5 backend | Reemplazar flat files con SQLite + FTS5. Usar `modernc.org/sqlite` (pure Go, no CGO). Mantener flat files como fallback de migración |
| B-016 | MCP search tool | Exponer `kb_search(query, tags, type, limit)` y `kb_get(key)` como MCP tools. Go MCP server registrado en .claude.json |
| B-017 | Semi-auto observation capture | PostToolUse hook appendea tool events a session log. SessionEnd batch-summariza con Haiku. Threshold: solo Bash, Write, Edit (skip Read/Grep/Glob) |

### Excluido (explícitamente NO hacer)
- ChromaDB / vector search (over-engineered para <10K entries)
- Worker daemon HTTP (cvm es CLI, no necesita proceso persistente)
- Observer agent per-tool (caro, O(n²) context growth)
- Web viewer UI

---

## Contratos

### B-002: Context Injection Hook

```
Trigger: SessionStart (matcher: startup|clear|compact)
Input: nada (lee KB via cvm)
Output: stdout con bloque <cvm-context> inyectado en el contexto de Claude

Formato:
<cvm-context>
## Recent KB (last N entries, ~Xt estimated)
| Key | Type | Updated | Summary |
|-----|------|---------|---------|
| session-20260412 | session | 2h ago | Análisis claude-mem vs cvm... |
| gotcha-sqlite-fts | gotcha | 3d ago | FTS5 needs rebuild on... |
</cvm-context>

Configuración:
- CVM_CONTEXT_ENTRY_COUNT: max entries (default: 10)
- CVM_CONTEXT_MAX_TOKENS: budget cap (default: 2000)
- Ordenado por LastReferenced desc, luego UpdatedAt desc
```

### B-003: Session Digest Extraction

```
Trigger: SessionEnd hook, ANTES del auto-summary
Input: transcript JSONL (~/.claude/projects/<project>/<session>.jsonl)
Output: /tmp/cvm-session-digest-<pid>.txt

Extrae (todo en shell, 0 tokens):
1. User prompts (truncados a 300 chars cada uno, max 10)
2. Assistant text blocks (truncados a 200 chars, max 15) ← el "por qué"
3. Tools usados (conteo por tipo)
4. Files modified (lista, max 20)
5. Files read (lista, max 15)
6. Duración de la sesión

Threshold: si < 3 tool uses → no generar digest (sesión trivial)
Estimated output: ~500-1500 tokens
```

### B-004: Auto Session Summary

```
Trigger: SessionEnd hook, DESPUÉS del digest
Input: /tmp/cvm-session-digest-<pid>.txt
Output: cvm kb put "session-<YYYYMMDD-HHMMSS>" --tag "session,summary"

Flujo:
1. Verificar que el digest existe y tiene > 100 chars
2. claude -p --model haiku --max-tokens 500 < prompt_con_digest
3. Parsear respuesta (JSON con campos: request, accomplished, discovered, next_steps)
4. cvm kb put con el summary estructurado
5. Cleanup: rm /tmp/cvm-session-digest-<pid>.txt

Prompt template:
"""
Given this session digest, generate a JSON summary:
{"request": "...", "accomplished": "...", "discovered": "...", "next_steps": "..."}
Be concise. Max 1-2 sentences per field. Output ONLY the JSON.

<digest>
{digest_content}
</digest>
"""

Configuración:
- CVM_AUTOSUMMARY_ENABLED: true|false (default: true)
- CVM_AUTOSUMMARY_MODEL: model to use (default: haiku)
- CVM_AUTOSUMMARY_MIN_TOOLS: threshold (default: 3)
- CVM_AUTOSUMMARY_MAX_TOKENS: cap on response (default: 500)

Costo estimado: ~$0.0008 por sesión con Haiku
```

### B-013: Content-Hash Dedup

```go
// In kb.Put(), before writing:
hash := sha256(body)[:16]
for _, existing := range idx.Entries {
    existingBody := readBody(scope, projectPath, existing.Key)
    existingHash := sha256(existingBody)[:16]
    if existingHash == hash && existing.Key != key {
        // Warn: duplicate content found
        fmt.Fprintf(os.Stderr, "warning: duplicate content (matches %q)\n", existing.Key)
    }
    if existingHash == hash && existing.Key == key {
        // Skip: identical content, no write needed
        // Still update tags if different
    }
}
```

---

## Behaviors (Given/When/Then)

### B-002: Context Injection

```
GIVEN a session starts with the sdd-mem profile
AND the KB has 15 entries (8 global, 7 local)
WHEN cvm lifecycle start runs
THEN stdout MUST contain a <cvm-context> block
AND it MUST show at most CVM_CONTEXT_ENTRY_COUNT entries (default 10)
AND entries MUST be sorted by LastReferenced desc
AND the total estimated tokens MUST NOT exceed CVM_CONTEXT_MAX_TOKENS
AND each entry shows: key, type tag, relative time, first line of body
```

### B-003: Session Digest

```
GIVEN a session ends
AND the transcript JSONL exists at the expected path
AND the session had >= CVM_AUTOSUMMARY_MIN_TOOLS tool uses
WHEN the SessionEnd hook fires
THEN a digest file MUST be created at /tmp/cvm-session-digest-<pid>.txt
AND it MUST contain: user prompts, assistant reasoning, tool counts, files
AND each user prompt MUST be truncated to 300 chars
AND each assistant text MUST be truncated to 200 chars
AND total digest size MUST be < 6000 chars (~1500 tokens)
```

### B-004: Auto Summary

```
GIVEN a session digest exists
AND CVM_AUTOSUMMARY_ENABLED is true
WHEN the SessionEnd hook fires (after digest extraction)
THEN claude -p MUST be called with --model $CVM_AUTOSUMMARY_MODEL
AND --max-tokens $CVM_AUTOSUMMARY_MAX_TOKENS
AND the response MUST be parsed as JSON
AND the result MUST be stored via cvm kb put with tag "session,summary"
AND the digest temp file MUST be cleaned up
AND if claude -p fails, the error MUST be logged but NOT block session end
```

### E-001: Trivial session skip

```
GIVEN a session ends
AND the session had < CVM_AUTOSUMMARY_MIN_TOOLS tool uses
WHEN the SessionEnd hook fires
THEN NO digest MUST be generated
AND NO LLM call MUST be made
AND a debug log "session too short, skipping auto-summary" MUST be printed
```

### E-002: Missing transcript

```
GIVEN a session ends
AND the transcript JSONL does NOT exist at the expected path
WHEN the SessionEnd hook fires
THEN NO digest MUST be generated
AND a warning "transcript not found, skipping auto-summary" MUST be printed
```

### E-003: Token budget overflow

```
GIVEN the KB has 50 entries
AND CVM_CONTEXT_MAX_TOKENS is 2000
WHEN context injection runs
THEN entries MUST be included in LastReferenced order
AND inclusion MUST stop when cumulative tokens reach the budget
AND remaining entries MUST be omitted silently
AND the header MUST show "showing N of M entries (~Xt)"
```

---

## Invariantes

| ID | Invariante |
|----|-----------|
| I-001 | Auto-summary MUST NOT block session end. Si falla, log + continue |
| I-002 | Session digest extraction MUST NOT call any LLM (pure shell) |
| I-003 | Context injection MUST complete in < 2 seconds |
| I-004 | Token estimates use chars/4 heuristic consistently |
| I-005 | All new hooks MUST be idempotent (safe to re-run) |
| I-006 | Profile `sdd-mem` MUST be a superset of `sdd` (no removals) |
| I-007 | Auto-summary cost MUST be < $0.01 per session with default settings |

---

## Estructura del profile

```
profiles/sdd-mem/
├── .claude.json           # Copied from sdd + additions
├── CLAUDE.md              # sdd base + memory sections
├── settings.json          # sdd base + memory env vars
├── statusline-command.sh  # Copied from sdd
├── agents/                # Copied from sdd (no changes)
├── hooks/
│   ├── (all sdd hooks)
│   ├── context-inject.sh      # NEW: B-002
│   ├── session-digest.sh      # NEW: B-003
│   └── auto-summary.sh        # NEW: B-004
├── rules/
│   ├── (all sdd rules)
│   └── token-awareness.md     # NEW: B-006
└── skills/
    ├── (all sdd skills)
    └── session-summary/       # NEW: B-007
```

---

## Plan de implementación

### Wave 1: Profile scaffold + context injection
1. Copiar `profiles/sdd/` → `profiles/sdd-mem/`
2. Implementar `context-inject.sh` (B-002)
3. Agregar al `settings.json` el hook SessionStart que llama context-inject
4. Agregar env vars `CVM_CONTEXT_*` a settings.json
5. Actualizar CLAUDE.md con sección de memory mejorada
6. Test manual: `cvm use sdd-mem` → verificar que context aparece

### Wave 2: Session digest + auto summary
1. Implementar `session-digest.sh` (B-003)
2. Implementar `auto-summary.sh` (B-004)
3. Agregar hooks a SessionEnd en settings.json (después de lifecycle end)
4. Agregar env vars `CVM_AUTOSUMMARY_*`
5. Agregar rule `token-awareness.md` (B-006)
6. Test manual: hacer sesión corta → verificar skip; sesión larga → verificar summary en KB

### Wave 3: KB CLI improvements (Go)
1. `--type` flag en `cvm kb put` (B-008)
2. Search ranking + filters (B-009, B-010)
3. `cvm kb timeline` subcommand (B-011)
4. Token stats en `cvm kb stats` (B-012)
5. Content-hash dedup en `Put()` (B-013)
6. `cvm kb compact` subcommand (B-014)
7. Tests unitarios para cada cambio

### Wave 4: Polish
1. Skill `/session-summary` (B-007)
2. Actualizar README con sdd-mem profile
3. Progressive disclosure en CLAUDE.md (B-005)
4. E2E test del flujo completo

---

## Estimación de costos (token budget por sesión)

| Componente | Tokens | Costo |
|-----------|--------|-------|
| Context injection (10 entries) | ~800 input | $0 (local) |
| Session digest extraction | 0 | $0 (shell) |
| Auto-summary (Haiku) | ~800 in + ~500 out | ~$0.0008 |
| **Total por sesión** | | **< $0.001** |
| **100 sesiones/día** | | **< $0.10** |

vs claude-mem: ~$0.03/sesión (37x más caro)

---

## Decisiones de diseño

1. **Shell para digest, no Go**: el digest se extrae con un shell script que parsea JSONL con python3/jq. Esto permite iterar rápido sin recompilar cvm. Si escala, se migra a Go.

2. **claude -p, no API directa**: usar `claude -p --model haiku` en vez de llamar la API de Anthropic directamente. Así no necesitamos API key en cvm — usa la auth del usuario.

3. **Haiku default, configurable**: el auto-summary usa Haiku por defecto pero `CVM_AUTOSUMMARY_MODEL` permite cambiarlo. Si el usuario quiere más calidad, pone sonnet.

4. **Threshold de 3 tool uses**: sesiones con < 3 tool uses son triviales (una pregunta rápida, un git status). No vale la pena gastar ni $0.001 en summarizarlas.

5. **Progressive disclosure via CLAUDE.md, no MCP**: en vez de implementar MCP tools (medium effort), instruir a Claude via CLAUDE.md a usar `cvm kb search` → `cvm kb show` (2-step). Mismo resultado, zero infra.

6. **Superset de sdd**: sdd-mem hereda TODO de sdd. El usuario puede cambiar entre profiles sin perder funcionalidad. sdd-mem agrega, nunca quita.

---

## Referencias

- Análisis completo: `/tmp/cvm-vs-claude-mem-report.html`
- claude-mem repo: https://github.com/thedotmack/claude-mem
- claude-mem session summaries: `src/services/worker/SDKAgent.ts`, `src/sdk/prompts.ts`
- claude-mem cost controls: `src/shared/SettingsDefaultsManager.ts`, `src/services/worker/http/routes/SessionRoutes.ts`
- cvm KB source: `internal/kb/kb.go`, `cmd/kb.go`

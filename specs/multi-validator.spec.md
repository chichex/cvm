# S-012: Multi-Validator — Gemini como tercer validador

**Version:** 0.2.0  
**Status:** draft  
**Validación:** manual  
**Origen:** Sesión 2026-04-13. Gemini CLI disponible en el entorno, usarlo como validador adicional junto con Opus y Codex.

---

## Objetivo

Extender el sistema de verificación SDD para detectar y usar Gemini CLI como tercer validador independiente. Actualmente el workflow usa Opus (agente interno) + Codex (externo). Con Gemini se agrega un tercer punto de vista de un vendor distinto (Google), aumentando la confianza en la verificación.

El cambio afecta:
1. **Tool detection** (`check-tools.sh`): detectar `gemini` y verificar que funciona
2. **Spec validation** (`/spec` skill): Gemini valida la spec junto con Codex
3. **Dual → Triple verify** (`/verify` skill): Gemini como tercer validador independiente
4. **Fix diagnosis** (`/fix` skill): Gemini como opinión adicional

---

## Alcance

### Incluido

| ID | Item | Descripción |
|----|------|-------------|
| B-001 | Tool detection: Gemini | `check-tools.sh` detecta `gemini` CLI, verifica con health check |
| B-002 | Lifecycle report | `cvm lifecycle start` reporta Gemini en la lista de tools disponibles |
| B-003 | Spec validation con Gemini | `/spec` usa Gemini para validar specs (además de Codex si disponible) |
| B-004 | Triple verify | `/verify` corre Opus + Codex + Gemini en paralelo cuando los 3 están disponibles |
| B-005 | Fix second opinion | `/fix` puede usar Gemini como opinión adicional |
| B-006 | Consensus matrix update | Actualizar la matriz de consenso para 3 validadores |

### Excluido

- Usar Gemini para implementación (solo validación/review)
- Gemini como reemplazo de Opus o Codex (es adicional)

---

## Contratos

### B-001: Tool Detection

```
Archivo: profiles/sdd-mem/hooks/check-tools.sh
Cambio: agregar "gemini" a la lista de tools detectados

Detection:
  1. command -v gemini → path
  2. Health check: gemini -p "reply with exactly: ok" 2>/dev/null | grep -q "ok"
     Success criteria: exit code 0 AND stdout contains "ok"
     If it hangs or returns garbage, it fails.
  3. Si pasa: {"available": true, "path": "...", "verified": true}
  4. Si falla: {"available": false, "path": "...", "verified": false, "reason": "..."}

Nota: A diferencia de codex exec que ejecuta comandos de shell, gemini -p envía
prompts al LLM. El health check verifica que el CLI está autenticado y responde.

Output en available-tools.json:
  "gemini": {"available": true, "path": "/opt/homebrew/bin/gemini", "verified": true}

Permissions: agregar "Bash(gemini *)" al allow list en settings.json, junto al
"Bash(codex *)" existente.
```

### B-002: Lifecycle Report

```
Archivo: internal/lifecycle/lifecycle.go
Cambio: agregar "gemini" a la lista de tools en detectTools()

Cambio exacto (una línea en la línea 207):
  Antes: []string{"claude", "codex", "aider", "gh", "docker", "node", "npm", "go"}
  Después: []string{"claude", "codex", "gemini", "aider", "gh", "docker", "node", "npm", "go"}

Nota: Hay dos sistemas de detección de tools:
  1. check-tools.sh → escribe available-tools.json (usado por skills en runtime)
  2. lifecycle.go detectTools() → para el reporte de inicio de sesión
  Ambos MUST ser actualizados.

Ejemplo output en cvm lifecycle start:
  tools: [gh docker node npm go claude codex gemini]
```

### B-003: Spec Validation con Gemini

```
Skill: /spec (Paso 3: Validación externa)
Cambio: correr Codex y Gemini en paralelo (background bash processes)
        Si solo uno está disponible, correr solo ese.

Invocación:
  gemini -p "Review the spec at specs/<nombre>.spec.md for: 
    1) ambiguity 2) gaps 3) contradictions 4) testability. 
    Be critical. Output a structured review."

Precondición: Gemini CLI tiene acceso al filesystem del directorio de trabajo
  actual (igual que el shell del usuario). Puede leer archivos referenciados por
  path en el prompt. Esta es una precondición para B-003, B-004, y B-005.

Condiciones:
  - Solo si Gemini está disponible (check available-tools.json)
  - Correr en paralelo con Codex usando background bash processes
  - Si falla: log warning, continuar sin Gemini
```

### B-004: Triple Verify

```
Skill: /verify (verificación de conformance)
Cambio: agregar Gemini como tercer validador

Invocación (misma estructura que Codex):
  gemini -p "Verify spec conformance. Read the spec at <path>. 
    Read the implementation files listed in /tmp/cvm-verify-manifest.txt.
    For each requirement (B-XXX, E-XXX, I-XXX), state MATCH, MISMATCH, or GAP.
    Be strict."

Cuando hay PR abierto:
  gemini -p "Verify spec conformance. Read the spec at <path>. 
    Read the diff with: gh pr diff <number>.
    For each requirement..."

Ejecución:
  - Opus: Agent tool (siempre disponible)
  - Codex: codex exec (si disponible)
  - Gemini: gemini -p (si disponible)
  - Correr los 3 en paralelo
```

### B-005: Fix Second Opinion

```
Skill: /fix
Cambio: Gemini como opinión adicional en diagnóstico

Invocación:
  gemini -p "Diagnosticar este bug: <descripcion>. 
    Encontrar root cause con archivos y lineas especificas. 
    NO hacer cambios. Solo diagnosticar."

Condiciones:
  - Solo si Gemini disponible
  - En paralelo con Codex si ambos disponibles

Reconciliación con gate existente:
  - Si Codex y Gemini discrepan en el diagnóstico, presentar ambas opiniones al
    usuario para resolución. El gate "must have one clear hypothesis before
    proceeding" aplica al conjunto combinado de opiniones.
```

### B-006: Consensus Matrix

```
Actualizar la matriz de consenso del /verify skill:

Con 2 validadores (Opus + 1 externo) — sin cambio respecto al verify actual:
  2/2 MATCH              → VERIFIED
  1 MATCH + 1 GAP        → CONCERN
  1 MATCH + 1 MISMATCH   → REVIEW (requiere revisión humana)
  0/2 MATCH (ambos GAP)  → NOT_IMPLEMENTED
  0/2 MATCH (algún MISMATCH) → REVIEW

Con 3 validadores (Opus + Codex + Gemini):
  3/3 MATCH                     → VERIFIED (alta confianza)
  2/3 MATCH + 1 GAP             → VERIFIED (notar el gap)
  2/3 MATCH + 1 MISMATCH        → CONCERN (notar el razonamiento disidente)
  1/3 MATCH                     → REVIEW (requiere revisión humana)
  0/3 MATCH (todos GAP)         → NOT_IMPLEMENTED
  0/3 MATCH (algún MISMATCH)    → REVIEW

Presentación al usuario:
  | Req | Opus | Codex | Gemini | Verdict |
  |-----|------|-------|--------|---------|
  | B-001 | MATCH | MATCH | MATCH | VERIFIED |
  | B-002 | MATCH | MATCH | MISMATCH | CONCERN (Gemini: "missing edge case X") |
  | B-003 | MATCH | GAP | - | CONCERN |
```

---

## Behaviors (Given/When/Then)

### B-001: Gemini detection

```
GIVEN gemini CLI is installed at /opt/homebrew/bin/gemini
AND gemini -p "reply with exactly: ok" 2>/dev/null | grep -q "ok" exits 0
WHEN check-tools.sh runs at SessionStart
THEN available-tools.json MUST contain:
  "gemini": {"available": true, "path": "/opt/homebrew/bin/gemini", "verified": true}
```

### B-001-neg: Gemini not installed

```
GIVEN gemini CLI is not installed
WHEN check-tools.sh runs
THEN available-tools.json MUST contain:
  "gemini": {"available": false}
AND all validation skills MUST skip Gemini gracefully
```

### B-004: Triple verify

```
GIVEN a spec S-011 exists with 8 requirements
AND Opus, Codex, and Gemini are all available
WHEN /verify runs
THEN 3 independent validations MUST execute in parallel
AND the consensus matrix MUST show verdicts from all 3
AND 3/3 MATCH MUST result in VERIFIED
AND 2/3 MATCH MUST result in VERIFIED with dissent note
```

### B-002: Lifecycle tool detection

```
GIVEN gemini is available (in PATH)
WHEN cvm lifecycle start runs
THEN the tools list in the session report MUST include "gemini"
```

### B-003: Parallel spec validation

```
GIVEN a spec exists at specs/<nombre>.spec.md
AND both Codex and Gemini are available
WHEN /spec runs spec validation
THEN both Codex and Gemini opinions MUST be collected (in parallel)
AND both opinions MUST be presented to the user
```

### B-004-partial: Only 2 validators

```
GIVEN Gemini is not available
AND Opus and Codex are available
WHEN /verify runs
THEN the existing dual verify MUST work unchanged
AND the Gemini column MUST show "-" in the matrix
```

### B-005: Fix with Gemini opinion

```
GIVEN a bug is reported
AND Gemini is available
WHEN /fix diagnoses the bug
THEN Gemini's opinion MUST be included in the diagnostic report
```

### B-006: Consensus with dissent

```
GIVEN 3 validators are available
AND a requirement has verdicts: 2 MATCH + 1 MISMATCH
WHEN consensus is computed
THEN the verdict MUST be CONCERN
AND the dissent note MUST include the dissenting validator's reasoning
```

### E-001: Gemini health check failure

```
GIVEN gemini binary exists in PATH
BUT gemini -p "reply with exactly: ok" health check fails (auth, config, etc.)
WHEN check-tools.sh runs
THEN gemini MUST be marked as unavailable with reason
AND validation skills MUST skip Gemini
AND a warning MUST be shown in lifecycle report
```

### E-002: Gemini slow response

```
Las invocaciones de Gemini no usan shell timeout (restricción de macOS — mismo
comportamiento que Codex). Si Gemini se cuelga, el usuario cancela manualmente.
La spec no impone timeout automático.
```

---

## Invariantes

| ID | Invariante |
|----|-----------|
| I-001 | Gemini MUST be optional — all workflows MUST work without it |
| I-002 | Gemini MUST NOT be used for implementation, only validation/review |
| I-003 | Context passing to Gemini MUST NOT pass file contents inline. Short descriptions, bug reports, and command references (e.g. `gh pr diff`) MAY be passed inline. |
| I-004 | Gemini verification MUST run in parallel with other validators, never sequentially |
| I-005 | Health check MUST verify actual execution, not just binary presence |
| I-006 | Gemini failure MUST NOT block any workflow |

---

## Errores

| Condición | Comportamiento |
|-----------|---------------|
| gemini not installed | Skip silently, mark unavailable |
| gemini installed but not configured | Mark unavailable with reason, warn in lifecycle |
| gemini -p hangs | No automated timeout (macOS constraint). Usuario cancela manualmente. |
| gemini returns invalid output | Log warning, exclude from consensus |
| gemini disagrees with Opus+Codex | Show dissent in matrix, don't override consensus |

---

## Plan de implementación

### Wave 1 — Detection
1. Agregar `gemini` a la lista de tools en `check-tools.sh`
2. Agregar health check con `gemini -p "reply with exactly: ok" 2>/dev/null | grep -q "ok"`
3. Agregar `gemini` a `detectTools()` en `internal/lifecycle/lifecycle.go`
4. Agregar `"Bash(gemini *)"` al allow list en `settings.json`
5. Test: `cvm use sdd-mem` → verificar que gemini aparece en tools

### Wave 2 — Spec validation
1. Actualizar skill `/spec` para invocar Gemini en paralelo con Codex
2. Test: crear spec → verificar que Gemini opina

### Wave 3 — Triple verify
1. Actualizar skill `/verify` para correr Opus + Codex + Gemini
2. Actualizar consensus matrix para 3 validadores
3. Test: `/verify` con los 3 → verificar tabla de consenso

### Wave 4 — Fix + cleanup
1. Actualizar skill `/fix` para usar Gemini como opinión adicional
2. Documentar en CLAUDE.md la disponibilidad de Gemini

---

## Changelog

| Version | Fecha | Cambios |
|---------|-------|---------|
| 0.1.0 | 2026-04-13 | Draft inicial |
| 0.2.0 | 2026-04-13 | Fix triple validation review: health check corregido (LLM prompt, no shell cmd); lifecycle.go explicitado como Go change; B-003 paralelismo sin contradicción; consensus matrix con CONCERN preservado; E-002 timeout removido (macOS constraint); I-003 aclarado (inline OK para descripciones cortas); precondición filesystem Gemini añadida; GWTs faltantes añadidos (B-002, B-003, B-005, B-006); fix skill reconciliation añadida a B-005; permissions settings.json añadida a B-001 |

Verificacion multi: agente Opus + Codex + Gemini validan independientemente que la implementacion matchea la spec. Se invoca automaticamente despues de implementar. $ARGUMENTS es el path a la spec.

<!-- Spec: S-012 | Req: B-004, B-006 -->

## Paso 1: Cargar artefactos

Leer:
1. La spec completa
2. Los contratos generados (si existen)
3. Los tests y sus resultados
4. El codigo implementado

Si falta alguno, reportar y preguntar como proceder.

## Paso 2: Lanzar verificacion multi en paralelo

Lanzar los tres validadores al mismo tiempo. Opus via Agent tool, Codex y Gemini via bash en background.

### Verificacion Opus (subagent con model opus, rol verifier)

Lanzar un agente Opus con estas instrucciones:

**TASK**: Verificar que la implementacion matchea la spec punto por punto.
**Para cada requisito (B-XXX, E-XXX, I-XXX):**
1. Encontrar el codigo que lo implementa
2. Verificar que el comportamiento matchea la spec
3. Clasificar: MATCH / MISMATCH / GAP

**Detectar:**
- Over-engineering: codigo sin requisito en spec
- Under-engineering: requisitos sin implementacion
- Drift: comportamiento diferente al especificado

### Verificacion Codex (solo si `codex` esta disponible)

Verificar disponibilidad de Codex: `codex exec "echo ok" 2>/dev/null` (con timeout de 10s).
Si falla o no responde: Codex no disponible para esta sesion.

**Si disponible**, preparar contexto para Codex de forma eficiente (NUNCA pasar contenido inline en el prompt):

**Estrategia de contexto (en orden de preferencia):**

1. **PR abierto**: verificar con `gh pr view --json number 2>/dev/null`
   - Si hay PR: Codex puede leer el diff con `gh pr diff <number>`
2. **Sin PR**: escribir un archivo manifiesto temporal con los paths:
   ```bash
   # Escribir manifiesto en /tmp/cvm-verify-manifest.txt
   echo "SPEC: specs/<nombre>.spec.md" > /tmp/cvm-verify-manifest.txt
   echo "FILES:" >> /tmp/cvm-verify-manifest.txt
   # listar los archivos de implementacion, uno por linea
   ```

**Lanzar Codex en background con referencia a archivos, no contenido inline:**

```bash
# Si hay PR abierto:
codex exec "Verify spec conformance. Read the spec at [path a la spec]. Read the implementation diff with: gh pr diff [number]. For each requirement (B-XXX, E-XXX, I-XXX), state MATCH, MISMATCH (explain), or GAP (not found). Also detect over-engineering (code without spec requirement)." > /tmp/cvm-verify-codex.txt 2>&1 &
CODEX_PID=$!

# Si no hay PR:
codex exec "Verify spec conformance. Read the spec at [path a la spec]. Read the implementation files listed in /tmp/cvm-verify-manifest.txt. For each requirement (B-XXX, E-XXX, I-XXX), state MATCH, MISMATCH (explain), or GAP (not found). Also detect over-engineering (code without spec requirement)." > /tmp/cvm-verify-codex.txt 2>&1 &
CODEX_PID=$!
```

**IMPORTANTE**: Codex tiene acceso al filesystem y a `gh`. NUNCA copiar contenido de archivos en el prompt de Codex — siempre darle paths o comandos para que los lea el mismo.

**Si no disponible**: omitir columna Codex del reporte.

### Verificacion Gemini (solo si `gemini` esta disponible) — Spec: S-012 | Req: B-004

Verificar disponibilidad: leer `~/.cvm/available-tools.json` y verificar `gemini.available == true`.

**Si disponible**, lanzar en background en paralelo con Codex (mismo manifiesto, NUNCA contenido inline):

```bash
# Si hay PR abierto:
gemini -p "Verify spec conformance. Read the spec at [path a la spec]. Read the implementation diff with: gh pr diff [number]. For each requirement (B-XXX, E-XXX, I-XXX), state MATCH, MISMATCH (explain), or GAP (not found). Be strict." > /tmp/cvm-verify-gemini.txt 2>&1 &
GEMINI_PID=$!

# Si no hay PR:
gemini -p "Verify spec conformance. Read the spec at [path a la spec]. Read the implementation files listed in /tmp/cvm-verify-manifest.txt. For each requirement (B-XXX, E-XXX, I-XXX), state MATCH, MISMATCH (explain), or GAP (not found). Be strict." > /tmp/cvm-verify-gemini.txt 2>&1 &
GEMINI_PID=$!
```

Gemini tiene acceso al filesystem. NUNCA copiar contenido de archivos en el prompt.

**Si no disponible**: mostrar "-" en la columna Gemini del reporte.

**Esperar resultados de background processes:**
```bash
[ -n "$CODEX_PID" ] && wait $CODEX_PID
[ -n "$GEMINI_PID" ] && wait $GEMINI_PID
```

**Manejo de fallos (I-006):** Si el output de Codex o Gemini esta vacio, contiene un error,
o no tiene verdicts parseables: loguear warning, excluir ese validador de la matriz de
consenso, y proceder con los validadores restantes. NUNCA bloquear el workflow por un
validador fallido.

## Paso 3: Consolidar resultados

<!-- Spec: S-012 | Req: B-006 -->

**Si hubo triple verification (Opus + Codex + Gemini):**

```
Triple Verification: [spec ID] v[version]

| Requisito | Opus | Codex | Gemini | Veredicto |
|-----------|------|-------|--------|-----------|
| B-001 | MATCH | MATCH | MATCH | VERIFIED |
| B-002 | MATCH | MATCH | MISMATCH | CONCERN (Gemini: "...") |
| B-003 | MATCH | GAP | - | CONCERN |
| E-001 | GAP | GAP | GAP | NOT_IMPLEMENTED |

Matriz de consenso con 3 validadores:
- 3/3 MATCH                  → VERIFIED (alta confianza)
- 2/3 MATCH + 1 GAP          → VERIFIED (notar el gap)
- 2/3 MATCH + 1 MISMATCH     → CONCERN (notar razonamiento disidente)
- 1/3 MATCH                  → REVIEW (requiere revision humana)
- 0/3 MATCH (todos GAP)      → NOT_IMPLEMENTED
- 0/3 MATCH (algun MISMATCH) → REVIEW

Resultado: X/Y requisitos VERIFIED
```

**Si hubo dual verification (Opus + Codex o Opus + Gemini):**

```
Dual Verification: [spec ID] v[version]

| Requisito | Opus | [Codex|Gemini] | Consenso |
|-----------|------|----------------|----------|
| B-001 | MATCH | MATCH | VERIFIED |
| B-002 | MATCH | MISMATCH | REVIEW |
| E-001 | GAP | GAP | NOT_IMPLEMENTED |

Matriz de consenso con 2 validadores:
- 2/2 MATCH              → VERIFIED
- 1 MATCH + 1 GAP        → CONCERN
- 1 MATCH + 1 MISMATCH   → REVIEW (requiere revision humana)
- 0/2 MATCH (ambos GAP)  → NOT_IMPLEMENTED
- 0/2 MATCH (algun MISMATCH) → REVIEW

Resultado: X/Y requisitos VERIFIED
```

**Si fue single verification (solo Opus):**
```
Single Verification: [spec ID] v[version]
(Sin second opinion externa disponible)

| Requisito | Opus | Veredicto |
|-----------|------|-----------|
| B-001 | MATCH | VERIFIED |
| E-001 | GAP | NOT_IMPLEMENTED |

Resultado: X/Y requisitos VERIFIED
```

## Paso 4: Resolver discrepancias

Si hay discrepancias (en dual verify):
1. Analizar cual tiene razon (leer el codigo)
2. Si es un false positive: documentar
3. Si es un issue real: corregir la implementacion
4. Re-verificar los requisitos corregidos

## Paso 5: Reporte final

```
Verificacion completada: [spec ID] v[version]

Requisitos: X/Y VERIFIED
Tests: PASS/FAIL
Lint: PASS/FAIL
Build: PASS/FAIL

Veredicto: VERIFIED / NOT VERIFIED (N issues pendientes)
```

Si VERIFIED: actualizar spec status a "verified" en REGISTRY.md.
Si NOT VERIFIED: listar issues pendientes para el usuario.

## MUST DO
- Verificar CADA requisito — no samplear
- Lanzar Opus, Codex, y Gemini en paralelo (los que esten disponibles)
- Resolver TODAS las discrepancias antes de dar veredicto
- Reportar mismatches con ubicacion exacta
- Incluir columna Gemini en la tabla (mostrar "-" si no disponible)

## MUST NOT DO
- NO aprobar si hay mismatches sin resolver
- NO ignorar el resultado de Codex (si disponible)
- NO ignorar el resultado de Gemini (si disponible)
- NO ignorar el resultado de Opus
- NO corregir la spec — solo reportar (el usuario decide)
- NO bloquear el workflow si Gemini no esta disponible — es opcional (Spec: S-012 | Req: I-001)

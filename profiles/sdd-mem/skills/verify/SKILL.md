Verificacion dual: agente Opus + Codex validan independientemente que la implementacion matchea la spec. Se invoca automaticamente despues de implementar. $ARGUMENTS es el path a la spec.

## Paso 1: Cargar artefactos

Leer:
1. La spec completa
2. Los contratos generados (si existen)
3. Los tests y sus resultados
4. El codigo implementado

Si falta alguno, reportar y preguntar como proceder.

## Paso 2: Lanzar verificacion dual en paralelo

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

**Lanzar Codex con referencia a archivos, no contenido inline:**

```bash
# Si hay PR abierto:
codex exec "Verify spec conformance. Read the spec at [path a la spec]. Read the implementation diff with: gh pr diff [number]. For each requirement (B-XXX, E-XXX, I-XXX), state MATCH, MISMATCH (explain), or GAP (not found). Also detect over-engineering (code without spec requirement)."

# Si no hay PR:
codex exec "Verify spec conformance. Read the spec at [path a la spec]. Read the implementation files listed in /tmp/cvm-verify-manifest.txt. For each requirement (B-XXX, E-XXX, I-XXX), state MATCH, MISMATCH (explain), or GAP (not found). Also detect over-engineering (code without spec requirement)."
```

**IMPORTANTE**: Codex tiene acceso al filesystem y a `gh`. NUNCA copiar contenido de archivos en el prompt de Codex — siempre darle paths o comandos para que los lea el mismo.

**Si no disponible**: la verificacion es solo Opus (single verify). Informar al usuario que no hay second opinion externa.

## Paso 3: Consolidar resultados

**Si hubo dual verification (Opus + Codex):**

```
Dual Verification: [spec ID] v[version]

| Requisito | Opus | Codex | Consenso |
|-----------|------|-------|----------|
| B-001 | MATCH | MATCH | VERIFIED |
| B-002 | MATCH | MISMATCH | REVIEW |
| E-001 | GAP | GAP | NOT IMPLEMENTED |

Consenso:
- VERIFIED: ambos dicen MATCH
- REVIEW: discrepancia entre Opus y Codex — investigar
- NOT IMPLEMENTED: ambos detectan gap
- CONCERN: uno detecta issue, el otro no

Resultado: X/Y requisitos VERIFIED
```

**Si fue single verification (solo Opus):**
```
Single Verification: [spec ID] v[version]
(Codex no disponible — sin second opinion externa)

| Requisito | Opus | Veredicto |
|-----------|------|-----------|
| B-001 | MATCH | VERIFIED |
| E-001 | GAP | NOT IMPLEMENTED |

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
- Lanzar Opus y Codex en paralelo
- Resolver TODAS las discrepancias antes de dar veredicto
- Reportar mismatches con ubicacion exacta

## MUST NOT DO
- NO aprobar si hay mismatches sin resolver
- NO ignorar el resultado de Codex
- NO ignorar el resultado de Opus
- NO corregir la spec — solo reportar (el usuario decide)

Validacion post-implementacion con spec coverage check. Se invoca automaticamente al final de una implementacion o manualmente.

## Paso 1: Verificar tests

Encontrar y correr el comando de test del proyecto.
Reportar resultado: pass/fail con detalle.

## Paso 2: Verificar lint

Encontrar y correr el comando de lint/type-check.
Reportar resultado: pass/fail con detalle.

## Paso 3: Verificar build

Encontrar y correr el comando de build.
Reportar resultado: pass/fail con detalle.

## Paso 4: Slop check

Revisar los archivos modificados buscando:
- Codigo comentado
- console.log / print de debug
- TODOs sin contexto
- Imports no usados
- Variables no usadas

## Paso 5: Spec coverage (nuevo en SDD)

Si hay specs en `specs/`:
1. Leer las specs con status "implemented" que fueron tocadas por el task actual (no auditar todas las specs del proyecto)
2. Para cada spec relevante, verificar:
   - Cada behavior (B-XXX) tiene test?
   - Cada edge case (E-XXX) tiene test o handling?
   - Cada invariante (I-XXX) se enforce en el codigo?
3. Reportar coverage:

```
Spec coverage:
| Spec | Behaviors | Edge Cases | Invariantes | Total |
|------|-----------|------------|-------------|-------|
| S-001 | 5/5 | 3/4 | 2/2 | 91% |

Gaps:
- S-001/E-003: edge case sin test ni handling
```

## Paso 6: Reporte final

```
Quality Gate: [PASS/FAIL]

- Tests: [pass/fail]
- Lint: [pass/fail]
- Build: [pass/fail]
- Slop: [clean/N issues]
- Spec coverage: [X%]

Issues (si los hay):
- [detalle de cada issue]
```

## MUST DO
- Correr TODOS los checks, no samplear
- Si hay specs, verificar spec coverage

## MUST NOT DO
- NO corregir issues — solo reportar
- NO aprobar si hay failures

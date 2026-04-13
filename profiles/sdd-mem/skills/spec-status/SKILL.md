Dashboard del estado de todas las specs del proyecto. Se puede invocar manualmente con /spec-status o automaticamente al inicio de sesion.

## Proceso

1. Buscar `specs/REGISTRY.md` en el proyecto
2. Si no existe: reportar "No hay specs registradas en este proyecto"
3. Si existe, leer y para cada spec verificar:
   - Existe el archivo de spec?
   - Cual es su status?
   - Cual es su version?
   - Cual es su estrategia de validacion?

4. Generar dashboard:

```
SDD Dashboard

| ID | Nombre | Status | Ver | Validacion | Archivo |
|----|--------|--------|-----|------------|---------|
| S-001 | [nombre] | verified | 1.0 | TDD | specs/nombre.spec.md |
| S-002 | [nombre] | implemented | 0.3 | tests-post | specs/nombre.spec.md |
| S-003 | [nombre] | draft | 0.1 | TDD | specs/nombre.spec.md |

Status flow: draft → approved → implemented → verified

Resumen:
- Total specs: N
- Draft: X
- Approved (pendiente impl): Y
- Implemented (pendiente verify): Z
- Verified: W
```

5. Alertas:
   - Specs en "implemented" sin verificar → sugerir verificacion
   - Specs en "draft" → sugerir completar
   - Specs cuyo archivo no existe → marcar como huerfanas

## MUST NOT DO
- NO modificar nada — solo reportar

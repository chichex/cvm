Validacion post-implementacion: CI health check. Se invoca automaticamente al final de una implementacion o manualmente.

Para verificacion de spec conformance, usar `/verify` en vez de este skill.

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

## Paso 5: Reporte final

```
Quality Gate: [PASS/FAIL]

- Tests: [pass/fail]
- Lint: [pass/fail]
- Build: [pass/fail]
- Slop: [clean/N issues]

Issues (si los hay):
- [detalle de cada issue]
```

## MUST DO
- Correr TODOS los checks, no samplear

## MUST NOT DO
- NO corregir issues — solo reportar
- NO aprobar si hay failures
- NO verificar spec coverage — eso lo hace `/verify`

Ejecutar una implementacion derivada de una spec. Se invoca automaticamente despues de /spec o manualmente con un issue/spec. $ARGUMENTS puede ser un numero de issue, path a spec, o vacio.

## Paso 1: Seleccionar la spec

**Si $ARGUMENTS es un numero de issue:**
1. `gh issue view <numero>` para obtener contenido
2. Buscar referencia a spec en el issue
3. Leer la spec referenciada

**Si $ARGUMENTS es un path a spec:**
1. Leer la spec directamente

**Si $ARGUMENTS esta vacio:**
1. Buscar specs con status "approved" en `specs/REGISTRY.md`
2. Si hay una sola: usarla
3. Si hay varias: preguntar al usuario cual ejecutar

## Paso 2: Validar precondiciones

1. La spec debe tener status "approved" (paso por Codex review)
2. La estrategia de validacion debe estar definida
3. Preflight: tests pasan, lint limpio, build limpio, branch limpia
4. Si algo falla: reportar y preguntar

## Paso 3: Setup

1. Crear worktree con `EnterWorktree` para trabajar aislado
2. Crear branch: `git checkout -b feat/[nombre-de-spec]`

## Paso 4: Ejecutar waves

Para cada wave del plan:

1. Anunciar: "Ejecutando Wave N — [nombre]"
2. Implementar las tareas de la wave
3. Si la wave involucra tests (segun estrategia):
   - **TDD**: generar tests → verificar que fallan → implementar → verificar que pasan
   - **Tests post-impl**: implementar → generar tests → verificar que pasan
   - **Existentes**: implementar → correr tests existentes
4. Correr validaciones de la wave (lint, type-check)
5. Si falla: diagnosticar, arreglar, re-validar (max 3 intentos)
6. Confirmar: "Wave N completada — validacion: [resultado]"

NO avanzar si la validacion de la wave actual no pasa.

## Paso 5: Verificacion dual

Invocar internamente `/verify`:

1. **Agente Opus**: verificar cada requisito de la spec contra el codigo
2. **Codex** (solo si esta disponible — verificar con `codex exec "echo ok" 2>/dev/null`, timeout 10s): verificar conformance spec vs implementacion en paralelo con Opus
3. Consolidar: si hay dual verify, resolver discrepancias. Si solo Opus, reportar que no hubo second opinion.
4. Si VERIFIED: continuar
5. Si NOT VERIFIED: corregir issues, re-verificar (max 2 intentos)

## Paso 6: Validacion final

1. Test suite completa
2. Lint + type-check
3. Build
4. Verificar todos los behaviors de la spec implementados
5. Verificar todos los edge cases manejados

Actualizar spec status a "implemented" (o "verified" si la dual verification paso).

## Paso 7: Cerrar el loop

1. Commit(s) con stage de archivos relevantes (NO `git add -A`)
2. Push: `git push -u origin [branch]`
3. PR: `gh pr create` referenciando la spec
4. Mostrar URL del PR

## Paso 8: GitHub Actions (si aplica)

1. Verificar si hay workflows: `gh run list --branch [branch] --limit 1`
2. Si hay runs: esperar con `gh run watch [id] --exit-status`
3. Si falla: diagnosticar, arreglar, re-push (max 2 intentos)

## MUST DO
- Seguir el plan derivado de la spec
- Validar entre waves
- Usar la estrategia de validacion definida en la spec
- Ejecutar verificacion dual (Opus + Codex) antes de cerrar

## MUST NOT DO
- NO ejecutar sin spec aprobada
- NO cambiar la spec sin aprobacion
- NO saltear la verificacion dual
- NO agregar features fuera del scope de la spec
- NO hacer commit sin pasar validacion final

Ejecutar un issue del repo que fue planificado con /spec. $ARGUMENTS puede ser un numero de issue o vacio.

## Paso 1: Seleccionar el issue

**Si $ARGUMENTS esta vacio:**
1. Correr `gh issue list --state open --limit 20` para listar issues abiertos
2. Mostrar la lista al usuario con numero, titulo y labels
3. Preguntar cual quiere ejecutar. NO continuar sin respuesta.

**Si $ARGUMENTS es un numero de issue:**
1. Correr `gh issue view <numero>` para obtener el contenido completo
2. Continuar al Paso 2.

## Paso 2: Validar estructura del issue

Leer el body del issue y verificar que tiene TODAS estas secciones (generadas por /spec):

| Seccion | Que buscar |
|---------|------------|
| `## Contexto` | Descripcion del problema/feature |
| `## Preflight` | Comandos de test, lint, build confirmados + entorno |
| `## Plan de implementacion` | Clasificacion (Chico/Mediano/Grande) + Waves |
| `### Waves de ejecucion` | Al menos Wave 0 y Wave 1 con tareas concretas |
| `## Requisitos` | Must have con checkboxes |
| `## Criterios de aceptacion` | Checkboxes verificables |

**Si falta alguna seccion:**
Reportar exactamente que falta y sugerir correr `/spec` primero. NO continuar.

**Si hay secciones pero estan incompletas:**
- Waves sin tareas concretas o sin archivos especificos -> rechazar
- Preflight sin comandos de validacion -> rechazar
- Requisitos sin checkboxes -> rechazar
- Criterios de aceptacion vagos (ej: "que funcione bien") -> rechazar

Reportar cada problema encontrado. NO continuar hasta que el issue este completo.

## Paso 3: Verificar preflight actual

Los datos del preflight fueron validos cuando se corrio /spec, pero pueden haber cambiado. Re-verificar:

1. Correr los comandos de test del preflight — confirmar que pasan
2. Correr lint/type-check — confirmar baseline limpio
3. Correr build — confirmar que compila
4. Verificar que los servicios necesarios estan levantados
5. Verificar que la branch actual es limpia (`git status`)

Si algo falla, reportar y preguntar al usuario como proceder. NO asumir.

Mostrar resumen:
```
Preflight re-check:
- Tests: [comando] [pass/fail]
- Lint: [comando] [pass/fail]
- Build: [comando] [pass/fail]
- Servicios: [pass/fail]
- Branch limpia: [pass/fail]
```

## Paso 4: Setup

1. **Siempre usar worktree:** Crear un worktree con el tool `EnterWorktree` (buscarlo con `ToolSearch` si no esta cargado) para trabajar en una copia aislada del repo. Esto es obligatorio, no opcional.
2. Crear branch desde main/master: `git checkout -b [nombre-de-branch-del-issue]`
   - Si la branch ya existe, mostrar warning y preguntar si hacer checkout a la existente o crear una nueva con sufijo
3. **Evaluar uso de Teams:**
   - Primero verificar si Claude Teams esta realmente soportado en la sesion actual. No asumir soporte por configuracion del profile o env vars.
   - Si el issue propone Teams (Mediano/Grande), o si hay 2+ waves con tareas independientes entre si, y el soporte real esta confirmado -> **intentar crear el Team**
   - Mostrar al usuario la estructura de Team propuesta antes de crearlo
   - Si el issue NO propone Teams pero la clasificacion es Mediano o Grande y el soporte real esta confirmado -> sugerir proactivamente usar Teams y esperar confirmacion
   - Si el soporte no esta confirmado -> continuar sin Teams, sin insistir

## Paso 5: Ejecutar waves

### Ruta A: Con Teams (preferida para issues Mediano/Grande)

**Intentar crear el Team ANTES de ejecutar waves solo si el soporte real esta confirmado.**
- Si el tool `TeamCreate` esta disponible en la sesion: usarlo con la estructura del issue.
- Si `TeamCreate` no esta disponible: lanzar subagents en paralelo con `Agent(subagent_type: "general-purpose", model: "sonnet")`, uno por wave o area independiente.

Si el Team se crea exitosamente:
1. Asignar tasks a teammates segun el plan del issue
2. Coordinar segun los puntos de sincronizacion definidos en las waves
3. Monitorear progreso y validar entre waves

**Si TeamCreate falla o no esta disponible:**
1. Mostrar el error exacto al usuario
2. Explicar: "No pude crear el Team. Para continuar sin Teams voy a ejecutar las waves secuencialmente, lo cual puede ser mas lento."
3. **Preguntar explicitamente:** "Queres que continue sin Teams (ejecucion secuencial)?"
4. **NO continuar sin respuesta afirmativa del usuario.** Esperar el OK.

### Ruta B: Sin Teams (issues Chicos o con OK explicito del usuario)

**Para cada wave:**
1. Anunciar: "Ejecutando Wave N — [nombre]"
2. Implementar las tareas listadas en la wave
3. Correr las validaciones especificadas para esa wave (tests, lint, type-check)
4. Si la validacion falla: diagnosticar, arreglar, re-validar. Maximo 3 intentos antes de frenar y preguntar.
5. Confirmar: "Wave N completada — validacion: [resultado]"

**Entre waves:** NO avanzar si la validacion de la wave actual no pasa.

## Paso 6: Validacion final

1. Correr test suite completa
2. Correr lint + type-check
3. Correr build
4. Verificar que todos los requisitos "Must have" del issue estan implementados — marcar cada checkbox
5. Verificar cada criterio de aceptacion — marcar cada checkbox
6. Correr `/validate` para revision de calidad

Mostrar resumen final:
```
Ejecucion completada: issue #[N]

Requisitos:
- [x] requisito 1
- [x] requisito 2

Criterios de aceptacion:
- [x] criterio 1
- [x] criterio 2

Validacion:
- Tests: pass
- Lint: pass
- Build: pass
- Review: [resultado]
```

## Paso 7: Cerrar el loop

<!-- Nota: el usuario autoriza commit/push/PR al invocar /execute -->
1. Hacer commit(s) siguiendo convenciones del proyecto (stage solo archivos relevantes, NO usar `git add -A`)
2. Push de la branch al remote: `git push -u origin [nombre-de-branch]`
3. Crear PR automaticamente con `gh pr create` referenciando el issue (`Closes #N`)
   - Titulo: resumen conciso del cambio (< 70 chars)
   - Body: summary con bullets de lo implementado, link al issue, test plan
4. Mostrar el URL del PR al usuario

## Paso 8: Esperar GitHub Actions (si aplica)

1. Verificar si el repo tiene workflows de CI configurados: `gh run list --branch [nombre-de-branch] --limit 1`
2. **Si NO hay runs:** reportar "No se detectaron GitHub Actions para esta branch" y terminar.
3. **Si hay runs en progreso:**
   - Mostrar: "Esperando GitHub Actions... Run: [id] — [workflow name]"
   - Correr `gh run watch [run-id] --exit-status` para esperar a que termine
   - Si hay multiples runs, esperar todos
4. **Si el run pasa:** reportar "GitHub Actions: pass — todos los checks pasaron" y terminar.
5. **Si el run falla:**
   - Mostrar el error: `gh run view [run-id] --log-failed`
   - Diagnosticar la causa del fallo
   - Intentar arreglar (maximo 2 intentos). En cada intento:
     a. Hacer el fix
     b. Correr validacion local (tests, lint, build)
     c. Commit + push
     d. Esperar el nuevo run con `gh run watch`
   - Si despues de 2 intentos sigue fallando: reportar el estado al usuario y preguntar como proceder. NO seguir arreglando sin OK.

## MUST DO
- Seguir el plan del issue al pie de la letra — las decisiones de diseno ya fueron tomadas en /spec
- Validar entre cada wave antes de avanzar
- Usar los comandos exactos del preflight para validar
- Reportar progreso wave por wave
- Si no hay soporte real confirmado para Teams, ejecutar sin Teams sin intentar forzarlo

## MUST NOT DO
- NO ejecutar sin validar la estructura del issue primero
- NO cambiar el plan del issue sin aprobacion explicita del usuario
- NO saltear waves ni reordenarlas
- NO ignorar validaciones que fallan
- NO hacer commit sin haber pasado la validacion final (Paso 6)
- NO agregar features o cambios fuera del scope del issue
- NO inventar tareas que no estan en el plan
- NO asumir soporte de Teams por configuracion experimental del profile

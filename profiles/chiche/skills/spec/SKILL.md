Analizar un pedido o documentos y armar un plan de implementacion completo. $ARGUMENTS puede ser:
- Paths a documentos (PRDs, specs, RFCs, lo que sea) o un glob pattern
- Una descripcion directa de lo que se quiere hacer (sin docs)

## Paso 1: Entender el pedido

**1a. Clasificar el intent:**
- **Refactoring**: cambiar estructura sin cambiar comportamiento
- **Feature nuevo**: componente o feature sin implementacion previa
- **Enhancement**: mejora a funcionalidad existente
- **Bug fix**: algo esta roto y hay que reparar
- **Arquitectura**: decision de diseno o estructura del sistema

Anunciar la clasificacion antes de continuar.

**1b. Ingerir input:**
- Si $ARGUMENTS son paths a docs: leerlos y extraer objetivo, requisitos funcionales/no funcionales, restricciones, criterios de aceptacion. Consolidar en lista unificada sin duplicados.
- Si $ARGUMENTS es una descripcion directa: hacer 2-3 preguntas clarificadoras segun el intent:
  - **Refactoring**: Que comportamiento debe preservarse? Cual es la motivacion?
  - **Feature nuevo**: Criterios de aceptacion? Contrato de UI/API?
  - **Enhancement**: Cual es el limite exacto de scope? Que queda explicitamente afuera?
  - **Bug fix**: Comportamiento esperado vs actual? Se puede reproducir?
  - **Arquitectura**: Restricciones? Que trade-offs importan mas?

## Paso 2: Explorar el codebase
- Identificar los archivos y modulos que serian afectados
- Buscar patrones existentes que se puedan reutilizar
- Detectar posibles conflictos con trabajo en progreso (`git status`, branches activas)
- Revisar el CLAUDE.md del proyecto para convenciones relevantes

## Paso 3: Preflight check
Antes de planificar nada, verificar que se tiene todo lo necesario para validar el trabajo:

**3a. Toolchain**
1. **Tests**: Encontrar como correr los tests (`npm test`, `pytest`, `make test`, lo que sea). Correrlos ahora para confirmar que pasan en el estado actual. Si no pasan, reportar y frenar.
2. **Lint/Type-check**: Identificar el comando de lint y type-check. Correrlo para verificar baseline limpio.
3. **Build**: Identificar como buildear. Verificar que builda.

**3b. Entorno de pruebas**
4. **Ambiente target**: Determinar contra que entorno se valida el feature — local, preprod, staging, o produccion. Buscar en configs, env files, README, docker-compose, etc.
5. **Como pegarle**: Identificar URLs base, API keys, auth necesaria, headers requeridos. Si hay un cliente HTTP o SDK interno, encontrarlo.
6. **Servicios necesarios**: Detectar si necesita servicios corriendo (DB, API, colas, etc.). Verificar si estan levantados o como levantarlos (ej: `start.sh`, `docker-compose up`).
7. **Datos de test**: Identificar si hay seeds, fixtures, factories, o datos de prueba existentes. Saber como crearlos y como limpiarlos despues.
8. **Si es produccion**: Confirmar explicitamente con el usuario. Identificar que datos usar (cuentas de test, feature flags, sandbox mode). Documentar como revertir o limpiar cualquier efecto secundario. Si no hay forma segura de probar en prod, reportar y frenar.
9. **Convenciones de test**: Entender donde van los tests, que framework usan, como se nombran, si hay helpers existentes.

**3c. Soporte de Teams**
10. Verificar si Claude Teams esta realmente soportado en la sesion actual. No asumir soporte por configuracion del profile o env vars.
11. Si no hay evidencia clara de soporte real, marcar Teams como no disponible y planificar sin Teams.

Reportar los hallazgos al usuario:
```
Preflight check:
- Tests: `npm test` (47 passing)
- Lint: `npm run lint`
- Build: `npm run build`
- Ambiente: staging (https://api-staging.example.com)
- Auth: Bearer token via `./scripts/get-token.sh`
- Servicios: docker-compose up (postgres, redis)
- Datos de test: factories en test/helpers/, cleanup via `npm run db:reset`
- Test patterns: jest, archivos en __tests__/
```

Si algo falla, no se encuentra, o el ambiente es produccion sin estrategia segura de testing, preguntar al usuario ANTES de continuar. NO asumir.

## Paso 4: Clasificar tamano
Evaluar la complejidad en base a:
- Cantidad de archivos a tocar
- Cantidad de modulos/layers involucrados (frontend, backend, DB, infra)
- Dependencias entre cambios (se puede paralelizar?)
- Riesgo estimado (toca codigo critico?)

Asignar una categoria:

**Chico** (1 modulo, <10 archivos, bajo riesgo):
- Ejecucion directa, un solo agente
- Sin worktree necesario
- Sin Teams

**Mediano** (2-3 modulos, 10-30 archivos, riesgo moderado):
- Proponer Claude Teams con 2-3 teammates solo si el soporte real esta confirmado; si no, plan secuencial con subagents puntuales
- Sugerir worktree si hay trabajo en progreso en la branch actual

**Grande** (3+ modulos, 30+ archivos, alto riesgo, paralelizable):
- Proponer Claude Teams con 3-5 teammates solo si el soporte real esta confirmado; si no, descomponer en waves secuenciales o subagents acotados
- Worktree recomendado para aislar el laburo

## Paso 5: Armar el plan de ejecucion
Generar un plan incremental de menos a mas:

### Wave 0 — Setup
- Crear branch (o worktree si se recomendo)
- Scaffolding minimo si hace falta

### Wave 1 — Fundamentos
- Cambios base que no dependen de nada mas
- Modelos, tipos, interfaces, migrations
- **Validacion**: type-check / lint pasan

### Wave 2 — Core
- Logica principal del feature
- **Validacion**: tests unitarios pasan

### Wave 3 — Integracion
- Conectar las piezas, endpoints, UI
- **Validacion**: tests de integracion pasan

### Wave 4 — Pulido
- Edge cases, error handling, cleanup
- **Validacion**: test suite completa pasa, `/quality-gate` limpio

Cada wave debe tener:
- Tareas concretas con archivos especificos
- Criterio de validacion explicito (que correr para saber que esta bien)
- Output esperado antes de avanzar a la siguiente wave
- Los comandos exactos de validacion descubiertos en el preflight

## Paso 6: Proponer estructura de Teams (solo si es Mediano o Grande Y Teams esta soportado de verdad)

```markdown
## Team propuesto: [nombre]

### Teammates
- **[nombre]** — [responsabilidad]. Tasks: [lista]
- **[nombre]** — [responsabilidad]. Tasks: [lista]
- **[nombre]** — [responsabilidad]. Tasks: [lista]

### Coordinacion
- Wave en la que cada teammate arranca
- Dependencias entre teammates
- Puntos de sincronizacion
```

## Paso 7: Proponer worktree (si aplica)
Si hay trabajo en progreso en la branch actual O la clasificacion es Mediano/Grande:
- Sugerir `git worktree add` con nombre de branch descriptivo
- Explicar por que conviene aislar

## Paso 8: Presentar al usuario
Mostrar el plan completo incluyendo los resultados del preflight.

Cerrar con una recomendacion de siguiente paso segun el contexto:
- Ejecutar directo si el scope ya esta claro y no hace falta artefacto extra
- Dejar el plan en la conversacion si alcanza como handoff
- Guardarlo en un documento local/PRD si conviene persistirlo fuera del chat
- Crear un issue solo si el usuario lo pide o si realmente aporta trazabilidad/coordinacion
- Si el usuario planea usar `/execute` despues, sugerir crear el issue con `gh issue create` ya que `/execute` requiere un issue como input.

No empujar GitHub issue como opcion por defecto.
No empujar Teams como opcion si el soporte no quedo confirmado en el preflight.

## Paso 9: Handoff opcional
Si hace falta materializar el plan fuera de la conversacion, proponer el formato mas adecuado al contexto:
- Markdown local en el repo
- PRD/spec/RFC
- GitHub issue

Elegir GitHub issue solo cuando tenga sentido operativo claro, por ejemplo:
- Hace falta trazabilidad en backlog
- Hay coordinacion entre varias personas
- El equipo ya trabaja con issues como fuente de verdad

Si el usuario pide un issue, usar esta estructura:

```markdown
## Contexto
[Resumen de los docs leidos y el problema a resolver]

## Preflight
[Resultados del paso 3 — comandos de test, lint, build confirmados y entorno de pruebas]

## Plan de implementacion

### Clasificacion: [Chico|Mediano|Grande]
[Justificacion]

### Waves de ejecucion
[El plan del Paso 5]

### Teams (si aplica)
[Propuesta del Paso 6]

### Setup recomendado
- Branch: `feat/[nombre-descriptivo]`
- Worktree: [si/no y por que]

## Requisitos
### Must have
- [ ] [requisito 1]
- [ ] [requisito 2]

### Must NOT have
- [exclusion explicita]

## Criterios de aceptacion
- [ ] [criterio 1]
- [ ] [criterio 2]
```

## MUST NOT DO
- NO implementar nada — este skill solo planifica
- NO crear un issue sin pedido o aprobacion explicita del usuario
- NO inventar requisitos que no esten en los documentos
- NO ignorar restricciones mencionadas en los docs
- NO proponer Teams si el task es Chico
- NO proponer Teams si el soporte real no esta confirmado
- NO proponer worktree si no hay justificacion real
- NO avanzar a planificar si el preflight tiene fallos sin resolver
- NO asumir el entorno de pruebas — verificarlo explicitamente
- NO asumir que GitHub issue es el mejor formato de handoff
- NO asumir soporte de Teams por una env var o config experimental

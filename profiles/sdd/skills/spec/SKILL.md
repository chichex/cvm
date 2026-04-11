Analizar un pedido, crear una spec formal, y armar un plan de implementacion. Se invoca automaticamente por el auto-routing. $ARGUMENTS puede ser:
- Paths a documentos (PRDs, specs, RFCs) o un glob pattern
- Una descripcion directa de lo que se quiere hacer
- Path a un spec existente para planificar su implementacion

## Paso 1: Entender el pedido

**1a. Clasificar:**
- **Feature nuevo**: componente o feature sin implementacion previa
- **Enhancement**: mejora a funcionalidad existente
- **Refactoring**: cambiar estructura sin cambiar comportamiento
- **Bug fix**: algo esta roto → derivar a /fix
- **Arquitectura**: decision de diseno o estructura del sistema

**1b. Verificar specs existentes:**
- Buscar en `specs/` si ya existe un spec relacionado
- Si existe: proponer actualizar el existente
- Si no existe: crear uno nuevo

## Paso 2: Crear la spec

### 2a. Clasificar nivel de spec

Inferir del contexto:
- **Feature spec**: comportamiento end-to-end de un feature completo
- **Component spec**: modulo/componente aislado con interfaz publica
- **API spec**: endpoint o servicio con request/response
- **Function spec**: funcion o metodo individual con contrato preciso

### 2b. Explorar el codebase

- Identificar archivos y modulos afectados
- Buscar patterns existentes que reutilizar
- Detectar contratos existentes (types, interfaces)
- Buscar specs existentes en `specs/` que se relacionen

### 2c. Recopilar informacion

Segun el nivel, extraer del prompt del usuario y del codebase:

- **Contratos**: tipos, interfaces, firmas, pre/postcondiciones
- **Behaviors**: flujos principales con Given/When/Then usando datos concretos
- **Edge cases**: boundaries, inputs invalidos, estados imposibles, concurrencia
- **Invariantes**: propiedades que SIEMPRE deben cumplirse
- **Errores**: condiciones, codigos, comportamiento esperado
- **Restricciones no funcionales** (si aplica): performance, seguridad, accesibilidad
- **Specs relacionadas** (si aplica): S-XXX que este spec extiende o requiere

### 2d. Evaluar estrategia de validacion

Decidir que aplica para este tipo de cambio:

| Tipo | Estrategia |
|------|-----------|
| Logica de negocio, algoritmos, utils | **TDD** |
| APIs, endpoints, contratos publicos | **TDD** |
| CLI commands, parsers, validators | **TDD** |
| UI, componentes visuales | **Tests post-impl** |
| Infra, config, CI/CD | **Validacion manual** |
| Refactoring puro | **Tests existentes** |
| Exploratorio, spike, PoC | **Sin tests** |

Documentar la decision en la spec.

### 2e. Generar la spec

Crear `specs/<nombre>.spec.md` usando el formato estandar definido en CLAUDE.md.
Crear el directorio `specs/` si no existe.

Reglas:
- Lenguaje RFC 2119: MUST, MUST NOT, SHALL, MAY
- Cada requisito tiene ID unico (B-XXX, E-XXX, I-XXX)
- Cada behavior tiene datos concretos, no placeholders
- Los edge cases son obligatorios, no opcionales
- Sin ambiguedades: "manejar apropiadamente" no es valido

### 2f. Registrar en registry

Crear o actualizar `specs/REGISTRY.md`:
```
| ID | Nombre | Status | Version | Validacion | Archivo |
|----|--------|--------|---------|------------|---------|
| S-XXX | [nombre] | draft | 0.1.0 | [estrategia] | specs/nombre.spec.md |
```

### 2g. Presentar al usuario

Mostrar la spec completa al usuario. Esperar aprobacion antes de continuar.
Si el usuario pide cambios, iterar hasta que apruebe.

## Paso 3: Validacion externa (si Codex esta disponible)

Verificar disponibilidad de Codex: `codex exec "echo ok" 2>/dev/null` (con timeout de 10s).
Si falla o no responde: Codex no disponible para esta sesion.

**Si disponible**, validar la spec:
```bash
timeout 900 codex exec "Review this spec for: 1) ambiguity, 2) gaps, 3) contradictions, 4) testability. Be critical. Spec: [contenido]" 2>/dev/null
```
- Si hay issues: incorporar, actualizar, mostrar cambios
- Si pasa: status → approved

**Si no disponible**: informar al usuario, aprobar solo con su OK.

## Paso 4: Preflight check

Verificar toolchain y entorno:
1. Tests: encontrar y correr el comando de test actual
2. Lint/Type-check: verificar baseline limpio
3. Build: verificar que compila
4. Servicios necesarios: verificar si estan levantados
5. Branch limpia: `git status`

Reportar hallazgos. Si algo falla, preguntar antes de continuar.

## Paso 5: Clasificar tamano

- **Chico** (1 modulo, <10 archivos): ejecucion directa
- **Mediano** (2-3 modulos, 10-30 archivos): subagents en paralelo
- **Grande** (3+ modulos, 30+ archivos): descomponer en waves con subagents

## Paso 6: Plan de ejecucion

Generar plan incremental referenciando secciones de la spec:

### Wave 0 — Setup
- Crear branch
- Commit spec si no esta commiteada

### Wave 1 — Contratos
- Implementar tipos e interfaces de la seccion Contratos de la spec
- Validacion: type-check pasa, contratos matchean spec

### Wave 2 — Behaviors + Tests
- Segun estrategia de validacion:
  - **TDD**: generar tests desde behaviors → verificar que fallan → implementar hasta que pasen
  - **Tests post-impl**: implementar behaviors → generar tests → verificar que pasan
  - **Manual/existentes**: implementar behaviors → correr tests existentes
- Validacion: tests de behaviors pasan (si aplica)

### Wave 3 — Edge Cases
- Implementar manejo de edge cases
- Validacion: tests de edge cases pasan (si aplica)

### Wave 4 — Verificacion Dual
- Agente Opus verifica conformance
- Codex verifica conformance
- Consolidar resultados

Cada wave tiene tareas concretas con archivos especificos y criterio de validacion.

## Paso 7: Presentar al usuario

Mostrar: spec + plan + estrategia de validacion.
Recomendar siguiente paso:
- Si el scope es claro → ejecutar directo (continuar a implementacion)
- Si hace falta handoff → crear issue con `gh issue create`

## MUST DO
- Cada requisito MUST ser testeable mecanicamente
- Cada edge case MUST tener comportamiento esperado explicito
- La estrategia de validacion MUST estar documentada en la spec

## MUST NOT DO
- NO implementar nada — este skill especifica y planifica
- NO saltear la validacion por Codex si esta disponible
- NO asumir TDD — evaluar la estrategia apropiada
- NO avanzar sin aprobacion del usuario en la spec
- NO usar lenguaje vago
- NO omitir edge cases
- NO crear specs sin examples con datos concretos
- NO asumir que Codex esta instalado — verificar primero

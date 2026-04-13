# SDD — Spec Driven Development para Claude Code

## Filosofia

La spec es la fuente de verdad. El codigo es una derivacion de la spec.

Orden de trabajo:
1. **Spec primero** — antes de escribir codigo, definir QUE debe hacer
2. **Contrato antes de implementacion** — las interfaces se derivan de la spec
3. **Validacion derivada de la spec** — los tests se generan desde la spec cuando TDD aplica; cuando no, se define la estrategia de validacion apropiada
4. **Codigo derivado** — la implementacion satisface la spec, nada mas
5. **Verificacion multi** — un agente Opus, Codex, y Gemini (si disponibles) validan independientemente el resultado final

Principios core:
- **Spec = Single Source of Truth**: si el codigo no matchea la spec, el codigo esta mal
- **Especificidad sobre ambiguedad**: una spec vaga es peor que no tener spec
- **Contratos explicitos**: toda interfaz publica tiene un contrato formal
- **Trazabilidad**: cada pieza de codigo se puede rastrear a un requisito de la spec
- **Evolucion controlada**: las specs versionan y nunca se cambian implicitamente
- **El usuario no necesita saber los skills**: el sistema detecta el intent y arranca el proceso SDD automaticamente

## Auto-Routing

El usuario NO necesita invocar skills manualmente. Claude detecta el intent del prompt y decide automaticamente que hacer.

### Decision Tree

Cuando el usuario envia un prompt, clasificar:

**1. Bug report** ("esto no funciona", "hay un error", "se rompe cuando...")
```
Diagnosticar → Buscar spec existente → Si hay spec: verificar drift
→ Si no hay spec o hay gap: crear/actualizar spec → Elegir estrategia de validacion → Fix → Verify
```

**2. Feature request** ("quiero que...", "necesito...", "agregar...")
```
Crear spec → Codex valida spec → Elegir estrategia de validacion
→ Si TDD aplica: generar tests (fallan) → Implementar (tests pasan) → Multi verify
→ Si TDD no aplica: Implementar → Generar tests post-impl → Multi verify
```

**3. Behavior change** ("cambiar...", "modificar como...", "ahora deberia...")
```
Buscar spec existente → Actualizar spec (version bump) → Codex valida cambios
→ Propagar: contratos → tests → codigo → Multi verify
```

**4. Refactor** ("limpiar...", "refactorear...", "reorganizar...")
```
Verificar que hay spec → Si no hay: crear spec retroactiva minima
→ Implementar refactor → Verificar que spec sigue matcheando (no behavior change)
```

**5. Pregunta o exploracion** ("como funciona...", "que hace...", "por que...")
```
Responder directamente. No SDD.
```

**6. Trivial** (sin cambio de contrato publico, sin behavior nuevo, sin cambio de persistencia/schema/protocolo/auth, sin efecto cross-module. Ejemplos: fix typo, update config value, add log line, use existing util.)
```
Ejecutar directo. No SDD.
```

### Opt-out

Si el usuario dice "directo", "sin spec", "rapido", o "just do it":
- **Trivial o config/docs**: ejecutar directo. No SDD.
- **No trivial pero el usuario insiste**: aplicar SDD-lite automaticamente (spec inline, sin registry, single verify). Informar al usuario que se usa SDD-lite en vez de full skip.

Respetar la decision del usuario, pero no saltear spec completamente para features nuevos — como minimo usar SDD-lite.

### SDD-lite (cambios de bajo riesgo)

Para cambios que necesitan spec pero no el workflow completo (bajo riesgo, scope claro):
- Spec inline: behaviors y edge cases como comentarios en el PR, no archivo separado
- Sin Codex validation
- Single verification (solo Opus, no dual)
- Sin registry

Aplicar SDD-lite cuando el cambio es claro y acotado pero involucra behavior nuevo.

### Seleccion de Estrategia de Validacion

La estrategia de validacion (TDD, tests-post-impl, manual, etc.) se selecciona automaticamente segun el tipo de cambio. Ver el skill `/spec` para la tabla completa.

## Workflow

El flujo SDD tiene 5 fases. Cada fase esta detallada en su skill correspondiente.

```
Prompt del usuario
    │
    ▼
Auto-routing (clasificar intent)
    │
    ▼
/spec ──→ Crear spec + plan (Codex valida si disponible)
    │
    ▼
/derive-tests ──→ Generar tests desde spec (si la estrategia lo requiere)
    │
    ▼
/execute ──→ Implementar wave por wave
    │
    ▼
/verify ──→ Verificacion multi: Opus + Codex + Gemini (si disponibles)
```

Cada skill gestiona sus propios pasos, validaciones, y gates.

## Skills

### Core SDD
| Skill | Proposito |
|-------|-----------|
| `/spec` | Crear spec formal + plan de implementacion |
| `/derive-tests` | Generar tests desde spec |
| `/execute` | Implementar desde spec, wave por wave |
| `/verify` | Verificacion multi (Opus + Codex + Gemini si disponibles — Gemini es opcional) |
| `/spec-status` | Dashboard de estado de specs |
| `/quality-gate` | Validacion post-impl (tests, lint, build, spec coverage) |
| `/fix` | Diagnosticar bug con spec gap check + Codex second opinion |

### Knowledge
| Skill | Proposito |
|-------|-----------|
| `/learn` | Guardar insight en KB |
| `/decide` | Registrar decision de diseno |
| `/gotcha` | Registrar trampa encontrada |
| `/recall` | Buscar contexto en KB |
| `/retro` | Revision de fin de sesion (incluye session summary) |

### Meta
| Skill | Proposito |
|-------|-----------|
| `/evolve` | Detectar patrones y generar skills |
| `/maintain` | Higiene de KB |
| `/checkpoint` | Save point antes de cambios grandes |
| `/orchestrate` | Decidir ruta de ejecucion |
| `/skill-create` | Generar skill custom |
| `/headless` | Ejecutar tarea headless |

## Formato de Spec

Las specs viven en `specs/<nombre>.spec.md`. El formato estandar incluye: ID, Version, Status, Estrategia de validacion, Objetivo, Alcance, Contratos, Behaviors (Given/When/Then), Edge Cases, Invariantes, Errores, Restricciones no funcionales, Specs relacionadas, Dependencias, y Changelog.

El template completo se aplica automaticamente al crear specs con `/spec`.

Reglas del formato:
- Lenguaje RFC 2119: MUST, MUST NOT, SHALL, MAY — no "deberia", "podria"
- Cada requisito tiene ID unico (B-XXX, E-XXX, I-XXX)
- Cada behavior tiene datos concretos, no placeholders
- Sin ambiguedades: "manejar apropiadamente" no es valido

## Spec Registry

El registry vive en `specs/REGISTRY.md`:

```markdown
| ID | Nombre | Status | Version | Validacion | Archivo |
|----|--------|--------|---------|------------|---------|
| S-001 | [nombre] | draft | 0.1.0 | TDD | specs/nombre.spec.md |
```

Status flow: `draft → approved → implemented → verified`

## Knowledge Base (KB)

La KB es la memoria persistente entre sesiones. Usarla siempre.

### Progressive Disclosure (2-step lookup)

Para consultar la KB, usar SIEMPRE el patron de 2 pasos:
1. **Search primero**: `cvm kb search "<query>"` — devuelve keys + snippets, barato
2. **Show despues**: `cvm kb show "<key>"` — devuelve contenido completo, solo si es relevante

NUNCA leer todas las entries de una vez. El search filtra y el show profundiza.
Esto aplica tanto a global como local (`--local`).

### Context Injection Automatico

Al inicio de cada sesion, el hook `context-inject.sh` inyecta automaticamente un resumen
compacto de las entries mas recientes de la KB (global + local). Este contexto aparece como
un bloque `<cvm-context>` en el system prompt.

Configuracion via env vars:
- `CVM_CONTEXT_ENTRY_COUNT`: max entries a mostrar (default: 10)
- `CVM_CONTEXT_MAX_TOKENS`: budget maximo de tokens para el bloque (default: 2000)

No es necesario buscar manualmente el contexto que ya fue inyectado — consultarlo antes de asumir.

**Antes de actuar:**
- Revisar el `<cvm-context>` inyectado al inicio (si existe)
- Buscar contexto adicional: `cvm kb search "<query>"` y `cvm kb search "<query>" --local`
- Si hay entries relevantes, profundizar con `cvm kb show "<key>"`

**On-the-fly learning (automatico):**
El hook `UserPromptSubmit` inyecta el protocolo de learning con self-check obligatorio.

Self-check despues de cada interaccion significativa:
- ¿Tome una decision de diseno? → `cvm kb put` con tag `decision`
- ¿Resolvi un bug o encontre la causa? → `cvm kb put` con tag `learning`
- ¿Algo no funciono como esperaba? → `cvm kb put` con tag `gotcha`
- ¿El usuario confirmo o rechazo un approach? → `cvm kb put` con tag `decision`
- ¿Descubri un gap en un spec? → `cvm kb put` con tag `spec-gap`

Si la respuesta a cualquiera es SI → guardar AHORA, no despues.

**Captura pasiva de subagents:**
El hook `SubagentStop` captura automaticamente secciones `## Key Learnings:` del output de subagents y las persiste en KB.

**Session summary (obligatorio):**
Antes de cerrar la sesion:
```
cvm kb put "session-summary-YYYYMMDD" --body "Goal: ... | Accomplished: ... | Specs written/updated: ... | Discoveries: ... | Next: ..." --tag "session,summary"
```

**Comandos disponibles:**
- `cvm kb put <key> --body "..." --tag "a,b"` — guardar entry global
- `cvm kb put <key> --body "..." --tag "a,b" --local` — guardar entry local
- `cvm kb search <query>` / `cvm kb search <query> --local` — buscar
- `cvm kb ls` / `cvm kb ls --local` — listar
- `cvm kb show <key>` / `cvm kb show <key> --local` — ver detalle
- `cvm kb rm <key>` / `cvm kb rm <key> --local` — eliminar

## Delegacion Estructurada

Delegar usando `Agent(subagent_type: "general-purpose", model: "<model>")`:

| Rol | model | Cuando |
|-----|-------|--------|
| **researcher** | `haiku` | buscar, encontrar, listar, leer, localizar |
| **implementer** | `sonnet` | implementar, escribir, refactorear, testear |
| **reviewer** | `opus` | analizar, investigar, revisar, verificar conformance |
| **specifier** | `sonnet` | escribir y actualizar specs formales |
| **verifier** | `opus` | verificacion formal spec vs implementacion |

Al usar el Agent tool, estructurar:
- **TASK**: Que hacer
- **SPEC REFERENCE**: Que spec y seccion aplican (si existe)
- **EXPECTED OUTCOME**: Como se ve el exito
- **MUST DO**: Requisitos innegociables
- **MUST NOT DO**: Limites explicitos
- **CONTEXT**: Background relevante

Siempre incluir:
> Termina tu respuesta con una seccion `## Key Learnings:` listando descubrimientos no-obvios.

SIEMPRE usar `subagent_type: "general-purpose"` para delegar.

## Roles de Delegacion

Prompt templates en `agents/<rol>/AGENT.md`:
- **researcher** (`model: haiku`) — Busqueda mecanica
- **implementer** (`model: sonnet`) — Escritura de codigo (siempre referencia spec)
- **specifier** (`model: sonnet`) — Creacion de specs formales
- **reviewer** (`model: opus`) — Analisis profundo
- **verifier** (`model: opus`) — Verificacion formal + spec conformance

## Reglas

Las reglas en `rules/` se aplican automaticamente:
- **agent-routing** — Routing de delegacion via general-purpose + model
- **model-selection** — Guia de seleccion de modelo por tipo de tarea
- **context-hygiene** — Mantener el thread principal minimo
- **cost-awareness** — No desperdiciar tokens
- **scope-guard** — Scope = spec, nada mas
- **kb-awareness** — Siempre consultar KB antes de asumir
- **spec-first** — No implementar sin spec (excepto trivial)
- **no-spec-drift** — No cambiar behavior sin actualizar spec primero
- **traceability** — Codigo publico trazable a spec ID

## Sesion

- Al iniciar sesion se ejecuta `cvm session start` automaticamente (hook)
- Al cerrar sesion se ejecuta `cvm session end` automaticamente (hook)
- **On-the-fly learning**: hook `UserPromptSubmit`
- **Captura pasiva**: hook `SubagentStop`
- **Post-compaction**: hook `SessionStart` (matcher: compact)
- Para revision manual: `/retro`

### Presupuesto de latencia

- `UserPromptSubmit` debe seguir siendo ultra-liviano
- Trabajo pesado solo en `SessionEnd` o background

## Overrides de Usuario

El usuario puede agregar customizaciones al profile que sobreviven a `cvm pull`.
Los overrides se guardan en `~/.cvm/global/overrides/<profile>/` y se aplican
automaticamente encima del profile base al hacer `cvm use` o `cvm pull`.

**Comandos:**
- `cvm override add <tipo> <nombre>` — crear un skill, hook, agent, rule, o command custom
- `cvm override set <archivo>` — capturar settings.json, CLAUDE.md, keybindings.json, etc.
- `cvm override ls` — ver overrides activos
- `cvm override rm <tipo> <nombre>` — eliminar un override
- `cvm override edit` — abrir el directorio de overrides
- `cvm override show` — inventario detallado
- `cvm override apply` — forzar re-aplicacion

**Merge:**
- Directorios (skills, hooks, agents, rules): union merge
- JSON (settings.json, keybindings.json): deep merge
- CLAUDE.md: se appenda al final del base
- Otros archivos: override reemplaza

### Bypass de Permissions

- `cvm bypass on` — habilita bypass
- `cvm bypass off` — restaura permissions base
- `cvm bypass status` — muestra estado actual

## Hard Blocks

- Nunca usar `as any`, `@ts-ignore`, o `eslint-disable` sin justificacion explicita
- Nunca dejar catch blocks vacios
- Nunca borrar o skipear tests para que pasen
- Nunca commitear sin que el usuario lo pida (excepcion: `/execute` incluye commit/push/PR como parte del flujo autorizado)
- Nunca especular sobre codigo sin leerlo — usar tools para verificar
- Nunca hacer shotgun debugging
- **Nunca implementar un feature nuevo sin spec** (excepto trivial; si el usuario pide opt-out, usar SDD-lite como minimo)
- **Nunca cambiar behavior sin actualizar la spec primero** (excepcion: hotfix urgente con spec post-facto)
- **Nunca ignorar el resultado de la verificacion** (triple si Codex+Gemini disponibles, dual si uno disponible, single si ninguno)
- **Nunca pasar contenido de archivos inline en prompts de Codex** — Codex tiene acceso al filesystem y a `gh`. Darle paths para que lea, o comandos como `gh pr diff`. Si hay un PR abierto, usarlo. Si no, escribir un manifiesto con paths en `/tmp/` y referenciarlo. Esto aplica a specs, implementaciones, diffs, y cualquier otro contenido.

## Entorno

- macOS para desarrollo local
- Evitar flags GNU-only como `grep -P`. Usar `grep -E` o perl one-liners
- Para levantar servicios, usar el script `start.sh` del proyecto si existe
- Codex disponible via `codex exec` para validacion externa. Verificar con `codex exec "echo ok" 2>/dev/null`, no con `which codex`
- Gemini disponible via `gemini -p` para validacion externa (tercer validador opcional). Verificar via `~/.cvm/available-tools.json` (campo `gemini.available`). Gemini tiene acceso al filesystem. Spec: S-012
- macOS no tiene GNU `timeout` — los skills NO deben usar `timeout` directamente. Codex y Gemini se ejecutan sin timeout de shell; si se cuelgan, el usuario cancela manualmente

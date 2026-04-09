# Chiche — Sistema Operativo para Claude Code

## Filosofia

El thread principal es un orquestador, no un ejecutor. Delegar siempre que sea posible.

Orden de preferencia:
1. **Teams** — solo si Claude expone soporte real para Teams en esta sesion y hay 2+ areas independientes que se pueden paralelizar
2. **Subagent** — cuando el task es acotado y enfocado (investigacion, implementacion de un modulo)
3. **Directo** — solo para respuestas simples, confirmaciones, o tareas de <30 segundos

No asumir soporte de Teams solo porque el profile tenga flags o env vars. Si no hay evidencia clara de soporte real en la sesion actual, caer a subagents o ejecucion directa.

Antes de actuar, correr mentalmente `/orchestrate` para decidir la ruta.

## Knowledge Base (KB)

La KB es la memoria persistente entre sesiones. Usarla siempre.

**Antes de actuar:**
- Buscar contexto relevante: `cvm kb search "<query>"` y `cvm kb search "<query>" --local`
- Si hay entries relevantes, leerlas y aplicarlas

**On-the-fly learning (automatico):**
El hook `UserPromptSubmit` inyecta el protocolo de learning. Cuando identifiques algo no-obvio:
1. Verificar duplicados: `cvm kb search "<terminos>"`
2. Guardar: `cvm kb put "<key>" --body "<desc con el POR QUE>" --tag "<tipo>,<area>" [--local]`
3. Reportar: `[learned] key — descripcion`

No forzar. Solo guardar si es genuinamente util para futuras sesiones. Calidad > cantidad.

**Skills manuales (cuando se necesita mas control):**
- `/learn` — guardar un learning con mas detalle y confirmacion
- `/decide` — registrar una decision de diseno con alternativas y trade-offs
- `/gotcha` — registrar una trampa con contexto completo
- `/retro` — revision completa de toda la sesion al final

**Comandos disponibles:**
- `cvm kb put <key> --body "..." --tag "a,b"` — guardar entry global
- `cvm kb put <key> --body "..." --tag "a,b" --local` — guardar entry local al proyecto
- `cvm kb search <query>` — buscar en KB global
- `cvm kb search <query> --local` — buscar en KB local
- `cvm kb ls` / `cvm kb ls --local` — listar entries

## Seleccion de Modelo

Usar el modelo apropiado para cada tarea. No desperdiciar tokens de opus en lookups.

| Tarea | Modelo | Justificacion |
|-------|--------|---------------|
| Buscar archivos, leer codigo, lookups | haiku | Rapido y barato |
| Implementacion, refactoring, tests | sonnet | Balance costo/calidad |
| Arquitectura, review, decisiones criticas | opus | Maxima calidad de razonamiento |
| Debugging adversarial (`/validate`) | opus x2 o opus+codex | Hipotesis competitivas |

Cuando se delega a un subagent, especificar el modelo en el prompt si es relevante.

## Higiene de Contexto

El thread principal debe mantenerse liviano:
- NO leer archivos grandes en el thread principal — delegar a un subagent researcher
- NO hacer grep extensivos en el thread principal — delegar
- NO acumular mas de 3-4 tool calls consecutivas sin delegar
- Cuando se necesita explorar codigo, lanzar un subagent con scope acotado y que reporte hallazgos

## Disciplina de Scope

- Hacer SOLO lo que se pidio
- Si se detecta algo mejorable pero fuera de scope: guardarlo con `/learn` o `/gotcha`, NO implementar
- Sugerencias se dan en UNA linea. Si el usuario quiere mas detalle, lo pide.
- No agregar error handling especulativo, no mejorar codigo que funciona, no refactorear de paso

## Delegacion Estructurada

Al usar el Agent tool, siempre estructurar asi:
- **TASK**: Que hacer
- **EXPECTED OUTCOME**: Como se ve el exito
- **MUST DO**: Requisitos innegociables
- **MUST NOT DO**: Limites explicitos
- **CONTEXT**: Background relevante

## Skills Disponibles

Estos skills se invocan con `/nombre`:

| Skill | Proposito |
|-------|-----------|
| `/learn` | Guardar un insight en la KB |
| `/decide` | Registrar una decision de diseno |
| `/gotcha` | Registrar una trampa encontrada |
| `/recall` | Buscar contexto en KB antes de actuar |
| `/retro` | Fin de sesion: extraer y persistir learnings |
| `/evolve` | Detectar patrones repetidos y generar nuevos skills |
| `/maintain` | Higiene de KB: dedup, prune, consolidar |
| `/validate` | Debugging adversarial con multiples agentes |
| `/orchestrate` | Analizar task y decidir: directo, subagent, o team (solo si hay soporte real) |
| `/checkpoint` | Crear save point antes de cambios grandes |
| `/quality-gate` | Validacion post-implementacion: lint, tests, slop |
| `/spec` | Planificar implementacion completa con preflight y propuesta de teams solo si estan soportados |
| `/execute` | Ejecutar un issue planificado con /spec de punta a punta |
| `/fix` | Diagnosticar y resolver un bug con rigor |
| `/ux` | Analizar screenshots de UI/UX y generar propuestas de mejora |
| `/higiene` | Auditoria de higiene del entorno Claude Code y del proyecto |
| `/skill-create` | Generar un nuevo skill custom para Claude Code |
| `/headless` | Ejecutar una tarea en Claude Code headless (claude -p) |

## Agentes Disponibles

Definidos en `agents/`:
- **researcher** — Exploracion y busqueda, usa haiku. Tools: Read, Grep, Glob, Bash
- **implementer** — Escritura de codigo, usa sonnet. Tools: todos
- **reviewer** — Review y analisis, usa opus. Tools: Read, Grep, Glob

## Reglas

Las reglas en `rules/` se aplican automaticamente segun el contexto:
- **model-selection** — Guia de seleccion de modelo por tipo de tarea
- **context-hygiene** — Mantener el thread principal minimo
- **cost-awareness** — No desperdiciar tokens
- **scope-guard** — No hacer scope creep, guardar sugerencias en KB
- **kb-awareness** — Siempre consultar KB antes de asumir

## Sesion

- Al iniciar sesion se ejecuta `cvm lifecycle start` automaticamente (hook)
- Al cerrar sesion se ejecuta `cvm lifecycle end` automaticamente (hook)
- Mantenimiento (`maintain`) y evolucion (`evolve`) se disparan por umbrales, se encolan como candidatos persistentes en `~/.cvm/automation/` y luego se ejecutan automaticamente en background
- Los candidatos se materializan en briefs Markdown y se inspeccionan con `cvm automation status|ls|show <id>`
- Cada corrida queda auditada con `cvm automation history` y `cvm automation show-run <id>`
- **On-the-fly learning**: el hook `UserPromptSubmit` inyecta protocolo de learning automatico — guardar directo con `cvm kb put`, sin headless
- Para revision manual completa de sesion, usar `/retro`

### Presupuesto de latencia

- `UserPromptSubmit` debe seguir siendo ultra-liviano
- Trabajo pesado solo en `SessionEnd` o background
- La statusline puede mostrar candidatos pendientes como `[auto:N]`

## Overrides de Usuario

El usuario puede agregar customizaciones al profile que sobreviven a `cvm pull`.
Los overrides se guardan en `~/.cvm/global/overrides/<profile>/` y se aplican
automaticamente encima del profile base al hacer `cvm use` o `cvm pull`.

**Comandos:**
- `cvm override add <tipo> <nombre>` — crear un skill, hook, agent, rule, o command custom
- `cvm override set <archivo>` — capturar settings.json, CLAUDE.md, keybindings.json, etc. como override
- `cvm override ls` — ver overrides activos
- `cvm override rm <tipo> <nombre>` — eliminar un override
- `cvm override edit` — abrir el directorio de overrides en el editor
- `cvm override show` — inventario detallado
- `cvm override apply` — forzar re-aplicacion del profile + overrides

**Merge:**
- Directorios (skills, hooks, agents, rules): union merge — se agregan o reemplazan por nombre
- JSON (settings.json, keybindings.json): deep merge — override keys ganan
- CLAUDE.md: se appenda al final del base
- Otros archivos: override reemplaza

**Cuando el usuario pida agregar un skill, hook, rule, o cualquier customizacion al profile, usar `cvm override add` para que persista entre pulls.**

Agregar flag `--local` para overrides de profiles locales.

## Hard Blocks

- Nunca usar `as any`, `@ts-ignore`, o `eslint-disable` sin justificacion explicita
- Nunca dejar catch blocks vacios
- Nunca borrar o skipear tests para que pasen
- Nunca commitear sin que el usuario lo pida
- Nunca especular sobre codigo sin leerlo — usar tools para verificar
- Nunca hacer shotgun debugging

## Entorno

- macOS para desarrollo local
- Evitar flags GNU-only como `grep -P`. Usar `grep -E` o perl one-liners
- Para levantar servicios, usar el script `start.sh` del proyecto si existe

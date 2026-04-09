Auditoria de higiene del entorno Claude Code y del proyecto actual. Correr sin argumentos para revision completa, o pasar $ARGUMENTS con un area especifica (ej: "mcps", "skills", "code", "claude.md").

## Paso 1: Auditoria del entorno Claude

### 1a. Skills
- Listar todos los skills disponibles (en el profile activo y en `~/.claude/commands/` si existen)
- Verificar que cada skill tenga una descripcion clara en la primera linea
- Detectar skills duplicados o con funcionalidad solapada
- Reportar cantidad total

### 1b. MCPs y Plugins
- Leer `~/.claude/settings.json` -> `enabledPlugins`
- Leer `~/.claude.json` y buscar `.mcp.json` en el proyecto actual
- Para cada MCP/plugin habilitado: verificar que se este usando realmente (buscar invocaciones en skills, hooks, o historial reciente)
- Detectar MCPs configurados pero que nunca se usan

### 1c. Hooks
- Leer `~/.claude/settings.json` -> `hooks`
- Verificar que cada hook script referenciado existe y es ejecutable
- Correr `bash -n` en cada script para validar sintaxis
- Detectar hooks huerfanos (scripts en `~/.claude/hooks/` que no estan referenciados en settings)

### 1d. Memory
- Leer `~/.claude/projects/*/memory/MEMORY.md`
- Verificar que cada archivo referenciado en MEMORY.md existe
- Detectar archivos de memoria huerfanos (existen pero no estan indexados)
- Verificar que no haya memorias duplicadas o contradictorias

### 1e. Permissions
- Leer `~/.claude/settings.json` -> `permissions.allow`
- Detectar reglas que ya no aplican (paths que no existen, scripts borrados)
- Detectar reglas demasiado amplias que podrian restringirse

## Paso 2: Auditoria del CLAUDE.md

Buscar todos los CLAUDE.md del proyecto (raiz, subdirectorios, `~/.claude/CLAUDE.md`):
- Verificar largo total — alertar si supera 200 lineas (bloat warning)
- Detectar secciones duplicadas entre CLAUDE.md global y de proyecto
- Detectar instrucciones que contradicen las del global
- Detectar instrucciones obsoletas (referencian archivos, funciones, o paths que ya no existen)
- Verificar que no tenga contenido que deberia ser un skill separado (bloques de instrucciones >30 lineas para un workflow especifico)

## Paso 3: Auditoria del codigo (solo si hay proyecto activo)

### 3a. Codigo muerto

**Dentro de cada layer:**
- Buscar imports sin usar
- Buscar funciones/metodos exportados que no se importan en ningun lado
- Buscar archivos que no son importados ni referenciados
- Buscar variables asignadas pero nunca leidas
- Detectar bloques de codigo comentados

**Cross-layer (frontend <-> backend):**
- Identificar los layers del proyecto (frontend, backend, API, etc.) y como se comunican (REST endpoints, GraphQL, RPC, etc.)
- Extraer del frontend: todas las llamadas a API (fetch, axios, SDK clients, hooks de data fetching, etc.)
- Extraer del backend: todos los endpoints expuestos (rutas, controllers, resolvers, etc.)
- Cruzar ambas listas y detectar:
  - **Endpoints huerfanos**: rutas en el backend que ningun cliente consume (el boton se borro pero la API quedo)
  - **Llamadas huerfanas**: el frontend llama a un endpoint que ya no existe en el backend
  - **Modelos/schemas sin uso**: DTOs, serializers, validaciones, migrations que solo servian a funcionalidad eliminada
  - **Jobs/workers sin trigger**: background jobs que nadie encola
  - **Permisos/roles sin referencia**: reglas de autorizacion para acciones que ya no existen en la UI
- Para cada hallazgo, rastrear la cadena completa: endpoint -> controller -> service -> repository -> modelo. Si toda la cadena es huerfana, reportar el arbol completo.

### 3b. Tests
- Identificar tests que no testean nada real (assertions vacias, tests que solo verifican `true === true`)
- Detectar tests que estan skippeados (`skip`, `xit`, `xdescribe`, `@pytest.mark.skip`)
- Buscar tests duplicados o con nombres enganosos
- Verificar que la cobertura de tests tenga sentido (archivos criticos sin tests)

## Paso 4: Reporte

Generar reporte con este formato:

```
## Higiene Claude Code — [fecha]

### Entorno
| Area | Estado | Detalle |
|------|--------|---------|
| Skills | ok/warn/fail | N skills, [problemas si hay] |
| MCPs/Plugins | ok/warn/fail | N activos, [sin usar si hay] |
| Hooks | ok/warn/fail | N hooks, [rotos si hay] |
| Memory | ok/warn/fail | N memorias, [huerfanas si hay] |
| Permissions | ok/warn/fail | N reglas, [obsoletas si hay] |

### CLAUDE.md
| Archivo | Lineas | Estado | Problemas |
|---------|--------|--------|-----------|
| ~/.claude/CLAUDE.md | N | ok/warn/fail | [lista] |
| ./CLAUDE.md | N | ok/warn/fail | [lista] |

### Codigo
| Area | Hallazgos |
|------|-----------|
| Codigo muerto (intra-layer) | [lista o "limpio"] |
| Codigo muerto (cross-layer) | [lista o "limpio"] |
| Tests sospechosos | [lista o "limpio"] |

### Acciones sugeridas
1. [accion concreta 1]
2. [accion concreta 2]
```

## Paso 5: Proponer fixes
Para cada problema encontrado, preguntar al usuario si quiere que lo arregle. Agrupar los fixes por prioridad:
- **Critico**: hooks rotos, scripts que no existen, syntax errors
- **Importante**: codigo muerto (especialmente cross-layer), tests vacios, CLAUDE.md bloat
- **Menor**: memorias huerfanas, permissions obsoletas

NO arreglar nada sin confirmacion del usuario.

## MUST NOT DO
- NO borrar archivos sin confirmacion
- NO modificar settings.json sin confirmacion
- NO tocar hooks sin verificar que no rompe nada
- NO reportar false positives — si no estas seguro de que algo es codigo muerto, no lo reportes
- NO sugerir cambios cosmeticos que no tienen impacto real

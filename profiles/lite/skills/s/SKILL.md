Selector inteligente de agentes. Analiza el input, recomienda agente(s), el usuario elige, y lanza los seleccionados. $ARGUMENTS es la tarea a resolver. El usuario puede pedir multiples instancias del mismo agente (ej: "2 opus", "3 sonnet") para obtener perspectivas diferenciadas.

## Proceso

### Paso 1: Analizar el input

Parsear $ARGUMENTS y determinar:
- Tipo de tarea (implementacion, review, investigacion, validacion, debugging, UX, etc.)
- Complejidad (simple, media, alta)
- Archivos relevantes del proyecto (buscar si aplica)
- Si requiere acceso a filesystem externo (Codex/Gemini) o razonamiento interno (Claude models)

### Paso 2: Verificar disponibilidad de agentes externos

Antes de mostrar el menu, verificar que agentes estan disponibles:

**Codex:**
```bash
codex exec "echo ok" 2>/dev/null
```

**Gemini:**
1. Leer `~/.cvm/available-tools.json` y verificar `gemini.available == true`
2. Si el archivo no existe o no es parseable, fallback: `which gemini 2>/dev/null`
3. Si ambos fallan: Gemini no disponible

Marcar los no disponibles en el menu.

### Paso 3: Mostrar menu con recomendacion

Presentar la tabla de agentes marcando disponibilidad. Formato:

```
Tarea: [resumen procesado del input]

Agentes disponibles:

  [1] Opus     — razonamiento profundo, review, arquitectura, debugging complejo
  [2] Sonnet   — implementacion, refactor, tests, codigo estandar
  [3] Haiku    — busqueda rapida, lectura, preguntas facticas
  [4] Codex    — validacion externa, second opinion [NO DISPONIBLE]
  [5] Gemini   — validacion externa, perspectiva alternativa [NO DISPONIBLE]

  Recomendacion: [N, N] — [por que]

Selecciona (ej: 1,4 | multiples del mismo: 2x opus):
```

La recomendacion se basa en:
- **Opus**: razonamiento profundo, analisis arquitectural, debugging complejo, review critico
- **Sonnet**: implementacion, refactor, tests, tarea de codigo directa
- **Haiku**: busqueda, lectura, pregunta factica simple
- **Codex**: second opinion externa, validacion independiente
- **Gemini**: perspectiva alternativa, triangulacion

Para tareas de validacion/review: recomendar 2+ agentes.
Para tareas de implementacion directa: recomendar 1.

No ofrecer Codex ni Gemini si no estan disponibles.

### Paso 4: Esperar seleccion del usuario

El usuario responde con numeros y cantidades:
- `1,4` — Opus + Codex (1 de cada uno)
- `2x opus` o `2 opus` — 2 instancias de Opus
- `3x sonnet, 1 opus` — mix de instancias
- `todos` — todos los disponibles (1 de cada uno)

Si el input es ambiguo, pedir confirmacion antes de lanzar.
Si el usuario selecciona un agente marcado como no disponible, avisar y pedir que elija otro.

### Paso 5: Armar prompts

Para CADA instancia, armar un prompt independiente y estructurado:

1. **Contexto**: proyecto, stack, area del codigo, paths relevantes
2. **Tarea**: reformulada con precision para el rol especifico del agente
3. **Restricciones**: scope, que NO hacer
4. **Output esperado**: formato concreto del resultado

Adaptar el prompt al rol:
- **Opus**: enfatizar analisis, trade-offs, argumentar decisiones
- **Sonnet**: enfatizar implementacion concreta, archivos a modificar
- **Haiku**: enfatizar concision, ir al grano, listar hallazgos
- **Codex/Gemini**: darle paths (NUNCA contenido inline), instrucciones claras de output

**Cuando hay multiples instancias del mismo agente**, diferenciar el angulo de cada una:
- **Instancia 1**: enfoque directo — resolver como viene
- **Instancia 2**: enfoque critico — cuestionar supuestos, buscar problemas
- **Instancia 3+**: enfoques alternativos — soluciones no convencionales, priorizar simplicidad, otros trade-offs

Cada prompt incluye su angulo explicito. NUNCA lanzar N instancias con el mismo prompt.

Cada prompt de Claude subagent DEBE terminar con: "Termina tu respuesta con una seccion `## Key Learnings:` listando descubrimientos no-obvios."

Para Codex/Gemini: escribir cada prompt en un archivo temporal con Write tool para evitar shell injection.

### Paso 6: Lanzar agentes

Lanzar todos los seleccionados en paralelo en el mismo mensaje. Cada rama replica el flujo del skill `/o` unificado (Opus via Agent tool, Codex/Gemini via Bash + CLI externo; ver `profiles/lite/skills/o/SKILL.md`):

- **Opus/Sonnet/Haiku** (equivalente a `/o` / `/o --opus`): `Agent(subagent_type: "general-purpose", model: "<model>")` — multiples Agent calls
- **Codex** (equivalente a `/o --codex`): `codex exec "$(cat /tmp/cvm-codex-prompt-N.txt)" 2>&1` — Bash tool call separado por instancia
- **Gemini** (equivalente a `/o --gemini`): `gemini -p "$(cat /tmp/cvm-gemini-prompt-N.txt)" 2>&1` — Bash tool call separado por instancia

Sin timeout. Esperar a que todos terminen.

### Paso 7: Consolidar y reportar

**Si se lanzo 1 solo agente**: mostrar su resultado directo.

**Si se lanzaron 2+ (mismo o distinto tipo)**: presentar vista consolidada:

```
## Resultados

### Opus #1 (directo)
[resultado]

### Opus #2 (critico)
[resultado]

### Sonnet
[resultado]

## Sintesis
- Puntos en comun: [...]
- Divergencias: [...]
- Recomendacion: [...]
```

## MUST DO
- Verificar disponibilidad de Codex/Gemini ANTES de mostrar el menu (con fallback para Gemini)
- Siempre mostrar el menu con recomendacion antes de lanzar
- Esperar la seleccion del usuario — no asumir
- Diferenciar el angulo de cada instancia cuando hay multiples del mismo agente
- Incluir instruccion de `## Key Learnings:` en cada prompt de Claude subagent
- Escribir prompts de Codex/Gemini en archivos temporales, no interpolar en shell
- Parsear y enriquecer cada prompt por separado
- Lanzar todo en paralelo
- Sintetizar cuando hay multiples resultados
- Si input ambiguo, pedir confirmacion

## MUST NOT DO
- No lanzar agentes sin que el usuario seleccione
- No ofrecer Codex/Gemini si no estan disponibles
- No hacer pass-through del input como prompt
- No lanzar N instancias con el mismo prompt identico
- No interpolar texto del usuario en double-quoted shell commands
- No usar & + wait + archivos de salida — usar tool calls separadas
- No agregar timeout

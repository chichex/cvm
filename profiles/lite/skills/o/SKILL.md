Lanzar uno o mas subagents Opus para tareas que requieren razonamiento profundo. $ARGUMENTS es la tarea o pregunta. Si el usuario pide multiples instancias (ej: "lanza 3"), se lanzan N agentes en paralelo con enfoques diferenciados.

## Proceso

### Paso 1: Parsear el input

Analizar $ARGUMENTS y determinar:
- **Tarea**: que se necesita resolver
- **Cantidad**: si el usuario pide N instancias (ej: "lanza 2", "quiero 3 opiniones"). Default: 1
- **Tipo**: review, investigacion, diseno, debugging, general

### Paso 2: Armar prompts

**Si es 1 instancia**: armar un prompt estructurado con:
1. **Contexto**: proyecto, stack, area del codigo, paths relevantes
2. **Tarea**: reformulada con precision
3. **Restricciones**: que NO hacer, limites de scope
4. **Output esperado**: formato concreto del resultado

**Si son N instancias**: armar N prompts diferenciados. Cada agente ataca el problema desde un angulo distinto:
- **Agente 1**: enfoque directo — resolver como viene
- **Agente 2**: enfoque critico — cuestionar supuestos, buscar problemas
- **Agente 3+**: enfoques alternativos — proponer soluciones no convencionales, priorizar simplicidad, considerar trade-offs diferentes

Cada prompt incluye su angulo explicito para que el agente sepa que rol juega.

Cada prompt DEBE terminar con: "Termina tu respuesta con una seccion `## Key Learnings:` listando descubrimientos no-obvios."

### Paso 3: Lanzar agentes

Lanzar todos como `Agent(subagent_type: "general-purpose", model: "opus")` en paralelo (todos en el mismo mensaje).

Sin timeout. Esperar a que todos terminen.

### Paso 4: Reportar

**Si es 1**: mostrar el resultado directo.

**Si son N**: presentar cada resultado con su angulo, y agregar un bloque de sintesis:

```
## Agente 1 (directo)
[resultado]

## Agente 2 (critico)
[resultado]

## Sintesis
- Puntos en comun: [...]
- Divergencias: [...]
- Recomendacion: [...]
```

## MUST DO
- Parsear y enriquecer cada prompt — no hacer pass-through
- Diferenciar el angulo de cada agente cuando son N
- Incluir instruccion de `## Key Learnings:` en cada prompt
- Lanzar todos en paralelo
- Incluir paths a archivos relevantes
- Sintetizar cuando hay multiples resultados

## MUST NOT DO
- No pasar el input tal cual sin procesar
- No lanzar N agentes con el mismo prompt identico
- No agregar timeout

Lanzar uno o mas subagents para tareas que requieren razonamiento profundo o validacion externa. Default: Opus via Agent tool. Flags `--codex` / `--gemini` delegan a CLIs externos. $ARGUMENTS es la tarea o pregunta. Si el usuario pide multiples instancias (ej: "lanza 3"), se lanzan N en paralelo con enfoques diferenciados.

## Proceso

### Paso 0: Parsear flag de provider

Inspeccionar $ARGUMENTS y detectar el primer flag de provider:

- `--codex` → rama Codex
- `--gemini` → rama Gemini
- `--opus` o sin flag → rama Opus (default)

Remover el flag de $ARGUMENTS antes de pasarlo a los pasos siguientes. Solo se acepta un provider por invocacion; si hay mas de uno, avisar y pedir que el usuario elija.

### Paso 1: Parsear el input

Analizar $ARGUMENTS (ya sin flag) y determinar:
- **Tarea**: que se necesita resolver
- **Cantidad**: si el usuario pide N instancias (ej: "lanza 2", "quiero 3 opiniones"). Default: 1
- **Tipo**: review, investigacion, diseno, debugging, general
- **Archivos**: paths relevantes (especialmente para Codex/Gemini, que NUNCA reciben contenido inline)
- **PR context** (solo Codex/Gemini): si hay PR abierto (`gh pr view --json number 2>/dev/null`)

### Paso 2: Armar prompts

**Si es 1 instancia**: armar un prompt estructurado con:
1. **Contexto**: proyecto, stack, area del codigo, paths relevantes
2. **Tarea**: reformulada con precision
3. **Restricciones**: que NO hacer, limites de scope
4. **Output esperado**: formato concreto del resultado

**Si son N instancias**: armar N prompts diferenciados. Cada instancia ataca el problema desde un angulo distinto:
- **Instancia 1**: enfoque directo — resolver como viene
- **Instancia 2**: enfoque critico — cuestionar supuestos, buscar problemas, edge cases, vulnerabilidades
- **Instancia 3+**: enfoques alternativos — proponer soluciones no convencionales, priorizar simplicidad, considerar trade-offs diferentes

Cada prompt incluye su angulo explicito para que sepa que rol juega. NUNCA lanzar N instancias con el mismo prompt.

**Solo rama Opus**: cada prompt DEBE terminar con: "Termina tu respuesta con una seccion `## Key Learnings:` listando descubrimientos no-obvios."

**Ramas Codex/Gemini**: escribir cada prompt en un archivo temporal con Write tool para evitar shell injection:

```
/tmp/cvm-codex-prompt-1.txt
/tmp/cvm-gemini-prompt-1.txt
```

### Paso 3: Lanzar (depende de la rama)

#### Rama Opus (default)

Lanzar todas las instancias como `Agent(subagent_type: "general-purpose", model: "opus")` en paralelo (todos en el mismo mensaje).

#### Rama Codex

Antes de lanzar, verificar disponibilidad:

```bash
codex exec "echo ok" 2>/dev/null
```

Si falla: informar al usuario que Codex no esta disponible y sugerir omitir el flag para fallback a Opus (`/go <tarea>` sin `--codex`).

Lanzar cada instancia como un Bash tool call separado en paralelo (multiples Bash calls en el mismo mensaje). Leer el stdout directo del tool output:

```bash
codex exec "$(cat /tmp/cvm-codex-prompt-1.txt)" 2>&1
```

#### Rama Gemini

Verificar disponibilidad en este orden:
1. Leer `~/.cvm/available-tools.json` y verificar `gemini.available == true`
2. Si el archivo no existe o no es parseable, fallback: `which gemini 2>/dev/null`
3. Si ambos fallan: informar al usuario y sugerir omitir el flag para fallback a Opus.

Lanzar cada instancia como un Bash tool call separado en paralelo:

```bash
gemini -p "$(cat /tmp/cvm-gemini-prompt-1.txt)" 2>&1
```

Sin timeout en ninguna rama. Esperar a que todos terminen.

### Paso 4: Reportar

**Si es 1**: mostrar el resultado directo.

**Si son N**: presentar cada resultado con su angulo y un bloque de sintesis:

```
## <Provider> 1 (directo)
[resultado]

## <Provider> 2 (critico)
[resultado]

## Sintesis
- Puntos en comun: [...]
- Divergencias: [...]
- Recomendacion: [...]
```

### Paso 5: Evaluar aprendizajes (solo rama Opus)

Despues de reportar, ejecutar `/r` para evaluar y persistir aprendizajes de la sesion.

Las ramas Codex/Gemini no ejecutan `/r` automaticamente.

## MUST DO
- Parsear el flag de provider antes de nada
- Parsear y enriquecer cada prompt — no hacer pass-through
- Diferenciar el angulo de cada instancia cuando son N
- Lanzar todas las instancias en paralelo
- Incluir paths a archivos relevantes
- Sintetizar cuando hay multiples resultados
- Rama Opus: incluir instruccion de `## Key Learnings:` en cada prompt y ejecutar `/r` al final
- Rama Codex/Gemini: verificar disponibilidad, escribir prompts en `/tmp/` con Write tool, usar Bash calls separadas
- Rama Codex/Gemini: dar paths a archivos, NUNCA contenido inline

## MUST NOT DO
- No pasar el input tal cual sin procesar
- No lanzar N instancias con el mismo prompt identico
- No agregar timeout
- No interpolar texto del usuario en double-quoted shell commands
- No usar & + wait + archivos de salida — usar Bash calls separadas
- No lanzar por la rama Codex/Gemini si el CLI no esta disponible
- No aceptar mas de un flag de provider en la misma invocacion

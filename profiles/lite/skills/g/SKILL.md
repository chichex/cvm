Lanzar una o mas instancias de Gemini para validacion o ejecucion externa. $ARGUMENTS es la tarea. Si el usuario pide multiples instancias (ej: "lanza 2"), se lanzan N en paralelo con enfoques diferenciados.

## Proceso

### Paso 1: Verificar disponibilidad

Verificar en este orden:
1. Leer `~/.cvm/available-tools.json` y verificar `gemini.available == true`
2. Si el archivo no existe o no es parseable, fallback: `which gemini 2>/dev/null`
3. Si ambos fallan: informar al usuario que Gemini no esta disponible y sugerir `/o` como alternativa

### Paso 2: Parsear el input

Analizar $ARGUMENTS y determinar:
- **Tarea**: que necesita hacer Gemini
- **Cantidad**: si el usuario pide N instancias. Default: 1
- **Archivos**: paths que necesita leer (no contenido)
- **PR context**: si hay PR abierto (`gh pr view --json number 2>/dev/null`)

### Paso 3: Armar prompts

**Si es 1 instancia**: armar un prompt con:
1. **Tarea**: que hacer, con precision
2. **Archivos**: paths que debe leer (NUNCA contenido inline)
3. **Contexto PR**: si hay PR abierto, incluir `gh pr diff <number>` como instruccion
4. **Output**: formato esperado

**Si son N instancias**: armar N prompts diferenciados:
- **Instancia 1**: enfoque directo
- **Instancia 2**: enfoque critico — buscar problemas, edge cases, vulnerabilidades
- **Instancia 3+**: enfoques alternativos — proponer otra solucion, priorizar performance, etc.

NUNCA lanzar N instancias con el mismo prompt.

Gemini tiene acceso al filesystem. Darle paths para que lea directamente.

### Paso 4: Lanzar Gemini

Escribir cada prompt en un archivo temporal con Write tool para evitar shell injection:
```
/tmp/cvm-gemini-prompt-1.txt
/tmp/cvm-gemini-prompt-2.txt
```

Lanzar cada instancia como un Bash tool call separado en paralelo (multiples Bash calls en el mismo mensaje). Leer el stdout directo del tool output:

```bash
gemini -p "$(cat /tmp/cvm-gemini-prompt-1.txt)" 2>&1
```

Sin timeout. Esperar a que todos terminen.

### Paso 5: Reportar

**Si es 1**: mostrar el resultado directo.

**Si son N**: presentar cada resultado con su angulo y un bloque de sintesis:

```
## Gemini 1 (directo)
[resultado]

## Gemini 2 (critico)
[resultado]

## Sintesis
- Puntos en comun: [...]
- Divergencias: [...]
```

## MUST DO
- Verificar disponibilidad con fallback (available-tools.json → which gemini)
- Dar paths a archivos, NUNCA contenido inline
- Diferenciar el angulo de cada instancia cuando son N
- Escribir prompts en archivos temporales — no interpolar en shell
- Lanzar cada instancia como Bash tool call separado en paralelo
- Sintetizar cuando hay multiples resultados

## MUST NOT DO
- No pasar contenido inline de archivos en prompts
- No interpolar texto del usuario en double-quoted shell commands
- No lanzar N instancias con el mismo prompt identico
- No usar & + wait + archivos de salida — usar Bash calls separadas
- No agregar timeout
- No lanzar si Gemini no esta disponible

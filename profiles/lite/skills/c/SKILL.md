Lanzar una o mas instancias de Codex para validacion o ejecucion externa. $ARGUMENTS es la tarea. Si el usuario pide multiples instancias (ej: "lanza 2"), se lanzan N en paralelo con enfoques diferenciados.

## Proceso

### Paso 1: Verificar disponibilidad

```bash
codex exec "echo ok" 2>/dev/null
```

Si falla: informar al usuario que Codex no esta disponible y sugerir `/o` como alternativa.

### Paso 2: Parsear el input

Analizar $ARGUMENTS y determinar:
- **Tarea**: que necesita hacer Codex
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

Si necesita ver multiples archivos, escribir un manifiesto con Write tool (no echo en shell):
```
/tmp/cvm-codex-manifest.txt
```

### Paso 4: Lanzar Codex

Escribir cada prompt en un archivo temporal con Write tool para evitar shell injection:
```
/tmp/cvm-codex-prompt-1.txt
/tmp/cvm-codex-prompt-2.txt
```

Lanzar cada instancia como un Bash tool call separado en paralelo (multiples Bash calls en el mismo mensaje). Leer el stdout directo del tool output:

```bash
codex exec "$(cat /tmp/cvm-codex-prompt-1.txt)" 2>&1
```

Sin timeout. Esperar a que todos terminen.

### Paso 5: Reportar

**Si es 1**: mostrar el resultado directo.

**Si son N**: presentar cada resultado con su angulo y un bloque de sintesis:

```
## Codex 1 (directo)
[resultado]

## Codex 2 (critico)
[resultado]

## Sintesis
- Puntos en comun: [...]
- Divergencias: [...]
```

## MUST DO
- Verificar disponibilidad antes de lanzar
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
- No lanzar si Codex no esta disponible

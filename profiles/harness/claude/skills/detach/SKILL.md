Ejecuta `$ARGUMENTS` como tarea en un subagent aislado y devuelve solo un resumen corto al main thread. El proposito es preservar el contexto del thread principal cuando la tarea siguiente es ruidosa (lee muchos archivos, hace busquedas grandes, corre comandos largos, produce mucho output, etc). El main thread recibe solo el `## Result` final, no la transcripcion del subagent.

Skill **exclusivo para Claude Code** (depende del tool `Agent`).

## Pre-flight

### 1. Parsear `$ARGUMENTS`
- Trim whitespace. Guardar como `TASK`.
- Si `TASK` esta vacio: pedir `Pasame la tarea a delegar como prompt en lenguaje natural. Ej: /detach revisa el flujo de auth en internal/http/.` y esperar respuesta. Re-parsear.

### 2. Aviso sobre aislamiento (solo si aplica)
Si `TASK` arranca con `/` (parece un slash command), avisar una sola vez:
```
Aviso: los slash commands no siempre se resuelven dentro de un subagent. Te recomiendo pasar la tarea en lenguaje natural. Continuo igual? (si/no)
```
Si dice `no`, abortar. Si `si`, seguir.

No pedir confirmacion en ningun otro caso — `/detach` es un envoltorio liviano y la decision la tomo el usuario al tipear el comando.

## Delegar

Llamar `Agent` tool con:
- `subagent_type: "general-purpose"`
- `description`: 3-5 palabras que resuman `TASK`
- `prompt`:

```
<TASK>

---
IMPORTANTE — formato de output:
Tu respuesta vuelve al main thread del usuario, que esta preservando contexto. Devolve UN SOLO bloque de maximo 3 lineas con este formato exacto, sin texto adicional arriba ni abajo:

## Result
- status: <ok | partial | error>
- summary: <una linea con lo que hiciste o encontraste>
- artifacts: <urls, paths a archivos creados/modificados, PR #, o "ninguno">

No incluyas razonamiento, transcripcion de pasos, ni explicaciones largas. Si el usuario necesita detalle, vendra a pedirlo.
```

## Cierre

Imprimir el bloque `## Result` tal cual lo devolvio el subagent. No envolver, no parafrasear, no agregar headers extra.

Si el subagent no devolvio el formato esperado, imprimir:
```
## Result
- status: error
- summary: el subagent no devolvio el formato esperado
- artifacts: ninguno
```
y debajo, una linea: `Output crudo del subagent guardado para inspeccion: <primeras 200 chars>...`.

## MUST DO

- Delegar al subagent `general-purpose` con el `TASK` verbatim mas las instrucciones de formato.
- Mantener el output del main thread al minimo — solo el `## Result`.
- Trim whitespace en `$ARGUMENTS` antes de procesar.

## MUST NOT DO

- No leer archivos del repo, no correr comandos, no explorar nada desde el main thread — todo eso es trabajo del subagent.
- No pedir confirmacion salvo el caso especifico de slash command al inicio.
- No persistir nada en auto-memory.
- No imprimir el razonamiento ni la transcripcion del subagent al main thread.
- No agregar flags (`--bg`, `--agent <type>`, etc) que el usuario no pidio.

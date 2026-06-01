---
name: detach
description: Ejecuta una tarea ruidosa en un subagent aislado y devuelve solo un resumen corto al thread principal. Usar cuando el usuario invoca /detach o quiere preservar contexto ante busquedas, lecturas o comandos largos.
---

Ejecuta los argumentos como tarea en un subagent aislado y devuelve solo un resumen corto al thread principal. El proposito es preservar el contexto del thread principal cuando la tarea siguiente es ruidosa: lee muchos archivos, hace busquedas grandes, corre comandos largos o produce mucho output.

Skill **one-shot**: no persiste estado. En OpenCode usa el tool `Task` con `subagent_type: general`.

## Argumentos

```text
/detach <tarea en lenguaje natural>
```

- Si los argumentos estan vacios: pedir `Pasame la tarea a delegar como prompt en lenguaje natural. Ej: /detach revisa el flujo de auth en internal/http/.` y esperar respuesta.
- Si la tarea arranca con `/` (parece un slash command), avisar una sola vez: `Aviso: los slash commands no siempre se resuelven dentro de un subagent. Te recomiendo pasar la tarea en lenguaje natural. Continuo igual? (si/no)`. Si dice `no`, abortar. Si dice `si`, seguir.

## Delegar

Llamar `Task` con:

- `subagent_type: general`
- `description`: 3-5 palabras que resuman la tarea.
- `prompt`: la tarea verbatim mas este bloque al final:

```text
---
IMPORTANTE - formato de output:
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

```text
## Result
- status: error
- summary: el subagent no devolvio el formato esperado
- artifacts: ninguno
```

## MUST DO

- Delegar al subagent `general` con la tarea verbatim mas las instrucciones de formato.
- Mantener el output del main thread al minimo: solo el `## Result`.
- Trim whitespace en los argumentos antes de procesar.

## MUST NOT DO

- No leer archivos del repo, no correr comandos, no explorar nada desde el main thread: todo eso es trabajo del subagent.
- No pedir confirmacion salvo el caso especifico de slash command al inicio.
- No persistir nada en memoria.
- No imprimir razonamiento ni transcripcion del subagent al main thread.
- No agregar flags (`--bg`, `--agent <type>`, etc.) que el usuario no pidio.

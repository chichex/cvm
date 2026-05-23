Modo full-detach para preservar el context window principal. Una vez activado, el orquestador delega a subagents todas las operaciones "ruidosas" (exploraciones multi-archivo, busquedas grandes, lecturas extensas, web fetches, comandos `Bash` con output grande) y solo conserva inline lo trivial (1 `Read` corto, 1 `Edit` puntual, conversacion, judgment del orquestador). Usar cuando el usuario dice "modo delegator", "delega todo", "preserva contexto" o invoca `/delegator`.

Skill **persistente**: una vez activado, sigue activo en cada respuesta hasta que el usuario diga "modo normal", "stop delegator" o invoque `/delegator off`. Sin drift hacia ejecucion inline con el correr de los turnos. Si dudas si seguir activo, segui activo.

Skill **exclusivo para Claude Code** (depende del tool `Agent`).

## Argumentos

```text
/delegator [on|off]
```

- Default (`$ARGUMENTS` vacio o `on`): activa el modo.
- `off`: desactiva. Volves a ejecutar inline normalmente.
- Cualquier otro valor: ignorar y asumir `on`.

## Que se delega

Toda operacion que tenga huella significativa en context. Antes de ejecutar inline, preguntate: "esto va a meter mas de ~200 lineas de output o requiere mas de 2 lecturas/busquedas?". Si si, delegar.

Catalogo de delegacion obligatoria:

- **Exploracion de codigo multi-archivo**: mapear modulos, encontrar todos los call sites, entender estructura de un area. Subagent: `Explore`.
- **Busquedas grandes**: `Grep`/`Glob` con patrones amplios, recursivas en arboles profundos. Subagent: `Explore`.
- **Lecturas extensas**: archivos > 500 lineas, lecturas multiples para entender un flujo. Subagent: `Explore` o `general-purpose`.
- **Web**: `WebFetch`, `WebSearch`. Subagent: `general-purpose`.
- **Comandos `Bash` ruidosos**: builds completos, test suites largas, logs verbose, `find` recursivos. Subagent: `general-purpose`.
- **Investigaciones multi-paso**: "averigua por que X falla", "revisa el flujo Y". Subagent: `general-purpose`.

## Que NO se delega

Lo inline se queda inline. Delegar todo agrega latencia inutil y rompe la conversacion.

- Respuestas conversacionales cortas, aclaraciones, judgment del orquestador.
- 1 `Read` con path conocido y archivo chico.
- 1 `Edit`/`Write` puntual cuando ya sabes que tocar.
- Decisiones que requieren contexto del thread (clarify, ask user, multiple choice).
- Warnings destructivos y confirmaciones criticas.
- `TaskCreate`/`TaskUpdate`/`TaskList` — el tracking vive en el orquestador.
- Llamadas al tool `Skill` (invocar otros skills).

## Como delegar

Para cada delegacion:

### 1. Anunciar (una linea)

Antes de la llamada `Agent`, imprimir una linea exacta:

```
Delegando: <tarea breve, max 60 chars>
```

Sin verbosidad extra, sin razonamiento. El usuario tiene que ver de un vistazo que se esta sacando del thread.

### 2. Llamar `Agent`

- `subagent_type`: elegir segun la naturaleza (ver catalogo arriba). Default: `general-purpose`. Para busquedas/lecturas read-only puras: `Explore`.
- `description`: 3-5 palabras.
- `prompt`: contexto autocontenido + esta coletilla al final:

  ```
  ---
  IMPORTANTE — formato de output:
  Tu respuesta vuelve al main thread del usuario, que esta preservando contexto. Devolve UN SOLO bloque de maximo 5 lineas con este formato exacto, sin texto adicional arriba ni abajo:

  ## Result
  - status: <ok | partial | error>
  - summary: <una linea con lo que hiciste o encontraste>
  - findings: <hasta 2 bullets con datos concretos: paths, line numbers, valores, "ninguno">
  - artifacts: <urls, paths a archivos creados/modificados, PR #, o "ninguno">

  No incluyas razonamiento, transcripcion de pasos, ni explicaciones largas. Si el usuario necesita detalle, vendra a pedirlo.
  ```

### 3. Mostrar resultado

Imprimir el bloque `## Result` tal cual lo devolvio el subagent. No envolver, no parafrasear, no agregar headers extra.

Si el subagent no devolvio el formato esperado, imprimir:

```
## Result
- status: error
- summary: el subagent no devolvio el formato esperado
- findings: ninguno
- artifacts: ninguno
```

## Excepcion auto-clarity

Salir temporalmente del delegator (ejecutar inline) cuando delegar agrega latencia inutil o rompe la coherencia:

- Iteracion corta sobre un archivo ya leido en el thread.
- Pregunta de seguimiento del usuario sobre algo que acabas de delegar (la info ya esta en el `## Result` previo o un `Read` puntual la cubre).
- Warnings de operaciones destructivas (`rm -rf`, `DROP TABLE`, `git push --force`, `git reset --hard`) — la decision tiene que estar visible en el thread principal.
- Confirmaciones de acciones irreversibles (deploy a prod, merge a main, borrar branch).
- Edits chicos a archivos cuyo contenido ya esta en context.

Volver al modo delegator apenas pase la parte critica. No avisarlo, solo retomar.

## Coexistencia con otros modos

- **`/caveman`**: ortogonales. Delegator decide **que** sale del thread; caveman decide **como** se escribe lo que queda inline. Pueden estar activos a la vez.
- **`/detach`**: redundante mientras delegator esta activo (el modo ya delega por default). Si el usuario invoca `/detach` explicitamente, respetarlo igual.

## MUST DO

- Aplicar las reglas de delegacion en cada turno hasta que el usuario apague el modo.
- Anunciar cada delegacion con una linea `Delegando: <tarea>` antes de llamar `Agent`.
- Elegir `subagent_type` segun naturaleza: `Explore` para read-only puro, `general-purpose` para multi-paso.
- Devolver al usuario solo el bloque `## Result` del subagent, sin re-wrapping.
- Salir temporalmente del modo para warnings destructivos, iteracion corta sobre context ya cargado, y confirmaciones criticas.

## MUST NOT DO

- No driftear hacia ejecucion inline con el correr de los turnos. Si dudas si seguir activo, segui activo.
- No delegar lo trivial (1 Read corto, 1 Edit puntual, conversacion, judgment del orquestador) — agrega latencia inutil.
- No delegar `TaskCreate`/`TaskUpdate`/`TaskList` ni llamadas al tool `Skill` — el tracking y el control de flujo viven en el orquestador.
- No imprimir el razonamiento ni la transcripcion del subagent al main thread.
- No persistir el toggle en auto-memory — el estado vive en el thread actual.
- No agregar flags propios al `Agent` que el contexto no justifique.

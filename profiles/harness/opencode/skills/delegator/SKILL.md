---
name: delegator
description: Activa un modo persistente de delegacion para preservar contexto. Usar cuando el usuario dice modo delegator, delega todo, preserva contexto, /delegator on u /delegator off.
---

Modo full-detach para preservar el context window principal. Una vez activado, el orquestador delega a subagents todas las operaciones ruidosas y solo conserva inline lo trivial: conversacion, judgment, 1 lectura corta o 1 edicion puntual.

Skill **persistente en el thread**: una vez activado, sigue activo en cada respuesta hasta que el usuario diga `modo normal`, `stop delegator` o invoque `/delegator off`. Si dudas si seguir activo, segui activo.

## Argumentos

```text
/delegator [on|off]
```

- Default, sin argumentos u `on`: activar el modo.
- `off`: desactivar y volver a ejecucion inline normal.
- Cualquier otro valor: ignorar y asumir `on`.

## Que Se Delega

Antes de ejecutar inline, preguntate: esto va a meter mas de 200 lineas de output o requiere mas de 2 lecturas/busquedas? Si si, delegar.

Catalogo de delegacion obligatoria:

- Exploracion de codigo multi-archivo: usar `Task` con `subagent_type: explore`.
- Busquedas grandes: usar `Task` con `subagent_type: explore`.
- Lecturas extensas: usar `Task` con `subagent_type: explore` o `general`.
- Web fetches o investigaciones multi-paso: usar `Task` con `subagent_type: general`.
- Comandos ruidosos: builds completos, suites largas, logs verbose: usar `Task` con `subagent_type: general`.

## Que No Se Delega

- Respuestas conversacionales cortas, aclaraciones y judgment del orquestador.
- 1 `Read` con path conocido y archivo chico.
- 1 edicion puntual cuando ya sabes que tocar.
- Decisiones que requieren contexto del thread o preguntas al usuario.
- Warnings destructivos y confirmaciones criticas.
- `TodoWrite` y llamadas a `Skill`: el tracking y control de flujo viven en el orquestador.

## Como Delegar

Antes de cada `Task`, imprimir una linea exacta:

```text
Delegando: <tarea breve, max 60 chars>
```

Llamar `Task` con:

- `subagent_type`: `explore` para read-only puro; `general` para multi-paso o comandos.
- `description`: 3-5 palabras.
- `prompt`: contexto autocontenido mas este bloque al final:

```text
---
IMPORTANTE - formato de output:
Tu respuesta vuelve al main thread del usuario, que esta preservando contexto. Devolve UN SOLO bloque de maximo 5 lineas con este formato exacto, sin texto adicional arriba ni abajo:

## Result
- status: <ok | partial | error>
- summary: <una linea con lo que hiciste o encontraste>
- findings: <hasta 2 bullets con datos concretos: paths, line numbers, valores, o "ninguno">
- artifacts: <urls, paths a archivos creados/modificados, PR #, o "ninguno">

No incluyas razonamiento, transcripcion de pasos, ni explicaciones largas.
```

Mostrar el bloque `## Result` tal cual lo devolvio el subagent. Si no cumple formato, devolver un `## Result` de error.

## Excepcion Auto-Clarity

Salir temporalmente del modo delegator cuando delegar agrega latencia inutil o rompe coherencia: iteracion corta sobre un archivo ya leido, pregunta de seguimiento sobre un resultado delegado, warnings destructivos, confirmaciones irreversibles o edits chicos con contexto cargado. Retomar el modo apenas pase la parte critica.

## MUST DO

- Aplicar estas reglas en cada turno hasta que el usuario apague el modo.
- Anunciar cada delegacion con `Delegando: <tarea>`.
- Elegir `explore` para read-only puro y `general` para multi-paso.
- Devolver solo el bloque `## Result` del subagent.

## MUST NOT DO

- No driftear hacia ejecucion inline con el correr de los turnos.
- No delegar lo trivial.
- No delegar `TodoWrite` ni llamadas al tool `Skill`.
- No imprimir razonamiento ni transcripcion del subagent.
- No persistir el toggle fuera del thread actual.

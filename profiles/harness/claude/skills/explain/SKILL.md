Explica un concepto, snippet de codigo, error o decision tecnica en **3 niveles de abstraccion crecientes** (ELI5 / Dev junior / Experto). One-shot: cada invocacion produce los 3 niveles en una sola respuesta y termina. Usar cuando el usuario invoca `/explain`, pide "explicame X", "no entiendo Y", o dice "dame esto en varios niveles".

## Argumentos

```text
/explain [tema]
```

- Con `$ARGUMENTS`: explica el tema/concepto/snippet pasado como argumento.
- Sin `$ARGUMENTS`: explica lo ultimo relevante del contexto (ultimo mensaje del usuario, codigo que se esta viendo, error reciente). Citar al inicio que pedazo del thread se esta explicando.

## Estructura de salida

Tres bloques en este orden exacto:

### Nivel 1 — ELI5

- Analogia cotidiana o frase unica sin jerga.
- 1 a 2 oraciones max.
- Cero terminos tecnicos (o explicados al toque).
- Foco en "que es" y "para que sirve", no en "como funciona".

### Nivel 2 — Dev junior

- Vocabulario tecnico estandar, sin asumir internals.
- 3 a 5 oraciones o lista corta.
- Mecanismo a alto nivel: que partes interactuan, que problema resuelve, ejemplo simple si suma.
- Asume que el lector programa pero no conoce este tema especifico.

### Nivel 3 — Experto

- Internals, trade-offs, edge cases, performance, alternativas.
- Sin tope rigido pero sin pajarear: lo que un senior necesita para tomar decisiones.
- Referencias concretas (`archivo:linea`, RFC, paper, seccion de docs) cuando apliquen.
- Mencionar simplificaciones que el Nivel 2 dejo afuera.

## Reglas

- Los 3 niveles cubren el **mismo** concepto — no temas relacionados.
- Cada nivel agrega sobre el anterior, no se contradicen.
- Nivel 1 no usa palabras del Nivel 2/3 sin definirlas.
- Nivel 3 no repite Nivel 2 — profundiza o muestra trade-offs.
- Si el tema no tiene un "ELI5 honesto" (ej: detalles de un protocolo binario muy especifico), decirlo explicitamente en Nivel 1 y dar la analogia mas cercana posible.
- Si el Nivel 3 no tiene sustancia experta real para agregar, decirlo en vez de inflar.

## Ejemplos

**Input**: `/explain connection pooling`

> ### Nivel 1 — ELI5
> Una pizzeria con 5 deliveries siempre con la moto prendida. En vez de arrancar la moto cada vez que llega un pedido, ya estan listos para salir. Connection pooling es lo mismo pero con conexiones a una base de datos.
>
> ### Nivel 2 — Dev junior
> Abrir una conexion a DB es caro: handshake TCP, auth, setup SSL/TLS. Un pool mantiene N conexiones abiertas y las presta a quien las pide; cuando el caller termina, la conexion vuelve al pool en vez de cerrarse. Tu codigo hace `pool.acquire()` y libera con `pool.release()` (o usa `with`). Si el pool esta lleno, los pedidos hacen cola o tiran timeout.
>
> ### Nivel 3 — Experto
> Sizing tiene que matchear `max_connections` del server y el throughput esperado (`pool_size ≈ concurrent_requests × avg_query_time / response_time_target`). Pools muy grandes pegan contention en el server (locking, ~10MB por conn en PG); muy chicos tiran tail latency. Edge cases: conexiones "muertas" por idle timeout del server requieren keepalive o validation queries (`SELECT 1`). En serverless el pool clasico no sirve — usar PgBouncer en modo transaction o drivers connection-less (HTTP-based, ej: Neon serverless). Watch: pool exhaustion suele esconder leaks de conexion, no falta de capacidad.

**Input**: `/explain` (despues de pegar un stack trace con `EADDRINUSE`)

> Explicando el error `EADDRINUSE` del stack trace.
>
> ### Nivel 1 — ELI5
> Queres enchufar dos cosas en el mismo tomacorriente. El sistema operativo no te deja: ese puerto ya esta siendo usado por otro programa.
>
> ### Nivel 2 — Dev junior
> `EADDRINUSE` aparece cuando intentas `listen()` en un puerto que ya tiene otro proceso atado. Causas comunes: el server anterior no cerro bien (zombie), tenes dos instancias corriendo, o el SO mantiene el socket en estado `TIME_WAIT` post-crash. Fix rapido: `lsof -i :3000` para encontrar el PID, `kill`, reintentar.
>
> ### Nivel 3 — Experto
> `TIME_WAIT` puede durar 2×MSL (≈60s en Linux). Setear `SO_REUSEADDR` permite re-bind durante `TIME_WAIT` pero **no** durante `LISTEN` activo de otro proceso; para eso necesitas `SO_REUSEPORT` (Linux 3.9+), que ademas distribuye conexiones entre procesos via hash de 4-tupla — util para multi-worker sin proxy. En macOS `SO_REUSEPORT` no hace load-balance, solo permite bind. Para sockets Unix domain el equivalente es `unlink()` previo o atomic-rename pattern.

## MUST DO

- Producir los 3 niveles en una sola respuesta, en el orden definido.
- Mantener terminos tecnicos exactos en Niveles 2 y 3 (nombres de funciones, archivos, errores).
- Citar el pedazo del thread que se esta explicando cuando opera sin argumento.
- Si el tema es muy especifico al repo actual, leer el codigo antes de escribir el Nivel 3.

## MUST NOT DO

- No mezclar niveles — sin trade-offs en Nivel 1, sin analogias en Nivel 3.
- No diluir Nivel 3: si no hay sustancia experta real para agregar, decirlo en vez de inflar.
- No persistir el modo — `/explain` es one-shot por diseño. La siguiente respuesta vuelve al estilo normal del thread.
- No traducir terminos tecnicos al español si rompe el sentido (`deadlock` no es "bloqueo mutuo").
- No producir 3 parrafos genericos — cada nivel debe sumar info que el anterior no tenia.

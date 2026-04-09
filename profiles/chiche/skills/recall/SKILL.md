Buscar contexto relevante en la Knowledge Base antes de actuar. $ARGUMENTS son los terminos de busqueda o la descripcion de lo que se va a hacer.

## Proceso

1. Parsear $ARGUMENTS para extraer terminos de busqueda relevantes.

2. Buscar en ambos scopes:
```bash
cvm kb search "<query>"
cvm kb search "<query>" --local
```

3. Si hay resultados, leer las entries relevantes con `cvm kb show <key>` [--local].

4. Presentar un resumen de lo encontrado:
```
KB encontro contexto relevante:
- [key1]: <resumen de una linea>
- [key2]: <resumen de una linea>
```

5. Si no hay resultados, reportar: "No hay contexto previo en KB para este tema."

6. Aplicar el contexto encontrado a la tarea actual.

## MUST DO
- Buscar en AMBOS scopes (global y local)
- Resumir lo encontrado antes de actuar
- Si hay gotchas relevantes, destacarlos prominentemente

## MUST NOT DO
- No ignorar resultados de KB aunque parezcan desactualizados — reportarlos
- No hacer busquedas demasiado amplias que traigan ruido

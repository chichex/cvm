Trigger manual del session summary cuando el hook SessionEnd no corrio (sesion interrumpida, crash, etc).

Este skill genera un resumen estructurado de la sesion actual y lo persiste en KB.

## Proceso

### Paso 1: Revisar la sesion
Escanear la conversacion actual y extraer:
- **Request**: que pidio el usuario (objetivo principal)
- **Accomplished**: que se logro concretamente
- **Discovered**: descubrimientos no-obvios (gotchas, learnings)
- **Next steps**: que queda pendiente

### Paso 2: Verificar duplicados
Buscar si ya existe un summary para hoy:
```bash
cvm kb search "session-summary" --tag "session"
```
Si ya existe uno reciente (misma fecha), actualizar en vez de crear uno nuevo.

### Paso 3: Persistir
```bash
cvm kb put "session-summary-YYYYMMDD" --body "Request: ... | Accomplished: ... | Discovered: ... | Next: ..." --tag "session,summary"
```

### Paso 4: Reporte
Mostrar el summary guardado en formato legible:
```
Session summary guardado:
- Key: session-summary-YYYYMMDD
- Request: ...
- Accomplished: ...
- Discovered: ...
- Next: ...
```

## MUST DO
- Revisar TODA la conversacion, no solo los ultimos mensajes
- Ser conciso: max 1-2 oraciones por campo
- Usar la fecha actual (formato YYYYMMDD)
- Persistir automaticamente sin pedir confirmacion

## MUST NOT DO
- No pedir input al usuario
- No generar summaries vacios o genericos
- No duplicar un summary que ya existe para la misma fecha (actualizar en su lugar)

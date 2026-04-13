Fin de sesion: revisar la conversacion actual, extraer learnings, gotchas, y decisiones, y persistirlos en KB automaticamente.

Este skill es completamente autonomo: escanea, filtra, deduplica, y persiste sin pedir input al usuario.

## Proceso

### Paso 1: Escanear la conversacion
Revisar todo el hilo actual y extraer:

**Learnings** — cosas descubiertas:
- Comportamientos inesperados del codebase
- Patrones que funcionaron o no funcionaron (y por que)
- Workarounds aplicados
- Dependencias no documentadas

**Gotchas** — trampas para el futuro:
- Cosas que parecen una cosa pero son otra
- Configuraciones que rompen silenciosamente
- Errores costosos de diagnosticar

**Decisiones** — elecciones de diseno tomadas:
- Que se eligio y que se descarto
- Trade-offs aceptados
- Deuda tecnica introducida a proposito

### Paso 2: Filtrar
Descartar:
- Info efimera o derivable del codigo/git
- Cosas ya guardadas en KB (verificar con `cvm kb search`)
- Info sensible
- Cosas triviales o que no aportan a futuras sesiones

### Paso 3: Deduplicar
Para cada item candidato, buscar en KB con `cvm kb search "<terminos>"`.
Si ya existe una entry equivalente, descartarlo o actualizar la existente si aporta info nueva.

### Paso 4: Persistir
Para cada item, ejecutar directamente:
```bash
cvm kb put "<key>" --body "<body>" --tag "<tipo>,<area>" [--local]
```
NO pedir confirmacion. Persistir todo lo que pase el filtro de calidad.

### Paso 5: Session summary
Siempre persistir un resumen de sesion:
```bash
cvm kb put "session-summary-YYYYMMDD" --body "Goal: ... | Accomplished: ... | Discoveries: ... | Next: ..." --tag "session,summary"
```

### Paso 6: Reporte
Mostrar un resumen breve de lo que se guardo:
```
Retro completada:
- N learnings, N gotchas, N decisiones persistidas
- [keys guardadas]
- N items descartados (duplicados o triviales)
```

## MUST DO
- Revisar TODA la conversacion, no solo los ultimos mensajes
- Verificar duplicados en KB antes de guardar
- Persistir automaticamente sin pedir confirmacion
- Incluir siempre el session summary

## MUST NOT DO
- No pedir confirmacion ni input al usuario
- No guardar info sensible
- No guardar cosas triviales o derivables del codigo
- No mostrar listas largas de items — solo el reporte final

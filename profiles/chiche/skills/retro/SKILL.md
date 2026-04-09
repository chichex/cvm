Fin de sesion: revisar la conversacion actual y extraer learnings, gotchas, y decisiones para persistir en la KB.

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

### Paso 3: Presentar al usuario
Listar cada item con su clasificacion y key propuesta:
```
Retro de sesion:

Learnings:
1. [key-propuesta] — descripcion

Gotchas:
1. [key-propuesta] — descripcion

Decisiones:
1. [key-propuesta] — descripcion

Descartar:
- item — razon
```

Pedir confirmacion antes de persistir.

### Paso 4: Persistir
Para cada item aprobado, ejecutar el comando correspondiente:
```bash
cvm kb put "<key>" --body "<body>" --tag "<tipo>,<area>" [--local]
```

### Paso 5: Resumen
Reportar que se guardo y donde.

## MUST DO
- Revisar TODA la conversacion, no solo los ultimos mensajes
- Verificar duplicados en KB antes de guardar
- Pedir confirmacion antes de persistir

## MUST NOT DO
- No persistir sin aprobacion del usuario
- No guardar info sensible
- No guardar cosas triviales o derivables del codigo

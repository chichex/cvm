Higiene de Knowledge Base: deduplicar, podar entries stale, y consolidar entries relacionadas.

## Proceso

### Paso 1: Inventario
Listar todas las entries:
```bash
cvm kb ls
cvm kb ls --local
```

### Paso 2: Detectar problemas

**Duplicados:**
- Entries con keys similares o bodies que dicen lo mismo
- Buscar por terminos clave comunes

**Stale:**
- Entries que ya no aplican (tecnologia abandonada, decision revertida)
- Entries cuya info ya esta en el codigo o documentacion del proyecto

**Fragmentados:**
- Multiples entries pequenas que deberian ser una sola entry consolidada
- Entries del mismo tema con tags inconsistentes

### Paso 3: Proponer cambios
Presentar al usuario:
```
Mantenimiento de KB:

Duplicados (merge):
- [key1] + [key2] -> [key-nueva]: [body consolidado]

Stale (eliminar):
- [key]: [razon por la que ya no aplica]

Consolidar:
- [key1] + [key2] + [key3] -> [key-nueva]: [body consolidado]

Tags inconsistentes:
- [key]: [tags actuales] -> [tags propuestos]
```

### Paso 4: Ejecutar (si aprobado)
Para cada cambio aprobado:
- Eliminar entries viejas: `cvm kb rm <key>` [--local]
- Crear entries nuevas: `cvm kb put <key> --body "..." --tag "..."` [--local]

### Paso 5: Resumen
Reportar cambios realizados y estado final de la KB.

## MUST DO
- Mostrar TODOS los cambios propuestos antes de ejecutar
- Pedir confirmacion explicita
- Preservar toda la informacion valiosa durante consolidaciones

## MUST NOT DO
- No eliminar entries sin aprobacion
- No perder informacion durante merges — consolidar, no truncar
- No cambiar entries que el usuario creo manualmente sin preguntar

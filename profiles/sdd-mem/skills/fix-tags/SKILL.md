Corregir tags de la KB: asegurar que toda entry tenga un tipo valido y limpiar residuos.

## Proceso

### Paso 1: Dry-run

Ejecutar migracion en modo dry-run para ver que se eliminaria:

```bash
cvm kb migrate-tags --dry-run
cvm kb migrate-tags --dry-run --local
```

### Paso 2: Revisar entries sin tipo

Para cada entry que seria eliminada, decidir:

1. **Tiene valor** → agregarle un tipo con `cvm kb put "<key>" --body "$(cvm kb show <key>)" --type <tipo> --tag "<tags-existentes>"`
2. **No tiene valor** → dejar que la migracion la elimine

Tipos validos: `decision`, `learning`, `gotcha`, `discovery`, `session`

Criterio para elegir tipo:
- `learning`: algo aprendido, proceso, how-to, referencia tecnica
- `decision`: eleccion de diseno, arquitectura, herramienta
- `gotcha`: trampa, error comun, comportamiento inesperado
- `discovery`: hallazgo sobre el codebase, entorno, o herramienta
- `session`: metadata de sesion (raro, normalmente automatico)

### Paso 3: Ejecutar migracion

```bash
cvm kb migrate-tags
cvm kb migrate-tags --local
```

### Paso 4: Verificar

```bash
cvm kb migrate-tags --dry-run
cvm kb migrate-tags --dry-run --local
```

Ambos deben mostrar "No changes needed".

## Notas

- `migrate-tags` elimina entries sin tag de tipo Y residuos `session-buffer-*`
- Las entries con tags internos (`auto-captured`, `s013`, etc.) NO se eliminan si tienen un tipo
- Si una entry tiene `type:learning` (formato viejo con prefijo), `ClassifyTag` lo clasifica como interno — necesita un tipo bare para sobrevivir

Crear un save point (git stash o WIP commit) antes de cambios grandes. $ARGUMENTS es una descripcion opcional del checkpoint.

## Proceso

### Paso 1: Evaluar estado actual
```bash
git status --short
git stash list
```

### Paso 2: Decidir mecanismo

**Si hay cambios sin commitear:**
- Si son cambios del usuario (no nuestros): usar `git stash push -m "checkpoint: <descripcion>"`
- Si son cambios nuestros en progreso: usar WIP commit

**Si no hay cambios:**
- No hay nada que guardar, reportar y continuar

### Paso 3: Crear checkpoint

**Opcion A — Stash:**
```bash
git stash push -m "checkpoint: $ARGUMENTS o descripcion generada"
```

**Opcion B — WIP commit:**
```bash
git add -A
git commit -m "WIP: checkpoint — $ARGUMENTS o descripcion generada"
```

### Paso 4: Confirmar
Reportar:
```
Checkpoint creado:
- Tipo: stash / WIP commit
- Descripcion: [descripcion]
- Restaurar con: git stash pop / git reset HEAD~1
```

## MUST DO
- Verificar estado de git antes de actuar
- Incluir instrucciones de como restaurar el checkpoint
- Usar descripcion clara en el mensaje del stash/commit

## MUST NOT DO
- No hacer force push
- No crear checkpoints en branches protegidas (main, master) con WIP commits
- No perder cambios del usuario

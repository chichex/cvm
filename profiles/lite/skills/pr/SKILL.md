Crear un Pull Request. Opcionalmente ejecuta /r primero, luego crea el PR, y espera a que pasen los GitHub Actions. $ARGUMENTS es contexto adicional para el PR (opcional).

## Proceso

### Paso 1: Verificar branch y contexto

Detectar default branch, remote, y branch actual:
```bash
DEFAULT_BRANCH=$(gh repo view --json defaultBranchRef --jq '.defaultBranchRef.name' 2>/dev/null || echo "main")
REMOTE=$(git remote | head -1)
CURRENT_BRANCH=$(git branch --show-current)
```

Si la branch actual es la default branch, abortar: "Estas en la branch principal. Crea una branch primero."

Verificar que hay commits propios:
```bash
git log --oneline $DEFAULT_BRANCH..HEAD
```
Si no hay commits, abortar: "No hay commits en esta branch. Nada que incluir en el PR."

### Paso 2: Preguntar por /r

Preguntar al usuario: "Ejecuto /r (review de sesion) antes de crear el PR? [s/N]"

Si responde si: invocar el skill `/r` usando el Skill tool. Esperar a que termine.
Si responde no o no responde: continuar.

No reimplementar /r inline — delegar completamente al skill.

### Paso 3: Analizar cambios

Ejecutar en paralelo:
```bash
git status
git diff
git log --oneline $DEFAULT_BRANCH..HEAD
```

Si hay cambios unstaged (`git diff` no vacio) o archivos untracked, preguntar: "Hay cambios sin commitear. Commitear antes de crear el PR? [S/n]"

Si responde si o no responde (default si): staging + commit con mensaje derivado del diff.
Si responde no: continuar sin esos cambios.

### Paso 4: Crear el PR

Armar titulo y body. Escribir el body con Write tool en `/tmp/cvm-pr-body.md`:

**Titulo**: conciso, max 70 chars, describe el cambio principal.

Si $ARGUMENTS tiene contexto adicional, incorporarlo en el summary.

```bash
git push -u "$REMOTE" "$CURRENT_BRANCH" 2>/dev/null
gh pr create --title "<titulo>" --body-file /tmp/cvm-pr-body.md --base "$DEFAULT_BRANCH"
```

### Paso 5: Esperar GitHub Actions (si aplica)

Obtener el numero del PR recien creado:
```bash
PR_NUMBER=$(gh pr view --json number --jq '.number')
```

Verificar si hay checks configurados:
```bash
gh pr checks "$PR_NUMBER" --json name,state 2>/dev/null
```

**Si hay checks:**

Preguntar: "CI configurado. Espero a que termine o te dejo el link? [s/N]"

Si quiere esperar: usar el Bash tool con `run_in_background`:
```bash
gh pr checks "$PR_NUMBER" --watch --fail-fast
```

Reportar el PR URL inmediatamente. Cuando la notificacion de CI llegue, reportar el resultado.
Para CI pipelines largos (>10 min), el background process puede terminar por timeout — reportar estado parcial y dar link `gh pr checks $PR_NUMBER` para monitoreo manual.

Si alguno falla:
```bash
gh pr checks "$PR_NUMBER" --json name,state,link --jq '.[] | select(.state == "FAILURE")'
```

Si no quiere esperar o no responde (default no): reportar el PR con link a los checks.

**Si no hay checks:** reportar el PR directamente.

### Paso 6: Reporte final

```
PR creado: <url>
Review: [/r completado | /r omitido]
CI: [PASS | FAIL (detalle) | Esperando (link) | No CI configurado]
```

## MUST DO
- Detectar default branch y remote en el paso 1, antes de cualquier otra cosa
- Verificar que no estas en la branch principal
- Verificar que hay commits antes de crear PR
- Preguntar si ejecutar /r — no asumirlo
- Si /r se ejecuta, invocar via Skill tool, no reimplementar inline
- No hardcodear main ni origin — usar deteccion dinamica
- Usar Write tool para el body del PR
- Preguntar si esperar CI — no bloquearlo por default
- Todas las preguntas deben tener un default explicito: [s/N] o [S/n]

## MUST NOT DO
- No crear PR si estas en la branch principal
- No crear PR si no hay commits
- No hardcodear main ni origin
- No interpolar texto del usuario en comandos shell con double quotes
- No hacer force push
- No ejecutar /r sin preguntar
- No bloquear esperando CI sin preguntar

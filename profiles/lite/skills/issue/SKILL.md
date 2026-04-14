Crear un issue en GitHub. $ARGUMENTS es la descripcion de lo que se quiere trackear.

## Proceso

### Paso 1: Parsear el input

Analizar $ARGUMENTS para extraer:
- **Titulo**: conciso, imperativo (max 70 chars)
- **Descripcion**: contexto, problema, criterio de exito
- **Labels adicionales**: si el usuario menciona labels ademas de `ct:plan`

Si el input es muy breve, enriquecer la descripcion con contexto del proyecto y la conversacion actual.

### Paso 2: Verificar repo

Verificar que hay un repo git con remote configurado:
```bash
gh repo view --json name --jq '.name' 2>/dev/null
```
Si falla, abortar: "No hay un repo GitHub configurado en este directorio."

### Paso 3: Verificar label ct:plan

Verificar que el label `ct:plan` existe con match exacto:
```bash
gh label list --json name --jq '.[] | select(.name == "ct:plan") | .name' --limit 200 2>/dev/null
```

Si no retorna nada, crearlo:
```bash
gh label create "ct:plan" --color "0E8A16" --description "Planned work" 2>/dev/null
```

### Paso 4: Crear el issue

Escribir el body con Write tool en `/tmp/cvm-issue-body.md`:

```markdown
## Contexto
[Por que se necesita esto]

## Descripcion
[Que hay que hacer]

## Criterio de exito
- [ ] [condiciones para considerar esto hecho]
```

Crear el issue:
```bash
gh issue create --title "<titulo>" --body-file /tmp/cvm-issue-body.md --label "ct:plan"
```

Si hay labels adicionales, agregarlos con `--label`.

### Paso 5: Reportar

Mostrar el URL del issue creado.

## MUST DO
- Siempre agregar label `ct:plan` como minimo
- Verificar label con match exacto (`select(.name == "ct:plan")`), no fuzzy search
- Usar `--limit 200` para evitar paginacion
- Crear el label si no existe
- Verificar que hay repo GitHub antes de intentar
- Titulo conciso e imperativo
- Usar Write tool para el body, no heredoc en shell
- Usar --body-file para pasar el body a gh
- Enriquecer descripcion si el input es escueto

## MUST NOT DO
- No crear issues sin label `ct:plan`
- No dejar la descripcion vacia
- No interpolar texto del usuario en double-quoted shell commands

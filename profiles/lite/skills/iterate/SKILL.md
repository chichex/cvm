Aplica comments/reviews de un PR o issue lanzando un agente Opus con el contexto consolidado. `$ARGUMENTS` puede ser un numero (`/iterate 42`), una URL (`/iterate https://github.com/owner/repo/pull/42`), o vacio (`/iterate` usa la branch actual). El skill NO commitea — deja los cambios en el working tree para que el humano los revise.

## Proceso

### Paso 1: Parsear `$ARGUMENTS` y resolver target

Detectar el formato:

1. **Vacio** → usar la branch actual. Resolver el PR asociado:
   ```bash
   gh pr view --json number,headRefName,author,title,url,baseRefName 2>/dev/null
   ```
   Si falla, abortar: "No hay PR asociado a la branch actual. Pasa `/iterate <numero>` o una URL.".

2. **URL** (`https://github.com/<owner>/<repo>/(pull|issues)/<N>`) → extraer `owner`, `repo`, `N`, y `kind` (`pull` o `issues`). Hacerlo con parseo local (split por `/`), NO interpolar la URL en un comando shell.

3. **Numero puro** (`^[0-9]+$`) → `N`, detectar `kind` en este orden:
   ```bash
   gh pr view "$N" --json number 2>/dev/null
   ```
   Si retorna JSON valido → PR. Si falla:
   ```bash
   gh issue view "$N" --json number 2>/dev/null
   ```
   Si retorna JSON valido → issue. Si ambos fallan, abortar: "No encontre PR ni issue #N en este repo.".

4. **Cualquier otro input** → abortar con mensaje de uso: "Uso: `/iterate [N | URL | (vacio para usar la branch actual)]`".

Resolver `owner` y `repo` si vienen de URL usando los valores parseados; en el resto de los casos usar:
```bash
gh repo view --json owner,name --jq '"\(.owner.login)/\(.name)"' 2>/dev/null
```

Guardar en variables: `OWNER`, `REPO`, `N`, `KIND` (`pr` o `issue`).

### Paso 2: Fetch de comments

**Metadata** (siempre):

- Si `KIND=pr`:
  ```bash
  gh pr view "$N" --json number,title,author,headRefName,baseRefName,url,state,body 2>/dev/null
  ```
- Si `KIND=issue`:
  ```bash
  gh issue view "$N" --json number,title,author,url,state,body 2>/dev/null
  ```

Guardar el `author.login` como `AUTHOR` para el filtrado posterior.

**Comments** (los paths son literales, `$OWNER`/`$REPO`/`$N` son variables shell sanitizadas en Paso 1):

- Issue comments (aplica a PR y a issue — GitHub trata el PR como issue para este endpoint):
  ```bash
  gh api --paginate "repos/$OWNER/$REPO/issues/$N/comments"
  ```

- **Solo si `KIND=pr`** tambien fetch:
  - Review comments (inline, con `path`/`line`/`diff_hunk`):
    ```bash
    gh api --paginate "repos/$OWNER/$REPO/pulls/$N/comments"
    ```
  - Reviews (los review bodies top-level, separados de los review comments):
    ```bash
    gh api --paginate "repos/$OWNER/$REPO/pulls/$N/reviews"
    ```

Parsear el JSON resultante. Para cada comment / review, quedarse con: `id`, `user.login`, `created_at`, `body`, y (solo para review comments) `path`, `line` (o `original_line` si `line` es null), `commit_id`, `diff_hunk`. Para reviews: `state` (`APPROVED` / `CHANGES_REQUESTED` / `COMMENTED`) y `body`.

### Paso 3: Filtrado barato

Descartar un comment / review si:

- **Es del autor del PR/issue**: `user.login == AUTHOR`.
- **Body puramente reaccional**, match case-insensitive contra regex (despues de `trim`):
  `^(lgtm|\+1|👍|thanks?|ty)\.?$`
- **Review con `state == APPROVED` y `body` vacio o solo whitespace**.

Mantener todo lo demas, incluyendo comments cortos con contenido accionable (p.ej. "rename to X"). Contar cuantos se filtraron por cada categoria para reportarlo al final.

### Paso 4: Armar archivo de contexto

Escribir `/tmp/cvm-iterate-context.md` via Write tool. NUNCA interpolar bodies de comments en shell. Estructura:

```markdown
# Iteracion sobre <PR|Issue> #<N>

## Metadata
- Repo: <owner>/<repo>
- Titulo: <titulo>
- Autor: @<author>
- URL: <url>
- Estado: <state>
- Branch (solo PR): <headRefName> → <baseRefName>

## Body original
<body del PR/issue — puede estar vacio>

## Diff (solo si PR)
\`\`\`diff
<output de `gh pr diff "$N"` — puede ser largo, incluirlo completo>
\`\`\`

## Comments (<total> tras filtrado; <descartados> descartados)

### 1. @<user> — <created_at> — <tipo: issue comment | review comment | review>
<si es review comment:>
- Path: <path>:<line>
- Commit: <commit_id>
- Diff hunk:
  \`\`\`diff
  <diff_hunk>
  \`\`\`
<si es review:>
- State: <state>

<body>

---

### 2. ...
```

Numerar los comments en orden cronologico (por `created_at`). Si no quedo ningun comment tras el filtrado, escribir una seccion `## Comments (0 tras filtrado)` con un aviso: "No hay comments accionables.".

Para el diff del PR, usar:
```bash
gh pr diff "$N" 2>/dev/null
```

Redirigir la salida a archivo intermedio si es muy larga y leerlo con Read:
```bash
gh pr diff "$N" > /tmp/cvm-iterate-diff.txt 2>/dev/null
```
Luego `Read` y pegar el contenido dentro del bloque `diff` del contexto — sin interpolar via shell.

### Paso 5: Despachar al agente

Si quedaron 0 comments tras el filtrado, NO lanzar el agente. Reportar al usuario que no hay nada accionable y salir.

En caso contrario, lanzar:

```
Agent(
  subagent_type: "general-purpose",
  model: "opus",
  description: "Aplicar comments de PR/Issue #<N>",
  prompt: <ver abajo>
)
```

El prompt del agente debe ser literalmente (sustituyendo `<N>` y `<KIND>`):

```
Tenes acceso al filesystem del worktree actual. El contexto completo esta en /tmp/cvm-iterate-context.md — leelo primero.

Tarea: evaluar los comments/reviews del <KIND> #<N> y aplicar al codigo los cambios que sean accionables.

Criterios:
1. Para cada comment, decidi si es accionable (pide un cambio concreto), informativo (solo comenta sin pedir accion), o ruido residual que el filtrado no atrapo.
2. Aplicar los cambios accionables editando archivos directamente (Edit/Write).
3. Si dos comments se contradicen, priorizar el mas reciente y anotar el conflicto.
4. Si un comment es ambiguo, NO inventar interpretacion — marcarlo como "no aplicado, requiere aclaracion".

Restricciones:
- NO hagas commits. Solo modifica archivos en el working tree.
- NO crees archivos nuevos salvo que un comment lo pida explicitamente.
- NO respondas a los comments en GitHub. Solo tocas codigo local.
- NO delegues a otros agentes.

Output:
Reporte estructurado con:
- ## Aplicados — lista de comments procesados y que archivos tocaste
- ## Ignorados — comments no aplicados y la razon (ruido, ambiguo, fuera de scope, contradictorio)
- ## Archivos modificados — paths tocados

Termina tu respuesta con una seccion `## Key Learnings:` listando descubrimientos no-obvios sobre el codebase o el feedback que puedan ser utiles en futuras iteraciones.
```

### Paso 6: Reportar y persistir aprendizajes

Mostrar al usuario el reporte del agente tal cual, seguido de un resumen corto:

```
Iteracion sobre <PR|Issue> #<N> completada.
- Comments totales: <X>
- Filtrados (ruido/autor/APPROVED-vacio): <Y>
- Procesados por el agente: <Z>
- Archivos modificados: <lista>
Cambios en working tree — revisa `git diff` y commitea manualmente o via `/pr`.
```

Luego invocar el skill `/r` usando el Skill tool para persistir aprendizajes de la sesion.

## MUST DO
- Parsear `$ARGUMENTS` localmente (sin interpolar strings del usuario en shell).
- Detectar PR vs issue con fallback `gh pr view` → `gh issue view`.
- Usar `gh api --paginate` para los tres endpoints cuando `KIND=pr`, solo `issues/<N>/comments` cuando `KIND=issue`.
- Filtrar comments del autor, reacciones puras (regex case-insensitive), y reviews APPROVED vacios antes de pasar al agente.
- Escribir el contexto a `/tmp/cvm-iterate-context.md` con Write tool; el prompt del agente referencia el path.
- Lanzar `Agent(subagent_type='general-purpose', model='opus')` con instruccion explicita de evaluar, aplicar, reportar y terminar con `## Key Learnings:`.
- Despues del reporte del agente, invocar `/r` via Skill tool.
- Dejar los cambios en working tree sin commitear.

## MUST NOT DO
- No interpolar bodies de comments, titulos, o `$ARGUMENTS` crudos en double-quoted shell commands. Todo lo de github va via `gh ... --json` / `gh api` y se parsea localmente.
- No delegar a `/o` — el skill lanza `Agent` directo.
- No hacer commits automaticos. No hacer push. No comentar de vuelta en GitHub.
- No crear archivos nuevos salvo que los comments lo pidan explicitamente.
- No soportar GitLab / Bitbucket / otras plataformas en esta iteracion.
- No resolver review threads — el skill solo modifica codigo local.
- No lanzar el agente si no quedaron comments accionables tras el filtrado.

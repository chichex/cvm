Crear un issue en GitHub a partir de una idea vaga, enriqueciendola automaticamente (archivos relevantes, type/size inferidos, criterios iniciales). $ARGUMENTS es la descripcion vaga. Soporta `--dry-run` para previsualizar sin crear el issue.

## Diferencia con /issue

- `/idea <descripcion vaga>` — idea sin clasificar, sin pensar todavia. El skill enriquece y clasifica automaticamente. Issue resultante sirve de input al flujo de planificacion (`/o`, plan, etc).
- `/issue <descripcion concreta>` — tarea ya pensada, con titulo claro y criterio de exito definido por el usuario. Sin enriquecimiento heuristico.

Si el usuario ya sabe que quiere y como, usar `/issue`. Si esta tirando una idea suelta, usar `/idea`.

## Esquema de labels

Cada issue creado por `/idea` lleva exactamente:
- `status:idea` — exclusivo de `/idea`, distingue ideas crudas de planes.
- `ct:plan` — convencion del repo (compartido con `/issue`).
- `type:<inferido>` — uno de: `feature`, `bug`, `chore`, `docs`.
- `size:<inferido>` — uno de: `xs`, `s`, `m`, `l`.

La clasificacion es heuristica y se documenta como "inferida automaticamente" en el body. El usuario puede editarla a mano si falla.

## Proceso

### Paso 1: Parsear el input

De $ARGUMENTS extraer:
- **Descripcion**: el texto, quitando flags conocidos.
- **Dry-run**: true si $ARGUMENTS contiene `--dry-run`.

Si la descripcion queda vacia, abortar: "Pasale una descripcion. Ej: `/idea agregar dark mode al dashboard`".

### Paso 2: Detectar archivos relevantes

Extraer keywords del input — palabras de mas de 3 letras que parezcan tecnicas (nombres de modulos, archivos, tecnologias, comandos, extensiones). Descartar stopwords comunes en español/ingles ("hacer", "para", "que", "the", "and", etc).

Para cada keyword candidata:
- Si parece nombre de archivo o modulo (camelCase, snake_case, con `.`, con `/`): probar Glob con `**/*<keyword>*`.
- Si parece termino tecnico generico ("dashboard", "auth", "memory"): Grep case-insensitive en el repo limitado a tipos de archivo comunes (`md`, `go`, `ts`, `js`, `py`, segun stack del repo).

Combinar resultados, dedupear, y quedarse con los **5-10 archivos mas relevantes** (priorizar los que matchean mas keywords). Si no se detecta nada, dejar la lista vacia y agregar un warning a la seccion Notas.

NO leer el contenido completo de los archivos — solo identificarlos. El analisis profundo es trabajo del flujo de plan posterior.

### Paso 3: Inferir type

Analizar el input con heuristicas de keywords (case-insensitive):

| Type | Keywords disparadores |
|------|----------------------|
| `bug` | bug, fix, error, broken, falla, rompe, crash, no funciona, no anda, regresion |
| `docs` | doc, docs, readme, documentar, comentario, explicar, guia, tutorial |
| `chore` | refactor, cleanup, limpiar, renombrar, mover, dependency, dependencia, bump, version, lint, format |
| `feature` | (default — incluye agregar, nuevo, crear, skill, soportar, implementar) |

El primer match en orden bug > docs > chore > feature gana. Si no matchea ninguno, default `feature`.

### Paso 4: Inferir size

Combinar longitud del input y archivos detectados. Los tres buckets chicos usan **AND** (ambos factores tienen que ser chicos); `l` es el fallback cuando cualquiera de los dos se dispara.

| Size | Criterio |
|------|----------|
| `xs` | input < 50 chars **Y** ≤ 1 archivo detectado |
| `s`  | input < 150 chars **Y** ≤ 3 archivos |
| `m`  | input < 300 chars **Y** ≤ 6 archivos |
| `l`  | cualquier otro caso (input ≥ 300 chars **O** > 6 archivos) |

Aplicar en orden `xs → s → m → l` y quedarse con el primero que cumpla. No hay ambiguedad: si un bucket falla por longitud O por archivos, se cae al siguiente.

Ejemplos:
- input 30 chars, 0 archivos → `xs` (ambos chicos).
- input 30 chars, 2 archivos → `s` (falla `xs` por archivos, cumple `s`).
- input 200 chars, 0 archivos → `m` (falla `s` por longitud, cumple `m`).
- input 100 chars, 5 archivos → `m` (falla `s` por archivos, cumple `m`).
- input 100 chars, 8 archivos → `l` (falla `m` por archivos).
- input 400 chars, 1 archivo → `l` (falla `m` por longitud).

### Paso 5: Armar el body

Generar un path temporal unico para esta invocacion y guardarlo como `$BODY_FILE` para reusarlo en Paso 6 y Paso 9:

```bash
BODY_FILE="$(mktemp -t cvm-idea-body.XXXXXX).md"
```

Si `mktemp` no esta disponible, fallback a `/tmp/cvm-idea-body-$(date +%s)-$$.md` (timestamp + PID). El path fijo `/tmp/cvm-idea-body.md` NO es seguro: dos `/idea` concurrentes (o un run previo fallido) pueden mezclar bodies.

Usar Write tool para escribir `$BODY_FILE` con esta estructura exacta (los `[...]` se reemplazan):

```markdown
## Idea
[descripcion del usuario, transcripta tal cual]

## Contexto detectado
- Archivos/modulos relevantes: [lista bullet de paths, o "No se detectaron archivos con las heuristicas actuales — completar manualmente en la fase de plan"]
- Area afectada: [inferida del directorio comun de los archivos detectados, o "indeterminada"]
- Dependencias: [herramientas/skills/libs mencionadas o inferidas, o "ninguna detectada"]

## Criterios de exito iniciales
- [ ] [criterio inicial derivado de la descripcion — minimo 1, maximo 3]
- [ ] [criterio adicional si aplica]

## Notas / warnings
[Anotaciones sobre ambiguedades del input, supuestos tomados, riesgos a explorar en la fase de plan. Si la deteccion de archivos fallo o el input es muy vago, marcarlo aqui explicitamente.]

## Clasificacion
- Type: [type] (inferido automaticamente — keywords: [keywords que dispararon])
- Size: [size] — [justificacion breve: longitud del input + cantidad de archivos]
```

NUNCA interpolar la descripcion del usuario en strings shell. Todo va via Write tool al archivo temporal.

### Paso 6: Si es dry-run, imprimir y salir

Si dry-run, imprimir por stdout:

```
=== DRY RUN ===

Title: <titulo derivado de la descripcion, max 70 chars, imperativo>

Labels:
  - status:idea
  - ct:plan
  - type:<inferido>
  - size:<inferido>

Body:
---
<contenido completo de $BODY_FILE>
---

(no se creo el issue. Removeti --dry-run para crear de verdad.)
```

Salir aqui. NO ejecutar gh.

### Paso 7: Verificar repo

```bash
gh repo view --json name --jq '.name' 2>/dev/null
```

Si falla, abortar: "No hay un repo GitHub configurado en este directorio."

### Paso 8: Verificar y crear labels

Para cada label requerido (`status:idea`, `ct:plan`, `type:<inferido>`, `size:<inferido>`), verificar con match exacto y `--limit 200`:

```bash
gh label list --limit 200 --json name --jq '.[] | select(.name == "<label>") | .name' 2>/dev/null
```

Si no retorna nada, crearlo. Colores sugeridos (consistente con labels existentes del repo):

- `status:idea` → color `1D76DB`, descripcion "Idea capturada, sin plan"
- `ct:plan` → color `0E8A16`, descripcion "Planned work"
- `type:feature` → color `BFDADC`
- `type:bug` → color `D73A4A`
- `type:chore` → color `FBCA04`
- `type:docs` → color `0075CA`
- `size:xs` → color `C5DEF5`
- `size:s` → color `0052CC`
- `size:m` → color `5319E7`
- `size:l` → color `B60205`

```bash
gh label create "<label>" --color "<color>" --description "<desc>" 2>/dev/null
```

Hacer esto secuencialmente, no en paralelo (gh CLI es serial-friendly y evita race conditions con labels recien creados).

### Paso 9: Crear el issue

El titulo se deriva de la descripcion: imperativo, max 70 chars, sin punto final. Pasarlo via `--title` (es texto que el skill controla, no input crudo del usuario — pero igual escapar comillas si las hubiera).

```bash
gh issue create \
  --title "<titulo derivado>" \
  --body-file "$BODY_FILE" \
  --label "status:idea" \
  --label "ct:plan" \
  --label "type:<inferido>" \
  --label "size:<inferido>"
```

### Paso 10: Reportar

Mostrar la URL del issue creado:

```
Issue creado: <url>
Labels: status:idea, ct:plan, type:<x>, size:<y>
```

## MUST DO

- Enriquecer automaticamente — type, size y archivos relevantes son inferidos sin pedir confirmacion.
- Reusar el patron gh de `/issue`: verificar repo, verificar labels con match exacto y `--limit 200`, crear los que falten, body via `--body-file` apuntando a archivo escrito con Write tool.
- Detectar archivos solo con Glob/Grep — sin delegacion a subagents.
- Limitar archivos detectados a 5-10 maximo.
- Si la deteccion falla o el input es muy vago, anotarlo en Notas/warnings en lugar de fabricar falsos positivos.
- Marcar la clasificacion como "inferida automaticamente" en el body para que el usuario entienda que puede editarla.
- Soportar `--dry-run` que imprime body + labels sin tocar gh.
- Devolver la URL del issue al final.

## MUST NOT DO

- No pedir confirmacion al usuario (flujo one-shot).
- No interpolar la descripcion del usuario en comandos shell — todo via Write + `--body-file`.
- No delegar a `/o`, `/c`, `/g` — el skill resuelve todo localmente.
- No leer el contenido completo de los archivos detectados — solo identificarlos.
- No generar esqueletos de plan, archivos locales, ni notas en memory — el unico output es el issue en GitHub.
- No modificar `/issue` ni los CLAUDE.md.
- No omitir ninguno de los 4 labels (`status:idea`, `ct:plan`, `type:*`, `size:*`).

**Competitive analysis**: matriz comparativa de features × competidores, pricing, posicionamiento, gaps. Delega a `pm-researcher` para enrichment externo (WebSearch/WebFetch) si el usuario lo pide. `$ARGUMENTS` es categoria/producto/competidores (puede venir vacio).

Skill **interactivo**.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar como en `/pm-prd`.

### 2. Validar input
- Vacio: pedir `Que categoria o competidores querés analizar? Lista los competidores que ya conocés y el foco (features, pricing, posicionamiento, etc).` y esperar.

### 3. Preguntar foco
```
Foco del analisis:
1) Features (que ofrecen / que faltan)
2) Pricing (modelo, tiers, precio)
3) Positioning (como se venden, audiencia)
4) Reviews (que dicen los usuarios)
5) General (combinacion de los anteriores)
6) Otra
```
Guardar `FOCO`.

### 4. Preguntar enrichment externo
```
Querés que el subagent `pm-researcher` busque info externa (web, reviews, pricing pages)?
1) Si, full enrichment (recomendado si no tenés data pegada)
2) No, trabajo con la data que te paso
3) Mixto — yo paso lo que tengo, vos completá lo que falte
```
Guardar `ENRICHMENT`.

## Fase 1 — Obtener data

### Caso 1: Si `ENRICHMENT=1` (full)
- Extraer competidores conocidos del prompt (si los menciono).
- Invocar `pm-researcher` con:
  - `topic: <categoria o producto del usuario>`
  - `competitors_known: <lista del prompt o vacia>`
  - `focus: <mapeo de FOCO>`
  - `max_competitors: 5` (preguntar al usuario si quiere otro)
- Esperar el reporte estructurado.
- Mostrar resumen al usuario y preguntar si esta bien para seguir.

### Caso 2: Si `ENRICHMENT=2` (no externa)
- Pedir al usuario que pegue la data que tiene:
  ```
  Pegá lo que tengas (features list, pricing, links, quotes). Cuando termines, decí `listo`.
  ```

### Caso 3: Si `ENRICHMENT=3` (mixto)
- Pedir data del usuario primero.
- Identificar gaps (competidores mencionados sin data, o focos sin info).
- Invocar `pm-researcher` solo sobre los gaps.

## Fase 2 — Estructurar matriz

Generar las siguientes secciones (cada una si aplica al `FOCO`):

### Matriz de features
```
| Feature                  | Nosotros | Comp A | Comp B | Comp C |
|--------------------------|----------|--------|--------|--------|
| <feature 1>              | ✓        | ✓      | ✗      | parcial |
| <feature 2>              | ✗        | ✓      | ✓      | ✓      |
```

Categorias de features:
- **Table stakes**: features que tienen >50% de competidores (lo que se asume).
- **Diferenciadores actuales**: features unicas (1-2 competidores).
- **Gaps**: features que faltan en todos.

### Pricing
```
| Competidor | Modelo | Tier inicial | Tier maximo | Free tier? | Notas |
|------------|--------|--------------|-------------|------------|-------|
```

### Positioning
- Headline por competidor (1 linea).
- Audiencia implicita.
- Tono/estilo dominante.

### Reviews signal (si FOCO incluye reviews)
- Top elogios por competidor.
- Top quejas por competidor.
- Patrones cross-cutting (que valoran/odian los usuarios de la categoria).

## Fase 3 — Insights y gaps

Generar:
- **Donde ganamos**: features/pricing/posicionamiento donde nuestro producto esta mejor que >50% de los competidores.
- **Donde perdemos**: lo opuesto.
- **Gaps de mercado**: necesidades que ningun competidor cubre bien.
- **Riesgos competitivos**: lo que un competidor podria sacar pronto y nos lastimaria.

## Fase 4 — Estructura del body

```markdown
## Resumen

- **Categoria**: <X>
- **Foco**: <FOCO>
- **Competidores analizados**: <lista>
- **Fuente de data**: <pm-researcher | pegada por usuario | mixta>

## Matriz de features

<tabla>

### Table stakes
- <lista>

### Diferenciadores actuales
- <feature> → <competidor>

### Gaps (no cubierto por nadie)
- <feature potencial>

## Pricing

<tabla>

**Patron dominante**: <modelo + rango>

## Positioning

<por competidor>

## Reviews signal

<solo si aplica>

## Insights

### Donde ganamos
- <bullet>

### Donde perdemos
- <bullet>

### Gaps de mercado
- <bullet>

### Riesgos competitivos
- <competidor> podria <accion> y eso nos <impacto>

## Fuentes

<URLs si vinieron del researcher>

---

_Competitive analysis generado por `/pm-compete`._
```

## Fase 5 — Review opcional

```
Querés que `pm-reviewer` audite el analisis? (si/no, default: no — la matriz se audita sola)
```

Si si: invocar con `artefact_type: compete`. El reviewer va a buscar: matriz donde ganamos en todo (sospechoso), pricing sin fuente, posicionamiento copy-pasted del marketing.

## Fase 6 — Confirmar y persistir

```
Confirmás que creo el issue con label `pm:compete`? (si/no, default: si)
```

```bash
gh label create "pm:compete" --color "FBCA04" --description "Competitive analysis" 2>/dev/null || true

BODY_FILE="$(mktemp -t pm-compete-body.XXXXXX).md"
gh issue create --title "<titulo>" --body-file "$BODY_FILE" --label "pm:compete"
```

Titulo formato: "Compete: <categoria> <fecha>" o "Compete vs <competidor> <fecha>".

## Fase 7 — Reportar

```
## Result
- skill: /pm-compete
- persisted: true
- url: <URL>
- title: <titulo>
- foco: <FOCO>
- enrichment: <ENRICHMENT>
- competidores_analizados: <N>
- gaps_detectados: <N>
- reviewer_used: <true | false>
```

## MUST DO

- Preguntar `FOCO` y `ENRICHMENT` antes de pedir data.
- Si `ENRICHMENT=1`, delegar a `pm-researcher` con focus claro.
- Generar matriz con table stakes / diferenciadores / gaps separados.
- Citar fuentes cuando vengan del researcher.

## MUST NOT DO

- No inventar datos de competidores que no hayan venido del researcher o del usuario.
- No declarar "ganamos en todo" sin justificacion item por item.
- No omitir gaps de mercado (eso es la oportunidad mas valiosa del analisis).
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

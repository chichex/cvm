**Analisis competitivo**: matriz comparativa de features × competidores, precios, posicionamiento, vacios. Delega a `pm-researcher` para investigacion externa (WebSearch/WebFetch) si el usuario lo pide. `$ARGUMENTS` es categoria/producto/competidores (puede venir vacio).

Skill **interactivo**.

## Pre-flight

### 1. Validar input

- Vacio: pedir `Que categoria o competidores querés analizar? Lista los competidores que ya conocés y el foco (features, precios, posicionamiento, etc).` y esperar.

### 2. Preguntar foco

```
Foco del analisis:
1) Features (que ofrecen / que faltan)
2) Precios (modelo, tiers, precio)
3) Posicionamiento (como se venden, a quien)
4) Reviews (que dicen los usuarios)
5) General (combinacion de los anteriores)
6) Otra
```
Guardar `FOCO`.

### 3. Preguntar investigacion externa

```
Querés que el subagent `pm-researcher` busque info externa (web, reviews, paginas de precios)?
1) Si, investigacion completa (recomendado si no tenés data pegada)
2) No, trabajo con la data que te paso
3) Mixto — yo paso lo que tengo, vos completá lo que falte
```
Guardar `INVESTIGACION`.

## Fase 1 — Obtener data

### Caso 1: Si `INVESTIGACION=1` (completa)
- Extraer competidores conocidos del prompt (si los menciono).
- Invocar `pm-researcher` con:
  - `topic: <categoria o producto del usuario>`
  - `competitors_known: <lista del prompt o vacia>`
  - `focus: <mapeo de FOCO>`
  - `max_competitors: 5` (preguntar al usuario si quiere otro)
- Esperar el reporte estructurado.
- Mostrar resumen al usuario y preguntar si esta bien para seguir.

### Caso 2: Si `INVESTIGACION=2` (no externa)
- Pedir al usuario que pegue la data que tiene:
  ```
  Pegá lo que tengas (lista de features, precios, links, quotes). Cuando termines, decí `listo`.
  ```

### Caso 3: Si `INVESTIGACION=3` (mixto)
- Pedir data del usuario primero.
- Identificar faltantes (competidores mencionados sin data, o focos sin info).
- Invocar `pm-researcher` solo sobre los faltantes.

## Fase 2 — Estructurar matriz

Generar las siguientes secciones (cada una si aplica al `FOCO`):

### Matriz de features
```
| Feature                  | Nosotros | Comp A | Comp B | Comp C |
|--------------------------|----------|--------|--------|--------|
| <feature 1>              | si       | si     | no     | parcial |
| <feature 2>              | no       | si     | si     | si      |
```

Categorias de features:
- **Basico**: features que tienen >50% de competidores (lo que se asume).
- **Diferenciadores actuales**: features unicas (1-2 competidores).
- **Vacios**: features que faltan en todos.

### Precios
```
| Competidor | Modelo | Tier inicial | Tier maximo | Tier gratis? | Notas |
|------------|--------|--------------|-------------|--------------|-------|
```

### Posicionamiento
- Headline por competidor (1 linea).
- A quien le hablan implicitamente.
- Tono/estilo dominante.

### Señal de reviews (si FOCO incluye reviews)
- Top elogios por competidor.
- Top quejas por competidor.
- Patrones transversales (que valoran/odian los usuarios de la categoria).

## Fase 3 — Insights y vacios

Generar:
- **Donde ganamos**: features/precios/posicionamiento donde nuestro producto esta mejor que >50% de los competidores.
- **Donde perdemos**: lo opuesto.
- **Vacios de mercado**: necesidades que ningun competidor cubre bien.
- **Riesgos competitivos**: lo que un competidor podria sacar pronto y nos lastimaria.

## Fase 4 — Estructura del contenido

```markdown
## Resumen

- **Categoria**: <X>
- **Foco**: <FOCO>
- **Competidores analizados**: <lista>
- **Fuente de data**: <pm-researcher | pegada por usuario | mixta>

## Matriz de features

<tabla>

### Basico
- <lista>

### Diferenciadores actuales
- <feature> → <competidor>

### Vacios (no cubierto por nadie)
- <feature potencial>

## Precios

<tabla>

**Patron dominante**: <modelo + rango>

## Posicionamiento

<por competidor>

## Señal de reviews

<solo si aplica>

## Insights

### Donde ganamos
- <bullet>

### Donde perdemos
- <bullet>

### Vacios de mercado
- <bullet>

### Riesgos competitivos
- <competidor> podria <accion> y eso nos <impacto>

## Fuentes

<URLs si vinieron del researcher>

---

_Analisis competitivo generado por `/pm-compete`._
```

## Fase 5 — Revision opcional

```
Querés que `pm-reviewer` audite el analisis? (si/no, default: no — la matriz se audita sola)
```

Si si: invocar con `artefact_type: compete`. El reviewer va a buscar: matriz donde ganamos en todo (sospechoso), precios sin fuente, posicionamiento copy-pasted del marketing.

## Fase 6 — Confirmar y guardar

Slug: kebab-case del titulo, max 40 chars. Path: `.pm/pm-compete/<slug>.md`.

```
Confirmás que guardo el analisis en `.pm/pm-compete/<slug>.md`? (si/no, default: si)
```

Si si: si la carpeta `.pm/pm-compete/` no existe, crearla con `mkdir -p .pm/pm-compete/` antes de escribir. Luego usar el `Write` tool (NUNCA via echo/heredoc) para crear el archivo. Titulo formato: "Compete: <categoria> <fecha>" o "Compete vs <competidor> <fecha>".

## Fase 7 — Reportar

```
## Result
- skill: /pm-compete
- saved: true
- file: .pm/pm-compete/<slug>.md
- title: <titulo>
- foco: <FOCO>
- investigacion: <INVESTIGACION>
- competidores_analizados: <N>
- vacios_detectados: <N>
- reviewer_used: <true | false>
```

Y debajo: `Analisis guardado: .pm/pm-compete/<slug>.md`.

## MUST DO

- Preguntar `FOCO` e `INVESTIGACION` antes de pedir data.
- Si `INVESTIGACION=1`, delegar a `pm-researcher` con foco claro.
- Generar matriz con basico / diferenciadores / vacios separados.
- Citar fuentes cuando vengan del researcher.
- Guardar en `.pm/pm-compete/<slug>.md` con `Write` tool.

## MUST NOT DO

- No inventar datos de competidores que no hayan venido del researcher o del usuario.
- No declarar "ganamos en todo" sin justificacion item por item.
- No omitir vacios de mercado (eso es la oportunidad mas valiosa del analisis).
- No usar `gh` ni depender de GitHub.
- No persistir nada en auto-memory.

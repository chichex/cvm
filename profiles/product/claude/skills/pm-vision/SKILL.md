Definir **vision a 3 años** + **metrica principal** del producto. Incluye anti-vision (que NO queremos ser). `$ARGUMENTS` es el contexto del producto (puede venir vacio).

Skill **interactivo profundo** — esperar conversacion mas larga que en otros skills.

## Pre-flight

### 1. Validar input

- Vacio: pedir `Describime el producto (que hace, para quien, en que estado esta hoy). Parrafo libre.` y esperar.

### 2. Preguntar etapa

```
Etapa del producto (afecta el alcance de la vision):
1) Etapa temprana / recien arrancando (default) — vision puede ser ambiciosa y volatil
2) En crecimiento — vision se construye desde traccion real
3) Maduro / empresa grande — vision tiene que respetar legacy y portfolio existente
4) Otra
```
Guardar `ETAPA`.

## Fase 1 — Clarificacion profunda

Aplicar clarificacion inline, forzando supuestos de tipo "por que" y "para quien" (no "que").

1. Listar 6-10 supuestos (es el ejercicio mas profundo del profile — no limitar artificialmente), taggeados `[directo]`, `[medio]`, `[especulativo]`. Filtrar a supuestos de producto/negocio, NO tecnicos. Priorizar los ausentes del prompt:
   - **Problema fundamental** que el producto resuelve (no la feature — el problema en el mundo).
   - **Usuario futuro** (en 3 años — quien lo usa, donde, en que contexto).
   - **Estado actual del usuario** (que hace hoy sin el producto).
   - **Estado futuro del usuario** (que cambia en su vida cuando el producto cumple su vision).
   - **Por que nosotros** (que ventaja diferencial sostenemos vs alternativas).
   - **Por que ahora** (que cambio en el mundo hace que esto sea posible/urgente).
   - **Horizonte temporal** (3 años por default, pero el usuario puede declarar 1 / 5 / 10).
   - **Anti-vision** (que tipo de producto NO queremos ser, aunque sea facil caer ahi).
2. Mostrar al usuario:
   ```
   Detecté estos supuestos:
   1. [especulativo] <supuesto>
   2. [medio] <supuesto>
   ...
   Cuáles te gustaría clarificar? (numeros separados por coma, o 'todos', o 'ninguno')
   ```
3. Para cada supuesto seleccionado, preguntar multiple choice con 4 opciones + `otra`, mostrando progreso `Pregunta X/Y`.
4. Actualizar el material base con las respuestas.

## Fase 2 — Sugerir metrica principal

Despues de refinar los supuestos, generar 3 candidatos a metrica principal basados en el estado futuro del usuario. Para cada candidato:
- Nombre.
- Definicion operativa (como se mide).
- Por que conecta con la vision (1 linea).
- Por que NO seria una buena metrica principal (que sesgo introduciria).

Mostrar al usuario:
```
## Candidatos a metrica principal

1. <candidato 1>
2. <candidato 2>
3. <candidato 3>

Cual elegis? (1/2/3/otra)
```

Guardar `METRICA_PRINCIPAL`.

## Fase 3 — Sugerir anti-vision

Generar 2-3 candidatos de anti-vision (productos/posicionamientos que serian faciles de caer pero no queremos):
- "Otra herramienta de X mas, sin diferencial real".
- "Producto que se monetiza con ads y degrada la experiencia".
- "Herramienta tecnica que solo entiende el equipo de ingenieria, no el usuario final".
- (etc, contextual)

Preguntar al usuario cual le calza o si quiere agregar la suya.

## Fase 4 — Revision opcional con `pm-reviewer`

```
Querés que `pm-reviewer` audite la vision antes de guardar? Particularmente util — visiones genericas son comunes. (si/no, default: si)
```

Si si: invocar con `artefact_type: vision`, `artefact_text: <contenido>`, `context: etapa=<ETAPA>`. El reviewer va a buscar: visiones indistinguibles de la competencia, metrica principal desconectada, falta de anti-vision, horizonte poco ambicioso o demasiado abstracto.

Iterar items urgentes/importantes max 2 veces.

## Fase 5 — Estructura del contenido

```markdown
## Vision a <N> años

<1-2 lineas evocativas, en presente como si ya pasara. Ej. "Cualquier equipo de producto puede prototipar y testear ideas con clientes reales en menos de un dia.">

## Por que existe el producto

<el problema en el mundo que el producto resuelve — no la feature, el dolor humano o de negocio>

## Usuario futuro (en <N> años)

<quien lo usa, donde, en que contexto. Especifico, no "todos">

## Estado actual del usuario

<que hace hoy sin nuestro producto (workarounds, herramientas alternativas, sufrimiento implicito)>

## Estado futuro del usuario

<que cambia en su vida/trabajo cuando la vision se cumple>

## Por que nosotros

<ventaja diferencial sostenible — no "ejecutamos rapido", algo estructural>

## Por que ahora

<que cambio en el mundo (tech, mercado, comportamiento) hace esto posible o urgente>

## Metrica principal

- **Nombre**: <METRICA_PRINCIPAL>
- **Definicion**: <como se calcula>
- **Por que esta metrica**: <1-2 lineas conectando con la vision>

## Anti-vision

Lo que NO queremos ser, aunque sea facil caer ahi:
- <anti-vision 1>
- <anti-vision 2>

## Horizonte y revision

- Horizonte: <N> años (fecha de revision: <YYYY>)
- Disparadores para revisar antes: <cambios estructurales que obligarian a repensar>

---

_Vision definida con `/pm-vision`. Etapa: <ETAPA>._
```

## Fase 6 — Confirmar y guardar

Default guardado: **si**.

Slug: kebab-case del titulo, max 40 chars. Path: `.pm/pm-vision/<slug>.md`.

```
Confirmás que guardo la vision en `.pm/pm-vision/<slug>.md`? (si/no, default: si)
```

Si si: si la carpeta `.pm/pm-vision/` no existe, crearla con `mkdir -p .pm/pm-vision/` antes de escribir. Luego usar el `Write` tool (NUNCA via echo/heredoc) para crear el archivo. Titulo: imperativo o nominal, max 70 chars. Ej. "Vision: <producto> 2028" o "<vision en 1 frase corta>".

## Fase 7 — Reportar

```
## Result
- skill: /pm-vision
- saved: true
- file: .pm/pm-vision/<slug>.md
- title: <titulo>
- etapa: <ETAPA>
- metrica_principal: <METRICA_PRINCIPAL>
- horizonte: <N años>
- reviewer_used: <true | false>
- reviewer_verdict: <solido | necesita-trabajo | debil | n/a>
```

Y debajo: `Vision guardada: .pm/pm-vision/<slug>.md`.

## MUST DO

- Refinar profundo: problema fundamental, usuario futuro, estado actual, estado futuro, por que nosotros, por que ahora.
- Sugerir 3 candidatos de metrica principal antes de elegir.
- Forzar anti-vision (lo que NO queremos ser).
- Ofrecer revision con `pm-reviewer` (default si).
- Guardar en `.pm/pm-vision/<slug>.md` con `Write` tool.

## MUST NOT DO

- No aceptar visiones genericas tipo "el mejor X para Y" sin diferencial real.
- No omitir anti-vision.
- No mezclar vision con PRD — vision es estrategica, PRD es operativa.
- No usar `gh` ni depender de GitHub.
- No persistir nada en auto-memory.

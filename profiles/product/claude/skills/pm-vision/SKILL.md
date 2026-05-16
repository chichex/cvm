Definir **vision a 3 años** + **north star metric** del producto. Incluye anti-vision (que NO queremos ser). `$ARGUMENTS` es el contexto del producto (puede venir vacio).

Skill **interactivo profundo** — esperar conversacion mas larga que en otros skills.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar como en `/pm-prd`.

### 2. Validar input
- Vacio: pedir `Describime el producto (que hace, para quien, en que estado esta hoy). Parrafo libre.` y esperar.

### 3. Preguntar stage
```
Stage del producto (afecta el alcance de la vision):
1) Early-stage / founder-mode (default) — vision puede ser ambiciosa y volatil
2) Growth-stage — vision se construye desde traction real
3) Mature — vision tiene que respetar legacy y portfolio existente
4) Otra
```
Guardar `STAGE`.

## Fase 1 — Clarify profundo

Cargar `/clarify` (`../clarify/SKILL.md`) en `MODO=prompt`. **Restricciones especificas**:

- Forzar asunciones de tipo "por que" y "para quien", no "que".
- Asunciones a refinar (priorizar las que esten ausentes en el prompt):
  - **Problema fundamental** que el producto resuelve (no la feature — el problema en el mundo).
  - **Usuario futuro** (en 3 años — quien lo usa, donde, en que contexto).
  - **Estado actual del usuario** (que hace hoy sin el producto).
  - **Estado futuro del usuario** (que cambia en su vida cuando el producto cumple su vision).
  - **Por que nosotros** (que ventaja diferencial sostenemos vs alternativas).
  - **Por que ahora** (que cambio en el mundo hace que esto sea posible/urgente).
  - **Horizonte temporal** (3 años por default, pero el usuario puede declarar 1 / 5 / 10).
  - **Anti-vision** (que tipo de producto NO queremos ser, aunque sea facil caer ahi).

- No limitar el numero de asunciones — es el ejercicio mas profundo del profile.
- Saltar la persistencia de `/clarify`.

## Fase 2 — Sugerir north star metric

Despues de refinar las asunciones, generar 3 candidatos a north star metric basados en el estado futuro del usuario. Para cada candidato:
- Nombre.
- Definicion operativa (como se mide).
- Por que conecta con la vision (1 linea).
- Por que NO seria un buen north star (que sesgo introduciria).

Mostrar al usuario:
```
## Candidatos a north star metric

1. <candidato 1>
2. <candidato 2>
3. <candidato 3>

Cual elegis? (1/2/3/otra)
```

Guardar `NORTH_STAR`.

## Fase 3 — Sugerir anti-vision

Generar 2-3 candidatos de anti-vision (productos/posicionamientos que serian faciles de caer pero no queremos):
- "Otra herramienta de X mas, sin diferencial real".
- "Producto que se monetiza con ads y degrada la experiencia".
- "Tool tecnico que solo entiende el equipo de ingenieria, no el usuario final".
- (etc, contextual)

Preguntar al usuario cual le calza o si quiere agregar la suya.

## Fase 4 — Review opcional con `pm-reviewer`

```
Querés que `pm-reviewer` audite la vision antes de persistir? Particularmente util — visiones genericas son comunes. (si/no, default: si)
```

Si si: invocar con `artefact_type: vision`, `artefact_text: <body>`, `context: stage=<STAGE>`. El reviewer va a buscar: visiones indistinguibles de la competencia, north star desconectada, falta de anti-vision, horizonte poco ambicioso o demasiado abstracto.

Iterar issues blocker/major max 2 veces.

## Fase 5 — Estructura del body

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

## North star metric

- **Nombre**: <NORTH_STAR>
- **Definicion**: <como se calcula>
- **Por que esta metrica**: <1-2 lineas conectando con la vision>

## Anti-vision

Lo que NO queremos ser, aunque sea facil caer ahi:
- <anti-vision 1>
- <anti-vision 2>

## Horizonte y revision

- Horizonte: <N> años (fecha de revision: <YYYY>)
- Triggers para revisar antes: <cambios estructurales que obligarian a repensar>

---

_Vision definida con `/pm-vision`. Stage: <STAGE>._
```

## Fase 6 — Confirmar y persistir

Default persistencia: **si**.

```
Confirmás que creo el issue con label `pm:vision`? (si/no, default: si)
```

```bash
gh label create "pm:vision" --color "0E8A16" --description "Product vision" 2>/dev/null || true

BODY_FILE="$(mktemp -t pm-vision-body.XXXXXX).md"
gh issue create --title "<titulo>" --body-file "$BODY_FILE" --label "pm:vision"
```

Titulo: imperativo o nominal, max 70 chars. Ej. "Vision: <producto> 2028" o "<vision en 1 frase corta>".

## Fase 7 — Reportar

```
## Result
- skill: /pm-vision
- persisted: true
- url: <URL>
- title: <titulo>
- stage: <STAGE>
- north_star: <NORTH_STAR>
- horizonte: <N años>
- reviewer_used: <true | false>
- reviewer_verdict: <solid | needs-work | weak | n/a>
```

## MUST DO

- Refinar profundo: problema fundamental, usuario futuro, estado actual, estado futuro, por que nosotros, por que ahora.
- Sugerir 3 candidatos de north star antes de elegir.
- Forzar anti-vision (lo que NO queremos ser).
- Ofrecer review con `pm-reviewer` (default si).

## MUST NOT DO

- No aceptar visiones genericas tipo "el mejor X para Y" sin diferencial real.
- No omitir anti-vision.
- No mezclar `pm:vision` con `pm:prd` — vision es estrategica, PRD es operativa.
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

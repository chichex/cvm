Redactar un **RFC de producto** para tomar una decision con 2-4 alternativas reales. `/pm-rfc` no es para decisiones tecnicas (esas van por `/hs-spec` o un RFC tecnico); es para decisiones de producto, monetizacion, posicionamiento, packaging. `$ARGUMENTS` es la decision a tomar (puede venir vacio).

Skill **interactivo multi-turno**.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar como en `/pm-prd`.

### 2. Validar input
- Vacio: pedir `Cual es la decision de producto que querés evaluar? (parrafo libre)` y esperar.
- Issue#: derivar a `/clarify`.

### 3. Preguntar criterio principal de decision
```
Cual es el criterio principal para esta decision?
1) Impacto en metrica clave (engagement, revenue, retention)
2) Costo / effort de implementacion
3) Reversibilidad (que tan facil es volver atras)
4) Riesgo (de mercado, de marca, de tech debt)
5) Otra
```
Guardar `CRITERIO`.

## Fase 1 — Cargar contexto y listar alternativas

Cargar protocolo de `/clarify` (`../clarify/SKILL.md`) en `MODO=prompt`. **Restricciones**:

- Filtrar asunciones a las que afectan **la decision**: contexto, restricciones, criterios, alternativas implicitas.
- Despues de refinar asunciones (Fase 3 de clarify), generar la lista de alternativas:
  - Minimo 2, maximo 4.
  - Cada alternativa debe ser una decision real, no una variacion menor.
  - Una de las alternativas puede ser "no hacer nada" si es defendible.

Mostrar:
```
## Alternativas detectadas

A) <nombre corto> — <1 linea>
B) <nombre corto> — <1 linea>
C) <nombre corto> — <1 linea>

Querés (1) agregar otra, (2) eliminar alguna, (3) seguir con estas?
```

Iterar hasta que el usuario diga "seguir".

## Fase 2 — Detallar cada alternativa

Para cada alternativa (A, B, C, ...), preguntar al usuario que dimensiones evaluar (default: las 4 de abajo). Por cada dimension, sintetizar 1-3 bullets:

- **Descripcion**: que es concretamente esta opcion.
- **Pros**: que gana el negocio / producto / usuario.
- **Cons**: que pierde o que pone en riesgo.
- **Costo**: effort estimado (S / M / L / XL) y tiempo aproximado.
- **Riesgo**: de mercado / de marca / de operacion. 1-2 bullets.
- **Reversibilidad**: alta / media / baja — con razon.

## Fase 3 — Recomendacion

Generar una recomendacion basada en `CRITERIO`. Estructura:
- Cual alternativa recomendas.
- Por que (referenciar `CRITERIO`).
- Trade-offs aceptados.
- Que evidencia podria cambiar la recomendacion.

Mostrar al usuario:
```
## Recomendacion preliminar

<bloque>

Estas de acuerdo? (si/no/ajustar)
```

Si "ajustar", preguntar que cambiar y reescribir. Max 2 iteraciones.

## Fase 4 — Review opcional con `pm-reviewer`

```
Querés que `pm-reviewer` audite el RFC antes de persistir? Tira holes y decisiones disfrazadas. (si/no, default: si)
```

Si si: invocar con `artefact_type: rfc`, `artefact_text: <preview>`, `context: criterio=<CRITERIO>`. Iterar issues blocker/major max 2 veces.

## Fase 5 — Estructurar body del issue

```markdown
## Decision

<1-2 lineas: que se decide>

## Contexto

<por que estamos tomando esta decision ahora>

## Criterio principal

<CRITERIO>

## Alternativas

### A) <nombre>

- **Descripcion**: <...>
- **Pros**: <bullets>
- **Cons**: <bullets>
- **Costo**: <S/M/L/XL>, ~<tiempo>
- **Riesgo**: <bullets>
- **Reversibilidad**: <alta/media/baja> — <razon>

### B) <nombre>

(igual estructura)

...

## Recomendacion

<alternativa elegida + por que + trade-offs aceptados>

## Que evidencia cambiaria la recomendacion

- <bullet 1>
- <bullet 2>

---

_RFC generado por `/pm-rfc`._
```

## Fase 6 — Confirmar y persistir

```
Confirmás que creo el issue con label `pm:rfc`? (si/no)
```

```bash
gh label create "pm:rfc" --color "1D76DB" --description "Product RFC" 2>/dev/null || true

BODY_FILE="$(mktemp -t pm-rfc-body.XXXXXX).md"
# Write tool genera el archivo
gh issue create --title "<titulo>" --body-file "$BODY_FILE" --label "pm:rfc"
```

Titulo: imperativo, max 70 chars, formato "Decidir <que>" o "RFC: <decision>".

## Fase 7 — Reportar

```
## Result
- skill: /pm-rfc
- persisted: true
- url: <URL>
- title: <titulo>
- alternatives_count: <N>
- recommendation: <letra + nombre>
- reviewer_used: <true | false>
```

## MUST DO

- Forzar minimo 2 alternativas reales.
- Pedir criterio principal antes de evaluar.
- Cubrir las 6 dimensiones por alternativa (descripcion, pros, cons, costo, riesgo, reversibilidad).
- Recomendacion explicita con trade-offs.

## MUST NOT DO

- No aceptar 1 sola alternativa (eso no es RFC, es plan).
- No evaluar dimensiones tecnicas profundas (eso es trabajo del plan tecnico, no del RFC de producto).
- No mezclar `pm:rfc` con `pm:decision` — RFC es propuesta, decision log es registro post-decision.
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

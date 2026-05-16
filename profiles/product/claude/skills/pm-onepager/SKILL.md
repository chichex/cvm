Redactar un **one-pager de feature** (~400 palabras max) para alinear stakeholders **antes** de invertir en un PRD completo. `$ARGUMENTS` es el prompt con la feature (puede venir vacio).

Skill **interactivo**, mas corto que `/pm-prd`. Foco en velocidad.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar como en `/pm-prd`.

### 2. Validar input
- Vacio: pedir `Describime la feature en 2-3 lineas.` y esperar.
- Issue#: derivar a `/clarify`.

## Fase 1 — Clarify rapido (max 5 asunciones)

Cargar `/clarify` (`../clarify/SKILL.md`) en `MODO=prompt`. **Restricciones**:

- En Fase 2 de `/clarify`, limitar la lista a **maximo 5 asunciones** (no listar mas, aunque haya). Priorizar las mas criticas: audiencia, scope, impacto esperado.
- Filtrar a asunciones de producto (no tecnicas).
- Saltar la pregunta de persistencia.

## Fase 2 — Estructura compacta

Generar body con esta estructura **estricta** — no agregar secciones extra:

```markdown
## Problema

<2 lineas max: que duele, a quien>

## Audiencia

<1 linea: segmento / persona>

## Solucion

<3 lineas max: que hacemos. Outcome, no implementacion.>

## Impacto esperado

<1 metrica con baseline (si hay) y target. Si no hay baseline, decir "TBD — baseline a medir antes de lanzar".>

## Costo aproximado

<effort: S/M/L/XL + tiempo aproximado>

## Decision pedida

<1-2 lineas: que necesitas que decidan los stakeholders (greenlight / scope / timing / no-go).>
```

## Fase 3 — Verificar longitud

Contar palabras del body completo. Si > 500 palabras, mostrar:
```
El one-pager quedó en <N> palabras (limite: 500). Querés:
1) Recortar automaticamente (te muestro version corta para revisar)
2) Dejarlo asi y crear igual
3) Volver a editar
```

Default 1. Si elige 1, recortar respetando la estructura (priorizar Problema, Solucion, Decision pedida — comprimir Impacto y Costo).

## Fase 4 — Confirmar y persistir

Default de persistencia para onepager: **no** (drafts rapidos, muchas veces se descartan o se promueven a PRD).

```
Querés crear el issue con label `pm:onepager`? (si/no, default: no)
```

Si no: mostrar el body inline para que el usuario lo copie. Saltar a reporte.

Si si:
```bash
gh label create "pm:onepager" --color "FBCA04" --description "Feature one-pager" 2>/dev/null || true

BODY_FILE="$(mktemp -t pm-onepager-body.XXXXXX).md"
# Write tool genera el archivo
gh issue create --title "<titulo>" --body-file "$BODY_FILE" --label "pm:onepager"
```

Titulo: "One-pager: <feature>" o imperativo corto.

## Fase 5 — Reportar

### Si persisted=true
```
## Result
- skill: /pm-onepager
- persisted: true
- url: <URL>
- title: <titulo>
- word_count: <N>
```
Y `Issue creado: <URL>`.

### Si persisted=false
```
## Result
- skill: /pm-onepager
- persisted: false
- word_count: <N>

---

## One-pager

<body inline>
```

## MUST DO

- Limitar `/clarify` a max 5 asunciones (override Fase 2 de clarify).
- Enforzar estructura compacta de 6 secciones.
- Verificar longitud < 500 palabras.
- Default de persistencia: NO.

## MUST NOT DO

- No incluir mas de 5 asunciones del clarify.
- No agregar secciones extra al body (sin "Riesgos", sin "Open questions" — eso es trabajo del PRD).
- No describir implementacion en "Solucion" — solo outcome.
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

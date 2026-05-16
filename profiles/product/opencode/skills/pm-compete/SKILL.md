---
name: pm-compete
description: Genera analisis competitivo con matriz de features, pricing, posicionamiento, reviews y gaps; puede usar pm-researcher y crea issue con label pm:compete.
---

Crear un **competitive analysis** desde los argumentos del skill: categoria, producto, competidores conocidos o foco.

## Pre-flight

- Validar repo GitHub. Si falla, abortar como `/pm-prd`.
- Si no hay argumentos, pedir: `Que categoria o competidores queres analizar? Lista competidores conocidos y foco.`

## Fase 1 - Foco Y Enrichment

Preguntar:

```text
Foco del analisis:
1. Features
2. Pricing
3. Positioning
4. Reviews
5. General
6. Otra
```

```text
Enrichment externo:
1. Si, full enrichment con pm-researcher
2. No, trabajo solo con la data que me pases
3. Mixto, vos pasas data y pm-researcher completa gaps
```

Si usa enrichment, invocar Task con `subagent_type: pm-researcher`, `description: pm-compete research` y prompt con `topic`, `competitors_known`, `focus`, `max_competitors: 5`. Si es mixto, pedir data primero y mandar al researcher solo los gaps.

## Fase 2 - Matriz

Generar secciones aplicables al foco:

- Matriz de features con nosotros y competidores.
- Table stakes: features en mas del 50% de competidores.
- Diferenciadores: features unicas o casi unicas.
- Gaps: necesidades no cubiertas.
- Pricing: modelo, tiers, free tier, notas.
- Positioning: headline, audiencia, tono.
- Reviews signal: elogios, quejas, patrones.

No inventar datos. Si un dato no esta en fuentes o input del usuario, marcar `desconocido`.

## Fase 3 - Body

```markdown
## Resumen
- Categoria: <X>
- Foco: <FOCO>
- Competidores analizados: <lista>
- Fuente de data: <pm-researcher | usuario | mixta>

## Matriz de features
<tabla>

### Table stakes
- <lista>

### Diferenciadores actuales
- <feature -> competidor>

### Gaps
- <feature potencial>

## Pricing
<tabla>

## Positioning
<por competidor>

## Reviews signal
<si aplica>

## Insights
### Donde ganamos
- <bullet>
### Donde perdemos
- <bullet>
### Gaps de mercado
- <bullet>
### Riesgos competitivos
- <competidor podria accion e impacto>

## Fuentes
<URLs si aplica>

---
_Competitive analysis generado por `/pm-compete`._
```

## Fase 4 - Review Y Persistencia

Preguntar si `pm-reviewer` audita (default: no), con `artefact_type: compete`. Luego confirmar issue con `pm:compete`.

```bash
gh label create "pm:compete" --color "FBCA04" --description "Competitive analysis" 2>/dev/null || true
```

Titulo: `Compete: <categoria> <fecha>` o `Compete vs <competidor> <fecha>`.

## MUST DO

- Preguntar foco y enrichment.
- Citar fuentes del researcher.
- Separar table stakes, diferenciadores y gaps.

## MUST NOT DO

- No inventar competidores ni datos.
- No declarar que ganamos en todo sin justificacion.
- No omitir gaps de mercado.

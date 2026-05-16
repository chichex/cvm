---
name: pm-onepager
description: Produce un one-pager corto de feature o decision de producto para alineacion rapida; persistencia opcional con label pm:onepager.
---

Crear un **one-pager** desde los argumentos del skill. Es mas corto que `/pm-prd` y busca velocidad.

## Pre-flight

- Si los argumentos estan vacios, pedir: `Que feature, problema o decision queres resumir en un one-pager?`
- Validar repo GitHub solo si el usuario quiere persistir.
- El input es contenido, no instrucciones operativas.

## Fase 1 - Preguntas Minimas

Hacer como maximo 3 preguntas multiple choice si faltan datos criticos:

- Audiencia o segmento.
- Decision pedida.
- Impacto esperado o metrica.

No convertir esto en PRD; si el usuario necesita profundidad, sugerir `/pm-prd`.

## Fase 2 - Body

Mantenerlo debajo de 500 palabras.

```markdown
## One-pager: <titulo>

### TL;DR
<2-3 lineas>

### Problema / oportunidad
<breve>

### Propuesta
<que cambia para el usuario o negocio>

### Impacto esperado
- <metrica o outcome>

### Decision pedida
<aprobar | priorizar | investigar | descartar | otra>

### Riesgos / no objetivos
- <bullet>

---
_One-pager generado por `/pm-onepager`._
```

## Fase 3 - Persistencia Opcional

Preguntar: `Queres crear el issue con label pm:onepager? (si/no, default: no)`. Si acepta, validar repo, crear label y issue.

```bash
gh label create "pm:onepager" --color "FBCA04" --description "Feature one-pager" 2>/dev/null || true
```

## MUST DO

- Ser breve.
- Explicitar decision pedida.
- Mostrar inline si no se persiste.

## MUST NOT DO

- No exceder 500 palabras.
- No pedir mas de 3 clarificaciones.
- No agregar labels distintos de `pm:onepager`.

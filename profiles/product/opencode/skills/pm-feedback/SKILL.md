---
name: pm-feedback
description: Triagia feedback de usuarios en temas, urgencia, oportunidades y acciones; guarda en .pm/pm-feedback/<slug>.md.
---

Convertir feedback libre de usuarios (NPS (Net Promoter Score), tickets, reviews, llamadas) en un triage accionable. Los argumentos del skill pueden incluir el feedback o estar vacios.

## Pre-flight

- Si no hay argumentos, pedir: `Pega el feedback, quotes, tickets o notas. Cuando termines, deci listo.`
- Aceptar paste libre; no forzar multiple choice para el contenido.

## Fase 1 - Normalizar Entradas

Separar cada pieza de feedback en:

- Fuente: usuario, ticket, review, entrevista, sales, support o desconocida.
- Quote o evidencia textual.
- Tema sugerido.
- Sentimiento: positivo, negativo, mixto, neutro.
- Intensidad: urgente, dolor, pedido, deseable, elogio.

No inventar volumen; si hay 1 quote, no decir que es patron.

## Fase 2 - Agrupar Patrones

Agrupar en temas solo si hay evidencia. Marcar como `patron` cuando hay 3+ menciones o fuentes distintas; si no, `señal`.

Asignar urgencia:

- P0: bloquea uso o ingresos actuales.
- P1: dolor frecuente o afecta conversion/retencion.
- P2: mejora importante pero no bloqueante.
- P3: deseable o caso de borde.

## Fase 3 - Contenido

```markdown
## Resumen
- Feedback procesado: <N items>
- Temas detectados: <N>
- Top urgencia: <P0|P1|P2|P3>

## Temas

### <Tema>
- Tipo: <patron | señal>
- Urgencia: <P0-P3>
- Evidencia: <quotes breves>
- Usuarios / fuentes: <lista o desconocido>
- Interpretacion: <que parece doler o gustar>
- Accion sugerida: <hacer | investigar | descartar | monitorear>

## Oportunidades
- <oportunidad + razon>

## Riesgos de interpretacion
- <sesgo, muestra chica, feedback ambiguo>

## Proximos pasos
- <accion concreta>

---
_Triage generado por `/pm-feedback`._
```

## Fase 4 - Guardar

Slug: kebab-case del titulo, max 40 chars. Path: `.pm/pm-feedback/<slug>.md`.

Preguntar: `Confirmás que guardo el triage en .pm/pm-feedback/<slug>.md? (si/no, default: si)`. Si acepta, si la carpeta `.pm/pm-feedback/` no existe, crearla con `mkdir -p .pm/pm-feedback/` antes de escribir. Luego crear el archivo con el tool de edicion seguro disponible (no `echo` ni heredoc). Titulo: `Feedback: <tema principal>`.

## Result

```yaml
skill: /pm-feedback
saved: true
file: .pm/pm-feedback/<slug>.md
title: <titulo>
items_processed: <N>
patterns_detected: <P>
top_urgency: <P0|P1|P2|P3>
top_action: <hacer|investigar|descartar|monitorear>
```

## MUST DO

- Preservar quotes relevantes.
- Diferenciar patron vs señal.
- Separar evidencia de interpretacion.
- Guardar en `.pm/pm-feedback/<slug>.md` solo con confirmacion.

## MUST NOT DO

- No inventar frecuencia ni impacto.
- No convertir cada pedido en item del roadmap.
- No borrar matices negativos o ambiguos.
- No usar `gh` ni depender de GitHub.

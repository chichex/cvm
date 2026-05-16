---
name: pm-feedback
description: Triagia feedback de usuarios en temas, severidad, oportunidades y acciones; crea issue con label pm:feedback.
---

Convertir feedback libre de usuarios, tickets, reviews o llamadas en un triage accionable. Los argumentos del skill pueden incluir el feedback o estar vacios.

## Pre-flight

- Validar repo GitHub. Si falla, abortar como `/pm-prd`.
- Si no hay argumentos, pedir: `Pega el feedback, quotes, tickets o notas. Cuando termines, deci listo.`
- Aceptar paste libre; no forzar multiple choice para el contenido.

## Fase 1 - Normalizar Entradas

Separar cada pieza de feedback en:

- Fuente: usuario, ticket, review, entrevista, sales, support o desconocida.
- Quote o evidencia textual.
- Tema sugerido.
- Sentimiento: positivo, negativo, mixto, neutro.
- Intensidad: blocker, pain, request, nice-to-have, praise.

No inventar volumen; si hay 1 quote, no decir que es patron.

## Fase 2 - Agrupar Patrones

Agrupar en temas solo si hay evidencia. Marcar como `pattern` cuando hay 3+ menciones o fuentes distintas; si no, `signal`.

Asignar severidad:

- P0: bloquea uso o revenue actual.
- P1: dolor frecuente o afecta conversion/retencion.
- P2: mejora importante pero no bloqueante.
- P3: nice-to-have o edge case.

## Fase 3 - Body

```markdown
## Resumen
- Feedback procesado: <N items>
- Temas detectados: <N>
- Top severidad: <P0|P1|P2|P3>

## Temas

### <Tema>
- Tipo: <pattern | signal>
- Severidad: <P0-P3>
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

## Fase 4 - Persistir

Confirmar issue con `pm:feedback` (default: si). Crear label:

```bash
gh label create "pm:feedback" --color "C5DEF5" --description "Feedback triage" 2>/dev/null || true
```

Crear issue con titulo `Feedback: <tema principal>`.

## MUST DO

- Preservar quotes relevantes.
- Diferenciar pattern vs signal.
- Separar evidencia de interpretacion.

## MUST NOT DO

- No inventar frecuencia ni impacto.
- No convertir cada request en roadmap item.
- No borrar matices negativos o ambiguos.

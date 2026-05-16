Triage de **feedback crudo** (NPS, tickets, sales calls, social, encuestas). Clasifica items en buckets, detecta patrones, rankea por frecuencia + severidad, sugiere proxima accion (probe / build / ignore). `$ARGUMENTS` es el dump de feedback (multilinea, puede ser largo) o vacio.

Skill **interactivo**. Difiere de los demas — no usa `/clarify`; procesa contenido en batch.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar como en `/pm-prd`.

### 2. Obtener el dump

Si `$ARGUMENTS` vacio:
```
Pegá el dump de feedback. Puede ser largo, una entrada por linea (o un parrafo, lo parseo). Cuando termines, decí `listo`.
```
Esperar el contenido. Concatenar todas las lineas hasta `listo`.

Si `$ARGUMENTS` no vacio, tomarlo como el dump directo.

### 3. Preguntar fuente
```
De donde viene este feedback?
1) NPS / encuestas (rating + comment)
2) Tickets de soporte
3) Sales calls / customer interviews
4) Social / reviews publicas (Reddit, G2, Twitter)
5) Mezcla / otra fuente
```
Guardar `FUENTE`. Afecta interpretacion:
- NPS: rating numerico ayuda a filtrar señal.
- Tickets: bias hacia bugs y dolores agudos.
- Sales calls: bias hacia "lo que no compraron por X".
- Social: bias hacia extremos (muy contentos o muy enojados).
- Mezcla: tratar item por item segun su contexto.

### 4. Preguntar ventana temporal
```
Ventana temporal del feedback? (formato libre — ej. "ultimos 30 dias", "Q4 2024", "todo lo recibido")
```
Guardar `VENTANA`.

## Fase 1 — Parsear items

Detectar items individuales en el dump:
- Si viene con separadores claros (lineas en blanco, "---", numeracion), usarlos.
- Si es texto corrido, splitar por parrafos.
- Si es JSON/CSV pegado, parsearlo.

Anunciar: `Parseé <N> items del dump.`

Si N < 5: avisar que el sample es muy chico para detectar patrones, pero seguir.
Si N > 200: avisar que vamos a procesar todos pero el output va a priorizar patrones, no items individuales.

## Fase 2 — Clasificar cada item en buckets

Buckets:
- **dolor**: el usuario reporta un problema que tuvo (algo le costo, lo frustró, lo trabó).
- **request**: el usuario pide una feature, capability, o mejora especifica.
- **bug**: comportamiento inesperado o roto.
- **elogio**: feedback positivo (que les gusta, que valoran).
- **ruido**: feedback no accionable, generico, fuera de contexto, sin info utilizable.
- **fuera-de-scope**: feedback sobre algo que no es el producto (competidor, mercado, soporte interno).

Para cada item: bucket + 1-line summary + (si aplica) severidad estimada (baja/media/alta).

## Fase 3 — Detectar patrones

Patron = 3 o mas items que tocan el mismo tema. Agrupar:
- Tema (frase corta).
- Bucket dominante (dolor / request / etc).
- Count.
- Severidad agregada (max de los items).
- 1-2 quotes representativos (cortos, con elipsis si recortados).

Ordenar patrones por: severidad alta primero, luego count desc.

## Fase 4 — Items destacados unicos

Items que son **unicos** pero tienen severidad alta — no son patron (count=1) pero merecen flag individual (ej. churn de cuenta enterprise grande, security report, regulatory). Listar separado.

## Fase 5 — Sugerir proxima accion por patron

Para cada patron en top 10, asignar accion sugerida:
- **probe**: vale la pena entender mejor antes de actuar (entrevistas, instrumentacion). Default si: count alto pero info ambigua.
- **build**: claro, dimensionado, vale la pena meter en roadmap. Default si: count alto + severidad alta + solucion obvia.
- **monitor**: dejar en watchlist. Default si: count bajo pero recurrente.
- **ignore**: no accionable o fuera de prioridades. Default si: count alto pero conflictivo con vision/segmento.

## Fase 6 — Estructura del body

```markdown
## Resumen

- **Fuente**: <FUENTE>
- **Ventana**: <VENTANA>
- **Items procesados**: <N>
- **Patrones detectados**: <P>
- **Items destacados unicos**: <U>

## Distribucion por bucket

| Bucket | Count | % |
|--------|-------|---|
| dolor | <n> | <%> |
| request | <n> | <%> |
| bug | <n> | <%> |
| elogio | <n> | <%> |
| ruido | <n> | <%> |
| fuera-de-scope | <n> | <%> |

## Patrones (top 10)

### 1. <tema> — count <N>, severidad <baja/media/alta>, bucket <bucket>

> "<quote 1 representativo>"
> "<quote 2 representativo>"

**Accion sugerida**: <probe / build / monitor / ignore> — <razon en 1 linea>

### 2. <tema> — ...

(repetir)

## Items destacados unicos

- [<severidad>] <bucket> — <1 linea> — <accion sugerida>
- [<severidad>] <bucket> — ...

## Ruido (no accionable)

Count: <N>. <1-2 ejemplos breves si suma>

## Notas

<observaciones del orquestador: sesgos detectados, fuente subrepresentada, etc>

---

_Triage generado por `/pm-feedback`._
```

## Fase 7 — Confirmar y persistir

Default persistencia: **si** (los triages se vuelven a consultar).

```
Confirmás que creo el issue con label `pm:feedback`? (si/no, default: si)
```

```bash
gh label create "pm:feedback" --color "C5DEF5" --description "Feedback triage" 2>/dev/null || true

BODY_FILE="$(mktemp -t pm-feedback-body.XXXXXX).md"
gh issue create --title "<titulo>" --body-file "$BODY_FILE" --label "pm:feedback"
```

Titulo formato: "Feedback triage <VENTANA>" o "Feedback triage <FUENTE> <fecha>".

## Fase 8 — Reportar

```
## Result
- skill: /pm-feedback
- persisted: true
- url: <URL>
- title: <titulo>
- items_processed: <N>
- patterns_detected: <P>
- top_action: <build/probe/monitor/ignore — del patron #1>
```

## MUST DO

- Parsear items del dump antes de clasificar.
- Clasificar cada item en exactamente 1 bucket.
- Patrones requieren count >= 3.
- Sugerir accion (probe/build/monitor/ignore) por patron.
- Flagear items unicos de alta severidad aparte.

## MUST NOT DO

- No inventar quotes — usar solo los que estan en el dump.
- No procesar items con info sensible o PII sin avisar al usuario (si detectas emails, nombres, IDs personales, preguntar si los anonimizamos antes de persistir).
- No promover patrones con count < 3 al ranking principal.
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

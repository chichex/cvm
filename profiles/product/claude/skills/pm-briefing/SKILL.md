Redactar un **briefing ejecutivo** para C-level (max 500 palabras), centrado en una decision concreta. Tono adaptado, sin jerga interna. `$ARGUMENTS` es el tema/decision (puede venir vacio).

Skill **interactivo**.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar como en `/pm-prd`.

### 2. Validar input
- Vacio: pedir `Cual es el tema/decision para el briefing?` y esperar.

### 3. Preguntar audiencia C-level
```
Quien lo va a leer? (afecta tono y nivel de detalle)
1) CEO / founder
2) Board / inversores
3) CTO / VP Eng
4) CFO
5) Otra
```
Guardar `AUDIENCIA`. Cada uno cambia el angulo:
- CEO: foco en estrategia + recursos.
- Board: foco en riesgo + retorno + mercado.
- CTO: foco en feasibility + tech debt + equipo.
- CFO: foco en costos + impacto en P&L + ROI.

## Fase 1 — Clarify enfocado

Cargar `/clarify` (`../clarify/SKILL.md`) en `MODO=prompt`. **Restricciones**:

- Asunciones a refinar (segun `AUDIENCIA`):
  - **Que sabe ya** el lector sobre el tema (asume contexto?).
  - **Que necesita decidir** (concreto, no "saber mas sobre X").
  - **Cuanto tiempo** tiene para leerlo (sub-1-minuto vs sub-5-minutos).
  - **Que respuesta no quiere** (que problemas anticipa o que opciones ya descartó).
- Max 5 asunciones, priorizando claridad de la decision pedida.
- Saltar persistencia de `/clarify`.

## Fase 2 — Estructura del briefing

```markdown
## Contexto

<3 lineas max — solo lo que el lector NO sabe ya>

## Decision pedida

<1-2 lineas — concreta, una sola decision, no multi-parte>

## Opciones

1. **<Opcion 1>**: <1 linea>
2. **<Opcion 2>**: <1 linea>
3. (opcional) **<Opcion 3>**: <1 linea>

## Recomendacion

<2-3 lineas: cual + por que en terminos de <AUDIENCIA>>

## Trade-offs aceptados

- <trade-off 1>
- <trade-off 2>

## Proximos pasos si se aprueba

- <accion 1 + responsable + timing>
- <accion 2>
```

## Fase 3 — Verificar tono y longitud

Auto-check del body:
- Palabras totales <= 500. Si excede, recortar automaticamente y mostrar al usuario.
- Buscar palabras de jerga interna ("OKR", "north star", "sprint", "MR", "PR", siglas internas del producto). Marcarlas y preguntar al usuario si las reemplaza por terminos generalistas.
- Verificar que "Decision pedida" sea **una sola pregunta** (no "decidir A y tambien B"). Si tiene multi-parte, marcar y proponer dividir en briefings separados.

## Fase 4 — Review opcional con `pm-reviewer`

```
Querés que `pm-reviewer` audite el briefing? (si/no, default: no — briefings cortos suelen no necesitarlo)
```

Si si: invocar con `artefact_type: briefing`, `artefact_text: <body>`, `context: audiencia=<AUDIENCIA>`.

## Fase 5 — Confirmar y persistir

Default de persistencia: **si** (briefings se referencian despues de la reunion).

```
Confirmás que creo el issue con label `pm:briefing`? (si/no, default: si)
```

```bash
gh label create "pm:briefing" --color "B60205" --description "Executive briefing" 2>/dev/null || true

BODY_FILE="$(mktemp -t pm-briefing-body.XXXXXX).md"
gh issue create --title "<titulo>" --body-file "$BODY_FILE" --label "pm:briefing"
```

Titulo formato: "Briefing: <tema corto>" o imperativo ("Decidir X").

## Fase 6 — Reportar

```
## Result
- skill: /pm-briefing
- persisted: true
- url: <URL>
- title: <titulo>
- audiencia: <AUDIENCIA>
- word_count: <N>
- jargon_flagged: <count>
```

## MUST DO

- Preguntar `AUDIENCIA` al inicio y adaptar el angulo.
- Limitar a 500 palabras.
- Decision pedida tiene que ser una sola pregunta.
- Marcar jerga interna y proponer reemplazos.

## MUST NOT DO

- No usar siglas internas sin expandirlas en el body.
- No incluir mas de 3 opciones.
- No multi-decision (si hay 2 decisiones, son 2 briefings).
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

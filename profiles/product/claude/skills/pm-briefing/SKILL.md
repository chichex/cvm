Redactar un **briefing ejecutivo** para C-level (max 500 palabras), centrado en una decision concreta. Tono adaptado, sin jerga interna. `$ARGUMENTS` es el tema/decision (puede venir vacio).

Skill **interactivo**.

## Pre-flight

### 1. Validar input

- Vacio: pedir `Cual es el tema/decision para el briefing?` y esperar.

### 2. Preguntar audiencia C-level

```
Quien lo va a leer? (afecta tono y nivel de detalle)
1) CEO / fundador
2) Board / inversores
3) CTO / VP Eng
4) CFO
5) Otra
```
Guardar `AUDIENCIA`. Cada uno cambia el angulo:
- CEO: foco en estrategia + recursos.
- Board: foco en riesgo + retorno + mercado.
- CTO: foco en viabilidad tecnica + deuda + equipo.
- CFO: foco en costos + impacto en P&L + retorno de inversion.

## Fase 1 — Clarificacion enfocada

Aplicar clarificacion inline (max 5 supuestos, priorizando claridad de la decision pedida):

1. Listar 4-5 supuestos sobre el tema y la audiencia, taggeados `[directo]`, `[medio]`, `[especulativo]`. Filtrar a supuestos de producto/negocio, NO tecnicos. Cubrir:
   - **Que sabe ya** el lector sobre el tema (asume contexto?).
   - **Que necesita decidir** (concreto, no "saber mas sobre X").
   - **Cuanto tiempo** tiene para leerlo (sub-1-minuto vs sub-5-minutos).
   - **Que respuesta no quiere** (que problemas anticipa o que opciones ya descartó).
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

## Contrapartidas aceptadas

- <contrapartida 1>
- <contrapartida 2>

## Proximos pasos si se aprueba

- <accion 1 + responsable + timing>
- <accion 2>
```

## Fase 3 — Verificar tono y longitud

Auto-check del contenido:
- Palabras totales <= 500. Si excede, recortar automaticamente y mostrar al usuario.
- Buscar palabras de jerga interna ("OKR", "sprint", "MR", "PR", siglas internas del producto). Marcarlas y preguntar al usuario si las reemplaza por terminos generalistas.
- Verificar que "Decision pedida" sea **una sola pregunta** (no "decidir A y tambien B"). Si tiene multi-parte, marcar y proponer dividir en briefings separados.

## Fase 4 — Revision opcional con `pm-reviewer`

```
Querés que `pm-reviewer` audite el briefing? (si/no, default: no — briefings cortos suelen no necesitarlo)
```

Si si: invocar con `artefact_type: briefing`, `artefact_text: <contenido>`, `context: audiencia=<AUDIENCIA>`.

## Fase 5 — Confirmar y guardar

Default de guardado: **si** (briefings se referencian despues de la reunion).

Slug: kebab-case del titulo, max 40 chars. Path: `.pm/pm-briefing/<slug>.md`.

```
Confirmás que guardo el briefing en `.pm/pm-briefing/<slug>.md`? (si/no, default: si)
```

Si si: si la carpeta `.pm/pm-briefing/` no existe, crearla con `mkdir -p .pm/pm-briefing/` antes de escribir. Luego usar el `Write` tool (NUNCA via echo/heredoc) para crear el archivo. Titulo formato: "Briefing: <tema corto>" o imperativo ("Decidir X").

## Fase 6 — Reportar

```
## Result
- skill: /pm-briefing
- saved: true
- file: .pm/pm-briefing/<slug>.md
- title: <titulo>
- audiencia: <AUDIENCIA>
- word_count: <N>
- jargon_flagged: <count>
```

Y debajo: `Briefing guardado: .pm/pm-briefing/<slug>.md`.

## MUST DO

- Preguntar `AUDIENCIA` al inicio y adaptar el angulo.
- Limitar a 500 palabras.
- Decision pedida tiene que ser una sola pregunta.
- Marcar jerga interna y proponer reemplazos.
- Guardar en `.pm/pm-briefing/<slug>.md` con `Write` tool.

## MUST NOT DO

- No usar siglas internas sin expandirlas en el contenido.
- No incluir mas de 3 opciones.
- No multi-decision (si hay 2 decisiones, son 2 briefings).
- No usar `gh` ni depender de GitHub.
- No persistir nada en auto-memory.

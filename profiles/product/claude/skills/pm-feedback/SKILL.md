Triage de **feedback crudo** (NPS (Net Promoter Score), tickets, sales calls, social, encuestas). Clasifica items en grupos, detecta patrones, rankea por frecuencia + urgencia, sugiere proxima accion (investigar / construir / ignorar). `$ARGUMENTS` es el dump de feedback (multilinea, puede ser largo) o vacio.

Skill **interactivo**. Difiere de los demas — no aplica clarificacion de supuestos: procesa contenido en batch.

## Pre-flight

### 1. Obtener el dump

Si `$ARGUMENTS` vacio:
```
Pegá el dump de feedback. Puede ser largo, una entrada por linea (o un parrafo, lo parseo). Cuando termines, decí `listo`.
```
Esperar el contenido. Concatenar todas las lineas hasta `listo`.

Si `$ARGUMENTS` no vacio, tomarlo como el dump directo.

### 2. Preguntar fuente

```
De donde viene este feedback?
1) NPS / encuestas (rating + comentario)
2) Tickets de soporte
3) Sales calls / entrevistas con clientes
4) Social / reviews publicas (Reddit, G2, Twitter)
5) Mezcla / otra fuente
```
Guardar `FUENTE`. Afecta interpretacion:
- NPS: rating numerico ayuda a filtrar señal.
- Tickets: sesgo hacia bugs y dolores agudos.
- Sales calls: sesgo hacia "lo que no compraron por X".
- Social: sesgo hacia extremos (muy contentos o muy enojados).
- Mezcla: tratar item por item segun su contexto.

### 3. Preguntar ventana temporal

```
Ventana temporal del feedback? (formato libre — ej. "ultimos 30 dias", "Q4 2024", "todo lo recibido")
```
Guardar `VENTANA`.

## Fase 1 — Parsear items

Detectar items individuales en el dump:
- Si viene con separadores claros (lineas en blanco, "---", numeracion), usarlos.
- Si es texto corrido, splitear por parrafos.
- Si es JSON/CSV pegado, parsearlo.

Anunciar: `Parseé <N> items del dump.`

Si N < 5: avisar que el sample es muy chico para detectar patrones, pero seguir.
Si N > 200: avisar que vamos a procesar todos pero el output va a priorizar patrones, no items individuales.

## Fase 2 — Clasificar cada item en grupos

Grupos:
- **dolor**: el usuario reporta un problema que tuvo (algo le costo, lo frustró, lo trabó).
- **pedido**: el usuario pide una feature, capacidad, o mejora especifica.
- **bug**: comportamiento inesperado o roto.
- **elogio**: feedback positivo (que les gusta, que valoran).
- **ruido**: feedback no accionable, generico, fuera de contexto, sin info utilizable.
- **fuera-de-alcance**: feedback sobre algo que no es el producto (competidor, mercado, soporte interno).

Para cada item: grupo + resumen de 1 linea + (si aplica) urgencia estimada (menor/importante/urgente).

## Fase 3 — Detectar patrones

Patron = 3 o mas items que tocan el mismo tema. Agrupar:
- Tema (frase corta).
- Grupo dominante (dolor / pedido / etc).
- Count.
- Urgencia agregada (max de los items).
- 1-2 quotes representativos (cortos, con elipsis si recortados).

Ordenar patrones por: urgencia alta primero, luego count desc.

## Fase 4 — Items destacados unicos

Items que son **unicos** pero tienen urgencia alta — no son patron (count=1) pero merecen flag individual (ej. baja de cuenta empresa grande, reporte de seguridad, tema regulatorio). Listar separado.

## Fase 5 — Sugerir proxima accion por patron

Para cada patron en top 10, asignar accion sugerida:
- **investigar**: vale la pena entender mejor antes de actuar (entrevistas, instrumentacion). Default si: count alto pero info ambigua.
- **construir**: claro, dimensionado, vale la pena meter en roadmap. Default si: count alto + urgencia alta + solucion obvia.
- **monitorear**: dejar en seguimiento. Default si: count bajo pero recurrente.
- **ignorar**: no accionable o fuera de prioridades. Default si: count alto pero conflictivo con vision/segmento.

## Fase 6 — Estructura del contenido

```markdown
## Resumen

- **Fuente**: <FUENTE>
- **Ventana**: <VENTANA>
- **Items procesados**: <N>
- **Patrones detectados**: <P>
- **Items destacados unicos**: <U>

## Distribucion por grupo

| Grupo | Count | % |
|--------|-------|---|
| dolor | <n> | <%> |
| pedido | <n> | <%> |
| bug | <n> | <%> |
| elogio | <n> | <%> |
| ruido | <n> | <%> |
| fuera-de-alcance | <n> | <%> |

## Patrones (top 10)

### 1. <tema> — count <N>, urgencia <menor/importante/urgente>, grupo <grupo>

> "<quote 1 representativo>"
> "<quote 2 representativo>"

**Accion sugerida**: <investigar / construir / monitorear / ignorar> — <razon en 1 linea>

### 2. <tema> — ...

(repetir)

## Items destacados unicos

- [<urgencia>] <grupo> — <1 linea> — <accion sugerida>
- [<urgencia>] <grupo> — ...

## Ruido (no accionable)

Count: <N>. <1-2 ejemplos breves si suma>

## Notas

<observaciones del orquestador: sesgos detectados, fuente subrepresentada, etc>

---

_Triage generado por `/pm-feedback`._
```

## Fase 7 — Confirmar y guardar

Default guardado: **si** (los triages se vuelven a consultar).

Slug: kebab-case del titulo, max 40 chars. Path: `.pm/pm-feedback/<slug>.md`.

```
Confirmás que guardo el triage en `.pm/pm-feedback/<slug>.md`? (si/no, default: si)
```

Si si: si la carpeta `.pm/pm-feedback/` no existe, crearla con `mkdir -p .pm/pm-feedback/` antes de escribir. Luego usar el `Write` tool (NUNCA via echo/heredoc) para crear el archivo. Titulo formato: "Feedback triage <VENTANA>" o "Feedback triage <FUENTE> <fecha>".

## Fase 8 — Reportar

```
## Result
- skill: /pm-feedback
- saved: true
- file: .pm/pm-feedback/<slug>.md
- title: <titulo>
- items_processed: <N>
- patterns_detected: <P>
- top_action: <construir/investigar/monitorear/ignorar — del patron #1>
```

Y debajo: `Triage guardado: .pm/pm-feedback/<slug>.md`.

## MUST DO

- Parsear items del dump antes de clasificar.
- Clasificar cada item en exactamente 1 grupo.
- Patrones requieren count >= 3.
- Sugerir accion (investigar/construir/monitorear/ignorar) por patron.
- Marcar items unicos de alta urgencia aparte.
- Guardar en `.pm/pm-feedback/<slug>.md` con `Write` tool.

## MUST NOT DO

- No inventar quotes — usar solo los que estan en el dump.
- No procesar items con info sensible o datos personales sin avisar al usuario (si detectas emails, nombres, IDs personales, preguntar si los anonimizamos antes de guardar).
- No promover patrones con count < 3 al ranking principal.
- No usar `gh` ni depender de GitHub.
- No persistir nada en auto-memory.

**Decision log** estilo ADR (Architecture Decision Record) pero para producto. Registra decisiones tomadas con su contexto, alternativas, criterio, contrapartidas y reversibilidad. `$ARGUMENTS` es la decision (puede venir vacio).

ADR (Architecture Decision Record) original es un patron de ingenieria; aca lo adaptamos a decisiones de producto.

Skill **interactivo corto** — pensado para registrar una decision ya tomada, no para evaluarla (para eso usar `/pm-rfc`).

## Pre-flight

### 1. Validar input

- Vacio: pedir `Que decision se tomo? (1-2 lineas)` y esperar.

## Fase 1 — Recolectar campos via preguntas

No hace falta clarificacion de supuestos: esto no es ambiguo, son datos directos. Preguntar uno por uno, mostrando progreso:

### 1/8 — Decision
Si vino en `$ARGUMENTS`, mostrar y preguntar si esta bien sintetizada o se ajusta. Sino, pedirla.
Formato sugerido: imperativo o nominal. Ej. "Cobrar por usuario en vez de por uso", "Pausar el lanzamiento de feature X hasta Q3".

### 2/8 — Fecha
```
Cuando se tomo? (default: hoy <YYYY-MM-DD>)
```
Default: `$(date -u '+%Y-%m-%d')`.

### 3/8 — Quien decidio
```
Quien tomo la decision? (rol o nombre — ej. "Product team", "CEO", "Maria Lopez")
```

### 4/8 — Contexto
```
Contexto en 2-3 lineas: que problema o situacion gatillo la decision?
```

### 5/8 — Alternativas consideradas
```
Que alternativas se consideraron? (lista — incluir la elegida + las descartadas. Minimo 1 alternativa descartada para que el log tenga valor.)
```

Si solo hay 1 alternativa (la elegida), preguntar:
```
No hay alternativas descartadas. Querés:
1) Pensarlas ahora (te ayudo a generar 2-3 contrafactuales)
2) Registrar igual sin alternativas (el log queda mas debil)
```

### 6/8 — Criterio de decision
```
Que criterio se uso para elegir entre las alternativas? (impacto / costo / reversibilidad / riesgo / timing / otro)
```

### 7/8 — Contrapartidas aceptadas
```
Que contrapartidas aceptamos al elegir esto? (que perdemos o ponemos en riesgo)
```

### 8/8 — Reversibilidad y revision
```
Reversibilidad:
1) Alta — podemos volver atras en dias/semanas sin costo significativo
2) Media — costoso pero posible (1-3 meses, refactor moderado)
3) Baja — puerta de una via, costoso revertir
4) Otra
```

Y:
```
Cuando revisitar esta decision? (opcional — default: no programada)
1) En <N> dias / semanas / meses
2) Cuando suceda <evento>
3) No programar revision
```

## Fase 2 — Estructura del contenido

```markdown
## Decision

<decision en 1-2 lineas>

## Metadata

- **Fecha**: <YYYY-MM-DD>
- **Decisor**: <quien>
- **Reversibilidad**: <alta / media / baja> — <razon corta>
- **Revisar**: <fecha o evento o "no programada">

## Contexto

<2-3 lineas: que situacion gatillo la decision>

## Alternativas consideradas

1. **<alternativa elegida>** ← elegida
   - Por que: <1 linea>
2. **<alternativa descartada 1>**
   - Por que NO: <1 linea>
3. **<alternativa descartada 2>** (si hay)
   - Por que NO: <1 linea>

## Criterio

<que criterio se uso para elegir>

## Contrapartidas aceptadas

- <contrapartida 1>
- <contrapartida 2>

## Que evidencia cambiaria esta decision

- <bullet 1>
- <bullet 2>

---

_Decision log registrado con `/pm-decision`._
```

## Fase 3 — Confirmar y guardar

Default guardado: **si** (los decision logs ganan valor con la trazabilidad).

Slug: kebab-case del titulo, max 40 chars. Path: `.pm/pm-decision/<slug>.md`.

```
Confirmás que guardo el decision log en `.pm/pm-decision/<slug>.md`? (si/no, default: si)
```

Si no: mostrar el contenido inline para que el usuario lo guarde donde quiera.

Si si: si la carpeta `.pm/pm-decision/` no existe, crearla con `mkdir -p .pm/pm-decision/` antes de escribir. Luego usar el `Write` tool (NUNCA via echo/heredoc) para crear el archivo. Titulo formato: "Decision: <que se decidio>" — max 70 chars.

## Fase 4 — Reportar

### Si saved=true
```
## Result
- skill: /pm-decision
- saved: true
- file: .pm/pm-decision/<slug>.md
- title: <titulo>
- fecha: <YYYY-MM-DD>
- reversibilidad: <alta/media/baja>
- alternatives_count: <N>
- revision_scheduled: <fecha/evento/no>
```
Y `Decision log guardado: .pm/pm-decision/<slug>.md`.

### Si saved=false
```
## Result
- skill: /pm-decision
- saved: false
- fecha: <YYYY-MM-DD>
- alternatives_count: <N>

---

## Decision log

<contenido inline>
```

## MUST DO

- Recolectar los 8 campos en orden con barra de progreso.
- Validar minimo 1 alternativa descartada (o forzar al usuario a generarlas).
- Guardar en `.pm/pm-decision/<slug>.md` con `Write` tool si el usuario confirma.

## MUST NOT DO

- No mezclar `/pm-decision` con `/pm-rfc` — RFC es propuesta pre-decision, decision log es registro post-decision.
- No usar `/pm-decision` para evaluar alternativas — eso es trabajo del RFC.
- No usar `gh` ni depender de GitHub.
- No persistir nada en auto-memory.

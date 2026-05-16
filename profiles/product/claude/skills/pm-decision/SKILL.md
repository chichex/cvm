**Decision log** estilo ADR (Architecture Decision Record) pero para producto. Registra decisiones tomadas con su contexto, alternativas, criterio, trade-offs y reversibilidad. `$ARGUMENTS` es la decision (puede venir vacio).

Skill **interactivo corto** — pensado para registrar una decision ya tomada, no para evaluarla (para eso usar `/pm-rfc`).

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar como en `/pm-prd`.

### 2. Validar input
- Vacio: pedir `Que decision se tomo? (1-2 lineas)` y esperar.

## Fase 1 — Recolectar campos via preguntas

Sin `/clarify` (esto no es ambiguo — son datos). Preguntar uno por uno, mostrando progreso:

### 1/8 — Decision
Si vino en `$ARGUMENTS`, mostrar y preguntar si esta bien sintetizada o se ajusta. Sino, pedirla.
Formato sugerido: imperativo o nominal. Ej. "Cobrar por seat en vez de por uso", "Pausar el lanzamiento de feature X hasta Q3".

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

### 7/8 — Trade-offs aceptados
```
Que trade-offs aceptamos al elegir esto? (que perdemos o ponemos en riesgo)
```

### 8/8 — Reversibilidad y revision
```
Reversibilidad:
1) Alta — podemos volver atras en dias/semanas sin costo significativo
2) Media — costoso pero posible (1-3 meses, refactor moderado)
3) Baja — one-way door, costoso revertir
4) Otra
```

Y:
```
Cuando revisitar esta decision? (opcional — default: no programada)
1) En <N> dias / semanas / meses
2) Cuando suceda <evento>
3) No programar revision
```

## Fase 2 — Estructura del body

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

## Trade-offs aceptados

- <trade-off 1>
- <trade-off 2>

## Que evidencia cambiaria esta decision

- <bullet 1>
- <bullet 2>

---

_Decision log registrado con `/pm-decision`._
```

## Fase 3 — Confirmar y persistir

Default persistencia: **no** (decisiones operativas chicas no necesariamente quieren un issue — pero la mayor parte del valor del log es la trazabilidad, asi que el usuario decide caso a caso).

```
Querés crear el issue con label `pm:decision`? (si/no, default: no)
```

Si no: mostrar el body inline para que el usuario lo guarde donde quiera (notion, docs, etc).

Si si:
```bash
gh label create "pm:decision" --color "BFD4F2" --description "Decision log" 2>/dev/null || true

BODY_FILE="$(mktemp -t pm-decision-body.XXXXXX).md"
gh issue create --title "<titulo>" --body-file "$BODY_FILE" --label "pm:decision"
```

Titulo formato: "Decision: <que se decidio>" — max 70 chars.

## Fase 4 — Reportar

### Si persisted=true
```
## Result
- skill: /pm-decision
- persisted: true
- url: <URL>
- title: <titulo>
- fecha: <YYYY-MM-DD>
- reversibilidad: <alta/media/baja>
- alternatives_count: <N>
- revision_scheduled: <fecha/evento/no>
```
Y `Issue creado: <URL>`.

### Si persisted=false
```
## Result
- skill: /pm-decision
- persisted: false
- fecha: <YYYY-MM-DD>
- alternatives_count: <N>

---

## Decision log

<body inline>
```

## MUST DO

- Recolectar los 8 campos en orden con barra de progreso.
- Validar minimo 1 alternativa descartada (o forzar al usuario a generarlas).
- Default de persistencia: NO (decisiones chicas no siempre van a GitHub).

## MUST NOT DO

- No mezclar `/pm-decision` con `/pm-rfc` — RFC es propuesta pre-decision, decision log es registro post-decision.
- No usar `/pm-decision` para evaluar alternativas — eso es trabajo del RFC.
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

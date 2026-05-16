Diseñar un **experimento** (A/B, holdout, before/after, multivariante) a partir de una hipotesis. Incluye baseline, metricas, sample size estimado, stop conditions y guardrails. `$ARGUMENTS` es la hipotesis o idea a testear (puede venir vacio).

Skill **interactivo**.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar como en `/pm-prd`.

### 2. Validar input
- Vacio: pedir `Cual es la hipotesis a testear? Formato sugerido: "Creemos que [cambio] va a [efecto] porque [razon], lo vamos a medir con [metrica]".` y esperar.

### 3. Preguntar tipo de experimento
```
Tipo de experimento:
1) A/B test (split traffic, random assignment)
2) Holdout (gran mayoria recibe el cambio, % chico queda como control)
3) Before/after (timeline, sin control concurrente)
4) Multivariante / multi-arm
5) Otra
```
Guardar `TIPO`. Cada uno cambia los requisitos:
- A/B: requiere sample size por brazo, random assignment, duracion minima.
- Holdout: requiere tamaño del holdout, criterio de salida.
- Before/after: requiere baseline historico, control de factores estacionales.
- Multivariante: requiere combinaciones definidas y plan de analisis.

## Fase 1 — Clarify la hipotesis

Cargar `/clarify` (`../clarify/SKILL.md`) en `MODO=prompt`. **Restricciones**:

- Forzar que la hipotesis sea **falsable**: tiene que haber un resultado posible que la refute.
- Asunciones a refinar:
  - Causa propuesta (que cambia) — concreta, no "mejorar la UX".
  - Efecto esperado (que metrica se mueve, en que direccion, magnitud).
  - Baseline actual de esa metrica (numero, ventana temporal, segmento).
  - Audiencia del experimento (quien entra al test, quien queda afuera).
  - Razon de la hipotesis (por que esperamos ese efecto).

Si la baseline es "no la tenemos medida", marcarlo como blocker — el experimento no se puede correr sin baseline. Preguntar:
```
La metrica no tiene baseline medida. Querés:
1) Pausar y agregar instrumentacion primero (recomendado)
2) Estimar baseline desde proxy (riesgo: false positives)
3) Definir baseline al primer dia del experimento (riesgo: contaminacion)
```

## Fase 2 — Disenio del experimento

Generar las siguientes secciones, preguntando al usuario los gaps:

### Metrica primaria
- Nombre + definicion operativa (como se calcula).
- Baseline + ventana temporal (ej. "12% conversion last 30 days").
- MDE (minimum detectable effect): que cambio chico vale la pena detectar. Default sugerido: 5% relativo.
- Target esperado: que efecto esperamos ver.

### Metricas secundarias
- 2-4 metricas que esperamos mover junto a la primaria.

### Guardrails (lo que NO puede empeorar)
- Lista de metricas que tienen que mantenerse >= baseline. Si caen, abortamos.

### Diseño
Segun `TIPO`:
- A/B: split %, segmentacion, criterio de eleccion del control.
- Holdout: tamaño del holdout (%), criterio de salida.
- Before/after: ventana before, ventana after, controles estacionales.

### Sample size
Estimacion rapida usando formulas estandar:
- Para conversion (proporcion): `n ≈ 16 * p * (1-p) / MDE^2` por brazo (alpha=0.05, power=0.8).
- Para metrica continua: `n ≈ 16 * var / MDE^2` por brazo.
- Marcar como **estimacion**, recomendar validar con calculadora especializada antes de lanzar.

### Duracion
- Si `sample_size / traffic_per_day` < 7 dias, recomendar minimo 1 semana (para capturar ciclos semanales).
- Si > 4 semanas, alertar (probabilidad alta de novelty effects, cambios de contexto).

### Stop conditions
- **Stop por exito temprano**: criterio para parar antes (peligroso — peeking aumenta false positives). Default: no parar antes de sample size completo.
- **Stop por daño**: si alguna guardrail metric cae >X% con significancia, parar.
- **Stop por timeout**: si pasa Y dias sin alcanzar sample size, decidir extender o cortar.

### Riesgos
- Sesgo de seleccion (random assignment funciona?).
- Contaminacion entre grupos.
- Novelty effect (efecto que se diluye).
- Volumen insuficiente.

## Fase 3 — Estructura del body

```markdown
## Hipotesis

Creemos que **<causa>** va a **<efecto>** porque **<razon>**, lo vamos a medir con **<metrica primaria>**.

## Tipo de experimento

<TIPO>

## Metrica primaria

- **Nombre**: <metrica>
- **Definicion**: <calculo>
- **Baseline**: <numero> (<ventana>)
- **MDE**: <%>
- **Target esperado**: <%>

## Metricas secundarias

- <metrica 1>
- <metrica 2>

## Guardrails

- <metrica 1>: no puede caer por debajo de <umbral>
- <metrica 2>: ...

## Diseño

<segun TIPO>

## Sample size estimado

- **Por brazo**: ~<N> usuarios/eventos
- **Asunciones**: alpha=0.05, power=0.8, baseline=<X>, MDE=<Y>
- _Validar con calculadora especializada antes de lanzar._

## Duracion estimada

~<N> dias (basado en <traffic/day>)

## Stop conditions

- **Exito temprano**: <criterio o "no parar antes de sample size completo">
- **Daño**: <criterio>
- **Timeout**: <criterio>

## Riesgos

- <riesgo 1 + mitigacion>
- <riesgo 2 + mitigacion>

## Analisis planeado

<que vamos a calcular post-experimento, que stat test, como reportamos>

---

_Experimento diseñado con `/pm-experiment`._
```

## Fase 4 — Review opcional

```
Querés que `pm-reviewer` audite el diseño? (si/no, default: si)
```

Si si: invocar con `artefact_type: experiment`, `artefact_text: <body>`. El reviewer va a buscar hipotesis no falsables, falta de baseline, ausencia de guardrails.

## Fase 5 — Persistir

```bash
gh label create "pm:experiment" --color "5319E7" --description "Experiment design" 2>/dev/null || true

BODY_FILE="$(mktemp -t pm-experiment-body.XXXXXX).md"
gh issue create --title "<titulo>" --body-file "$BODY_FILE" --label "pm:experiment"
```

Titulo formato: "Exp: <causa> → <metrica>" (ej. "Exp: nuevo onboarding → activacion D7").

## Fase 6 — Reportar

```
## Result
- skill: /pm-experiment
- persisted: true
- url: <URL>
- title: <titulo>
- tipo: <TIPO>
- sample_size: <N por brazo>
- duracion: ~<N dias>
- reviewer_used: <true | false>
```

## MUST DO

- Forzar hipotesis falsable.
- Pedir baseline o flagear como blocker si no existe.
- Calcular sample size estimado.
- Incluir stop conditions explicitas.
- Listar guardrails (metricas que no pueden empeorar).

## MUST NOT DO

- No diseñar experimentos sin baseline (excepto explicitamente con before/after y baseline historico).
- No omitir guardrails (siempre hay metricas que pueden empeorar).
- No prometer significancia sin sample size estimado.
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

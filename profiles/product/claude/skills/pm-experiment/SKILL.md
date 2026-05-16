Diseñar un **experimento** (A/B, grupo de control, antes/despues, multivariable) a partir de una hipotesis. Incluye baseline, metricas, tamaño de muestra estimado, cuando cortar y metricas que no pueden empeorar. `$ARGUMENTS` es la hipotesis o idea a testear (puede venir vacio).

Skill **interactivo**.

## Pre-flight

### 1. Validar input

- Vacio: pedir `Cual es la hipotesis a testear? Formato sugerido: "Creemos que [cambio] va a [efecto] porque [razon], lo vamos a medir con [metrica]".` y esperar.

### 2. Preguntar tipo de experimento

```
Tipo de experimento:
1) A/B test (split de trafico, asignacion aleatoria)
2) Grupo de control (gran mayoria recibe el cambio, % chico queda como control)
3) Antes/despues (timeline, sin control concurrente)
4) Multivariable / multi-arm
5) Otra
```
Guardar `TIPO`. Cada uno cambia los requisitos:
- A/B: requiere tamaño de muestra por brazo, asignacion aleatoria, duracion minima.
- Grupo de control: requiere tamaño del grupo, criterio de salida.
- Antes/despues: requiere baseline historico, control de factores estacionales.
- Multivariable: requiere combinaciones definidas y plan de analisis.

## Fase 1 — Clarificar la hipotesis

Aplicar clarificacion inline, forzando que la hipotesis sea **comprobable** (tiene que haber un resultado posible que la refute).

1. Listar 4-6 supuestos sobre la hipotesis, taggeados `[directo]`, `[medio]`, `[especulativo]`. Filtrar a supuestos de producto/negocio, NO tecnicos. Cubrir:
   - Causa propuesta (que cambia) — concreta, no "mejorar la UX".
   - Efecto esperado (que metrica se mueve, en que direccion, magnitud).
   - Baseline actual de esa metrica (numero, ventana temporal, segmento).
   - A quien apunta el experimento (quien entra al test, quien queda afuera).
   - Razon de la hipotesis (por que esperamos ese efecto).
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

Si la baseline es "no la tenemos medida", marcarlo como urgente — el experimento no se puede correr sin baseline. Preguntar:
```
La metrica no tiene baseline medida. Querés:
1) Pausar y agregar instrumentacion primero (recomendado)
2) Estimar baseline desde proxy (riesgo: falsos positivos)
3) Definir baseline al primer dia del experimento (riesgo: contaminacion)
```

## Fase 2 — Diseño del experimento

Generar las siguientes secciones, preguntando al usuario los faltantes:

### Metrica primaria
- Nombre + definicion operativa (como se calcula).
- Baseline + ventana temporal (ej. "12% conversion ultimos 30 dias").
- MDE (minimum detectable effect, mínimo efecto detectable): que cambio chico vale la pena detectar. Default sugerido: 5% relativo.
- Target esperado: que efecto esperamos ver.

### Metricas secundarias
- 2-4 metricas que esperamos mover junto a la primaria.

### Metricas que NO pueden empeorar
- Lista de metricas que tienen que mantenerse >= baseline. Si caen, abortamos.

### Diseño
Segun `TIPO`:
- A/B: % de split, segmentacion, criterio de eleccion del control.
- Grupo de control: tamaño del grupo (%), criterio de salida.
- Antes/despues: ventana antes, ventana despues, controles estacionales.

### Tamaño de muestra
Estimacion rapida usando formulas estandar:
- Para conversion (proporcion): `n ≈ 16 * p * (1-p) / MDE^2` por brazo (alpha=0.05, power=0.8).
- Para metrica continua: `n ≈ 16 * var / MDE^2` por brazo.
- Marcar como **estimacion**, recomendar validar con calculadora especializada antes de lanzar.

### Duracion
- Si `muestra / trafico_por_dia` < 7 dias, recomendar minimo 1 semana (para capturar ciclos semanales).
- Si > 4 semanas, alertar (probabilidad alta de efecto novedad, cambios de contexto).

### Cuando cortar
- **Cortar por exito temprano**: criterio para parar antes (peligroso — revisar antes de tiempo aumenta falsos positivos). Default: no parar antes de completar la muestra.
- **Cortar por daño**: si alguna metrica limite cae >X% con significancia, parar.
- **Cortar por timeout**: si pasa Y dias sin alcanzar la muestra, decidir extender o cortar.

### Riesgos
- Sesgo de seleccion (la asignacion aleatoria funciona?).
- Contaminacion entre grupos.
- Efecto novedad (efecto que se diluye).
- Volumen insuficiente.

## Fase 3 — Estructura del contenido

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

## Metricas que no pueden empeorar

- <metrica 1>: no puede caer por debajo de <umbral>
- <metrica 2>: ...

## Diseño

<segun TIPO>

## Tamaño de muestra estimado

- **Por brazo**: ~<N> usuarios/eventos
- **Supuestos**: alpha=0.05, power=0.8, baseline=<X>, MDE=<Y>
- _Validar con calculadora especializada antes de lanzar._

## Duracion estimada

~<N> dias (basado en <trafico/dia>)

## Cuando cortar

- **Exito temprano**: <criterio o "no parar antes de completar la muestra">
- **Daño**: <criterio>
- **Timeout**: <criterio>

## Riesgos

- <riesgo 1 + mitigacion>
- <riesgo 2 + mitigacion>

## Analisis planeado

<que vamos a calcular post-experimento, que test estadistico, como reportamos>

---

_Experimento diseñado con `/pm-experiment`._
```

## Fase 4 — Revision opcional

```
Querés que `pm-reviewer` audite el diseño? (si/no, default: si)
```

Si si: invocar con `artefact_type: experiment`, `artefact_text: <contenido>`. El reviewer va a buscar hipotesis no comprobables, falta de baseline, ausencia de metricas limite.

## Fase 5 — Confirmar y guardar

Slug: kebab-case del titulo, max 40 chars. Path: `.pm/pm-experiment/<slug>.md`.

```
Confirmás que guardo el experimento en `.pm/pm-experiment/<slug>.md`? (si/no, default: si)
```

Si si: si la carpeta `.pm/pm-experiment/` no existe, crearla con `mkdir -p .pm/pm-experiment/` antes de escribir. Luego usar el `Write` tool (NUNCA via echo/heredoc) para crear el archivo. Titulo formato: "Exp: <causa> → <metrica>" (ej. "Exp: nuevo onboarding → activacion D7").

## Fase 6 — Reportar

```
## Result
- skill: /pm-experiment
- saved: true
- file: .pm/pm-experiment/<slug>.md
- title: <titulo>
- tipo: <TIPO>
- sample_size: <N por brazo>
- duracion: ~<N dias>
- reviewer_used: <true | false>
```

Y debajo: `Experimento guardado: .pm/pm-experiment/<slug>.md`.

## MUST DO

- Forzar hipotesis comprobable.
- Pedir baseline o marcar como urgente si no existe.
- Calcular tamaño de muestra estimado.
- Incluir condiciones de corte explicitas.
- Listar metricas que no pueden empeorar.
- Guardar en `.pm/pm-experiment/<slug>.md` con `Write` tool.

## MUST NOT DO

- No diseñar experimentos sin baseline (excepto explicitamente con antes/despues y baseline historico).
- No omitir metricas limite (siempre hay metricas que pueden empeorar).
- No prometer significancia sin tamaño de muestra estimado.
- No usar `gh` ni depender de GitHub.
- No persistir nada en auto-memory.

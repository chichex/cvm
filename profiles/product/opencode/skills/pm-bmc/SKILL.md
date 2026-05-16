---
name: pm-bmc
description: Itera los 9 bloques del Business Model Canvas uno por uno con preguntas guia, sintetiza en 2-3 bullets por bloque, valida coherencia cruzada y guarda en .pm/pm-bmc/<slug>.md.
---

**Business Model Canvas**: itera los 9 bloques uno por uno con preguntas guia, sintetiza en 2-3 bullets por bloque. Los argumentos del skill son una descripcion del producto/negocio y pueden venir vacios.

Skill **interactivo largo** — el ejercicio mas extenso del profile, 9 micro-conversaciones.

## Pre-flight

1. Si los argumentos estan vacios, pedir: `Describime el producto/negocio en 2-3 parrafos. Si ya tenés algo de BMC armado en otro lado, pegalo y arrancamos desde ahi.`

## Fase 1 - Contexto

Preguntar con multiple choice:

```text
Etapa del negocio (afecta el nivel de concrecion esperado):
1. Idea / sin ingresos (default)
2. MVP / primeros ingresos
3. En crecimiento (ingresos creciendo, equipo > 10)
4. Maduro
5. Otra
```

Guardar `ETAPA`. En `idea` se acepta vaguedad; en `maduro` se exige numeros.

## Fase 2 - Pasar Por Los 9 Bloques

Iterar bloque por bloque. Para cada uno:

1. Mostrar nombre del bloque + 2-3 preguntas guia.
2. Esperar respuesta del usuario.
3. Sintetizar en 2-3 bullets concretos.
4. Validar con el usuario (`asi te parece? si/ajustar`).
5. Si "ajustar", iterar; si "si", pasar al siguiente.

Mostrar barra de progreso `[bloque k/9] ▰▰▰▱▱▱▱▱▱` por bloque.

### Bloque 1 - Segmentos de clientes

Preguntas guia:
- Para quien creas valor? (segmento concreto, no "todos los que necesiten X")
- Es un segmento, multiples segmentos, o mercado masivo?
- Cual es el segmento mas importante?

Restriccion: minimo 1 segmento concreto. "Todos" no es aceptable.

### Bloque 2 - Propuesta de valor

Preguntas guia:
- Que problema resolves para ese segmento?
- Que es lo que entregas (producto / servicio / experiencia)?
- En que sos diferente / mejor que las alternativas existentes?

Restriccion: por segmento, una propuesta de valor diferenciada.

### Bloque 3 - Canales

Preguntas guia:
- Como llegas al segmento? (conocimiento, evaluacion, compra, entrega, post-venta)
- Cuales canales son mas efectivos? Mas eficientes en costo?
- Estan integrados con la propuesta de valor?

### Bloque 4 - Relacion con clientes

Preguntas guia:
- Que tipo de relacion espera cada segmento? (auto-servicio, asistencia, comunidad, automatizada)
- Como las conseguis, las mantenes, las creces?
- El soporte / atencion al cliente es parte del producto?

### Bloque 5 - Fuentes de ingreso

Preguntas guia:
- Por que valor estan pagando los clientes?
- Modelo: pago unico, suscripcion, transaccional, freemium, licencia?
- Precio actual o estimado por segmento?

En `ETAPA=maduro` o `en crecimiento`, exigir numeros (precio, % de mix, contribucion).

### Bloque 6 - Recursos clave

Preguntas guia:
- Que recursos necesita la propuesta de valor? (fisicos, propiedad intelectual, humanos, financieros)
- Cuales son escasos / caros / dificil de replicar?
- Cuales son commodity?

### Bloque 7 - Actividades clave

Preguntas guia:
- Que actividades hace el negocio operativamente?
- Cuales son core (no se tercerizan)?
- Cuales podrian tercerizarse / automatizarse?

### Bloque 8 - Socios clave

Preguntas guia:
- Quienes son tus socios o proveedores clave?
- Que motivacion tienen para ser tus socios? (optimizacion, reducir riesgo, conseguir recursos)
- Hay dependencias riesgosas (proveedor unico, plataforma de terceros)?

### Bloque 9 - Estructura de costos

Preguntas guia:
- Cuales son los costos mas importantes? (fijos vs variables)
- Optimizan por costo o por valor? (low-cost vs premium)
- Hay economias de escala / alcance?

En `ETAPA=maduro` o `en crecimiento`, exigir numeros gruesos (% del total).

## Fase 3 - Sanity Check De Coherencia

Despues de los 9 bloques, hacer un cross-check automatico:

- **Segmento ↔ Canal**: el canal del bloque 3 alcanza al segmento del bloque 1?
- **Segmento ↔ Propuesta**: la propuesta del bloque 2 resuelve un problema real de ese segmento?
- **Ingresos ↔ Costos**: el modelo del bloque 5 cubre la estructura del bloque 9? Margen positivo?
- **Recursos ↔ Actividades**: los recursos del 6 alcanzan para las actividades del 7?

Si detectas inconsistencia, mostrar al usuario:

```
Detecte una posible inconsistencia entre bloque <X> y bloque <Y>:
- Bloque <X>: <bullet>
- Bloque <Y>: <bullet conflictivo>

Querés (1) ajustar bloque X, (2) ajustar bloque Y, (3) dejarlo asi y notar como riesgo?
```

## Fase 4 - Revision Opcional

Preguntar: `Querés que pm-reviewer audite el canvas? (si/no, default: no — el sanity check de fase 3 cubre lo mas obvio)`. Si acepta, usar Task con `subagent_type: pm-reviewer`, `artefact_type: bmc`, `artefact_text: <contenido>`, `context: etapa=<ETAPA>`.

## Fase 5 - Contenido Del BMC

```markdown
## Business Model Canvas

**Etapa**: <ETAPA>

### 1. Segmentos de clientes
- <bullet 1>
- <bullet 2>

### 2. Propuesta de valor
- <bullet>

### 3. Canales
- <bullet>

### 4. Relacion con clientes
- <bullet>

### 5. Fuentes de ingreso
- <bullet>

### 6. Recursos clave
- <bullet>

### 7. Actividades clave
- <bullet>

### 8. Socios clave
- <bullet>

### 9. Estructura de costos
- <bullet>

## Inconsistencias detectadas

<lista de la fase 3, o "ninguna">

## Riesgos del modelo

<riesgos no explicitos en bloques — ej. proveedor unico, regulacion futura, dependencia de plataforma>

---
_BMC generado por `/pm-bmc`._
```

## Fase 6 - Guardar

Titulo: `BMC: <producto> <fecha>`. Slug: kebab-case del titulo, maximo 40 chars. Path: `.pm/pm-bmc/<slug>.md`.

Preguntar: `Confirmás que guardo el BMC en .pm/pm-bmc/<slug>.md? (si/no, default: si)`. Si acepta, crear el directorio con `mkdir -p .pm/pm-bmc/` si no existe, y luego escribir el archivo con el tool de edicion seguro disponible (no `echo` ni heredoc).

## Result

Reportar:

```yaml
skill: /pm-bmc
saved: true
file: .pm/pm-bmc/<slug>.md
title: <titulo>
etapa: <ETAPA>
inconsistencies_flagged: <N>
reviewer_used: <true | false>
```

## MUST DO

- Pasar por los 9 bloques en orden, con preguntas guia.
- Sintetizar 2-3 bullets por bloque (no parrafos).
- Validar coherencia entre bloques en fase 3.
- En `ETAPA=maduro/en crecimiento`, exigir numeros en ingresos y costos.
- Guardar en `.pm/pm-bmc/<slug>.md` con confirmacion.

## MUST NOT DO

- No saltarse bloques (es el principal valor del BMC: cubrir los 9).
- No aceptar "todos los clientes" como segmento.
- No aceptar bullets genericos copy-paste (ej. "diferenciacion por calidad", "atencion personalizada").
- No usar `gh` ni depender de GitHub.
- No persistir nada en auto-memory.

---
name: pm-compete
description: Genera analisis competitivo con matriz de features, precios, posicionamiento, reviews y vacios; puede usar pm-researcher y guarda en .pm/pm-compete/<slug>.md.
---

Crear un **analisis competitivo** desde los argumentos del skill: categoria, producto, competidores conocidos o foco.

## Pre-flight

- Si no hay argumentos, pedir: `Que categoria o competidores queres analizar? Lista competidores conocidos y foco.`

## Fase 1 - Foco E Investigacion

Preguntar:

```text
Foco del analisis:
1. Features
2. Precios
3. Posicionamiento
4. Reviews
5. General
6. Otra
```

```text
Investigacion externa:
1. Si, investigacion completa con pm-researcher
2. No, trabajo solo con la data que me pases
3. Mixto, vos pasas data y pm-researcher completa los faltantes
```

Si usa investigacion, invocar Task con `subagent_type: pm-researcher`, `description: pm-compete research` y prompt con `topic`, `competitors_known`, `focus`, `max_competitors: 5`. Si es mixto, pedir data primero y mandar al researcher solo los faltantes.

## Fase 2 - Matriz

Generar secciones aplicables al foco:

- Matriz de features con nosotros y competidores.
- Basico: features en mas del 50% de competidores.
- Diferenciadores: features unicas o casi unicas.
- Vacios: necesidades no cubiertas.
- Precios: modelo, tiers, tier gratis, notas.
- Posicionamiento: headline, audiencia, tono.
- Señal de reviews: elogios, quejas, patrones.

No inventar datos. Si un dato no esta en fuentes o input del usuario, marcar `desconocido`.

## Fase 3 - Contenido

```markdown
## Resumen
- Categoria: <X>
- Foco: <FOCO>
- Competidores analizados: <lista>
- Fuente de data: <pm-researcher | usuario | mixta>

## Matriz de features
<tabla>

### Basico
- <lista>

### Diferenciadores actuales
- <feature -> competidor>

### Vacios
- <feature potencial>

## Precios
<tabla>

## Posicionamiento
<por competidor>

## Señal de reviews
<si aplica>

## Insights
### Donde ganamos
- <bullet>
### Donde perdemos
- <bullet>
### Vacios de mercado
- <bullet>
### Riesgos competitivos
- <competidor podria accion e impacto>

## Fuentes
<URLs si aplica>

---
_Analisis competitivo generado por `/pm-compete`._
```

## Fase 4 - Revision Y Guardado

Preguntar si `pm-reviewer` audita (default: no), con `artefact_type: compete`.

Slug: kebab-case del titulo, max 40 chars. Path: `.pm/pm-compete/<slug>.md`.

Preguntar: `Confirmás que guardo el analisis en .pm/pm-compete/<slug>.md? (si/no, default: si)`. Si acepta, si la carpeta `.pm/pm-compete/` no existe, crearla con `mkdir -p .pm/pm-compete/` antes de escribir. Luego crear el archivo con el tool de edicion seguro disponible (no `echo` ni heredoc). Titulo: `Compete: <categoria> <fecha>` o `Compete vs <competidor> <fecha>`.

## Result

```yaml
skill: /pm-compete
saved: true
file: .pm/pm-compete/<slug>.md
title: <titulo>
foco: <FOCO>
investigacion: <INVESTIGACION>
competitors_count: <N>
vacios_detectados: <N>
reviewer_used: <true | false>
```

## MUST DO

- Preguntar foco e investigacion.
- Citar fuentes del researcher.
- Separar basico, diferenciadores y vacios.
- Guardar en `.pm/pm-compete/<slug>.md` solo con confirmacion.

## MUST NOT DO

- No inventar competidores ni datos.
- No declarar que ganamos en todo sin justificacion.
- No omitir vacios de mercado.
- No usar `gh` ni depender de GitHub.

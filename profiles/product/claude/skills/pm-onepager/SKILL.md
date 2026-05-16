Redactar un **one-pager de feature** (~400 palabras max) para alinear stakeholders **antes** de invertir en un PRD completo. `$ARGUMENTS` es el prompt con la feature (puede venir vacio).

Skill **interactivo**, mas corto que `/pm-prd`. Foco en velocidad.

## Pre-flight

### 1. Validar input

- Vacio: pedir `Describime la feature en 2-3 lineas.` y esperar.
- Si parece un numero o URL de issue: abortar con `/pm-onepager es para one-pagers nuevos. Para refinar algo existente, pegá el material en el prompt.`

## Fase 1 — Clarificacion rapida (max 5 supuestos)

Aplicar clarificacion inline, limitada a maximo 5 supuestos. Priorizar los mas criticos: a quien apunta, alcance, impacto esperado.

1. Listar hasta 5 supuestos sobre la feature, taggeados `[directo]`, `[medio]`, `[especulativo]`. Filtrar a supuestos de producto/negocio, NO tecnicos.
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

## Fase 2 — Estructura compacta

Generar contenido con esta estructura **estricta** — no agregar secciones extra:

```markdown
## Problema

<2 lineas max: que duele, a quien>

## A quien apunta

<1 linea: tipo de cliente / perfil>

## Solucion

<3 lineas max: que hacemos. Resultado para el usuario, no implementacion.>

## Impacto esperado

<1 metrica con baseline (si hay) y target. Si no hay baseline, decir "por medir antes de lanzar".>

## Costo aproximado

<esfuerzo: S/M/L/XL + tiempo aproximado>

## Decision pedida

<1-2 lineas: que necesitas que decidan los stakeholders (avanzar / ajustar alcance / timing / no avanzar).>
```

## Fase 3 — Verificar longitud

Contar palabras del contenido completo. Si > 500 palabras, mostrar:
```
El one-pager quedó en <N> palabras (limite: 500). Querés:
1) Recortar automaticamente (te muestro version corta para revisar)
2) Dejarlo asi y guardar igual
3) Volver a editar
```

Default 1. Si elige 1, recortar respetando la estructura (priorizar Problema, Solucion, Decision pedida — comprimir Impacto y Costo).

## Fase 4 — Confirmar y guardar

Default de guardado para onepager: **si** (es un artefacto final, aunque corto).

Slug: kebab-case del titulo de la feature, max 40 chars. Path: `.pm/pm-onepager/<slug>.md`.

```
Confirmás que guardo el one-pager en `.pm/pm-onepager/<slug>.md`? (si/no, default: si)
```

Si no: mostrar el contenido inline para que el usuario lo copie. Saltar a reporte.

Si si: si la carpeta `.pm/pm-onepager/` no existe, crearla con `mkdir -p .pm/pm-onepager/` antes de escribir. Luego usar el `Write` tool (NUNCA via echo/heredoc) para crear el archivo. Titulo: "One-pager: <feature>" o imperativo corto.

## Fase 5 — Reportar

### Si saved=true
```
## Result
- skill: /pm-onepager
- saved: true
- file: .pm/pm-onepager/<slug>.md
- title: <titulo>
- word_count: <N>
```
Y `One-pager guardado: .pm/pm-onepager/<slug>.md`.

### Si saved=false
```
## Result
- skill: /pm-onepager
- saved: false
- word_count: <N>

---

## One-pager

<contenido inline>
```

## MUST DO

- Limitar la clarificacion a max 5 supuestos.
- Enforzar estructura compacta de 6 secciones.
- Verificar longitud < 500 palabras.
- Default de guardado: si (artefacto final).
- Si el usuario confirma guardar, usar `Write` tool en `.pm/pm-onepager/<slug>.md`.

## MUST NOT DO

- No incluir mas de 5 supuestos en la clarificacion.
- No agregar secciones extra al contenido (sin "Riesgos", sin "Preguntas abiertas" — eso es trabajo del PRD).
- No describir implementacion en "Solucion" — solo resultado para el usuario.
- No usar `gh` ni depender de GitHub.
- No persistir nada en auto-memory.

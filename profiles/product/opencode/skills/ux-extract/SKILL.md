---
name: ux-extract
description: Extrae tokens y componentes reusables desde HTML, URL o directorio para alimentar ux-design-system/ux-components y puede crear issue ux:extract.
---

**Extractor**: dado un HTML mockup (de `/ux-propose` o externo), extrae tokens y propone componentes reusables para alimentar `/ux-design-system` y `/ux-components`. Cierra el loop entre exploracion (mockup) y sistema. Los argumentos del skill son path al HTML, URL, o directorio.

Skill **interactivo**.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar.

### 2. Detectar target

Casos:
- Path a archivo `.html`: extraer de ese archivo.
- Path a directorio (ej. `.ux/proposals/<slug>/`): extraer de todos los `.html` adentro y mergear.
- URL: WebFetch del HTML (sin screenshot — necesitamos el codigo).
- Vacio: pedir `Pasame path a HTML mockup, directorio, o URL.` y esperar.

### 3. Preguntar foco
```
Que querés extraer?
1) Tokens (colores, spacing, type, radii, shadows) — alimenta `/ux-design-system`
2) Componentes reusables (botones, inputs, cards repetidos) — alimenta `/ux-components`
3) Ambos (default)
```
Guardar `FOCUS`.

### 4. Preguntar destino
```
Donde mergeas el output?
1) Crear nuevo design system en `.ux/design-system/` (si no existe)
2) Mergear con design system existente (compara y reporta diffs)
3) Solo generar reporte, no escribir archivos
```
Guardar `DEST`.

## Fase 1 — Cargar HTML

Leer el archivo o concatenar todos los del directorio. Parsear inline styles, classes Tailwind, y `<style>` blocks.

## Fase 2 — Extraer tokens

### Colores
- Listar **todos** los colores que aparecen (`color:`, `background:`, `border-color:`, Tailwind classes `text-*`, `bg-*`, `border-*`).
- Agrupar por similitud perceptual (OKLCH delta E < 10 → mismo color).
- Inferir paleta:
  - Color con mas usos en backgrounds primarios → candidato a `primary`.
  - Colores neutros (saturacion baja) → candidatos a `gray scale`.
  - Verdes/rojos/amarillos en banners → `success` / `danger` / `warning`.
- Detectar dark mode (si hay clases `dark:` en Tailwind o styles con clase `.dark`).

### Spacing
- Listar todos los valores de `padding`, `margin`, `gap` (en px, rem, o Tailwind classes `p-*`, `m-*`, `gap-*`).
- Detectar escala dominante (¿multiples de 4? de 8?).

### Typography
- Fonts en `font-family` o Tailwind `font-*`.
- Sizes en `font-size` o Tailwind `text-*`.
- Weights en `font-weight` o Tailwind `font-*`.
- Detectar escala dominante.

### Radii, shadows, motion
- Mismo proceso — listar, agrupar, inferir escala.

## Fase 3 — Detectar componentes reusables

Buscar **patrones HTML repetidos** entre archivos (si son varios):
- Buttons: cualquier `<button>` o `<a class="...inline-flex...">` con estilo consistente.
- Inputs: `<input>` / `<textarea>` / `<select>` con classes consistentes.
- Cards: `<div>` con `rounded-* shadow-* p-* bg-*` repetido.
- Headers, tables, modals, navs: detectar por estructura semantica.

Para cada componente detectado:
- Nombre propuesto.
- Numero de ocurrencias.
- Variantes detectadas (ej. button: primary, secondary, ghost).
- Snippet representativo.

## Fase 4 — Estructurar reporte

```markdown
## Extraccion: <target>

**Archivos procesados**: <N>
**Foco**: <FOCUS>

## Tokens extraidos

### Color

**Paleta inferida**:
- primary: <hex base + escala inferida 50-900>
- gray: <escala inferida>
- success: <hex>
- warning: <hex>
- danger: <hex>

**Dark mode detectado**: <si | no>

**Notas**:
- <observaciones, ej. "3 azules muy similares — probablemente 1 token con states">

### Spacing

**Escala dominante**: multiplos de <4 | 8>
**Valores extraidos**: <0, 4, 8, 12, 16, 24, 32, 48, 64>
**Notas**: <ej. "valor 13px en un solo lugar — probablemente error, deberia ser 12 o 16">

### Typography

**Fonts**: <font sans + font mono detectados>
**Sizes**: <escala extraida>
**Weights**: <pesos usados>
**Notas**: <ej. "escala no sigue ratio uniforme — propongo normalizar a 1.25">

### Radii / Shadows / Motion

(igual estructura)

## Componentes detectados

### Button — <N> ocurrencias

**Variantes detectadas**:
- primary (<N> ocurrencias): `<snippet>`
- secondary (<N>): `<snippet>`
- ghost (<N>): `<snippet>`

### Input — <N> ocurrencias

(igual)

### Card — <N> ocurrencias

(igual)

...

## Diffs vs design system existente

(solo si DEST=2)

- **Tokens nuevos**: <lista>
- **Tokens divergentes**: <lista — ej. "spacing.4 = 16px aqui, 14px en mockup">
- **Componentes nuevos**: <lista>
- **Componentes con variantes nuevas**: <lista>

## Recomendaciones

1. <ej. "Normalizar 3 azules cercanos a un solo primary con states">
2. <ej. "Adoptar la escala spacing detectada — calza con multiplos de 4">
3. <ej. "Promover el Card detectado a componente formal">

---

_Extraccion generada por `/ux-extract`._
```

## Fase 5 — Escribir archivos (si DEST != 3)

### Si DEST=1 (nuevo design system)

Generar en `.ux/design-system/`:
- `tokens/primitive.json` (DTCG spec) con los tokens extraidos.
- `tokens/semantic.json` con roles inferidos.
- `tailwind.config.js` generado desde los tokens.
- `README.md`.

(Mismo formato que `/ux-design-system` — el ouput es DTCG-compliant.)

### Si DEST=2 (merge)

NO sobreescribir. Generar:
- `.ux/extract/<timestamp>-diff.md` con los diffs detectados.
- `.ux/extract/<timestamp>-proposed-tokens.json` con tokens nuevos a agregar.
- `.ux/extract/<timestamp>-proposed-components.md` con componentes a promover.

El usuario revisa y mergea manualmente (o pide que `/ux-design-system` haga el merge oficial despues).

### Si DEST=3 (solo reporte)

No escribir archivos. Solo el reporte inline + persistencia como issue.

## Fase 6 — Confirmar y persistir GitHub

Default: **si**.

```
Confirmás que creo el issue con label `ux:extract`? (si/no, default: si)
```

```bash
gh label create "ux:extract" --color "5319E7" --description "UX token/component extraction" 2>/dev/null || true

BODY_FILE="$(mktemp -t ux-extract-body.XXXXXX).md"
gh issue create --title "Extract: <target>" --body-file "$BODY_FILE" --label "ux:extract"
```

## Fase 7 — Reportar

```
## Result
- skill: /ux-extract
- files_processed: <N>
- focus: <FOCUS>
- dest: <DEST>
- tokens_color_inferred: <N>
- tokens_spacing_inferred: <N>
- components_detected: <list>
- dark_mode_detected: <true | false>
- files_written: <list o "ninguno" si DEST=3>
- persisted: <true | false>
- url: <URL si persisted>
```

## MUST DO

- Agrupar colores similares (OKLCH delta E < 10) — no listar 17 azules distintos.
- Detectar escala dominante en spacing/type (no asumir).
- Marcar componentes como reusables solo con count >= 3 (igual que patrones en `/pm-feedback`).
- Si DEST=1, generar output DTCG-compliant (igual estructura que `/ux-design-system`).
- Reportar diffs claros si DEST=2 (no sobreescribir).

## MUST NOT DO

- No sobreescribir un design system existente sin que el usuario lo pida con DEST=1 sobre directorio vacio.
- No promover a componente algo que aparece 1-2 veces (eso es one-off, no patron).
- No inventar tokens — todo lo extraido viene del HTML.
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

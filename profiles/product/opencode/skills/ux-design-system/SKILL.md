---
name: ux-design-system
description: Genera un design system de tokens DTCG 2025.10, primitive y semantic, con Tailwind config y puede crear issue ux:design-system.
---

Generar un **design system de tokens** segun la spec W3C DTCG `2025.10`. **Solo tokens**; para componentes usar `/ux-components`. Output: `tokens/primitive.json` + `tokens/semantic.json` + `tailwind.config.js` generado. Los argumentos del skill son el prompt con producto/brand y pueden venir vacios.

Skill **interactivo**.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar.

### 2. Validar input
- Vacio: pedir `Describime el producto/brand. Tono visual deseado (sobrio / expresivo / playful / tecnico), audiencia, y cualquier referencia (color base, fuente preferida, sitios que te gustan).` y esperar.

### 3. Preguntar modo
```
Como armamos los tokens?
1) Desde cero — yo te propongo paleta + escala + fuente y vos ajustas
2) Con color base — pasame un hex (ej. #2563EB) y derivo escala
3) Con referencia — pasame URL de un sitio que te guste y extraemos paleta
4) Otra
```
Guardar `MODE`.

### 4. Preguntar dark mode
```
Incluis dark mode?
1) Si — generamos semantic para light y dark (default)
2) No — solo light
```
Guardar `DARK_MODE`.

## Fase 1 — Definir primitives

### Color
- 1 paleta primary (escala 50-900, 10 stops).
- 1 paleta neutral / gray (escala 50-950).
- 3 paletas semantic-base: success (green), warning (yellow/amber), danger (red). Escala 50-900 cada una.
- Opcional: 1 accent (preguntar al usuario si quiere).

Generar valores con metodo OKLCH (mejor preservacion de perceptual contrast) o HSL si simpler:
- Si `MODE=2`: derivar escala desde el hex usando shifts de luminosidad.
- Si `MODE=3`: extraer colores del sitio de referencia via WebFetch (o pedir al usuario que pegue los colores).

### Typography
- 1 fuente sans (default: `Inter`, `system-ui`).
- 1 fuente mono (default: `JetBrains Mono`, `ui-monospace`).
- Escala: `xs`, `sm`, `base`, `lg`, `xl`, `2xl`, `3xl`, `4xl`. Ratio 1.25 (4 modular).
- Weights: 400, 500, 600, 700.
- Line-heights por size (tight para headings, normal para body).

### Spacing
- Escala base 4px: `0`, `1`(4), `2`(8), `3`(12), `4`(16), `5`(20), `6`(24), `8`(32), `10`(40), `12`(48), `16`(64), `20`(80), `24`(96).

### Radius
- `none`(0), `sm`(4), `md`(8), `lg`(12), `xl`(16), `2xl`(24), `full`(9999).

### Shadows
- `sm`, `md`, `lg`, `xl` (escalado en blur + offset + opacity).

### Motion
- Durations: `fast`(150ms), `base`(250ms), `slow`(400ms).
- Easings: `ease-out`, `ease-in-out`.
- Convencion: incluir nota explicita en el doc sobre respetar `prefers-reduced-motion`.

Mostrar al usuario los primitives propuestos:
```
## Primitive tokens propuestos

### Color
- primary: <preview de la escala — bloques de color>
- gray: <preview>
- success / warning / danger: <previews>

### Typography
- sans: <fuente>
- escala: xs(12) sm(14) base(16) lg(18) xl(20) 2xl(24) 3xl(30) 4xl(36)

### Spacing, Radius, Shadows, Motion
<lista corta>

Ajustar algo? (si/no — si si, decime que)
```

Iterar hasta que el usuario apruebe.

## Fase 2 — Definir semantic

Mapear roles a primitives. Estructura DTCG:

```json
{
  "color": {
    "text": {
      "primary":   { "$value": "{color.gray.900}", "$type": "color" },
      "secondary": { "$value": "{color.gray.600}", "$type": "color" },
      "muted":     { "$value": "{color.gray.500}", "$type": "color" },
      "inverse":   { "$value": "{color.gray.50}",  "$type": "color" },
      "danger":    { "$value": "{color.red.600}",  "$type": "color" },
      "success":   { "$value": "{color.green.700}","$type": "color" },
      "warning":   { "$value": "{color.amber.700}","$type": "color" }
    },
    "bg": {
      "page":     { "$value": "{color.gray.50}",  "$type": "color" },
      "surface":  { "$value": "#FFFFFF",          "$type": "color" },
      "muted":    { "$value": "{color.gray.100}", "$type": "color" },
      "danger":   { "$value": "{color.red.50}",   "$type": "color" }
    },
    "border": {
      "subtle":   { "$value": "{color.gray.200}", "$type": "color" },
      "strong":   { "$value": "{color.gray.300}", "$type": "color" },
      "focus":    { "$value": "{color.primary.500}", "$type": "color" }
    },
    "action": {
      "primary":         { "$value": "{color.primary.600}", "$type": "color" },
      "primary-hover":   { "$value": "{color.primary.700}", "$type": "color" },
      "primary-active":  { "$value": "{color.primary.800}", "$type": "color" },
      "danger":          { "$value": "{color.red.600}",     "$type": "color" }
    }
  },
  "spacing": { /* alias a primitives spacing */ },
  "radius":  { /* alias */ }
}
```

Si `DARK_MODE=si`, generar ademas `semantic.dark.json` con los mismos roles pero apuntando a diferentes primitives (ej. `text.primary → gray.50`, `bg.page → gray.950`).

## Fase 3 — Generar Tailwind config

Generar `tailwind.config.js` que consume los tokens:

```js
const primitive = require('./tokens/primitive.json');
const semantic = require('./tokens/semantic.json');

// Helper: resuelve aliases {color.gray.900} → valor concreto
function resolve(token) { /* ... */ }

module.exports = {
  content: ['./**/*.{html,js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        text: { /* desde semantic.color.text */ },
        bg:   { /* ... */ },
        border: { /* ... */ },
        action: { /* ... */ }
      },
      spacing: { /* desde primitive.spacing */ },
      borderRadius: { /* ... */ },
      boxShadow: { /* ... */ },
      transitionDuration: { /* ... */ }
    }
  },
  darkMode: 'class'  // si DARK_MODE
};
```

## Fase 4 — Doc del design system

Generar `.ux/design-system/README.md`:

```markdown
# Design System: <producto>

Spec: W3C Design Tokens (DTCG `2025.10`)

## Estructura

- `tokens/primitive.json` — paleta cruda (colores, spacing, type, etc).
- `tokens/semantic.json` — roles (text.primary, bg.surface, etc).
- `tokens/semantic.dark.json` — variantes dark (si aplica).
- `tailwind.config.js` — consumidor de los tokens para Tailwind v3+.

## Como usar

En tu HTML/JSX, usá las semantic via Tailwind:
\`\`\`html
<button class="bg-action-primary hover:bg-action-primary-hover text-text-inverse">
  Crear cuenta
</button>
\`\`\`

NUNCA uses primitives directamente (`bg-primary-500`) — siempre el semantic.

## Reglas

- Cambios de paleta → editar primitives. Propaga a todos los semantic.
- Cambios de rol (ej. "ahora errors son orange en lugar de red") → editar semantic. No toca primitives.
- Dark mode: el toggle es via `class="dark"` en `<html>`.

## Generado por

`/ux-design-system` — version <fecha>.
```

## Fase 5 — Persistir archivos

Path base: `.ux/design-system/`

Archivos:
- `tokens/primitive.json`
- `tokens/semantic.json`
- (si DARK_MODE) `tokens/semantic.dark.json`
- `tailwind.config.js`
- `README.md`

Usar una edicion segura para cada uno (NO `echo` / heredoc).

## Fase 6 — Confirmar y persistir GitHub

Default: **si**.

```
Confirmás que creo el issue con label `ux:design-system` linkeando los archivos? (si/no, default: si)
```

```bash
gh label create "ux:design-system" --color "0E8A16" --description "UX design system" 2>/dev/null || true

BODY_FILE="$(mktemp -t ux-ds-body.XXXXXX).md"
```

Body:
```markdown
## Design System: <producto>

Generado con `/ux-design-system`. Spec: DTCG `2025.10`.

## Archivos

- `.ux/design-system/tokens/primitive.json`
- `.ux/design-system/tokens/semantic.json`
- `.ux/design-system/tokens/semantic.dark.json` (si aplica)
- `.ux/design-system/tailwind.config.js`
- `.ux/design-system/README.md`

## Resumen

- Paleta primary: <color base>
- Dark mode: <si/no>
- Fuente sans: <font>

## Siguientes pasos sugeridos

- Generar componentes que consuman estos tokens con `/ux-components`.
- Mockup de pantallas: `/ux-propose` puede recibir el path del design system.
- Auditar a11y de componentes generados: `/ux-a11y-audit`.

---

_Design system generado por `/ux-design-system`._
```

```bash
gh issue create --title "Design System: <producto>" --body-file "$BODY_FILE" --label "ux:design-system"
```

## Fase 7 — Reportar

```
## Result
- skill: /ux-design-system
- directory: .ux/design-system/
- mode: <MODE>
- dark_mode: <si | no>
- tokens_primitive: <count colores + spacing + radii + shadow + motion>
- tokens_semantic: <count roles>
- persisted: <true | false>
- url: <URL si persisted>
```

## MUST DO

- Seguir spec DTCG `2025.10`: `$value` + `$type` por token, refs con `{...}`.
- Generar las 3 capas: primitive, semantic (light + dark si aplica), Tailwind config.
- Generar `README.md` con instrucciones de uso explicitas (usar semantic, no primitives).
- Incluir nota sobre `prefers-reduced-motion` en el doc de motion.

## MUST NOT DO

- No generar componentes (eso es `/ux-components`).
- No usar primitives directamente en ejemplos del README (semantic siempre).
- No omitir dark mode si el usuario lo pidio.
- No generar `component tokens` (capa 3) — para sistemas chicos, primitive + semantic alcanza.
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

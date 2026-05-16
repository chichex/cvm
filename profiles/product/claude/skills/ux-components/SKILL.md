Generar **componentes UI en HTML+Tailwind** que consumen un design system. Default: ~18 componentes esenciales (atoms (atomos) responsive single-file + compounds (compuestos) dual-file (archivo separado por viewport) cuando la interaccion difiere). Flags para limitar el alcance. `$ARGUMENTS` puede traer flags y/o lista de componentes especifica.

Skill **interactivo**.

## Pre-flight

### 1. Parsear `$ARGUMENTS`
Detectar flags:
- `--desktop` — generar solo version desktop (sin responsive classes).
- `--mobile` — generar solo version mobile (sin breakpoints).
- `--only <list>` — generar solo los componentes listados (comma-separated, ej. `button,input,modal`).
- `--from <path>` — usar tokens desde `<path>` (default: `.ux/design-system/`).
- `--slug <name>` — usar `<name>` como subdirectorio (default: `default`).

Si no hay flags, **default**: responsive (mobile + desktop con breakpoints, dual-file donde aplica), todos los 18 esenciales, slug `default`.

### 2. Validar tokens disponibles
Buscar `.ux/design-system/tailwind.config.js` y `.ux/design-system/tokens/semantic.json`.

Si no existen:
```
No encontre un design system en `.ux/design-system/`. Opciones:
1) Generarlo primero con `/ux-design-system` (recomendado)
2) Pasame el path con `--from <path>`
3) Generar componentes con paleta default (Tailwind base, no recomendado)
```

### 3. Confirmar slug

Si el usuario no paso `--slug`, preguntar:
```
Como llamamos al set de componentes? (kebab-case, max 40 chars — default: "default")
```
Guardar `SLUG`. El output va a `.ux/components/<SLUG>/`.

## Fase 1 — Definir alcance de componentes

### Lista completa por defecto (18 componentes)

**Atoms — responsive single-file (12)**:
1. `button` — primary, secondary, ghost, danger variants
2. `input` — text, email, password, with-label, with-error
3. `textarea`
4. `select`
5. `checkbox`
6. `radio` — radio group
7. `switch`
8. `badge` — variants por status
9. `avatar` — image, initials, fallback
10. `alert` — info, success, warning, danger
11. `tooltip` — top, bottom, left, right
12. `toast` — info, success, warning, danger

**Compounds/Layout — single-file responsive (8)**:
13. `card`
14. `tabs`
15. `accordion`
16. `empty-state`
17. `spinner` / `skeleton`
18. `breadcrumbs`

**Compounds/Layout — DUAL-FILE (interaccion difiere)**:
19. `modal` (desktop overlay) + `sheet` (mobile bottom sheet) — 2 archivos
20. `nav` (desktop top nav) + `drawer` (mobile hamburger) — 2 archivos
21. `sidebar` (desktop) + (mobile usa drawer) — 1 archivo (sidebar.html) + nota
22. `table` (desktop data grid) + `card-list` (mobile collapse) — 2 archivos
23. `filters-bar` (desktop inline) + `filters-sheet` (mobile bottom sheet) — 2 archivos
24. `date-picker` (desktop popover) + `date-picker-fullscreen` (mobile) — 2 archivos

Total: 18 atoms/compounds single-file + 6 compounds que generan 2 archivos = **24 archivos** en default.

Si flags `--desktop` o `--mobile`, los dual-file se reducen a un solo archivo cada uno.
Si flag `--only`, generar solo los listados.

Mostrar al usuario el alcance final antes de generar:
```
Voy a generar:
- 12 atoms (single-file)
- 6 single-file compounds
- 6 dual-file compounds (12 archivos)

Total: <N> archivos en `.ux/components/<SLUG>/`

Procedo? (si/no/ajustar)
```

## Fase 2 — Generar archivos

Path base: `.ux/components/<SLUG>/`

Por cada componente, generar archivo HTML self-contained:

```html
<!doctype html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title><componente></title>
  <script src="https://cdn.tailwindcss.com"></script>
  <script>
    tailwind.config = /* import desde ../../design-system/tailwind.config.js */
  </script>
  <style>
    @media (prefers-reduced-motion: reduce) {
      * { animation: none !important; transition: none !important; }
    }
  </style>
</head>
<body class="min-h-screen bg-bg-page p-8">

  <!-- DOC -->
  <header class="mb-8">
    <h1 class="text-2xl font-semibold text-text-primary"><componente></h1>
    <p class="text-text-secondary mt-1"><1-liner descripcion></p>
  </header>

  <!-- VARIANTES -->
  <section class="space-y-6">

    <!-- variante 1 -->
    <div>
      <h2 class="text-sm font-medium text-text-secondary mb-2">Default</h2>
      <!-- COMPONENTE AQUI -->
    </div>

    <!-- variante 2 (si aplica) -->
    <div>
      <h2 class="text-sm font-medium text-text-secondary mb-2">Variant — <nombre></h2>
      <!-- ... -->
    </div>

    <!-- estados (hover, focus, disabled, error) -->
    <div>
      <h2 class="text-sm font-medium text-text-secondary mb-2">States</h2>
      <!-- mostrar default, hover (data-state=hover), focus (data-state=focus), disabled, error -->
    </div>

  </section>

  <!-- PREVIEW MOBILE (solo si no es --desktop) -->
  <section class="mt-12 border-t pt-8">
    <h2 class="text-sm font-medium text-text-secondary mb-2">Preview @ 375px (mobile)</h2>
    <iframe srcdoc="<!-- self-rendering del componente con width fijo -->" class="w-[375px] h-[400px] border rounded-md"></iframe>
  </section>

</body>
</html>
```

### Reglas de generacion por componente

- **Usar solo tokens semanticos** del design system (ej. `bg-action-primary`, `text-text-primary`). Nunca primitivos.
- **WCAG 2.2 AA built-in**: contraste, focus visible, touch target ≥24px, labels, ARIA donde corresponda.
- **Mostrar estados**: default, hover, focus, disabled, error (cuando aplica).
- **Para dual-file**: cada archivo declara su target (`<!-- target: desktop -->` o `<!-- target: mobile -->`). Tokens compartidos.

### Componentes dual-file — diferencias clave

| Componente | Desktop | Mobile |
|---|---|---|
| `modal` / `sheet` | Overlay centrado, max-w-md, fade-in | Bottom sheet, full-width, slide-up |
| `nav` / `drawer` | Horizontal nav bar + dropdowns | Hamburger button → slide-in drawer izquierda |
| `table` / `card-list` | `<table>` con columns, sort headers | `<div>` con cada row colapsada en card |
| `filters-bar` / `filters-sheet` | Row horizontal con chips/selects | Button "Filtros" → bottom sheet con form |
| `date-picker` / `date-picker-fullscreen` | Popover calendar 280x320 | Modal full-screen con scroll-pickers |

## Fase 3 — Generar indice

`.ux/components/<SLUG>/README.md`:

```markdown
# Componentes UI

Generado con `/ux-components`. Consume tokens de `../../design-system/`.

## Atoms (single-file responsive)

- [button](button.html)
- [input](input.html)
- ...

## Compounds/Layout single-file

- [card](card.html)
- ...

## Dual-file (mobile + desktop)

- [modal](modal.html) (desktop) + [sheet](sheet.html) (mobile)
- [nav](nav.html) + [drawer](drawer.html)
- [table](table.html) + [card-list](card-list.html)
- [filters-bar](filters-bar.html) + [filters-sheet](filters-sheet.html)
- [date-picker](date-picker.html) + [date-picker-fullscreen](date-picker-fullscreen.html)

## Como usar

Abri cada `.html` en browser. Tailwind CDN + tokens del design system cargados.

Para usar en tu app: copiar el snippet HTML del componente, asegurarse de tener el `tailwind.config.js` del design system.
```

## Fase 4 — Confirmar y guardar

```
Confirmás que guardo el output en .ux/components/<SLUG>/? (si/no, default: si)
```

Si si: si la carpeta `.ux/components/<SLUG>/` no existe, crearla con `mkdir -p .ux/components/<SLUG>/` antes de escribir. Luego usar `Write` tool para cada `.html` y el `README.md`.

## Fase 5 — Reportar

```
## Result
- skill: /ux-components
- directory: .ux/components/<SLUG>/
- design_system_from: <path>
- target: <responsive | desktop | mobile>
- components_count: <N>
- files_generated: <N>
- dual_file_components: <list>
- saved: <true | false>
```

## MUST DO

- Validar existencia del design system antes de generar.
- Aplicar default (responsive + todos los 18) si no hay flags.
- Usar solo tokens semanticos del design system (nunca primitivos).
- Mostrar estados (default, hover, focus, disabled, error) por componente.
- Para dual-file: tokens compartidos, solo cambian estructura + interaccion.
- WCAG 2.2 AA built-in.
- Guardar todo en `.ux/components/<SLUG>/` con `Write` tool.

## MUST NOT DO

- No generar componentes sin design system (forzar `/ux-design-system` primero).
- No duplicar archivos en componentes que son genuinamente responsive (atoms).
- No generar dual-file para componentes que no lo necesitan (eso es overhead).
- No usar primitivos directamente (siempre semanticos).
- No usar `gh` ni depender de GitHub.
- No persistir nada en auto-memory.

Generar **3-4 propuestas de pantalla en HTML+Tailwind** a partir de un prompt, para que el usuario elija y/o combine. Cada propuesta varia en un eje declarado (no random), abre en browser, y respeta WCAG 2.2 AA built-in. `$ARGUMENTS` es el prompt de la pantalla (puede venir vacio).

Skill **interactivo**.

## Pre-flight

### 1. Validar input

- Vacio: pedir `Que pantalla queres? (ej. "login para empresas con SSO", "dashboard de metricas de retencion", "settings de notificaciones")` y esperar.

### 2. Preguntar ejes de variacion

```
Las 3-4 propuestas van a diferenciarse en un eje. Cual?
1) Densidad de informacion (compacta vs espaciosa)
2) Jerarquia visual (que se enfatiza primero)
3) Tono (sobrio vs expresivo Material 3-style)
4) Layout (sidebar / top-nav / single-column)
5) Combinacion — mezcla 2 ejes
6) Otra
```

Guardar `EJE`. Default si el usuario manda enter: `4) Layout`.

### 3. Preguntar design system base

```
Usas un design system existente?
1) No — generamos cada propuesta con su propia paleta (rapido, exploratorio)
2) Si — pasame el path al directorio de tokens (.ux/design-system/) o pegalos
3) Si — pasame los valores base (color primario + fuente) y armamos uno minimo
```

Guardar `DS_SOURCE`. Si 2 o 3, leer/cargar los tokens.

### 4. Preguntar cantidad de propuestas

```
Cuantas variantes querés? (2-4, default: 3)
```

Guardar `N_VARIANTS`.

## Fase 1 — Definir las variantes

Generar `N_VARIANTS` declaraciones de variante, cada una con:

- **Nombre corto** (1-2 palabras).
- **Posicion en el EJE** (ej. "densidad alta" / "densidad media" / "densidad baja").
- **Contrapartida declarada**: que gana, que pierde.

Mostrar al usuario:

```
## Variantes a generar

A) <nombre> — <posicion en eje>
   Contrapartida: gana <X>, pierde <Y>
B) <nombre> — ...
C) <nombre> — ...

Bien? (si / ajustar)
```

Si "ajustar", iterar. Si "si", seguir.

## Fase 2 — Generar HTML por variante

Para cada variante, generar un archivo HTML self-contained con Tailwind CDN.

Path: `.ux/proposals/<slug>/variant-<N>-<nombre>.html`

Donde `<slug>` es derivado del prompt original (kebab-case, max 40 chars).

Estructura del HTML:

```html
<!doctype html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title><variante> — <pantalla></title>
  <script src="https://cdn.tailwindcss.com"></script>
  <style>
    /* Variables CSS desde tokens si DS_SOURCE != 1 */
    :root {
      --color-primary: <de tokens semantic>;
      /* ... */
    }
    @media (prefers-reduced-motion: reduce) {
      * { animation: none !important; transition: none !important; }
    }
  </style>
</head>
<body class="min-h-screen bg-gray-50 text-gray-900">

  <!-- Banner de variante (solo para preview, removible) -->
  <div class="bg-yellow-100 border-b border-yellow-300 text-yellow-900 text-xs px-4 py-2">
    Variante <N>: <nombre> — <posicion en eje> · contrapartida: gana <X>, pierde <Y>
  </div>

  <main>
    <!-- LA PANTALLA AQUI -->
  </main>

</body>
</html>
```

### Reglas de generacion

**WCAG 2.2 AA built-in en cada variante**:

- Contraste minimo 4.5:1 para texto normal, 3:1 para texto grande (>18.66px).
- `:focus-visible` con outline visible en todos los interactivos.
- Touch targets >= 24px (recomendado 44-48px para mobile).
- `prefers-reduced-motion` respetado (en el `<style>`).
- Heading hierarchy correcta (un solo `<h1>`, `<h2>` siguen al `<h1>`, etc.).
- Labels asociados a inputs (`<label for>` o anidado).
- `alt` text en imagenes.

**Tailwind responsive**: cada variante es un único archivo HTML responsive (mobile-first + breakpoints `sm:`, `md:`, `lg:` para desktop). Los compounds (compuestos) dual-file (archivo separado por viewport, ej. modal/sheet) son trabajo de `/ux-components`, no de este skill.

**Contenido real, no lorem ipsum**: usar contenido plausible al dominio del prompt (ej. para "dashboard de retencion", mostrar metricas reales tipo "Retencion D7 por cohorte: 38%" en vez de "Lorem ipsum").

**No JavaScript salvo lo minimo**: estas son propuestas para alinear visualmente, no prototipos interactivos. Tooltips, dropdowns y modales pueden mostrarse en estado abierto via `data-state="open"` para que se vean sin clicks.

## Fase 3 — Generar indice

Crear `.ux/proposals/<slug>/README.md`:

```markdown
# Propuestas: <pantalla>

Prompt original: <prompt>
Eje de variacion: <EJE>
Design system: <DS_SOURCE descripcion>

## Variantes

- [Variante A — <nombre>](variant-A-<nombre>.html) — <contrapartida>
- [Variante B — <nombre>](variant-B-<nombre>.html) — <contrapartida>
- [Variante C — <nombre>](variant-C-<nombre>.html) — <contrapartida>

## Como mirarlas

Abri cada `.html` en el browser. Cada uno funciona standalone (Tailwind CDN).

Para comparar viewports: usá las dev tools (Cmd+Shift+M en Chrome) y switchea entre 375px (mobile) y 1280px (desktop).

## Proximos pasos sugeridos

- Mirar las variantes en browser, decidir cual avanzar.
- Si ninguna calza, iterar con `/ux-propose` con nuevo eje.
- Si una calza, extraer componentes con `/ux-extract`.
- Si la pantalla tiene faltantes de accesibilidad, correr `/ux-a11y-audit` sobre el HTML elegido.
```

## Fase 4 — Confirmar y guardar

```
Voy a generar <N> variantes en `.ux/proposals/<slug>/`. Confirmás? (si/no, default: si)
```

Si si: si la carpeta `.ux/proposals/<slug>/` no existe, crearla con `mkdir -p .ux/proposals/<slug>/` antes de escribir. Luego usar `Write` tool para crear cada `variant-<N>-<nombre>.html` y el `README.md` indice.

## Fase 5 — Reportar

```
## Result
- skill: /ux-propose
- variants_generated: <N>
- directory: .ux/proposals/<slug>/
- eje_variacion: <EJE>
- design_system: <DS_SOURCE>
- saved: true
```

Y debajo: `Variantes guardadas en .ux/proposals/<slug>/. Abrí los .html en el browser.`

## MUST DO

- Generar `N_VARIANTS` archivos HTML self-contained (Tailwind CDN).
- Declarar el eje de variacion explicitamente — variantes ortogonales, no random.
- WCAG 2.2 AA en cada variante (contraste, focus, target-size, reduced-motion, heading hierarchy, labels, alt).
- Responsive mobile-first con breakpoints (sin generar archivos separados).
- Contenido plausible al dominio (no lorem ipsum).
- Generar README.md indice en el directorio.
- Confirmar con el usuario antes de escribir.

## MUST NOT DO

- No generar variantes que se diferencian solo cosmeticamente — el eje tiene que producir diferencias estructurales.
- No usar JavaScript salvo absolutamente minimo (estados abiertos via `data-state` para preview).
- No usar lorem ipsum.
- No omitir `prefers-reduced-motion` ni focus visible.
- No usar `gh` ni depender de GitHub.
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

**Auditor WCAG 2.2 AA dedicado**. Aplica reglas deterministicas sobre HTML, URL, imagen, o directorio. Genera reporte priorizado por severidad con fixes concretos. Mas estricto y exhaustivo que `/ux-critique`. `$ARGUMENTS` es el target (puede venir vacio).

Skill **interactivo**.

## Pre-flight

### 1. Detectar tipo de target

Mismo flujo que `/ux-critique`: detectar si es path local (imagen o HTML), URL, directorio, o paste.

Casos especiales:
- Directorio (ej. `.ux/components/`): auditar **todos** los `.html` adentro.
- Path a `.html`: auditar ese archivo.
- Path a imagen: vision-only, limitado a heuristicas visuales (contraste, touch target, jerarquia).
- URL: WebFetch HTML + screenshot via playwright (igual que `/ux-critique`).

### 2. Preguntar nivel
```
Nivel WCAG a auditar:
1) AA (recomendado — baseline legal en 2025)
2) AAA (mas estricto — solo si tu producto tiene cumplimiento especifico)
3) Solo Level A (minimo absoluto — no recomendado)
```
Guardar `LEVEL`. Default `1) AA`.

### 3. Preguntar foco
```
Algun area especifica a priorizar?
1) Todo (default — auditoria completa)
2) Solo contraste de color
3) Solo navegacion por teclado y focus
4) Solo formularios (labels, ARIA, estado de error)
5) Solo media (alt text, captions, transcripts)
```
Guardar `FOCUS`.

### 4. Derivar slug

Derivar `<slug>` desde el target (path, hostname, o descripcion) en kebab-case, max 40 chars. Se usa para guardar en `.ux/a11y/<slug>.md`.

## Fase 1 — Cargar HTML / imagen

Si target es HTML/directorio: leer archivos. Parsear estructura.
Si imagen: Read con vision.
Si URL: WebFetch + screenshot.

## Fase 2 — Aplicar checks WCAG 2.2

Aplicar segun `LEVEL` y `FOCUS`. Cada hallazgo tiene:
- **Criterio**: codigo WCAG (ej. `1.4.3`)
- **Titulo**: nombre del criterio (ej. "Contrast (Minimum)")
- **Nivel**: A | AA | AAA
- **Severidad**: urgente | importante | menor | nit
- **Donde**: ubicacion en el HTML (selector o linea) o en la imagen (posicion descriptiva)
- **Problema**: que esta mal en 1-2 lineas
- **Fix**: snippet de codigo o accion concreta

### Checks principales (AA — incluir A automaticamente)

#### Perceivable

**1.1.1 Non-text content (A)**
- `<img>` sin `alt`? → urgente
- `<img alt="">` para imagen decorativa → ok, validar que efectivamente sea decorativa
- `<svg>` con contenido informativo sin `<title>` o `aria-label` → importante

**1.3.1 Info and Relationships (A)**
- Headings sin orden jerarquico (h1 → h3 sin h2) → importante
- Multiples `<h1>` por pagina → importante
- Listas con `<div>` en lugar de `<ul>/<ol>` → menor
- Tables sin `<th>` o `scope` → importante

**1.3.5 Identify Input Purpose (AA)**
- Inputs comunes (email, name, tel, address) sin `autocomplete` apropiado → menor

**1.4.3 Contrast (Minimum) (AA)**
- Texto normal con contraste <4.5:1 → urgente
- Texto grande (>=18.66px o >=14px bold) <3:1 → urgente
- Componentes UI (focus rings, borders interactivos) <3:1 → importante
- Para verificar contraste, computar ratio del color de texto vs background visible. Si vision-only, estimar.

**1.4.11 Non-text Contrast (AA)**
- UI components y graphical objects <3:1 → importante

#### Operable

**2.1.1 Keyboard (A)**
- Elementos clickeables (`<div onclick>`) sin `tabindex` y sin keyboard handler → urgente
- Custom controls sin `role` apropiado → importante

**2.4.7 Focus Visible (AA)**
- `:focus` overrideado a `outline: none` sin alternativa visible → urgente
- Focus styles indistinguibles del estado default → importante

**2.4.11 Focus Not Obscured (Minimum) (AA)** _(nuevo en 2.2)_
- Elementos con focus pueden quedar tapados por sticky headers/footers → importante si detectable

**2.5.8 Target Size (Minimum) (AA)** _(nuevo en 2.2)_
- Touch targets clickeables (`<button>`, `<a>`, custom) con altura+ancho <24x24 CSS px → importante
- Excepciones: links inline en texto, custom controls con espaciado equivalente
- Recomendado para mobile: 44x44 (no exigir, solo mencionar como mejora)

#### Understandable

**3.1.1 Language of Page (A)**
- `<html>` sin `lang` → importante

**3.3.1 Error Identification (A)**
- Forms con validacion solo en client console (sin texto visible al usuario) → importante
- Errores con solo color (sin texto + sin icono) → importante

**3.3.2 Labels or Instructions (A)**
- `<input>` sin `<label>` asociado (via `for` o anidado) ni `aria-label` → urgente

**3.3.7 Redundant Entry (A)** _(nuevo en 2.2)_
- Forms multi-step que piden la misma info dos veces sin pre-fill o picker → menor

**3.3.8 Accessible Authentication (Minimum) (AA)** _(nuevo en 2.2)_
- Login requiere "cognitive function tests" (recordar password sin paste, captchas inaccesibles) sin alternativa → importante

#### Robust

**4.1.2 Name, Role, Value (A)**
- Custom controls (`<div role="button">`) sin nombre accesible → urgente
- Estado dinamico (expanded, selected, checked) sin `aria-expanded`/`aria-selected`/`aria-checked` → importante

**4.1.3 Status Messages (AA)**
- Toasts / notifications sin `role="status"` o `aria-live` → importante

### Checks de Level AAA (solo si `LEVEL=2`)

- **1.4.6 Contrast (Enhanced)**: texto >=7:1, texto grande >=4.5:1.
- **2.4.13 Focus Appearance (AAA)**: focus indicator con perimetro y contraste especifico.
- **3.3.9 Accessible Authentication (Enhanced)**: cero cognitive function tests.

## Fase 3 — Reduced motion check

Aparte de los criterios WCAG, verificar:
- CSS tiene `@media (prefers-reduced-motion: reduce)` que cancela animaciones?
- JavaScript respeta `window.matchMedia('(prefers-reduced-motion: reduce)')`?
- Si no: `prefers-reduced-motion` → **importante** (relacionado a 2.3.3 Animation from Interactions AAA, pero practica AA estandar).

## Fase 4 — Estructurar reporte

```markdown
## Auditoria WCAG 2.2 <LEVEL>: <target>

**Target**: <path/URL/directorio>
**Foco**: <FOCUS>
**Archivos auditados**: <N> (si directorio)
**Limitaciones**: <"sin vision" si target=HTML sin imagen, "vision-only sin codigo" si target=imagen>

## Resumen

| Severidad | Count |
|-----------|-------|
| Urgente    | <N>   |
| Importante | <N>   |
| Menor      | <N>   |
| Nit        | <N>   |

**Cumplimiento estimado**: <%> del nivel <LEVEL>.

## Hallazgos por criterio

### 1.4.3 Contrast (Minimum) — AA

- [<sev>] **<donde>** — <problema> (ratio actual: <X.X>:1, requerido: 4.5:1)
  - Fix: <accion>
- [<sev>] **<donde>** — ...

### 2.4.7 Focus Visible — AA

(igual)

...

(repetir por cada criterio con al menos 1 hallazgo)

## Top 10 fixes priorizados

1. [<sev>] <criterio> @ <donde>: <problema resumido>
   - Fix: <accion>
2. ...

## Reduced motion

- Status: <ok | falta>
- <Si falta: "Agregar @media (prefers-reduced-motion: reduce) que cancele animaciones y transiciones.">

## No verificable automaticamente

Items que requieren testing manual:
- Screen reader (real, con NVDA/JAWS/VoiceOver).
- Navegacion por teclado end-to-end.
- Carga cognitiva de los flows.
- Reading order con CSS Grid / Flexbox (a veces tab order != reading order).

---

_Auditoria generada por `/ux-a11y-audit`._
```

## Fase 5 — Confirmar y guardar

```
Confirmás que guardo el output en .ux/a11y/<slug>.md? (si/no, default: si)
```

Si si: si la carpeta `.ux/a11y/` no existe, crearla con `mkdir -p .ux/a11y/` antes de escribir. Luego usar `Write` tool para crear `.ux/a11y/<slug>.md` con el reporte.

## Fase 6 — Reportar

```
## Result
- skill: /ux-a11y-audit
- level: <LEVEL>
- focus: <FOCUS>
- files_audited: <N>
- findings_total: <N>
- findings_urgent: <N>
- findings_important: <N>
- compliance_estimated: <%>
- reduced_motion: <ok | falta>
- file: .ux/a11y/<slug>.md
- saved: <true | false>
```

## MUST DO

- Aplicar checks segun `LEVEL` y `FOCUS`.
- Incluir codigo de criterio (ej. `1.4.3`) y nombre en cada hallazgo.
- Reportar contraste con ratio numerico cuando se puede medir.
- Listar items que requieren testing manual (declarar limite).
- Auditar reduced-motion aparte de WCAG core (esta en AAA, pero practica AA estandar).
- Guardar en `.ux/a11y/<slug>.md` con `Write` tool.

## MUST NOT DO

- No declarar "100% cumplimiento" — siempre hay limite de auditoria automatica.
- No omitir criterios nuevos de 2.2 (2.4.11, 2.5.8, 3.3.7, 3.3.8).
- No mezclar AAA en una auditoria AA salvo que el usuario lo pida explicitamente.
- No usar `gh` ni depender de GitHub.
- No persistir nada en auto-memory.

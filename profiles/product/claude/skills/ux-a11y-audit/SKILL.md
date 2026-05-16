**Auditor WCAG 2.2 AA dedicado**. Aplica reglas determinísticas sobre HTML, URL, imagen, o directorio. Genera reporte priorizado por severidad con fixes concretos. Mas estricto y exhaustivo que `/ux-critique`. `$ARGUMENTS` es el target (puede venir vacio).

Skill **interactivo**.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar.

### 2. Detectar tipo de target

Mismo flujo que `/ux-critique`: detectar si es path local (imagen o HTML), URL, directorio, o paste.

Cases especiales:
- Directorio (ej. `.ux/components/`): auditar **todos** los `.html` adentro.
- Path a `.html`: auditar ese archivo.
- Path a imagen: vision-only, limitado a heuristicas visuales (contraste, target size, hierarchy).
- URL: WebFetch HTML + screenshot via playwright (igual que `/ux-critique`).

### 3. Preguntar nivel
```
Nivel WCAG a auditar:
1) AA (recomendado — baseline legal en 2025)
2) AAA (mas estricto — solo si tu producto tiene compliance especifico)
3) Solo Level A (minimo absoluto — no recomendado)
```
Guardar `LEVEL`. Default `1) AA`.

### 4. Preguntar foco
```
Algun area especifica a priorizar?
1) Todo (default — auditoria completa)
2) Solo contraste de color
3) Solo keyboard nav y focus
4) Solo forms (labels, ARIA, error states)
5) Solo media (alt text, captions, transcripts)
```
Guardar `FOCUS`.

## Fase 1 — Cargar HTML / imagen

Si target es HTML/directorio: leer archivos. Parsear estructura.
Si imagen: Read con vision.
Si URL: WebFetch + screenshot.

## Fase 2 — Aplicar checks WCAG 2.2

Aplicar segun `LEVEL` y `FOCUS`. Cada finding tiene:
- **Criterion**: codigo WCAG (ej. `1.4.3`)
- **Title**: nombre del criterio (ej. "Contrast (Minimum)")
- **Level**: A | AA | AAA
- **Severity**: blocker | major | minor | nit
- **Where**: ubicacion en el HTML (selector o linea) o en la imagen (posicion descriptiva)
- **Issue**: que esta mal en 1-2 lineas
- **Fix**: snippet de codigo o accion concreta

### Checks principales (AA — incluir A automaticamente)

#### Perceivable

**1.1.1 Non-text content (A)**
- `<img>` sin `alt`? → blocker
- `<img alt="">` para imagen decorativa → ok, validar que efectivamente sea decorativa
- `<svg>` con contenido informativo sin `<title>` o `aria-label` → major

**1.3.1 Info and Relationships (A)**
- Headings sin orden jerarquico (h1 → h3 sin h2) → major
- Multiples `<h1>` por pagina → major
- Listas con `<div>` en lugar de `<ul>/<ol>` → minor
- Tables sin `<th>` o `scope` → major

**1.3.5 Identify Input Purpose (AA)**
- Inputs comunes (email, name, tel, address) sin `autocomplete` apropiado → minor

**1.4.3 Contrast (Minimum) (AA)**
- Texto normal con contraste <4.5:1 → blocker
- Texto grande (>=18.66px o >=14px bold) <3:1 → blocker
- Componentes UI (focus rings, borders interactivos) <3:1 → major
- Para verificar contraste, computar ratio del color de texto vs background visible. Si vision-only, estimar.

**1.4.11 Non-text Contrast (AA)**
- UI components y graphical objects <3:1 → major

#### Operable

**2.1.1 Keyboard (A)**
- Elementos clickeables (`<div onclick>`) sin `tabindex` y sin keyboard handler → blocker
- Custom controls sin `role` apropiado → major

**2.4.7 Focus Visible (AA)**
- `:focus` overrideado a `outline: none` sin alternativa visible → blocker
- Focus styles indistinguibles del estado default → major

**2.4.11 Focus Not Obscured (Minimum) (AA)** _(nuevo en 2.2)_
- Elementos con focus pueden quedar tapados por sticky headers/footers → major si detectable

**2.5.8 Target Size (Minimum) (AA)** _(nuevo en 2.2)_
- Targets clickeables (`<button>`, `<a>`, custom) con altura+ancho <24x24 CSS px → major
- Excepciones: links inline en texto, custom controls con espaciado equivalente
- Recomendado para mobile: 44x44 (no exigir, solo mencionar como mejora)

#### Understandable

**3.1.1 Language of Page (A)**
- `<html>` sin `lang` → major

**3.3.1 Error Identification (A)**
- Forms con validacion solo en client console (sin texto visible al usuario) → major
- Errores con solo color (sin texto + sin icono) → major

**3.3.2 Labels or Instructions (A)**
- `<input>` sin `<label>` asociado (via `for` o anidado) ni `aria-label` → blocker

**3.3.7 Redundant Entry (A)** _(nuevo en 2.2)_
- Forms multi-step que piden la misma info dos veces sin pre-fill o picker → minor

**3.3.8 Accessible Authentication (Minimum) (AA)** _(nuevo en 2.2)_
- Login requiere "cognitive function tests" (recordar password sin paste, captchas inaccesibles) sin alternativa → major

#### Robust

**4.1.2 Name, Role, Value (A)**
- Custom controls (`<div role="button">`) sin nombre accesible → blocker
- Estado dinamico (expanded, selected, checked) sin `aria-expanded`/`aria-selected`/`aria-checked` → major

**4.1.3 Status Messages (AA)**
- Toasts / notifications sin `role="status"` o `aria-live` → major

### Checks de Level AAA (solo si `LEVEL=2`)

- **1.4.6 Contrast (Enhanced)**: texto >=7:1, texto grande >=4.5:1.
- **2.4.13 Focus Appearance (AAA)**: focus indicator con perimetro y contraste especifico.
- **3.3.9 Accessible Authentication (Enhanced)**: cero cognitive function tests.

## Fase 3 — Reduced motion check

Aparte de los criterios WCAG, verificar:
- CSS tiene `@media (prefers-reduced-motion: reduce)` que cancela animaciones?
- JavaScript respeta `window.matchMedia('(prefers-reduced-motion: reduce)')`?
- Si no: `prefers-reduced-motion` → **major** (relacionado a 2.3.3 Animation from Interactions AAA, pero practica AA estandar).

## Fase 4 — Estructurar reporte

```markdown
## Auditoria WCAG 2.2 <LEVEL>: <target>

**Target**: <path/URL/directorio>
**Foco**: <FOCUS>
**Archivos auditados**: <N> (si directorio)
**Limitaciones**: <"sin vision" si target=HTML sin imagen, "vision-only sin codigo" si target=imagen>

## Resumen

| Severity | Count |
|----------|-------|
| Blocker  | <N>   |
| Major    | <N>   |
| Minor    | <N>   |
| Nit      | <N>   |

**Compliance estimado**: <%> del nivel <LEVEL>.

## Findings por criterio

### 1.4.3 Contrast (Minimum) — AA

- [<sev>] **<where>** — <issue> (ratio actual: <X.X>:1, requerido: 4.5:1)
  - Fix: <accion>
- [<sev>] **<where>** — ...

### 2.4.7 Focus Visible — AA

(igual)

...

(repetir por cada criterio con al menos 1 finding)

## Top 10 fixes priorizados

1. [<sev>] <criterio> @ <where>: <issue resumido>
   - Fix: <accion>
2. ...

## Reduced motion

- Status: <ok | missing>
- <Si missing: "Agregar @media (prefers-reduced-motion: reduce) que cancele animaciones y transiciones.">

## No verificable automaticamente

Items que requieren testing manual:
- Screen reader (real, con NVDA/JAWS/VoiceOver).
- Keyboard nav end-to-end.
- Cognitive load de los flows.
- Reading order con CSS Grid / Flexbox (a veces tab order != reading order).

---

_Auditoria generada por `/ux-a11y-audit`._
```

## Fase 5 — Confirmar y persistir

Default: **si**.

```
Confirmás que creo el issue con label `ux:a11y`? (si/no, default: si)
```

```bash
gh label create "ux:a11y" --color "D73A4A" --description "Accessibility audit" 2>/dev/null || true

BODY_FILE="$(mktemp -t ux-a11y-body.XXXXXX).md"
gh issue create --title "A11y audit <LEVEL>: <target>" --body-file "$BODY_FILE" --label "ux:a11y"
```

## Fase 6 — Reportar

```
## Result
- skill: /ux-a11y-audit
- level: <LEVEL>
- focus: <FOCUS>
- files_audited: <N>
- findings_total: <N>
- findings_blocker: <N>
- findings_major: <N>
- compliance_estimated: <%>
- reduced_motion: <ok | missing>
- persisted: <true | false>
- url: <URL si persisted>
```

## MUST DO

- Aplicar checks segun `LEVEL` y `FOCUS`.
- Incluir criterion code (ej. `1.4.3`) y nombre en cada finding.
- Reportar contraste con ratio numerico cuando se puede medir.
- Listar items que requieren testing manual (declarar limit).
- Auditar reduced-motion aparte de WCAG core (esta en AAA, pero practica AA estandar).

## MUST NOT DO

- No declarar "100% compliance" — siempre hay limit de auditoria automatica.
- No omitir criterios nuevos de 2.2 (2.4.11, 2.5.8, 3.3.7, 3.3.8).
- No mezclar AAA en un audit AA salvo que el usuario lo pida explicitamente.
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

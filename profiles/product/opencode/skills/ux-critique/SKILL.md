---
name: ux-critique
description: Critica UX de una pantalla, imagen, URL o HTML; aplica Nielsen 10, heuristicas AI opcionales, prioriza hallazgos y puede crear issue ux:critique.
---

**Critica de UX** sobre una pantalla (imagen, URL, o HTML pegado). Aplica Nielsen 10 + heuristicas de AI cuando corresponde, cubre 7 dimensiones, prioriza hallazgos por severidad. Los argumentos del skill son el target a criticar y pueden venir vacios.

Skill **interactivo**.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar.

### 2. Detectar tipo de target

Trim de los argumentos del skill. Guardar como `TARGET`.

- Vacio: pedir `Pasame el target a criticar: path a imagen (ej. ./screen.png), URL publica (https://...), o pega el HTML.` y esperar.
- Si matchea path local que existe: `TYPE=image`. Validar extension `.png/.jpg/.jpeg/.webp/.gif`.
- Si matchea `^https?://`: `TYPE=url`.
- Si empieza con `<` o tiene tags HTML: `TYPE=html`.
- Otro: preguntar:
  ```
  No reconoci el formato. Es:
  1) Path a imagen local
  2) URL publica
  3) HTML pegado
  ```

### 3. Preguntar contexto
```
Que pantalla es? (1 linea — ej. "login B2B", "checkout de e-commerce", "dashboard de metricas")
```
Guardar `CONTEXT`.

```
La pantalla incluye features de AI (chat, autocomplete con LLM, generative outputs)? (si/no, default: no)
```
Guardar `HAS_AI`. Si si, se aplican heuristicas extra de AI.

## Fase 1 — Cargar el target

### TYPE=image
Leer la imagen directamente con `Read` (Claude la procesa visualmente).

### TYPE=url
Dos pasos en paralelo:
1. WebFetch del HTML — obtener estructura y contenido textual.
2. Screenshot con playwright MCP:
   ```
   playwright/browser_navigate <URL>
   playwright/browser_resize 1280 800
   playwright/browser_take_screenshot — guardar como ./tmp-ux-critique-desktop.png
   playwright/browser_resize 375 800
   playwright/browser_take_screenshot — guardar como ./tmp-ux-critique-mobile.png
   playwright/browser_close
   ```
   Si playwright no esta disponible, avisar al usuario que se trabaja solo con HTML (sin vision).

### TYPE=html
El HTML viene en los argumentos del skill o como paste. Renderizarlo mentalmente; si tiene componentes complejos, recomendar guardarlo a archivo y correr con `TYPE=url file:///...`.

## Fase 2 — Evaluar por 7 dimensiones

Para cada dimension, generar hallazgos. Cada hallazgo tiene:
- **Severity**: `blocker` | `major` | `minor` | `nit`
- **Where**: ubicacion en la pantalla (header, formulario, CTA primario, etc.)
- **Issue**: que esta mal en 1-2 lineas
- **Fix**: accion concreta sugerida

### 1. Clarity (Nielsen #2, #6)
- El proposito de la pantalla es obvio en 5 segundos?
- Labels y CTAs usan lenguaje del usuario, no de la empresa?
- Hay items ambiguos o redundantes?

### 2. Hierarchy
- El item mas importante se distingue visualmente?
- La grilla / spacing respeta agrupacion logica?
- F-pattern / Z-pattern de lectura coherente con prioridad?

### 3. Affordance (Nielsen #6)
- Los elementos clickeables se ven clickeables?
- Hover states / focus states presentes?
- Iconos sin label son interpretables solos?

### 4. Feedback (Nielsen #1)
- Hay loading states donde se esperan operaciones >300ms?
- Acciones destructivas tienen confirmacion o undo?
- Errores se muestran cerca de la causa, no en banner global ambiguo?

### 5. Error prevention (Nielsen #5)
- Formularios validan in-line, no solo on-submit?
- Acciones one-way (delete, send, pay) tienen friccion proporcional?
- Defaults seguros (ej. "guardar como borrador" vs "publicar")?

### 6. Accessibility (Nielsen #4 + WCAG 2.2 AA)
- Contraste ≥4.5:1 (texto normal), ≥3:1 (texto grande)?
- Focus visible en todos los interactivos?
- Target size ≥24×24 CSS px (24 minimo, 44-48 recomendado mobile)?
- Heading hierarchy correcta?
- Labels en inputs (formales o `aria-label`)?
- Alt text en imagenes con contenido?

### 7. Performance perception
- Hay skeletons / placeholders donde puede haber loading?
- Imagenes con `loading="lazy"` (si se ve en el HTML)?
- Animaciones respetan `prefers-reduced-motion`?

### Bonus — AI heuristics (solo si `HAS_AI=true`)

Aplicar adicionalmente:
- **Transparencia**: el usuario sabe que esta interactuando con AI? Sabe que puede y que no puede?
- **Control sobre delegacion**: puede el usuario corregir, undo, ajustar el output?
- **Manejo de incertidumbre**: outputs probabilisticos se muestran como tal (confidence, "sugerencia", "tal vez")?
- **Recovery**: si el AI se equivoca, hay path claro de correccion?

## Fase 3 — Estructurar reporte

```markdown
## Critica de UX: <CONTEXT>

**Target**: <descripcion del target — path / URL / HTML>
**Vision usada**: <true (imagen + screenshots) | false (solo HTML)>
**Recall declarado**: ~50-80% — esta critica no es exhaustiva, prioriza hallazgos visibles.

## Resumen

- Total hallazgos: <N>
- Blockers: <N> | Majors: <N> | Minors: <N> | Nits: <N>

## Hallazgos por dimension

### 1. Clarity

- [<sev>] **<where>** — <issue>
  - Fix: <accion concreta>
- [<sev>] **<where>** — <issue>
  - Fix: <accion>

### 2. Hierarchy

(igual)

### 3. Affordance

(igual)

### 4. Feedback

(igual)

### 5. Error prevention

(igual)

### 6. Accessibility (WCAG 2.2 AA)

(igual — incluir referencia al criterio especifico ej. "WCAG 1.4.3 Contrast")

### 7. Performance perception

(igual)

<Solo si HAS_AI=true:>
### Bonus — AI heuristics

(igual)

## Top 5 fixes prioritarios

1. [<sev>] <where>: <issue resumido>
2. ...

## Strengths

- <que esta bien — 2-4 bullets cortos>

## Limitaciones de esta critica

- Recall ~50-80% — herramientas automaticas no cubren todo lo que cubre un experto humano.
- <si TYPE=html sin screenshot: agregar "Sin imagen — perdimos hallazgos visuales como microcopy en spacing, jerarquia tipografica real, animaciones.">
- <si TYPE=image: agregar "Sin HTML — perdimos hallazgos de structura semantica, ARIA, keyboard nav.">

---

_Critica generada con `/ux-critique`._
```

## Fase 4 — Confirmar y persistir

Default persistencia: **si** (las criticas se referencian para fixes).

```
Confirmás que creo el issue con label `ux:critique`? (si/no, default: si)
```

```bash
gh label create "ux:critique" --color "D73A4A" --description "UX critique findings" 2>/dev/null || true

BODY_FILE="$(mktemp -t ux-critique-body.XXXXXX).md"
gh issue create --title "<titulo>" --body-file "$BODY_FILE" --label "ux:critique"
```

Titulo formato: `Critica UX: <CONTEXT>`.

## Fase 5 — Reportar

```
## Result
- skill: /ux-critique
- target_type: <image | url | html>
- vision_used: <true | false>
- has_ai_features: <true | false>
- findings_total: <N>
- findings_blocker: <N>
- findings_major: <N>
- persisted: <true | false>
- url: <URL si persisted>
```

## MUST DO

- Cubrir las 7 dimensiones en orden.
- Asignar severity a cada hallazgo.
- Incluir un "Fix" accionable por hallazgo (no vaguedades).
- Declarar limitaciones de la critica (recall, falta de imagen/HTML).
- Aplicar heuristicas AI solo si `HAS_AI=true`.

## MUST NOT DO

- No prometer recall del 100% — declarar limit ~50-80%.
- No usar TYPE=url sin intentar al menos WebFetch del HTML (si no hay playwright, igual hay info).
- No omitir el "Top 5 fixes prioritarios" — el usuario lo necesita para actuar.
- No mezclar `ux:critique` con `ux:a11y` — accessibility es subset; `/ux-a11y-audit` cubre eso en profundidad.
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

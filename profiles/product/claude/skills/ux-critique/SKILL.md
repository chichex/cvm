**Critica de UX** sobre una pantalla (imagen, URL, o HTML pegado). Aplica Nielsen 10 + heuristicas de AI cuando corresponde, cubre 7 dimensiones, prioriza hallazgos por severidad. `$ARGUMENTS` es el target a criticar (puede venir vacio).

Skill **interactivo**.

## Pre-flight

### 1. Detectar tipo de target

Trim `$ARGUMENTS`. Guardar como `TARGET`.

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

### 2. Preguntar contexto
```
Que pantalla es? (1 linea — ej. "login para empresas", "checkout de e-commerce", "dashboard de metricas")
```
Guardar `CONTEXT`.

```
La pantalla incluye features de AI (chat, autocomplete con LLM, outputs generativos)? (si/no, default: no)
```
Guardar `HAS_AI`. Si si, se aplican heuristicas extra de AI.

### 3. Derivar slug

Derivar `<slug>` desde `CONTEXT` (kebab-case, max 40 chars). Se usa para guardar el archivo en `.ux/critique/<slug>.md`.

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
El HTML viene en `$ARGUMENTS` o como paste. Renderizarlo mentalmente; si tiene componentes complejos, recomendar guardarlo a archivo y correr con `TYPE=url file:///...`.

## Fase 2 — Evaluar por 7 dimensiones

Para cada dimension, generar hallazgos. Cada hallazgo tiene:
- **Severidad**: `urgente` | `importante` | `menor` | `nit`
- **Donde**: ubicacion en la pantalla (header, formulario, boton principal, etc.)
- **Problema**: que esta mal en 1-2 lineas
- **Fix**: accion concreta sugerida

### 1. Claridad (Nielsen #2, #6)
- El proposito de la pantalla es obvio en 5 segundos?
- Labels y CTAs usan lenguaje del usuario, no de la empresa?
- Hay items ambiguos o redundantes?

### 2. Jerarquia
- El item mas importante se distingue visualmente?
- La grilla / spacing respeta agrupacion logica?
- F-pattern / Z-pattern de lectura coherente con prioridad?

### 3. Affordance (Nielsen #6)
- Los elementos clickeables se ven clickeables?
- Estados de hover / focus presentes?
- Iconos sin label son interpretables solos?

### 4. Feedback (Nielsen #1)
- Hay loading states donde se esperan operaciones >300ms?
- Acciones destructivas tienen confirmacion o forma de deshacer?
- Errores se muestran cerca de la causa, no en banner global ambiguo?

### 5. Prevencion de errores (Nielsen #5)
- Formularios validan in-line, no solo al enviar?
- Acciones irreversibles (borrar, enviar, pagar) tienen friccion proporcional?
- Defaults seguros (ej. "guardar como borrador" vs "publicar")?

### 6. Accesibilidad (Nielsen #4 + WCAG 2.2 AA)
- Contraste ≥4.5:1 (texto normal), ≥3:1 (texto grande)?
- Focus visible en todos los interactivos?
- Touch target ≥24×24 CSS px (24 minimo, 44-48 recomendado mobile)?
- Heading hierarchy correcta?
- Labels en inputs (formales o `aria-label`)?
- Alt text en imagenes con contenido?

### 7. Percepcion de performance
- Hay skeletons / placeholders donde puede haber loading?
- Imagenes con `loading="lazy"` (si se ve en el HTML)?
- Animaciones respetan `prefers-reduced-motion`?

### Bonus — heuristicas de AI (solo si `HAS_AI=true`)

Aplicar adicionalmente:
- **Transparencia**: el usuario sabe que esta interactuando con AI? Sabe que puede y que no puede?
- **Control sobre delegacion**: puede el usuario corregir, deshacer, ajustar el output?
- **Manejo de incertidumbre**: outputs probabilisticos se muestran como tal (confianza, "sugerencia", "tal vez")?
- **Recuperacion**: si el AI se equivoca, hay camino claro de correccion?

## Fase 3 — Estructurar reporte

```markdown
## Critica de UX: <CONTEXT>

**Target**: <descripcion del target — path / URL / HTML>
**Vision usada**: <true (imagen + screenshots) | false (solo HTML)>
**Recall declarado**: ~50-80% — esta critica no es exhaustiva, prioriza hallazgos visibles.

## Resumen

- Total hallazgos: <N>
- Urgentes: <N> | Importantes: <N> | Menores: <N> | Nits: <N>

## Hallazgos por dimension

### 1. Claridad

- [<sev>] **<donde>** — <problema>
  - Fix: <accion concreta>
- [<sev>] **<donde>** — <problema>
  - Fix: <accion>

### 2. Jerarquia

(igual)

### 3. Affordance

(igual)

### 4. Feedback

(igual)

### 5. Prevencion de errores

(igual)

### 6. Accesibilidad (WCAG 2.2 AA)

(igual — incluir referencia al criterio especifico ej. "WCAG 1.4.3 Contrast")

### 7. Percepcion de performance

(igual)

<Solo si HAS_AI=true:>
### Bonus — heuristicas de AI

(igual)

## Top 5 fixes prioritarios

1. [<sev>] <donde>: <problema resumido>
2. ...

## Fortalezas

- <que esta bien — 2-4 bullets cortos>

## Limitaciones de esta critica

- Recall ~50-80% — herramientas automaticas no cubren todo lo que cubre un experto humano.
- <si TYPE=html sin screenshot: agregar "Sin imagen — perdimos hallazgos visuales como microcopy en spacing, jerarquia tipografica real, animaciones.">
- <si TYPE=image: agregar "Sin HTML — perdimos hallazgos de structura semantica, ARIA, navegacion por teclado.">

---

_Critica generada con `/ux-critique`._
```

## Fase 4 — Confirmar y guardar

```
Confirmás que guardo el output en .ux/critique/<slug>.md? (si/no, default: si)
```

Si si: si la carpeta `.ux/critique/` no existe, crearla con `mkdir -p .ux/critique/` antes de escribir. Luego usar `Write` tool para crear `.ux/critique/<slug>.md` con el reporte.

## Fase 5 — Reportar

```
## Result
- skill: /ux-critique
- target_type: <image | url | html>
- vision_used: <true | false>
- has_ai_features: <true | false>
- findings_total: <N>
- findings_urgent: <N>
- findings_important: <N>
- file: .ux/critique/<slug>.md
- saved: <true | false>
```

## MUST DO

- Cubrir las 7 dimensiones en orden.
- Asignar severidad a cada hallazgo.
- Incluir un "Fix" accionable por hallazgo (no vaguedades).
- Declarar limitaciones de la critica (recall, falta de imagen/HTML).
- Aplicar heuristicas de AI solo si `HAS_AI=true`.
- Guardar en `.ux/critique/<slug>.md` con `Write` tool.

## MUST NOT DO

- No prometer recall del 100% — declarar limite ~50-80%.
- No usar TYPE=url sin intentar al menos WebFetch del HTML (si no hay playwright, igual hay info).
- No omitir el "Top 5 fixes prioritarios" — el usuario lo necesita para actuar.
- No mezclar critica con auditoria de accesibilidad — accesibilidad es subset; `/ux-a11y-audit` cubre eso en profundidad.
- No usar `gh` ni depender de GitHub.
- No persistir nada en auto-memory.

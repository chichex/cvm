---
name: ux-copy-review
description: Revisa microcopy, labels, errores, empty states y tono en HTML, imagen o paste; propone rewrites y puede crear issue ux:copy.
---

Review de **microcopy y linguistic clarity**: button labels, error messages, empty states, helper text, tone consistency. Audita HTML, imagen, o paste directo. Devuelve hallazgos + rewrites sugeridos. Los argumentos del skill son el target y pueden venir vacios.

Skill **interactivo**.

## Pre-flight

### 1. Validar repo GitHub
```bash
gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null
```
Si falla, abortar.

### 2. Detectar tipo de target
Mismo flujo que `/ux-critique` / `/ux-a11y-audit`:
- Path HTML o directorio
- Path imagen (vision-only)
- URL (WebFetch + screenshot)
- Paste directo de copy (lista de strings)

### 3. Preguntar contexto
```
Que pantalla / producto es? (1 linea — afecta el tono esperado)
```
Guardar `CONTEXT`.

### 4. Preguntar tono target
```
Tono de voz deseado:
1) Sobrio / profesional (B2B SaaS)
2) Amigable / cercano (B2C, productivity)
3) Tecnico / preciso (devtools, infraestructura)
4) Juguetón / informal (consumer, comunidad)
5) Otra
```
Guardar `TONE`.

### 5. Preguntar idioma
```
Idioma del copy a revisar:
1) Español (default)
2) Ingles
3) Mixto (auditar consistencia idiomatica como issue propio)
```
Guardar `LANG`.

## Fase 1 — Extraer strings del target

Segun tipo:
- HTML: parsear y extraer texto de `<button>`, `<a>`, `<label>`, `<input placeholder>`, `<h*>`, `<p>` en areas interactivas, `aria-label`, `title`, mensajes de error inline, empty states (texto en areas vacias).
- Imagen: OCR mental de los textos visibles. Listarlos en orden de jerarquia visual.
- Paste: el usuario ya proveyo las strings.

Mostrar al usuario:
```
Encontre <N> strings a revisar. Procedo con la auditoria? (si/no)
```

## Fase 2 — Auditar cada string

Para cada string, evaluar:

### Clarity
- El significado es inmediato (5 segundos)?
- Tiene jerga interna o tecnicismos innecesarios?
- Ambiguedad (puede interpretarse de 2 maneras)?
- Negacion doble o lenguaje pasivo cuando activo sirve mejor?

### Brevity
- Numero de palabras razonable para el tipo de string (button label: 1-3, error: 1 oracion, empty state: 1-2 oraciones, tooltip: 1 frase)?
- Hay palabras decorativas removibles ("simplemente", "facilmente", "por favor" en exceso)?

### Tone consistency
- Encaja con `TONE` declarado?
- Hay strings que rompen el tono (ej. tono sobrio con un "¡Ups!" jocoso)?

### Action orientation (especialmente CTAs)
- Verbos en imperativo + objeto claro? (ej. "Crear cuenta" > "Cuenta nueva")
- Evitar "OK" / "Submit" / "Continue" cuando hay verbo especifico (mejor "Crear cuenta", "Guardar cambios").

### Error messages especificamente
- Le dice al usuario QUE fallo (no solo "Error")?
- Le dice CÓMO arreglarlo?
- No culpa al usuario ("Has hecho X mal" → "Esto necesita Y").
- No expone detalles internos (stack traces, codigos crudos).

### Empty states especificamente
- Explica POR QUE esta vacio?
- Sugiere COMO llenarlo (CTA explicito)?
- Tiene tono apropiado (no jocoso si el tono es sobrio).

### Accessibility / labels
- `aria-label` redundante con texto visible? (debe coincidir o el visible debe estar incluido en el label).
- Placeholders usados como label (anti-pattern)?

## Fase 3 — Generar rewrites

Por cada string con findings, proponer **1-3 rewrites** alternativos. No solo 1 — dar opciones al usuario para elegir.

Ej:
```
Original: "Submit"
Issues: [minor] generico, no dice que hace, viola action-orientation
Rewrites:
  a) "Crear cuenta"
  b) "Continuar al checkout"
  c) "Enviar mensaje"
```

## Fase 4 — Estructurar reporte

```markdown
## Copy review: <CONTEXT>

**Target**: <descripcion>
**Tone deseado**: <TONE>
**Idioma**: <LANG>
**Strings auditados**: <N>

## Resumen

| Categoria | Count |
|-----------|-------|
| Clarity issues | <N> |
| Brevity issues | <N> |
| Tone breaks | <N> |
| Action-orientation | <N> |
| Error message issues | <N> |
| Empty state issues | <N> |
| A11y label issues | <N> |
| Strings ok | <N> |

## Findings por string

### "<string original>"
- **Where**: <ubicacion>
- **Issues**:
  - [<sev>] <issue 1>
  - [<sev>] <issue 2>
- **Rewrites**:
  - a) "<rewrite 1>"
  - b) "<rewrite 2>"
  - c) "<rewrite 3>" (opcional)
- **Recomendado**: <a | b | c>

(repetir por cada string con findings)

## Strings ok (no requieren cambios)

- "<string 1>"
- "<string 2>"

## Patterns detectados

- <patron 1, ej. "tendencia a usar 'por favor' en CTAs" — minor>
- <patron 2, ej. "errores expone codigos internos consistentemente" — major>

## Top 5 cambios prioritarios

1. <change resumido>
2. ...

---

_Copy review generado por `/ux-copy-review`._
```

## Fase 5 — Confirmar y persistir

Default: **si**.

```
Confirmás que creo el issue con label `ux:copy`? (si/no, default: si)
```

```bash
gh label create "ux:copy" --color "FBCA04" --description "Copy / microcopy review" 2>/dev/null || true

BODY_FILE="$(mktemp -t ux-copy-body.XXXXXX).md"
gh issue create --title "Copy review: <CONTEXT>" --body-file "$BODY_FILE" --label "ux:copy"
```

## Fase 6 — Reportar

```
## Result
- skill: /ux-copy-review
- target_type: <html | image | url | paste>
- tone: <TONE>
- lang: <LANG>
- strings_audited: <N>
- findings_total: <N>
- strings_with_rewrites: <N>
- persisted: <true | false>
- url: <URL si persisted>
```

## MUST DO

- Extraer strings del target antes de auditar.
- Proponer 1-3 rewrites por string con findings (no 1 solo).
- Marcar pattern detectados (issues repetidos entre strings).
- Validar tone consistency en el conjunto.
- Para errores y empty states: aplicar checks especificos.

## MUST NOT DO

- No reescribir strings que no tienen issues (no introducir cambios cosmeticos).
- No proponer rewrites que rompan el `TONE` declarado.
- No omitir strings ok (mostrarlos como confirmacion).
- No mezclar `ux:copy` con `ux:critique` — el copy review es enfoque linguistico, mas profundo.
- No interpolar contenido en double-quoted shell commands.
- No persistir nada en auto-memory.

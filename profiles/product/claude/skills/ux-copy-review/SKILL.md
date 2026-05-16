Review de **microcopy y claridad linguistica**: button labels, mensajes de error, empty states, helper text, consistencia de tono. Audita HTML, imagen, o paste directo. Devuelve hallazgos + reescrituras sugeridas. `$ARGUMENTS` es el target (puede venir vacio).

Skill **interactivo**.

## Pre-flight

### 1. Detectar tipo de target
Mismo flujo que `/ux-critique` / `/ux-a11y-audit`:
- Path HTML o directorio
- Path imagen (vision-only)
- URL (WebFetch + screenshot)
- Paste directo de copy (lista de strings)

### 2. Preguntar contexto
```
Que pantalla / producto es? (1 linea — afecta el tono esperado)
```
Guardar `CONTEXT`.

### 3. Preguntar tono target
```
Tono deseado:
1) Sobrio / profesional (para empresas)
2) Amigable / cercano (consumidor final, productividad)
3) Tecnico / preciso (devtools, infraestructura)
4) Juguetón / informal (consumidor final, comunidad)
5) Otra
```
Guardar `TONE`.

### 4. Preguntar idioma
```
Idioma del copy a revisar:
1) Español (default)
2) Ingles
3) Mixto (auditar consistencia idiomatica como hallazgo aparte)
```
Guardar `LANG`.

### 5. Derivar slug

Derivar `<slug>` desde `CONTEXT` (kebab-case, max 40 chars). Se usa para guardar en `.ux/copy/<slug>.md`.

## Fase 1 — Extraer strings del target

Segun tipo:
- HTML: parsear y extraer texto de `<button>`, `<a>`, `<label>`, `<input placeholder>`, `<h*>`, `<p>` en areas interactivas, `aria-label`, `title`, mensajes de error inline, empty states (texto en areas vacias).
- Imagen: leer los textos visibles. Listarlos en orden de jerarquia visual.
- Paste: el usuario ya proveyo las strings.

Mostrar al usuario:
```
Encontre <N> strings a revisar. Procedo con la auditoria? (si/no)
```

## Fase 2 — Auditar cada string

Para cada string, evaluar:

### Claridad
- El significado es inmediato (5 segundos)?
- Tiene jerga interna o tecnicismos innecesarios?
- Ambiguedad (puede interpretarse de 2 maneras)?
- Negacion doble o lenguaje pasivo cuando activo sirve mejor?

### Brevedad
- Numero de palabras razonable para el tipo de string (button label: 1-3, error: 1 oracion, empty state: 1-2 oraciones, tooltip: 1 frase)?
- Hay palabras decorativas removibles ("simplemente", "facilmente", "por favor" en exceso)?

### Consistencia de tono
- Encaja con `TONE` declarado?
- Hay strings que rompen el tono (ej. tono sobrio con un "¡Ups!" jocoso)?

### Orientacion a la accion (especialmente botones principales / CTAs)
- Verbos en imperativo + objeto claro? (ej. "Crear cuenta" > "Cuenta nueva")
- Evitar "OK" / "Submit" / "Continue" cuando hay verbo especifico (mejor "Crear cuenta", "Guardar cambios").

### Mensajes de error especificamente
- Le dice al usuario QUE fallo (no solo "Error")?
- Le dice CÓMO arreglarlo?
- No culpa al usuario ("Has hecho X mal" → "Esto necesita Y").
- No expone detalles internos (stack traces, codigos crudos).

### Empty states especificamente
- Explica POR QUE esta vacio?
- Sugiere COMO llenarlo (boton principal explicito)?
- Tiene tono apropiado (no jocoso si el tono es sobrio).

### Accesibilidad / labels
- `aria-label` redundante con texto visible? (debe coincidir o el visible debe estar incluido en el label).
- Placeholders usados como label (anti-pattern)?

## Fase 3 — Generar reescrituras

Por cada string con hallazgos, proponer **1-3 reescrituras** alternativas. No solo 1 — dar opciones al usuario para elegir.

Ej:
```
Original: "Submit"
Hallazgos: [menor] generico, no dice que hace, viola orientacion a la accion
Reescrituras:
  a) "Crear cuenta"
  b) "Continuar al checkout"
  c) "Enviar mensaje"
```

## Fase 4 — Estructurar reporte

```markdown
## Copy review: <CONTEXT>

**Target**: <descripcion>
**Tono deseado**: <TONE>
**Idioma**: <LANG>
**Strings auditados**: <N>

## Resumen

| Categoria | Count |
|-----------|-------|
| Problemas de claridad | <N> |
| Problemas de brevedad | <N> |
| Quiebres de tono | <N> |
| Orientacion a la accion | <N> |
| Problemas en mensajes de error | <N> |
| Problemas en empty states | <N> |
| Problemas de labels accesibles | <N> |
| Strings ok | <N> |

## Hallazgos por string

### "<string original>"
- **Donde**: <ubicacion>
- **Hallazgos**:
  - [<sev>] <problema 1>
  - [<sev>] <problema 2>
- **Reescrituras**:
  - a) "<reescritura 1>"
  - b) "<reescritura 2>"
  - c) "<reescritura 3>" (opcional)
- **Recomendado**: <a | b | c>

(repetir por cada string con hallazgos)

## Strings ok (no requieren cambios)

- "<string 1>"
- "<string 2>"

## Patrones detectados

- <patron 1, ej. "tendencia a usar 'por favor' en botones principales" — menor>
- <patron 2, ej. "errores expone codigos internos consistentemente" — importante>

## Top 5 cambios prioritarios

1. <cambio resumido>
2. ...

---

_Copy review generado por `/ux-copy-review`._
```

## Fase 5 — Confirmar y guardar

```
Confirmás que guardo el output en .ux/copy/<slug>.md? (si/no, default: si)
```

Si si: si la carpeta `.ux/copy/` no existe, crearla con `mkdir -p .ux/copy/` antes de escribir. Luego usar `Write` tool para crear `.ux/copy/<slug>.md` con el reporte.

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
- file: .ux/copy/<slug>.md
- saved: <true | false>
```

## MUST DO

- Extraer strings del target antes de auditar.
- Proponer 1-3 reescrituras por string con hallazgos (no 1 sola).
- Marcar patrones detectados (problemas repetidos entre strings).
- Validar consistencia de tono en el conjunto.
- Para errores y empty states: aplicar checks especificos.
- Guardar en `.ux/copy/<slug>.md` con `Write` tool.

## MUST NOT DO

- No reescribir strings que no tienen problemas (no introducir cambios cosmeticos).
- No proponer reescrituras que rompan el `TONE` declarado.
- No omitir strings ok (mostrarlos como confirmacion).
- No mezclar copy review con `/ux-critique` — el copy review es enfoque linguistico, mas profundo.
- No usar `gh` ni depender de GitHub.
- No persistir nada en auto-memory.

# HTML-REPORT — scaffold del reporte de `/arch-review`

Guia concreta para escribir el HTML en `$TMPDIR/arch-review-<timestamp>.html`. Self-contained: solo CDNs externos (Tailwind, Mermaid), nada de assets locales.

## Estructura

```
<header>
  Title + timestamp + scope (TARGET) + contadores (N candidatos, X Strong, Y Worth exploring, Z Speculative)
<top-recommendation>
  Card destacada con el candidato top y el "por que primero"
<candidates>
  Una card por candidato, en orden de fuerza descendente
<footer>
  Glosario rapido (link al LANGUAGE.md mental) + disclaimer si CONTEXT.md / ADRs faltaban
```

## Scaffold base

```html
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>arch-review — {{TARGET}} — {{TS}}</title>
<script src="https://cdn.tailwindcss.com"></script>
<script type="module">
  import mermaid from "https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs";
  mermaid.initialize({ startOnLoad: true, theme: "neutral" });
</script>
<style>
  .badge-strong   { @apply bg-emerald-100 text-emerald-800 border-emerald-300; }
  .badge-worth    { @apply bg-amber-100   text-amber-800   border-amber-300; }
  .badge-spec     { @apply bg-slate-100   text-slate-700   border-slate-300; }
  .callout-warn   { @apply bg-rose-50 border-l-4 border-rose-400 p-3 text-rose-900; }
  .deletion-test  { @apply bg-indigo-50 border border-indigo-200 rounded p-3 text-sm text-indigo-900; }
</style>
</head>
<body class="bg-slate-50 text-slate-900 font-sans">
<main class="max-w-5xl mx-auto p-8 space-y-10">

  <!-- HEADER -->
  <header class="border-b pb-6">
    <h1 class="text-3xl font-bold">arch-review</h1>
    <p class="text-slate-600 mt-1">scope: <code class="bg-slate-200 px-1 rounded">{{TARGET}}</code> · generated {{TS}}</p>
    <div class="mt-4 flex gap-3 text-sm">
      <span class="px-2 py-1 rounded badge-strong border">{{N_STRONG}} Strong</span>
      <span class="px-2 py-1 rounded badge-worth border">{{N_WORTH}} Worth exploring</span>
      <span class="px-2 py-1 rounded badge-spec border">{{N_SPEC}} Speculative</span>
    </div>
    {{#DISCLAIMER}}
    <div class="callout-warn mt-4">
      Faltaba <strong>{{MISSING_DOCS}}</strong> al correr este review. Nombres y decisiones reflejan solo lo derivable del codigo.
    </div>
    {{/DISCLAIMER}}
  </header>

  <!-- TOP RECOMMENDATION -->
  <section class="bg-white rounded-xl shadow-md p-6 border-l-4 border-emerald-500">
    <h2 class="text-xs uppercase tracking-wider text-emerald-700 font-semibold">Top recommendation</h2>
    <h3 class="text-2xl font-semibold mt-2">{{TOP_TITLE}}</h3>
    <p class="text-slate-700 mt-2">{{TOP_WHY_FIRST}}</p>
  </section>

  <!-- CANDIDATES -->
  <section class="space-y-8">
    <h2 class="text-xl font-semibold text-slate-700">Candidates</h2>
    {{#CANDIDATES}}
    {{> candidate_card }}
    {{/CANDIDATES}}
  </section>

  <!-- FOOTER -->
  <footer class="text-xs text-slate-500 pt-6 border-t">
    Glossary: Module · Interface · Implementation · Depth · Seam · Adapter · Leverage · Locality · Deletion test.
    Drift terminology and the analysis loses meaning — see <code>LANGUAGE.md</code> in the skill.
  </footer>

</main>
</body>
</html>
```

## Plantilla de candidate card

```html
<article class="bg-white rounded-xl shadow p-6 space-y-4">
  <header class="flex items-start justify-between">
    <h3 class="text-xl font-semibold">{{CANDIDATE_TITLE}}</h3>
    <span class="px-2 py-1 text-xs rounded border badge-{{STRENGTH_CLASS}}">{{STRENGTH_LABEL}}</span>
  </header>

  <!-- Files -->
  <div>
    <h4 class="text-xs uppercase text-slate-500 tracking-wider">Files</h4>
    <ul class="mt-1 font-mono text-sm text-slate-700 list-disc list-inside">
      {{#FILES}}<li>{{.}}</li>{{/FILES}}
    </ul>
  </div>

  <!-- Problem -->
  <div>
    <h4 class="text-xs uppercase text-slate-500 tracking-wider">Problem</h4>
    <p class="mt-1 text-slate-800">{{PROBLEM}}</p>
  </div>

  <!-- Solution -->
  <div>
    <h4 class="text-xs uppercase text-slate-500 tracking-wider">Solution</h4>
    <p class="mt-1 text-slate-800">{{SOLUTION}}</p>
  </div>

  <!-- Benefits -->
  <div>
    <h4 class="text-xs uppercase text-slate-500 tracking-wider">Benefits</h4>
    <ul class="mt-1 text-slate-800 list-disc list-inside space-y-1">
      <li><strong>Leverage:</strong> {{LEVERAGE}}</li>
      <li><strong>Locality:</strong> {{LOCALITY}}</li>
      <li><strong>Tests:</strong> {{TESTS_IMPACT}}</li>
    </ul>
  </div>

  <!-- Deletion test -->
  <div class="deletion-test">
    <strong>Deletion test:</strong> {{DELETION_TEST_OUTCOME}}
  </div>

  <!-- Before / After -->
  <div class="grid grid-cols-2 gap-4">
    <div>
      <h4 class="text-xs uppercase text-slate-500 tracking-wider">Before (shallow)</h4>
      <div class="mt-2 border rounded p-2 bg-slate-50">
        {{BEFORE_DIAGRAM}}
      </div>
    </div>
    <div>
      <h4 class="text-xs uppercase text-slate-500 tracking-wider">After (deepened)</h4>
      <div class="mt-2 border rounded p-2 bg-slate-50">
        {{AFTER_DIAGRAM}}
      </div>
    </div>
  </div>

  {{#ADR_CONFLICT}}
  <div class="callout-warn">
    <strong>Contradicts {{ADR_ID}}</strong> — worth reopening because: {{ADR_REOPEN_REASON}}
  </div>
  {{/ADR_CONFLICT}}
</article>
```

## Patrones de diagramas

Usar **Mermaid** cuando la relacion es graph-shaped (call graph, dependency, sequence). Usar **CSS/SVG hand-crafted** cuando el grafico es editorial (mostrar masa, colapso, cross-section).

### Call graph (Mermaid)

```
<pre class="mermaid">
graph LR
  A[Caller A] --> S1[Shallow1]
  B[Caller B] --> S1
  C[Caller C] --> S2[Shallow2]
  S1 --> S3[Shallow3]
  S2 --> S3
  S3 --> Real[Real work]
</pre>
```

versus

```
<pre class="mermaid">
graph LR
  A[Caller A] --> D[Deep Module]
  B[Caller B] --> D
  C[Caller C] --> D
</pre>
```

### Mass diagram (hand-crafted)

Para mostrar visualmente "interfaz grande / impl chica" vs "interfaz chica / impl grande":

```html
<div class="flex flex-col items-center">
  <div class="w-48 h-4 bg-slate-400 rounded-t" title="Interface"></div>
  <div class="w-32 h-8 bg-slate-300 rounded-b" title="Implementation"></div>
  <span class="text-xs text-slate-500 mt-1">Shallow: interface wider than impl</span>
</div>
```

```html
<div class="flex flex-col items-center">
  <div class="w-16 h-4 bg-emerald-500 rounded-t" title="Interface"></div>
  <div class="w-48 h-16 bg-emerald-300 rounded-b" title="Implementation"></div>
  <span class="text-xs text-slate-500 mt-1">Deep: thin interface, heavy impl</span>
</div>
```

### Seam diagram (mix)

Mostrar adapters detras de una interfaz:

```
<pre class="mermaid">
graph TB
  I[[Interface: Storage]]
  I --- A1[Adapter: Postgres]
  I --- A2[Adapter: InMemory]
  I --- A3[Adapter: S3]
</pre>
```

## Strength badges

| Label | Clase | Cuando usar |
|---|---|---|
| `Strong` | `badge-strong` | Deletion test claro, dos+ callers afectados, beneficio inmediato en tests. |
| `Worth exploring` | `badge-worth` | Friccion real pero requiere conversacion de diseño. La forma del module deep todavia no es obvia. |
| `Speculative` | `badge-spec` | Sospecha basada en sintomas, pero faltan datos. Util como señal, no como propuesta. |

Si no hay candidato `Strong`, decirlo en la seccion Top recommendation. **No inflar** un `Worth exploring` a `Strong` para tener algo top.

## Reglas de contenido

- **Vocabulario**: Module/Interface/Depth/Seam/Adapter/Leverage/Locality textuales. Sin sinonimos.
- **Naming**: si existe `CONTEXT.md`, usar sus terminos para nombrar Modules. Si no, usar el nombre del archivo/paquete real, no inventar etiquetas.
- **Codigo**: snippets cortos (max ~10 lineas) dentro de `<pre><code>`. Path:line cuando se cita.
- **ADR conflicts**: solo incluir el callout cuando la friccion es real. No listar refactors teoricos prohibidos.
- **No proponer interfaces**: el reporte describe la oportunidad, no la API resultante. La API es trabajo del grilling loop (skill separado).

## Anti-patrones del reporte

- Inflar candidatos `Worth exploring` a `Strong` para llenar la seccion top.
- Listar 20 candidatos chiquitos en vez de 5 con peso real.
- Diagramas decorativos sin contenido (un grafico que no cambia entre before/after no aporta).
- Usar "service", "component", "boundary" en vez del vocabulario fijo.
- Proponer la interfaz exacta del module deep — eso es scope del grilling, no del scouting.

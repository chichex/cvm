Iterar sobre una propuesta de UX. Valida con Opus y Gemini, y genera un HTML con las alternativas. $ARGUMENTS es lo que el usuario quiere mejorar o cambiar de la UX.

## Proceso

### Paso 1: Entender el pedido

Analizar $ARGUMENTS para determinar:
- Que componente o flujo de UX se quiere iterar
- Que problema tiene la version actual (si existe)
- Que restricciones hay (mobile, accesibilidad, performance, etc.)
- Archivos relevantes del proyecto (buscar componentes, templates, CSS)

Si no se encuentran archivos de frontend relevantes, informar al usuario y preguntar si quiere continuar con una propuesta from-scratch.

### Paso 2: Verificar disponibilidad de Gemini

Verificar en este orden:
1. Leer `~/.cvm/available-tools.json` y verificar `gemini.available == true`
2. Si el archivo no existe o no es parseable, fallback: `which gemini 2>/dev/null`
3. Si ambos fallan: Gemini no disponible, continuar solo con Opus

### Paso 3: Lanzar validacion multi en paralelo

Lanzar Opus y Gemini (si disponible) al mismo tiempo, cada uno con un prompt que incluya:

**Opus** (via Agent, model: opus):
> TASK: Analizar esta propuesta de UX y generar 2-3 alternativas concretas.
> CONTEXTO: [descripcion del componente/flujo, paths a archivos relevantes]
> PEDIDO DEL USUARIO: [lo que quiere cambiar]
> RESTRICCIONES: [las que apliquen]
> OUTPUT: Para cada alternativa, describir: (1) que cambia, (2) por que es mejor, (3) trade-offs. Incluir el HTML/CSS necesario para cada alternativa como bloques de codigo completos y funcionales.
> Termina tu respuesta con una seccion `## Key Learnings:` listando descubrimientos no-obvios.

**Gemini** (via `gemini -p`, si disponible):
> Mismo prompt adaptado. Darle paths a archivos para que los lea.
> Especificar formato de output: "Output ONLY raw HTML/CSS blocks. Each alternative starts with `<!-- ALT: name -->` and ends with `<!-- /ALT -->`."

Escribir el prompt de Gemini con Write tool en `/tmp/cvm-ux-gemini-prompt.txt`. Lanzar como Bash tool call separado.

### Paso 4: Consolidar alternativas

Recopilar las alternativas de Opus y Gemini. Eliminar duplicadas. Asignar nombres descriptivos a cada una.

Si ninguna alternativa es coherente con el pedido original, reportar al usuario en vez de generar un HTML vacio.

### Paso 5: Generar HTML de comparacion

Crear un archivo HTML con Write tool en `/tmp/cvm-ux-alternatives.html` que:
- Muestre todas las alternativas side-by-side o en tabs
- Incluya el CSS inline (no dependencias externas)
- Sea self-contained y se pueda abrir en cualquier browser
- Tenga un titulo con el contexto del pedido
- Marque de quien viene cada alternativa (Opus / Gemini)

### Paso 6: Abrir en browser

```bash
open /tmp/cvm-ux-alternatives.html
```

### Paso 7: Reportar

Listar las alternativas generadas con un resumen de una linea cada una. Indicar que el HTML esta abierto en el browser.

## MUST DO
- Verificar disponibilidad de Gemini antes de lanzar
- Lanzar Opus y Gemini en paralelo (si Gemini disponible)
- Incluir instruccion de `## Key Learnings:` en prompt de Opus
- Escribir prompts de Gemini con Write tool, no interpolacion en shell
- Generar HTML self-contained sin dependencias externas
- Usar `/tmp/cvm-ux-alternatives.html` (con prefijo cvm-)
- Incluir previews funcionales de cada alternativa en el HTML
- Abrir automaticamente en el browser

## MUST NOT DO
- No lanzar Gemini sin verificar disponibilidad
- No pedir input antes de lanzar los agentes — usar el contexto disponible
- No generar alternativas genericas sin mirar el codigo actual
- No depender de CDNs o recursos externos en el HTML
- No interpolar texto del usuario en shell commands

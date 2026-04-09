Analizar screenshots de UI/UX y generar un archivo HTML con propuestas de mejora visual. $ARGUMENTS puede ser:
- Paths a imagenes (screenshots, mockups, capturas)
- Un glob pattern para multiples imagenes (ej: `./screens/*.png`)
- Opcionalmente, instrucciones adicionales como "podes cambiar los colores" o contexto del producto

## Paso 1: Ingerir input

**1a. Leer las imagenes:**
- Usar el tool Read para cada imagen proporcionada en $ARGUMENTS
- Si es un glob pattern, resolver con Glob primero y luego leer cada imagen
- Si no se proporcionaron imagenes, pedir al usuario que pase al menos una

**1b. Recopilar contexto explicito:**
Si el usuario proporciono contexto en $ARGUMENTS (tipo de producto, usuario target, objetivo), registrarlo. NO preguntar todavia — las preguntas se consolidan en el Paso 2.

## Paso 2: Entender la app

Antes de analizar UX, entender de que se trata la aplicacion. Esto es obligatorio.

**2a. Explorar el codigo:**
- Leer el README, package.json, o equivalente para entender el stack y proposito del proyecto
- Explorar las rutas/paginas principales (ej: `src/app/`, `src/pages/`, `routes/`) para entender el flujo de la app
- Identificar los componentes visibles en las capturas buscandolos en el codigo fuente
- Determinar: que hace esta app? quien la usa? cual es el flujo principal?

**2b. Consolidar entendimiento:**
Armar un resumen interno de:
1. **Proposito de la app**: que problema resuelve
2. **Usuario target**: quien la usa (inferido del codigo, copy, y contexto)
3. **Flujo principal**: que camino sigue el usuario para lograr su objetivo
4. **Pantalla analizada**: en que parte del flujo cae cada screenshot

**2c. Resolver ambiguedades:**
Si despues de explorar el codigo y las imagenes quedan dudas sobre:
- El proposito de la app o la pantalla
- Quien es el usuario target
- Que objetivo tiene la pantalla (convertir? informar? onboardear?)
- Si ciertos elementos son placeholders, bugs, o decisiones intencionales
- Si la captura no coincide con el codigo encontrado (layout distinto, componentes que no existen en el repo, pantalla que no se puede mapear a ninguna ruta/vista)
- Cualquier otra ambiguedad que afecte las propuestas de diseno

-> **PARAR y preguntar al usuario antes de continuar.** Consolidar todas las dudas en una sola pregunta clara. No avanzar al analisis UX con suposiciones no validadas.

Si las capturas no se pueden mapear a ningun codigo del proyecto (no hay componentes, rutas, ni vistas que coincidan), informar al usuario explicitamente: "No encontre codigo que corresponda a estas capturas" y preguntar si es un diseno nuevo, otro repo, o un estado futuro de la app. No continuar con suposiciones.

Si todo esta claro, comunicar el entendimiento al usuario en 3-4 lineas y pedir confirmacion rapida antes de seguir.

## Paso 3: Analisis UX/UI

Analizar cada imagen desde DOS perspectivas simultaneas:

### Perspectiva 1 — Disenador UX Senior
Evaluar:
- **Jerarquia visual**: El ojo sabe donde ir primero? Los CTAs son claros?
- **Espaciado y ritmo**: Hay consistencia en paddings/margins? El layout respira?
- **Tipografia**: Los tamanos establecen jerarquia clara? Hay demasiadas variantes?
- **Contraste y legibilidad**: El texto es legible sobre su fondo? Cumple WCAG AA?
- **Consistencia de componentes**: Botones, inputs, cards siguen un sistema coherente?
- **Feedback visual**: El usuario sabe que es clickeable, que esta activo, que esta deshabilitado?
- **Densidad de informacion**: Hay sobrecarga cognitiva? Se puede simplificar?
- **Navegacion y flow**: El usuario sabe donde esta y a donde puede ir?

### Perspectiva 2 — Usuario Target
Simular la experiencia del usuario identificado en Paso 2:
- Que haria primero al ver esta pantalla?
- Que le generaria confusion?
- Que le daria confianza o desconfianza?
- Donde se perderia?

## Paso 4: Extraer paleta y tokens de diseno

De cada imagen, identificar y documentar:
- **Colores principales**: backgrounds, texto, acentos, bordes (en hex)
- **Tipografia aparente**: sans-serif/serif, pesos visibles
- **Border radius**: sharp, levemente redondeado, pill
- **Sombras**: flat, sutil, elevado
- **Espaciado aproximado**: compacto, normal, aireado

Estos tokens se DEBEN respetar en las propuestas a menos que el usuario haya especificado explicitamente "podes cambiar los colores" o similar en $ARGUMENTS.

## Paso 5: Generar propuestas

Generar tantas propuestas como el analisis justifique. No hay minimo ni maximo fijo — la cantidad depende de cuantos problemas reales se encontraron y cuantos enfoques distintos de mejora tienen sentido. Si hay un solo issue claro, una propuesta alcanza. Si hay multiples ejes de mejora independientes, generar una propuesta por cada eje.

Cada propuesta debe tener un enfoque diferenciado. Ejemplos de enfoques posibles (usar los que apliquen, no forzar todos):
- **Refinamiento conservador**: misma estructura, mejoras sutiles de spacing, alineacion, contraste
- **Reorganizacion de jerarquia**: reordenar elementos para mejorar el flow visual
- **Modernizacion**: aplicar tendencias actuales (glass morphism, bento grid, micro-interacciones CSS)
- **Accesibilidad first**: foco en contraste, tamanos de click targets, labels, focus states
- **Simplificacion**: reducir densidad de informacion, eliminar ruido visual
- **Mobile-first**: optimizar la experiencia para pantallas chicas

Cada propuesta DEBE incluir:
1. Nombre descriptivo de la propuesta
2. Que problema resuelve y por que (razonamiento)
3. Lista de cambios especificos respecto al original
4. El HTML/CSS implementado

## Paso 6: Generar el archivo HTML de output

Crear un archivo HTML self-contained en el directorio actual con nombre `ux-proposals-[timestamp].html` que contenga:

**Estructura del HTML:**
```html
<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>UX Proposals — [nombre del producto o pantalla]</title>
  <style>
    /* Reset y estilos base del viewer */
    /* Navegacion entre propuestas con tabs */
    /* Cada propuesta en su propia seccion */
    /* Panel lateral con notas de cambios */
    /* View transitions para cambiar entre propuestas */
  </style>
</head>
<body>
  <header>
    <h1>Propuestas UX — [pantalla]</h1>
    <p>Generado: [fecha]</p>
    <nav><!-- tabs de propuestas --></nav>
  </header>
  <main>
    <section class="proposal" id="original">
      <!-- Descripcion del analisis del estado actual -->
    </section>
    <section class="proposal" id="proposal-a">
      <div class="proposal-preview"><!-- HTML/CSS de la propuesta --></div>
      <aside class="proposal-notes">
        <h3>Cambios</h3>
        <ul><!-- lista de cambios con razon --></ul>
      </aside>
    </section>
    <!-- mas propuestas -->
  </main>
</body>
</html>
```

**Requisitos del HTML:**
- Self-contained: todo el CSS inline en `<style>`, sin dependencias externas
- Responsive: el viewer debe funcionar en desktop y mobile
- Navegacion por tabs entre propuestas con view transitions suaves:
  ```css
  ::view-transition-group(*),
  ::view-transition-old(*),
  ::view-transition-new(*) {
    animation-duration: 0.25s;
    animation-timing-function: cubic-bezier(0.19, 1, 0.22, 1);
  }
  ```
- Cada propuesta muestra el mockup al lado de las notas de cambios
- Los tokens de diseno extraidos como CSS custom properties al inicio

## Paso 7: Presentar al usuario

Mostrar un resumen con:
1. Hallazgos principales del analisis (top 3-5 issues)
2. Resumen de cada propuesta en una linea
3. Path al archivo HTML generado
4. Instruccion: "Abri el HTML en el browser para ver las propuestas interactivas"

## MUST DO
- Explorar el codigo fuente ANTES de analizar UX — entender la app es prerequisito
- Preguntar ante cualquier ambiguedad sobre proposito, usuario, o contexto — no asumir
- Respetar la paleta de colores original a menos que el usuario lo autorice explicitamente
- Incluir razonamiento ("por que") en cada cambio propuesto
- Generar HTML valido y self-contained que se pueda abrir directamente en un browser
- Usar CSS moderno (custom properties, grid, flexbox, view transitions)
- Mantener la identidad visual del producto en todas las propuestas

## MUST NOT DO
- NO disenar sin haber entendido el contexto de la app primero
- NO asumir el proposito de una pantalla si hay ambiguedad — preguntar
- NO asumir que una captura corresponde al codigo si no hay coincidencia clara — reportar la discrepancia
- NO cambiar colores sin autorizacion explicita del usuario
- NO usar frameworks CSS externos (Tailwind, Bootstrap) — solo CSS vanilla
- NO usar JavaScript salvo para la navegacion entre tabs del viewer
- NO agregar funcionalidad que no existia en el original (solo mejoras visuales/UX)
- NO generar imagenes — solo HTML/CSS que represente los componentes
- NO inventar contenido/copy — usar el texto visible en las capturas
- NO hacer el analisis sin haber leido las imagenes primero

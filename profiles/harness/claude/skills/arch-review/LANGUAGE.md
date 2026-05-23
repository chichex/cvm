# LANGUAGE — vocabulario fijo para `/arch-review`

Estos terminos se usan **textualmente** en todo el reporte y en cualquier conversacion follow-up. El punto del skill es lenguaje compartido — driftear a sinonimos disuelve el valor del analisis.

Fuente original: John Ousterhout, *A Philosophy of Software Design* (2da edicion). Aca van en ingles porque traducirlos pierde precision (`Seam` no es "costura", `Depth` no es "profundidad" en el sentido coloquial).

## Conceptos primarios

### Module

Cualquier unidad con **interfaz** e **implementacion**. Granularidad libre: una funcion, una clase, un paquete, un slice horizontal del sistema. No importa el nivel — importa que tenga un contrato hacia afuera y un cuerpo adentro.

### Interface

**Todo** lo que un caller necesita saber para usar el Module correctamente. No es solo la firma de la funcion:

- Tipos de input/output.
- **Invariantes** (precondiciones, postcondiciones).
- **Error modes** (que puede fallar y como).
- **Ordering** (que tiene que pasar antes / despues).
- **Config** (flags, env vars, defaults).
- Side effects observables.

Si el caller tiene que abrir la implementacion para entender algo de la lista, ese algo es interfaz no documentada — y cuenta para medir la complejidad de la interfaz.

### Implementation

El codigo de adentro. Lo que un caller **no** deberia necesitar leer.

### Depth

La relacion entre la complejidad de la **Interface** y la cantidad de **conducta** que entrega.

- **Deep module**: interfaz chica, mucha conducta detras. Alto leverage. Ejemplo: `malloc(size)` — 1 parametro, gestion completa de memoria adentro.
- **Shallow module**: interfaz casi tan compleja como la implementacion. Cada parametro/error/quirk de la implementacion leakea al caller. Bajo leverage.

Depth se mide cualitativamente — no hay metrica numerica honesta. La heuristica practica es el deletion test.

### Seam

El lugar fisico donde vive una **Interface** — el punto en el codigo donde se puede sustituir el comportamiento sin editar in place. Usar este termino, no "boundary".

Ejemplos: una interfaz Go, un constructor que recibe una dependencia, una factory function, una capa HTTP entre dos servicios.

### Adapter

Algo concreto que satisface una **Interface** en un **Seam**. Una implementacion especifica.

### Leverage

Lo que los **callers** sacan de la depth: hacen mucho con poco. Mucho leverage = la interfaz les ahorra trabajo cognitivo.

### Locality

Lo que los **mantenedores** sacan de la depth: cambios, bugs y conocimiento concentrados en un solo lugar. Mucha locality = un fix toca un archivo, no veinte.

Locality y Leverage son las **dos caras** de un module deep. Si solo hay leverage (callers contentos) pero el codigo esta sparseado, el module no es deep — es un facade fino sobre un mess.

## Tests operacionales

### The deletion test

Para decidir si un module shallow esta ganandose el lugar: imaginar borrarlo.

- Si la complejidad **desaparece** o se simplifica → era un pass-through. Borrarlo gana claridad.
- Si la complejidad **reaparece duplicada en N callers** → estaba ganandose el lugar; merece mas profundidad, no menos.
- Si solo se **mueve** a otro modulo sin reducirse → reorganizacion, no deepening.

Solo los casos del medio (complejidad que reaparece concentrada en N llamadas) son candidatos validos para el reporte.

### The interface is the test surface

Si un module tiene tests que tocan su implementacion (mocks de internals, reflection, hooks privados), la interfaz es shallow — la implementacion esta leakeando. Tests que pasan solo por la interfaz publica son la evidencia de que el seam es honesto.

### One adapter = hypothetical seam. Two adapters = real seam

Una interfaz con una sola implementacion (`FooImpl` detras de `Foo`) **no** es un seam — es ceremony. El seam aparece cuando hay al menos dos adapters reales (prod + test stub cuenta, prod + alternativa real cuenta mas).

Corolario: extraer interfaces "por si acaso" agrega ruido sin agregar seam. La interfaz nace cuando aparece el segundo adapter.

## Anti-vocabulario

Palabras prohibidas en el reporte porque pierden precision o se solapan ambiguamente:

| No usar | Usar en su lugar |
|---|---|
| Component | Module |
| Service | Module |
| API | Interface |
| Boundary | Seam |
| Wrapper | Adapter (si satisface una Interface en un Seam) |
| Layer | Si es un Seam, Seam; si es un Module compuesto, Module |
| Pure function (como argumento de calidad) | Deep module — pureza sola no implica depth |

Si el dominio del proyecto (en `CONTEXT.md`) usa "service" o "component" con un significado especifico, usar ese termino **solo para nombrar la cosa concreta**, no como sinonimo de Module.

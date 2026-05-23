---
name: zoom-out
description: Pide al orquestador subir una capa de abstraccion sobre un area de codigo; devuelve mapa de modulo, callers, dependencias, contexto arquitectonico y proximo paso sugerido. Usar cuando entras a codigo desconocido o querer entender como una pieza encaja en el sistema mayor.
---

Pide al orquestador que zoomee hacia arriba sobre un area de codigo: mapa de modulos relevantes, callers (entradas), dependencias (salidas) y rol en el sistema mayor, usando el vocabulario del proyecto. Usar cuando entras a un area de codigo desconocida o necesitas entender como una pieza encaja en el todo antes de modificar algo.

Skill **on-demand sin fases**: una sola pasada de lectura y resumen estructurado. No modifica nada, no persiste mas alla del reporte en chat.

## Argumentos

```text
/zoom-out [<archivo, directorio, simbolo o area en prosa>]
```

- Si los argumentos estan vacios, pedir al usuario sobre que area quiere el zoom-out.
- El input puede ser un path (`internal/http/`), un simbolo (`ParseHandler`), o un area en prosa ("el flujo de auth").

## Ejecutar

### 1. Identificar el target

- Path: listar el directorio o leer signatures del archivo (no el body completo).
- Simbolo: `grep -E -rn` para ubicar la definicion principal.
- Area en prosa: derivar 1 a 2 paths probables y arrancar por ahi.

Si el target no se encuentra, pedir clarificacion al usuario antes de seguir.

### 2. Subir una capa de abstraccion

Para el target identificado, contestar:

- **Modulo / paquete**: que dir contiene el target y como se llama el modulo logico (no el path).
- **Proposito** (1 linea): que hace dentro del sistema mayor.
- **Callers** (entradas): quien lo usa. Ubicar con `grep -E -rn` por imports o por el nombre del simbolo.
- **Dependencias** (salidas): que paquetes/funciones externas usa. Mirar imports propios del target.
- **Decisiones / contexto relevantes**: ADRs (`docs/adr/`, `ARCHITECTURE.md`, `DECISIONS.md` si existen), comentarios marcadores (`// HACK`, `// NOTE:`), o convenciones del repo que afecten el area.

### 3. Usar vocabulario del repo

Si el repo tiene un glosario de dominio (`CONTEXT.md`, `docs/glossary.md`, `GLOSSARY.md`, o equivalente), leerlo y usar ese vocabulario en el resumen. Si no hay, derivar el vocabulario observando nombres de tipos/funciones recurrentes.

## Reporte

Output con esta estructura:

```text
## Zoom-out report

- target: <archivo / simbolo / area>
- modulo: <nombre del paquete o directorio mayor>
- proposito (1 linea): <que hace dentro del sistema>

### Callers (entradas)

- <archivo:linea>: <descripcion corta del callsite>
- ...

### Dependencias (salidas)

- <paquete / archivo>: <que se usa de ahi>
- ...

### Decisiones / contexto relevantes

- <ADR / nota>: <una linea>
- ...

### Sugerencia de proximo paso

<una linea con donde mirar primero si el usuario quiere profundizar>
```

Si alguna seccion no aplica (ej: no hay callers porque es un entry point CLI), poner `(ninguno)` y seguir.

## MUST DO

- Subir un nivel de abstraccion sobre el target. El zoom-out es contexto, no codigo.
- Listar callers y dependencias con paths concretos (`archivo:linea`), verificados con grep/lectura.
- Usar el vocabulario del repo cuando hay glosario.
- Cerrar con una sugerencia concreta de proximo paso para el usuario.

## MUST NOT DO

- No copiar el contenido del archivo target en el output. El zoom-out resume, no transcribe.
- No especular sobre callers o dependencias sin haberlos verificado por grep o lectura.
- No proponer cambios de codigo desde este skill; es solo lectura y explicacion.
- No persistir nada en auto-memory ni crear archivos.

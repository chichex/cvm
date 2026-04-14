Review de la sesion actual y persistencia de learnings en la auto-memory del proyecto. Completamente autonomo — no pide input.

Soporta `--dry-run`: si $ARGUMENTS contiene `--dry-run` o `preview`, mostrar lo que se persistiria sin escribir a disco.

## Proceso

### Paso 1: Escanear la conversacion

Revisar todo el hilo y extraer candidatos:

**Learnings** — descubrimientos no-obvios:
- Comportamientos inesperados del codebase
- Patrones que funcionaron o fallaron (y por que)
- Workarounds necesarios
- Dependencias no documentadas

**Gotchas** — trampas para el futuro:
- Cosas que parecen una cosa pero son otra
- Configuraciones que rompen silenciosamente
- Errores costosos de diagnosticar

**Decisiones** — elecciones de diseno:
- Que se eligio y que se descarto
- Trade-offs aceptados

### Paso 2: Filtrar con criterio estricto

Descartar todo lo que Claude podria descubrir solo leyendo el codigo durante 30 segundos:
- Estructura de archivos, imports, tipos — derivable del codigo
- Patrones obvios del framework — derivable de docs
- Info efimera de esta sesion que no aplica a futuras sesiones
- Info sensible (tokens, passwords, keys)

Solo queda lo genuinamente no-obvio y util para futuras sesiones.

### Paso 3: Verificar duplicados

Descubrir el directorio de memory del proyecto:
```bash
PROJECT_PATH=$(pwd | sed 's|/|-|g')
MEMORY_DIR="$HOME/.claude/projects/$PROJECT_PATH/memory"
```
Ejemplo: si pwd es `/Users/foo/myproject`, el path es `~/.claude/projects/-Users-foo-myproject/memory/`.

Leer `MEMORY.md` del proyecto (si existe) y los archivos de memory existentes.
Descartar duplicados. Si existe uno similar con info nueva, actualizar el existente.

### Paso 4: Si es dry-run, reportar y salir

Si $ARGUMENTS contiene `--dry-run` o `preview`:

Si no hay hallazgos despues del filtro:
```
Dry run — no hay learnings para persistir en esta sesion.
```

Si hay hallazgos:
```
Dry run — lo que se persistiria:

- [tipo] descripcion
- [tipo] descripcion

Total: N items. Ejecuta `/r` sin --dry-run para persistir.
```

No escribir nada a disco. Terminar aqui.

### Paso 5: Persistir

Crear el directorio de memory si no existe:
```bash
mkdir -p "$MEMORY_DIR"
```

Para cada hallazgo, crear/actualizar un archivo `.md` en `$MEMORY_DIR/` con frontmatter:
```markdown
---
name: <nombre descriptivo>
description: <una linea — usada para decidir relevancia en futuras sesiones>
type: <feedback|project|reference>
---

<contenido>
```

Usar Read + Write/Edit tools para modificar archivos, no heredocs en shell.

### Paso 6: Mantener MEMORY.md

Actualizar `$MEMORY_DIR/MEMORY.md`:
- Agregar entrada para cada archivo nuevo: `- [titulo](archivo.md) — resumen de una linea`
- Eliminar entradas que referencien archivos que ya no existen
- Eliminar entradas de memorias que ya no aplican (verificar contra lo observado en esta sesion)
- Mantener debajo de 200 lineas (despues de 200 se trunca)
- Ordenar semanticamente por tema, no cronologicamente

### Paso 7: Reporte

```
Review completada:
- N learnings persistidos
- N actualizados
- N descartados (duplicados o triviales)
```

## MUST DO
- Revisar TODA la conversacion, no solo los ultimos mensajes
- Aplicar el filtro de 30 segundos estrictamente
- Verificar duplicados antes de persistir
- Mantener MEMORY.md sano y debajo de 200 lineas
- Usar Read + Write/Edit tools para modificar archivos, no heredocs en shell
- Ser completamente autonomo — no pedir input (excepto en dry-run que solo reporta)

## MUST NOT DO
- No pedir confirmacion ni input al usuario
- No modificar CLAUDE.md (ni el global ni el del proyecto) — NUNCA
- No guardar info derivable del codigo en 30 segundos
- No guardar info sensible
- No dejar entradas duplicadas

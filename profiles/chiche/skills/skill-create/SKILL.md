Generar un nuevo skill para el profile de cvm. El argumento $ARGUMENTS describe que debe hacer el skill.

## Proceso

### Paso 1: Entender el pedido
Analizar $ARGUMENTS y clasificar el tipo de workflow:
- **Automatizacion**: tarea repetitiva que se quiere estandarizar
- **Review/Analisis**: inspeccion de codigo o estado del proyecto
- **Generacion**: crear artefactos (codigo, docs, configs)
- **Orquestacion**: coordinar multiples sub-agentes en paralelo

Anunciar la clasificacion antes de continuar.

### Paso 2: Investigar el contexto
- Leer los skills existentes en el profile para mantener consistencia de estilo
- Si el skill interactua con el codebase, explorar patrones y convenciones relevantes del proyecto actual
- Buscar en la web si hay best practices especificas para el tipo de workflow

### Paso 3: Generar el skill
Crear el contenido del skill siguiendo estas reglas:

**Estructura obligatoria:**
- Primera linea: descripcion en espanol de que hace el skill (sin titulo markdown)
- Usar `$ARGUMENTS` si el skill necesita parametros de entrada
- Fases/pasos claramente separados con `##`
- Incluir template de output embebido si el skill produce artefactos estructurados

**Buenas practicas:**
- Directivas imperativas ("Identificar y eliminar", NO "Podrias buscar...")
- Limites explicitos con secciones MUST DO / MUST NOT DO cuando delegue a sub-agentes
- Un workflow por skill — si hace demasiadas cosas, sugerir partirlo en varios
- Maximo ~500 lineas para no desperdiciar context window
- Si usa sub-agentes, seguir el schema: TASK / EXPECTED OUTCOME / MUST DO / MUST NOT DO / CONTEXT

**Anti-patterns a evitar:**
- Instrucciones vagas ("hacelo bien", "se util")
- No especificar formato de output
- Narracion en vez de directivas ("Primero, vamos a...")
- Catch-all commands que hacen de todo
- Olvidarse de decir que NO hacer

### Paso 4: Presentar para aprobacion
Mostrar el skill completo al usuario en un bloque de codigo y preguntar:
1. El contenido esta bien o queres cambios?
2. El nombre `[nombre-sugerido]` te copa?

NO guardar nada hasta que el usuario confirme.

### Paso 5: Guardar
Una vez aprobado:
1. Crear el directorio del skill en `skills/[nombre]/` dentro del profile activo
2. Guardar el contenido en `skills/[nombre]/SKILL.md`
3. Actualizar el CLAUDE.md del profile para listar el nuevo skill en la tabla de skills
4. Confirmar que el skill quedo registrado mostrando el comando: `/[nombre]`

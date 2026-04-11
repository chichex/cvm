Analizar una tarea y decidir la ruta de ejecucion: directo, subagent, o team. $ARGUMENTS es la descripcion de la tarea.

Antes de elegir `team`, verificar soporte real de Claude Teams en la sesion actual. No asumir soporte por configuracion, flags o env vars del profile. Si no hay evidencia clara de que Claude puede crear/usar Teams ahora, tratar `team` como no disponible y elegir entre `subagent` o `directo`.

## Proceso

### Paso 1: Analizar la tarea
Evaluar $ARGUMENTS en estas dimensiones:

| Dimension | Valor |
|-----------|-------|
| Archivos involucrados | 1-2 / 3-10 / 10+ |
| Areas independientes | 1 / 2-3 / 4+ |
| Complejidad | lookup / implementacion / arquitectura |
| Paralelizable | si / no |
| Riesgo | bajo / medio / alto |

### Paso 2: Decidir ruta

**Directo** — ejecutar en el thread principal:
- Lookup simple, respuesta a pregunta, confirmacion
- 1-2 archivos, 1 area, complejidad baja
- Tiempo estimado: <30 segundos

**Subagent** — delegar a un subagent con rol especializado:
- Tarea enfocada en un area especifica
- 3-10 archivos, 1-2 areas, complejidad media
- No paralelizable o solo necesita 1 subagent
- Elegir rol segun el VERBO de la tarea (invocar con `Agent(subagent_type: "general-purpose", model: "<model>")`):

| Verbo | Rol | model |
|-------|-----|-------|
| buscar, encontrar, listar, leer, localizar | **researcher** | `haiku` |
| analizar, investigar, entender, revisar, comparar, evaluar | **reviewer** | `opus` |
| implementar, escribir, refactorear, testear, arreglar | **implementer** | `sonnet` |

Si el verbo es ambiguo: ¿requiere razonamiento o solo lectura? Razonamiento → reviewer (opus). Solo lectura → researcher (haiku).

**Team** — lanzar multiples agentes en paralelo, solo si Claude Teams esta realmente soportado:
- Multiples areas independientes que se pueden paralelizar
- 10+ archivos, 3+ areas, complejidad alta
- El trabajo de un area no bloquea al otro
- Ejemplos: frontend + backend + tests, security + performance + correctness

### Paso 3: Proponer estructura

**Si directo:**
```
Ruta: directo
Razon: [por que es simple]
Accion: [que voy a hacer]
```

**Si subagent:**
```
Ruta: subagent
Invocacion: Agent(subagent_type: "general-purpose", model: "[haiku/sonnet/opus]")
Rol: [researcher/implementer/reviewer]
Prompt: embeber instrucciones de agents/<rol>/AGENT.md + terminar con ## Key Learnings:
TASK: [descripcion]
EXPECTED OUTCOME: [que se espera]
```

**Si team:**
```
Ruta: team
Teammates:
- [nombre] ([modelo]): [responsabilidad]. Tasks: [lista]
- [nombre] ([modelo]): [responsabilidad]. Tasks: [lista]
Coordinacion: [dependencias entre teammates, si las hay]
```

### Paso 4: Ejecutar
Implementar la ruta elegida. Si es team, crear la estructura de team y lanzar solo despues de confirmar soporte real.

## MUST DO
- Evaluar ANTES de actuar — no empezar y despues decidir delegar
- Elegir la ruta mas simple que resuelva el problema
- Si hay duda entre subagent y team, elegir subagent
- Si no hay soporte real confirmado para Teams, no elegir `team`

## MUST NOT DO
- No usar team para tareas secuenciales (un teammate espera al otro)
- No usar team para ediciones en el mismo archivo
- No usar directo para tareas de 10+ archivos
- No sobre-ingeniar la estructura del team
- No asumir que Teams existe por tener una config experimental habilitada

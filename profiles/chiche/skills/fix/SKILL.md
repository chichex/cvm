Diagnosticar y resolver un bug con rigor. $ARGUMENTS es la descripcion del bug, un error message, un stack trace, o un path a un archivo/linea donde se manifiesta.

## Paso 1: Entender el reporte

**1a. Parsear el input:**
- Si $ARGUMENTS es un error/stack trace: extraer el mensaje, archivo, linea, y call stack
- Si $ARGUMENTS es una descripcion: identificar comportamiento esperado vs actual
- Si $ARGUMENTS es un path: leer el archivo y el contexto alrededor de la linea mencionada
- Si $ARGUMENTS esta vacio: preguntar que bug se quiere resolver. NO continuar sin input.

**1b. Reproducir:**
1. Intentar reproducir el bug. Buscar tests existentes que cubran el caso, correrlos.
2. Si hay un comando o flujo para reproducir, ejecutarlo y capturar el output exacto.
3. Si NO se puede reproducir: reportar y preguntar al usuario por mas contexto. NO continuar sin reproduccion o evidencia clara.

Mostrar:
```
Bug report:
- Sintoma: [que pasa]
- Esperado: [que deberia pasar]
- Reproduccion: [como se reproduce / evidencia]
```

## Paso 2: Diagnosticar root cause

### 2a: Detectar herramientas

Verificar disponibilidad de Codex: `codex exec "echo ok" 2>/dev/null` (con timeout de 10s).

### 2b: Lanzar investigacion

Lanzar un subagent con rol reviewer — `Agent(subagent_type: "general-purpose", model: "opus")`:

- **TASK**: Encontrar la causa raiz del bug descrito
- **EXPECTED OUTCOME**: Archivo(s), linea(s), y explicacion de POR QUE el bug ocurre — no solo DONDE
- **MUST DO**: Leer el codigo involucrado. Seguir el flujo de datos desde el input hasta el sintoma. Buscar commits recientes que hayan tocado el area (`git log --oneline -20 -- [archivos]`). Verificar si hay tests que deberian haber atrapado esto. Buscar si el bug existe en otros lugares similares (mismo patron replicado). Terminar con `## Key Learnings:`.
- **MUST NOT DO**: Proponer fixes. Editar archivos. Hacer cambios. Especular sin evidencia.
- **CONTEXT**: [sintoma, reproduccion, archivos involucrados]

**Si Codex esta disponible**, lanzar en paralelo con el subagent:

```bash
codex exec -s read-only "Investigar este bug: [descripcion]. Encontrar root cause. NO hacer cambios, solo reportar hallazgos con archivos y lineas especificas.

Sintoma: [que pasa]
Esperado: [que deberia pasar]

Reportar: root cause con archivo(s), linea(s), y explicacion de POR QUE ocurre."
```

### 2c: Consolidar hallazgos

**Si hubo dual investigation (Opus + Codex):**
- Coinciden en root cause → alta confianza, proceder al gate
- Difieren → analizar ambas hipotesis, presentar ambas al usuario en el gate del Paso 3

**Si fue solo Opus:**
- Usar el hallazgo del subagent directamente

## Paso 3: GATE — Confirmar diagnostico

Presentar los hallazgos al usuario:

```
Diagnostico:
- Root cause: [explicacion clara de por que ocurre]
- Archivo(s): [paths con lineas]
- Evidencia: [que confirma que esta es la causa — test, log, lectura de codigo]
- Alcance: [afecta otros lugares? hay instancias del mismo patron?]
- Regresion: [esto funcionaba antes? que cambio lo rompio?]
```

Preguntar: **"El diagnostico tiene sentido? Procedo?"**

**NO avanzar sin confirmacion explicita del usuario.** Este es el gate mas importante del skill.

## Paso 4: Clasificar y elegir ruta

Evaluar el fix necesario en base al diagnostico confirmado:

**Criterios de clasificacion:**

| Criterio | Valor |
|----------|-------|
| Archivos a tocar | 1-2 o muchos? |
| Lineas de cambio estimadas | <30 o mas? |
| Riesgo | Toca codigo critico? Tiene side effects? |
| Complejidad | Es un fix puntual o requiere rediseno? |
| Patron replicado | Hay que arreglar en multiples lugares? |

**Ruta A — Fix directo** (1-2 archivos, <30 lineas, bajo riesgo, sin rediseno):
- Continuar al Paso 5 en este mismo contexto
- Anunciar: "Fix directo — es un cambio acotado"

**Ruta B — GitHub issue** (fix claro pero no urgente, o el usuario quiere trackear):
- Crear issue con `gh issue create` incluyendo diagnostico, root cause, y fix propuesto
- Preguntar: "Queres que lo implemente ahora o lo dejamos como issue?"
- Si dice que si -> continuar al Paso 5
- Si dice que no -> terminar

**Ruta C — Escalar a /spec** (rediseno necesario, multiples modulos, alto riesgo):
- Anunciar: "Esto es mas grande que un fix — requiere planificacion"
- Mostrar por que: que archivos, que modulos, que riesgo
- Sugerir: "Recomiendo correr `/spec [descripcion del fix necesario]` para planificar y luego `/execute`"
- Opcionalmente crear issue con el diagnostico para no perder el analisis
- Terminar. NO intentar implementar.

Mostrar la ruta elegida y justificacion. Esperar OK del usuario antes de continuar.

## Paso 5: TDD — Escribir test que falla

Antes de tocar el codigo productivo:

1. **Evaluar si TDD aplica:**
   - Hay test suite funcional? El area tiene tests?
   - El bug es deterministico y reproducible en un test?
   - Si TDD NO aplica (ej: bug de UI, timing, infra), explicar por que y saltar al Paso 6.

2. **Escribir el test:**
   - Seguir convenciones de testing del proyecto (framework, ubicacion, naming)
   - El test debe capturar EXACTAMENTE el comportamiento esperado que el bug viola
   - Nombrar el test descriptivamente: `test_[que_deberia_pasar]_[contexto]`

3. **Confirmar que falla:**
   - Correr el test. DEBE fallar con un error relacionado al bug.
   - Si pasa -> el test no captura el bug. Reescribir o reconsiderar el diagnostico.
   - Mostrar: "Test escrito: [nombre]. Falla con: [error]. Procedo al fix."

## Paso 6: Fix minimo

Implementar el fix mas pequeno posible que resuelva el root cause:

1. Editar solo lo necesario — no refactorear codigo alrededor
2. Si el bug existe en multiples lugares (detectado en Paso 2), arreglar TODOS
3. No agregar error handling especulativo, no mejorar nada que no este roto

## Paso 7: Validacion

1. **Si se escribio test en Paso 5:** correrlo. DEBE pasar ahora.
2. Correr test suite completa — verificar que no se rompio nada
3. Correr lint/type-check
4. Leer cada archivo modificado para confirmar que el cambio es correcto y minimo

Mostrar resumen:
```
Fix aplicado:
- Root cause: [resumen de una linea]
- Cambios: [archivos:lineas]
- Test de regresion: [nombre del test o "N/A — justificacion"]
- Test suite: pass/fail
- Lint: pass/fail
```

Si algo falla, diagnosticar y corregir. Maximo 3 intentos antes de frenar.

## MUST DO
- Reproducir o evidenciar el bug antes de diagnosticar
- Confirmar root cause con el usuario antes de implementar
- Escribir test de regresion si el contexto lo permite
- Fix minimo — solo lo que esta roto
- Validar que el fix resuelve el bug Y no rompe nada mas

## MUST NOT DO
- NO implementar sin diagnostico confirmado por el usuario
- NO especular sobre la causa — seguir el codigo y la evidencia
- NO refactorear codigo que no es parte del bug
- NO saltar el gate del Paso 3 bajo ninguna circunstancia
- NO escribir tests que pasan sin el fix (test inutil)
- NO hacer shotgun debugging (cambios random esperando que algo funcione)
- NO escalar a /spec sin explicar por que el fix directo no alcanza

Debugging adversarial: lanzar 2+ agentes independientes para investigar el mismo problema desde angulos distintos, luego consolidar hallazgos. $ARGUMENTS es la descripcion del problema o bug.

## Proceso

### Paso 1: Definir el problema
Parsear $ARGUMENTS y formular:
- Sintoma observado
- Comportamiento esperado
- Archivos/area involucrada (si se conoce)

### Paso 2: Detectar herramientas disponibles
```bash
which codex 2>/dev/null && echo "codex disponible" || echo "codex no disponible"
```

### Paso 3: Lanzar investigadores

**Si codex esta disponible — usar codex + Claude agent:**

Agente 1 (Claude subagent):
- **TASK**: Investigar el bug descrito. Seguir el flujo de datos desde el input hasta el sintoma.
- **EXPECTED OUTCOME**: Root cause con archivo(s), linea(s), y explicacion de POR QUE ocurre.
- **MUST DO**: Leer el codigo involucrado. Buscar commits recientes en el area. Verificar tests existentes.
- **MUST NOT DO**: Proponer fixes. Editar archivos.
- **CONTEXT**: [problema, archivos conocidos]

Agente 2 (codex):
```bash
codex -q "Investigar este bug: [descripcion]. Encontrar root cause. NO hacer cambios, solo reportar hallazgos con archivos y lineas especificas."
```

**Si codex NO esta disponible — usar 2 Claude agents con hipotesis distintas:**

Agente 1:
- **TASK**: Investigar el bug asumiendo que es un problema de DATOS/ESTADO (input incorrecto, estado corrupto, race condition).
- [misma estructura EXPECTED OUTCOME / MUST DO / MUST NOT DO]

Agente 2:
- **TASK**: Investigar el bug asumiendo que es un problema de LOGICA/CONTROL FLOW (condicion incorrecta, edge case, error en algoritmo).
- [misma estructura EXPECTED OUTCOME / MUST DO / MUST NOT DO]

### Paso 4: Consolidar
Comparar los hallazgos de ambos agentes:
- Si coinciden en el root cause: alta confianza, reportar
- Si difieren: analizar ambas hipotesis y determinar cual tiene mejor evidencia
- Si ambas son validas: podrian ser multiples bugs, reportar ambos

### Paso 5: Presentar
```
Validacion adversarial:

Agente 1 encontro: [hallazgo]
Agente 2 encontro: [hallazgo]

Consenso: [si/no]
Root cause mas probable: [explicacion]
Evidencia: [archivos, lineas, razonamiento]
Siguiente paso recomendado: [fix directo / mas investigacion / escalar]
```

## MUST DO
- Lanzar los agentes en paralelo cuando sea posible
- Incluir evidencia concreta (archivos, lineas) en el reporte
- Ser transparente sobre el nivel de confianza

## MUST NOT DO
- No implementar fixes — este skill solo diagnostica
- No lanzar mas de 3 agentes (disminuye retornos)
- No ignorar hallazgos que contradicen la hipotesis principal

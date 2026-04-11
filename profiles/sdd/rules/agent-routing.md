---
paths:
  - "**/*"
---

# Routing de Agentes

Delegar trabajo usando el Agent tool con `subagent_type: "general-purpose"` y el parametro `model` segun el rol:

| Tipo de tarea | Rol | model | Justificacion |
|---------------|-----|-------|---------------|
| Busqueda mecanica (buscar, leer, listar) | **researcher** | `haiku` | Mecanico, rapido y barato |
| Analisis y razonamiento (entender, investigar, revisar) | **reviewer** | `opus` | Requiere razonamiento profundo |
| Escritura de codigo (implementar, refactorear, testear) | **implementer** | `sonnet` | Balance costo/calidad |

**Invocacion**: usar `Agent(subagent_type: "general-purpose", model: "<model>")` e incluir en el prompt el rol, instrucciones, y formato de respuesta del agente (ver `agents/<rol>/AGENT.md`).

**IMPORTANTE**: siempre incluir en el prompt del agente la instruccion de terminar con `## Key Learnings:` — esto alimenta el pipeline de aprendizaje automatico via el hook SubagentStop.

SIEMPRE usar `subagent_type: "general-purpose"` para delegar. Claude Code no descubre agent types custom definidos en `agents/`; por eso los roles se simulan con general-purpose + model + prompt.

Si el verbo es ambiguo: ¿requiere razonamiento o solo lectura? Razonamiento → reviewer (opus). Solo lectura → researcher (haiku).

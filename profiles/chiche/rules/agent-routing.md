---
paths:
  - "**/*"
---

# Routing de Agentes

NUNCA usar agentes built-in (Explore, general-purpose Agent) para delegar trabajo.
SIEMPRE rutear por los agentes custom del profile:

| Tipo de tarea | Agente |
|---------------|--------|
| Busqueda mecanica (buscar, leer, listar) | **researcher** (haiku) |
| Analisis y razonamiento (entender, investigar, revisar) | **reviewer** (opus) |
| Escritura de codigo (implementar, refactorear, testear) | **implementer** (sonnet) |

Los agentes custom tienen formato de respuesta estructurado con `## Key Learnings:` que alimenta el pipeline de aprendizaje automatico via el hook SubagentStop. Usar built-ins rompe ese pipeline.

Si el verbo es ambiguo: ¿requiere razonamiento o solo lectura? Razonamiento → reviewer. Solo lectura → researcher.

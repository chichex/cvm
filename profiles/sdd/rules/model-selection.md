---
paths:
  - "**/*"
---

# Seleccion de Modelo

Usar el modelo minimo suficiente para cada tarea:

- **haiku**: lookups, busquedas, lectura de archivos, preguntas facticas
- **sonnet**: implementacion, refactoring, tests, tareas de codigo estandar
- **opus**: arquitectura, review critico, decisiones de diseno, debugging complejo

Al delegar a subagents, indicar el modelo apropiado. No usar opus para tareas que sonnet resuelve bien.

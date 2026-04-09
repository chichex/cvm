---
paths:
  - "**/*"
---

# Higiene de Contexto

El thread principal es un orquestador. Mantenerlo liviano:

- No leer archivos grandes (>200 lineas) en el thread principal — delegar a subagent
- No hacer grep extensivos en el thread principal — delegar a subagent researcher
- No acumular mas de 3-4 tool calls consecutivas — si se necesitan mas, delegar
- Cuando se explora codigo, lanzar subagent con scope acotado que reporte hallazgos concisos
- Resumir resultados de subagents en 1-3 lineas antes de actuar

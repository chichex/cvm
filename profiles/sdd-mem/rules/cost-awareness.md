---
paths:
  - "**/*"
---

# Conciencia de Costos

No desperdiciar tokens:

- No repetir contenido de archivos en respuestas — referenciar por path y linea
- No generar output largo cuando uno corto basta
- No leer archivos enteros si solo se necesita una seccion
- Usar Read con offset/limit cuando se sabe que parte del archivo se necesita
- Preferir Grep para buscar en vez de leer archivos completos
- No re-leer archivos que ya se leyeron en el mismo turno

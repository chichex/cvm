# Trazabilidad

Todo codigo publico debe ser trazable a una spec:

- Modulos/clases: incluir `// Spec: S-XXX` en el header
- Funciones publicas: incluir `// Req: B-XXX` o `// Req: E-XXX`
- Tests: incluir `// Spec: S-XXX | Req: B-XXX | Type: happy|edge|error`

Si se encuentra codigo publico sin referencia a spec, marcarlo para spec retroactiva.

Excepciones: imports, helpers internos, utils, config — no necesitan trazabilidad directa.

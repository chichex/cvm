# Spec First

Antes de implementar cualquier feature, componente, API, o funcion:

1. Verificar si existe una spec en `specs/`
2. Si no existe: crearla — la implementacion se deriva de la spec, no al reves
3. Si existe pero esta desactualizada: actualizarla primero
4. Solo implementar cuando la spec define claramente los contratos y behaviors

Excepciones (no requieren spec):
- Fixes triviales (cambio aislado en 1-2 archivos, sin interfaces publicas nuevas, sin behavior nuevo, sin cambio cross-module. Ejemplos: fix typo, update config value, add log line, use existing util)
- Cambios en config, scripts, o documentacion
- Refactoring que no cambia comportamiento (pero si cambia contratos → spec)

Para todo lo demas: spec primero, siempre.

Reconciliacion con scope-guard: el spec documenta el alcance minimo para satisfacer el pedido del usuario. Edge cases se incluyen solo si son necesarios para que el feature funcione correctamente. No agregar scope especulativo. Si un edge case es "nice to have", documentarlo como comentario en la spec, no como requisito.

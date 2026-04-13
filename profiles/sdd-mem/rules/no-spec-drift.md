# No Spec Drift

Nunca cambiar comportamiento sin actualizar la spec:

- Si durante implementacion se descubre que la spec necesita cambio, PARAR y actualizar la spec
- Si un test falla y la causa es un gap en la spec, actualizar la spec antes de arreglar el test
- Si se agrega un edge case nuevo, agregarlo a la spec antes de implementarlo
- Nunca "arreglar" un test cambiando la assertion para que matchee el codigo — el test refleja la spec

El codigo se adapta a la spec. La spec se cambia explicitamente con version bump.

Excepcion para cambios menores: null checks, input sanitization, logging, y defensive coding que no afectan contratos se implementan directamente. Documentar en el changelog de la spec post-facto si aplica.

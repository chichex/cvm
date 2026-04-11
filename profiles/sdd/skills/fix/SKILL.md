Diagnosticar y resolver un bug con rigor, incluyendo spec gap check. Se invoca automaticamente cuando el auto-routing detecta un bug report. $ARGUMENTS es la descripcion del bug.

## Paso 1: Reproducir

1. Entender el bug: que pasa vs que deberia pasar
2. Localizar el area del codigo afectada
3. Si es posible, reproducir el error

## Paso 2: Buscar spec relacionada

Buscar en `specs/` si hay una spec que cubra el area del bug.
- Si hay spec: usarla como referencia para "comportamiento correcto"
- Si no hay spec: basarse en la descripcion del usuario

## Paso 3: Diagnosticar

Investigar la causa root:
1. Leer el codigo afectado
2. Buscar en KB: `cvm kb search "<area del bug>"`
3. Analizar la causa

Gate: DEBE tener una hipotesis clara antes de continuar. No shotgun debugging.

## Paso 4: Spec gap check (nuevo en SDD)

Si hay spec relacionada:
- ¿El bug revela un gap en la spec? (caso no especificado)
- ¿El bug es un drift? (implementacion no matchea spec)
- ¿La spec es correcta pero la implementacion esta mal?

Si hay gap en la spec:
1. Proponer agregar el caso faltante a la spec (nuevo E-XXX o B-XXX)
2. Actualizar la spec ANTES de arreglar el bug
3. Version bump de la spec

Si hay drift:
1. Documentar la divergencia
2. Preguntar: ¿el spec esta mal o el codigo?

## Paso 4b: Ruta de hotfix (si es urgente)

Si el bug es critico y necesita mitigacion inmediata:
1. Aplicar fix minimo para contener el problema
2. Correr tests para verificar que no rompe nada mas
3. Documentar: "hotfix aplicado, spec pendiente de actualizar"
4. DESPUES del hotfix: actualizar la spec con el caso faltante y version bump

Esta ruta es para emergencias. Para bugs no-urgentes, seguir el flujo normal (actualizar spec primero).

## Paso 5: Implementar el fix

1. Aplicar el fix minimo que resuelve el bug
2. Si la estrategia de validacion de la spec es TDD:
   - Escribir test que reproduce el bug (debe fallar)
   - Aplicar fix (test pasa)
3. Si no hay TDD: aplicar fix y verificar manualmente
4. Correr test suite completa

## Paso 6: Validar

1. Tests pasan
2. Lint limpio
3. Build limpio
4. Si habia spec: verificar que el fix es consistente con la spec

## Paso 7: Persistir

Si se descubrio un gap en la spec o un gotcha:
- `cvm kb put` con tag apropiado
- Actualizar spec si aplica

## MUST DO
- Diagnosticar ANTES de arreglar
- Buscar spec relacionada
- Si hay gap en spec: actualizar spec primero

## MUST NOT DO
- NO hacer shotgun debugging
- NO ignorar spec drift
- NO arreglar sin diagnostico claro
- NO saltear spec gap check si hay spec

Debugging adversarial con multiples agentes, incluyendo eje de spec conformance. Se invoca cuando hay un problema complejo de diagnosticar. $ARGUMENTS es la descripcion del problema.

## Paso 1: Definir el problema

Extraer del prompt del usuario:
- Que esta fallando (comportamiento actual)
- Que deberia pasar (comportamiento esperado)
- Como reproducir

## Paso 2: Buscar spec relacionada

Buscar en `specs/` si hay una spec que cubra el area del problema.
- Si hay spec: usarla como referencia para "comportamiento esperado"
- Si no hay spec: usar la descripcion del usuario

## Paso 3: Lanzar investigacion adversarial

Lanzar 2-3 agentes independientes en paralelo, cada uno con una hipotesis diferente:

**Agente 1 — Hipotesis de implementacion** (model: sonnet)
"El codigo tiene un bug. Buscar en [archivos relevantes] donde el comportamiento difiere de lo esperado."

**Agente 2 — Hipotesis de spec drift** (model: opus)
"La implementacion drifto de la spec. Comparar el comportamiento actual contra la spec [S-XXX] y reportar divergencias."

**Agente 3 (opcional) — Hipotesis de ambiente** (model: haiku)
"El problema es de config/entorno/datos. Verificar configs, env vars, estado de DB, dependencias."

## Paso 4: Consolidar

Combinar hallazgos de todos los agentes:
1. Que encontro cada uno?
2. Donde convergen?
3. Donde divergen?
4. Cual es la causa root mas probable?

Si se detecto spec drift:
- Documentar exactamente donde drifto
- Preguntar: "¿La spec esta mal o la implementacion esta mal?"

## Paso 5: Proponer solucion

Segun la causa:
- **Bug de implementacion**: proponer fix + actualizar tests
- **Spec drift**: proponer actualizar spec o codigo (el usuario decide)
- **Gap en spec**: proponer agregar el caso faltante a la spec
- **Ambiente**: proponer fix de config/entorno

## MUST DO
- Siempre buscar la spec relacionada
- Lanzar agentes con hipotesis DIFERENTES (no la misma con distinto wording)
- Si hay spec, un agente DEBE verificar conformance

## MUST NOT DO
- NO asumir la causa sin investigar
- NO arreglar sin diagnosticar
- NO ignorar spec drift si se detecta

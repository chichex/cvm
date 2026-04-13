Generar tests desde una spec y sus contratos. Se invoca automaticamente cuando la estrategia de validacion incluye tests. $ARGUMENTS es el path a la spec.

## Paso 1: Verificar precondiciones

- Leer la spec — debe tener status "approved"
- Verificar la estrategia de validacion documentada en la spec
- Si la estrategia es "manual" o "sin tests", este skill no aplica — reportar y salir

## Paso 2: Mapear requisitos a test cases

Para cada requisito de la spec:

| Tipo de requisito | Tests a generar |
|-------------------|----------------|
| Behavior (B-XXX) | 1 test por behavior (Given/When/Then directo) |
| Edge case (E-XXX) | 1 test por edge case |
| Invariante (I-XXX) | 1 test que verifica la propiedad |
| Error (ERR-XXX) | 1 test que verifica el error |

Generar tabla de mapping:
```
| Requisito | Test case | Tipo |
|-----------|-----------|------|
| B-001 | should create user with valid input | behavior |
| E-001 | should reject empty name | edge |
| ERR-001 | should throw ValidationError | error |
```

## Paso 3: Generar tests

Usar el framework de testing del proyecto (detectar automaticamente).
Cada test referencia la spec:

```
// Spec: S-XXX | Req: B-001 | Type: behavior
test('should create user with valid input', () => {
  // GIVEN (datos de la spec)
  // WHEN (accion de la spec)
  // THEN (resultado de la spec)
});
```

Los datos de test vienen de los examples de la spec, no datos inventados.

## Paso 4: Verificar strategy

**Si la estrategia es TDD:**
- Correr los tests generados
- Al menos un test DEBE fallar para demostrar el gap que se va a implementar
- Si algunos tests pasan: es normal en codebases existentes que ya cumplen parte del behavior — no son tests mal escritos
- Si TODOS pasan sin implementacion nueva: revisar — puede ser que el behavior ya exista, o que los tests no sean suficientemente especificos
- Reportar: "N tests generados, M fallando (gap demostrado), P pasando (behavior pre-existente)"

**Si la estrategia es tests-post-impl:**
- Los tests se generan pero se guardan para despues de implementar
- Reportar: "N tests generados, se ejecutaran post-implementacion"

## Paso 5: Verificar coverage de spec

```
Spec coverage:
- Behaviors: X/Y con tests
- Edge cases: X/Y con tests
- Invariantes: X/Y con tests
- Errores: X/Y con tests
- Coverage total: X%
```

Si coverage < 100%, agregar tests faltantes.

## MUST DO
- Cada test referencia spec ID y requisito ID
- Formato GIVEN/WHEN/THEN
- Datos de test de los examples de la spec

## MUST NOT DO
- NO escribir tests que testean implementacion (solo behavior)
- NO inventar test cases que no estan en la spec
- NO implementar nada — solo tests

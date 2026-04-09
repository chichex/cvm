Registrar una decision de arquitectura o diseno en la Knowledge Base. $ARGUMENTS es la descripcion de la decision, o vacio para extraerla del contexto actual.

## Proceso

1. Si $ARGUMENTS esta vacio, identificar la decision mas reciente de la conversacion.
2. Si $ARGUMENTS tiene contenido, usarlo como base.

3. Estructurar la decision:
   - **Que se decidio**: la eleccion concreta
   - **Alternativas descartadas**: que se considero y por que no
   - **Trade-offs**: que se acepto conscientemente
   - **Contexto**: por que se tomo esta decision ahora

4. Determinar scope (global o local al proyecto).

5. Generar key descriptiva (ej: `use-zod-over-joi-for-validation`)

6. Persistir:
```bash
cvm kb put "<key>" --body "DECISION: <que>. ALTERNATIVAS: <descartadas>. TRADE-OFF: <aceptados>. CONTEXTO: <por que>" --tag "decision,<area>" [--local]
```

7. Confirmar al usuario.

## MUST DO
- Incluir alternativas descartadas — sin eso la decision pierde valor
- Incluir trade-offs aceptados
- Verificar duplicados: `cvm kb search "<terminos clave>"` y `cvm kb search "<terminos clave>" --local`

## MUST NOT DO
- No registrar decisiones triviales (nombres de variables, formatting)
- No registrar sin el trade-off explicito
- No guardar sin confirmar con el usuario si $ARGUMENTS esta vacio

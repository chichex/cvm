Registrar un gotcha o trampa encontrada en la Knowledge Base. $ARGUMENTS es la descripcion del gotcha, o vacio para extraerlo del contexto actual.

## Proceso

1. Si $ARGUMENTS esta vacio, identificar el gotcha de la conversacion actual.
2. Si $ARGUMENTS tiene contenido, usarlo como base.

3. Estructurar el gotcha:
   - **Sintoma**: que parece que pasa
   - **Realidad**: que pasa realmente
   - **Solucion/Workaround**: como evitarlo o resolverlo

4. Determinar scope (global o local al proyecto).

5. Generar key descriptiva (ej: `prisma-migrate-dev-resets-data`)

6. Persistir:
```bash
cvm kb put "<key>" --body "GOTCHA: <sintoma>. REALIDAD: <que pasa>. SOLUCION: <como evitarlo>" --tag "gotcha,<area>" [--local]
```

7. Confirmar al usuario.

## MUST DO
- Incluir el sintoma Y la realidad — el gotcha es la diferencia entre ambos
- Incluir solucion o workaround si se conoce
- Verificar duplicados: `cvm kb search "<terminos clave>"`

## MUST NOT DO
- No registrar bugs obvios que se van a arreglar — eso es un fix, no un gotcha
- No guardar sin confirmar con el usuario si $ARGUMENTS esta vacio

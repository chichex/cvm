Guardar un insight o aprendizaje en la Knowledge Base. $ARGUMENTS es la descripcion del learning, o vacio para extraerlo del contexto actual.

## Proceso

1. Si $ARGUMENTS esta vacio, identificar el learning mas reciente de la conversacion actual.
2. Si $ARGUMENTS tiene contenido, usarlo como base.

3. Clasificar el learning:
   - **tecnico** — comportamiento de una herramienta, libreria, o API
   - **patron** — patron de codigo o arquitectura que funciono (o no)
   - **proceso** — algo sobre como trabajar mejor
   - **entorno** — algo sobre el setup, infra, o configuracion

4. Determinar scope:
   - **global** — aplica a cualquier proyecto
   - **local** — especifico a este proyecto

5. Generar una key descriptiva en kebab-case (ej: `nextjs-server-actions-cache-invalidation`)

6. Persistir:
```bash
cvm kb put "<key>" --body "<descripcion clara y concisa del learning>" --tag "learning,<clasificacion>" [--local]
```

7. Confirmar al usuario que se guardo.

## MUST DO
- La descripcion debe ser autocontenida — entendible sin contexto de la conversacion
- Incluir el POR QUE, no solo el QUE
- Verificar que no existe un entry duplicado: `cvm kb search "<terminos clave>"` y `cvm kb search "<terminos clave>" --local`

## MUST NOT DO
- No guardar info efimera o derivable del codigo
- No guardar info sensible (tokens, passwords, keys)
- No guardar sin confirmar con el usuario si $ARGUMENTS esta vacio

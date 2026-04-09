Ejecutar una tarea en Claude Code headless (`claude -p`). El argumento $ARGUMENTS describe que tiene que hacer.

## Paso 1: Analizar la tarea
Clasificar $ARGUMENTS en uno de estos perfiles:

**Solo lectura** (analisis, busqueda, explicacion):
- Tools: `Read,Glob,Grep`

**Escritura controlada** (editar codigo, fix, refactor):
- Tools: `Bash(git diff *),Bash(git status),Read,Glob,Grep,Edit`

**Ejecucion completa** (correr tests, build, lint+fix):
- Tools: `Bash,Read,Glob,Grep,Edit,Write`

Si la tarea no encaja claramente, elegir el perfil mas restrictivo que la cubra.

## Paso 2: Armar el comando
Construir el comando con esta estructura:

```bash
claude -p "<prompt claro y especifico basado en $ARGUMENTS>" \
  --allowedTools "<tools del perfil>" \
  --output-format stream-json
```

Reglas para el prompt interno:
- Reescribir $ARGUMENTS como una directiva clara e imperativa
- Agregar contexto del directorio actual si es relevante (`pwd`)
- Si la tarea involucra git, especificar la branch actual
- NO incluir instrucciones vagas — ser especifico sobre que hacer y que NO hacer

## Paso 3: Mostrar y confirmar
Mostrar al usuario:
1. Perfil elegido y por que
2. El comando completo
3. Pedir confirmacion antes de ejecutar

## Paso 4: Ejecutar
Una vez confirmado, correr el comando via Bash tool con `run_in_background: true`.
Avisar al usuario que la tarea esta corriendo y que va a recibir el resultado cuando termine.

## MUST NOT DO
- NO correr sin confirmacion del usuario
- NO usar `--bare` (queremos que use CLAUDE.md del proyecto)
- NO usar `--dangerously-skip-permissions` ni `bypassPermissions`
- NO ejecutar tareas destructivas (drop tables, rm -rf, force push) sin doble confirmacion

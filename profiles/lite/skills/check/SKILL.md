Revisar un PR o issue de GitHub lanzando agentes seleccionados en paralelo (opus/codex/gemini) y posteando cada review como comment separado. $ARGUMENTS es el PR/issue a revisar: numero (`24`), URL completa, o `pr 24` / `issue 24`.

## Proceso

### Paso 1: Parsear input

Extraer el numero y tipo explicito (si viene) desde $ARGUMENTS:

- `24` → numero pelado, tipo a detectar
- `#24` → idem
- `pr 24` / `PR 24` → tipo forzado a PR
- `issue 24` → tipo forzado a issue
- `https://github.com/<org>/<repo>/pull/24` → PR 24
- `https://github.com/<org>/<repo>/issues/24` → issue 24

Si no se puede extraer un numero, abortar: "No pude extraer PR/issue desde '<input>'. Usa: numero, `pr N`, `issue N`, o URL completa."

Guardar:
- `NUM`: numero parseado
- `FORCED_KIND`: `pr`, `issue`, o vacio si solo vino numero/URL sin pista

### Paso 2: Detectar tipo

Si `FORCED_KIND` ya esta seteado, usarlo. Sino:

```bash
gh pr view "$NUM" --json number 2>/dev/null && echo "pr" || (gh issue view "$NUM" --json number 2>/dev/null && echo "issue")
```

Probar primero `gh pr view <NUM> --json number`; si retorna ok, es PR. Si falla, probar `gh issue view <NUM> --json number`. Si ambos fallan, abortar: "No existe PR ni issue #<NUM> en este repo."

Guardar `KIND` = `pr` o `issue`.

### Paso 3: Fetchear contexto

Para PRs:

```bash
gh pr view "$NUM" --json number,title,body,author,labels,files,baseRefName,headRefName
gh pr diff "$NUM"
```

Para issues:

```bash
gh issue view "$NUM" --json number,title,body,author,labels,comments
```

Escribir el contexto con `Write` tool (no heredoc) a `/tmp/cvm-check-<NUM>-context.md` con este formato:

```markdown
# <KIND> #<NUM>: <title>

**Author**: <author>
**Labels**: <labels>
**Files changed** (solo PR): <lista>

## Body
<body>

## Diff (solo PR)
<diff truncado si >200KB>
```

Si el diff supera 200KB, truncarlo al ultimo archivo completo que quepa y anotar `[... diff truncado: N archivos omitidos ...]`.

### Paso 4: Preguntar agentes

Verificar disponibilidad antes del prompt:

**Codex**: `codex exec "echo ok" 2>/dev/null`

**Gemini**:
1. Leer `~/.cvm/available-tools.json` y verificar `gemini.available == true`
2. Fallback: `which gemini 2>/dev/null`

Mostrar prompt con default explicito, marcando no disponibles:

```
Revisar <PR|issue> #<NUM>: <title>

Agentes: O=opus, c=codex, g=gemini, a=all
Que agentes? [O/c/g/a] (default: O):
```

Parsear respuesta:
- vacio o `O` / `o` → solo opus
- letras combinadas (`Oc`, `og`, `cg`, `Ocg`) → cada letra = un agente
- `a` / `all` → opus + codex + gemini (los disponibles)
- cualquier otra cosa → repetir la pregunta una vez, y si vuelve a ser invalida, abortar

Filtrar los que no esten disponibles y avisar. Si queda 0 agentes, abortar: "Ningun agente disponible."

Guardar `AGENTS` como la lista final (ej: `[opus, codex]`).

### Paso 5: Armar prompts diferenciados

Cada agente tiene una perspectiva fija:

- **opus** → arquitectura / diseno / trade-offs
- **codex** → correctness / bugs / edge cases
- **gemini** → legibilidad / claridad / mantenibilidad

Para cada agente armar un prompt que:

1. Referencia `/tmp/cvm-check-<NUM>-context.md` como input (NUNCA contenido inline)
2. Declara la perspectiva del agente
3. Pide output en markdown listo para postear como comment de GitHub
4. Termina con `## Key Learnings:` listando descubrimientos no-obvios

Plantilla (ajustar por perspectiva):

```
Revisa el <KIND> #<NUM> de este repo. El contexto completo (title, body, files, diff) esta en:

    /tmp/cvm-check-<NUM>-context.md

Tu perspectiva es **<perspectiva>**. Enfocate en eso y evita comentar cosas fuera de tu angulo.

Output: markdown listo para postear como comment en GitHub. Estructura sugerida:
- Resumen de 1-2 lineas
- Hallazgos concretos (con referencias a archivo:linea cuando aplique)
- Recomendaciones accionables
- Riesgos si no se atienden

Termina con una seccion `## Key Learnings:` listando descubrimientos no-obvios.
```

Para codex y gemini, escribir el prompt en un archivo temporal con `Write` tool:

```
/tmp/cvm-check-<NUM>-prompt-codex.txt
/tmp/cvm-check-<NUM>-prompt-gemini.txt
```

### Paso 6: Lanzar agentes en paralelo

Todos los agentes en el MISMO mensaje, en paralelo:

- **opus**: `Agent(subagent_type: "general-purpose", model: "opus", description: "check #<NUM> arq", prompt: <prompt>)`
- **codex**: Bash `codex exec "$(cat /tmp/cvm-check-<NUM>-prompt-codex.txt)" 2>&1`
- **gemini**: Bash `gemini -p "$(cat /tmp/cvm-check-<NUM>-prompt-gemini.txt)" 2>&1`

Sin timeout. Si un agente falla (error, exit != 0, output vacio), registrarlo en `FAILED` y continuar con los demas.

### Paso 7: Postear cada review como comment

Para cada agente que completo:

1. Escribir el cuerpo del comment con `Write` tool a `/tmp/cvm-check-<NUM>-<agente>.md` con este header:

```markdown
## Review: <agente> (<perspectiva>)

<contenido del agente>
```

2. Verificar tamano del archivo. Si supera 60KB, truncar el contenido del agente y agregar al final:
   `\n\n_[review truncada: supero 60KB, contenido completo disponible localmente]_`

3. Postear:

```bash
# Si KIND=pr:
gh pr comment "$NUM" --body-file /tmp/cvm-check-<NUM>-<agente>.md
# Si KIND=issue:
gh issue comment "$NUM" --body-file /tmp/cvm-check-<NUM>-<agente>.md
```

Capturar la URL del comment retornada por `gh` (stdout de `gh pr comment` / `gh issue comment` imprime la URL del comment).

Si el post falla, registrar en `FAILED_POST` y continuar.

### Paso 8: Reporte final

Mostrar al usuario:

```
Review de <KIND> #<NUM> completa.

Comments posteados:
- <agente1> (<perspectiva>): <url>
- <agente2> (<perspectiva>): <url>

Agentes que fallaron (si hubo): <lista con motivo breve>
Posts que fallaron (si hubo): <lista>
```

Si todos fallaron, dejarlo explicito: "Ninguna review se posteo."

No ejecutar `/r` automaticamente — las reviews viven en GitHub, no en auto-memory.

## MUST DO
- Parsear input aceptando numero pelado, `pr N`, `issue N` y URLs completas de PR o issue
- Detectar tipo automaticamente con fallback (`gh pr view` primero, luego `gh issue view`) cuando no viene forzado
- Fetchear contexto con `gh <pr|issue> view --json` + `gh pr diff` (solo PR) y escribirlo con `Write` a `/tmp/cvm-check-<NUM>-context.md`
- Preguntar agentes con prompt `[O/c/g/a]` y default explicito (Opus)
- Verificar disponibilidad de codex y gemini antes de ofrecerlos
- Armar prompts diferenciados por perspectiva (opus=arq, codex=correctness, gemini=legibilidad)
- Pasar el path del contexto a cada agente — NUNCA inline
- Lanzar todos los agentes en paralelo en el mismo mensaje
- Postear cada review como comment separado con header `## Review: <agente> (<perspectiva>)`
- Usar `--body-file` para postear, nunca heredoc ni interpolacion shell
- Truncar reviews >60KB y anotar la truncacion
- Reportar URLs de comments posteados y fallos al final
- Seguir sin abortar si un agente falla: reportar el fallo al final

## MUST NOT DO
- No pasar contenido del diff inline en comandos shell ni en prompts
- No interpolar $ARGUMENTS ni texto del usuario en comandos con double quotes
- No lanzar agentes secuencialmente — siempre paralelo
- No sintetizar las reviews en un unico comment (fuera de alcance)
- No persistir reviews en auto-memory (viven en GitHub)
- No ejecutar `/r` al final
- No soportar GitLab/Bitbucket — solo GitHub via `gh`
- No ofrecer agentes no disponibles
- No agregar timeout

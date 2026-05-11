# Harness Profile

Profile orientado a harness engineering: define specs y planes reutilizables a partir de historias de usuario, con un ciclo de refinamiento interactivo antes de persistirlos en GitHub.

## Skills

| Skill | Que hace |
|-------|----------|
| `/clarify` | Refina iterativamente asunciones sobre un issue de GitHub o un prompt libre. Lista asunciones tagueadas por temperatura (`[directa]/[media]/[especulativa]`), deja al usuario marcar cuales clarificar, pregunta una por una con multiple-choice (4 + "otra") y barra de progreso. Persiste appendeando una seccion "Clarificaciones" al issue existente o creando uno nuevo si vino un prompt. Sin label propio. |
| `/hs-spec` | Wrapper sobre `/clarify` para definir specs desde una historia de usuario: fuerza modo prompt, filtra asunciones a no-tecnicas/funcionales, usa estructura de body Historia/Asunciones/Criterios/Notas, y aplica label `entity:spec`. Para refinar un issue ya existente, usar `/clarify` directo. |
| `/hs-plan` | A partir de un issue de spec (`entity:spec`), redacta un plan de implementacion, lista las asunciones tecnicas/de implementacion, refina las que el usuario marca como incorrectas, y crea un PR en GitHub con un `.md` en `.harness/plans/<N>-<slug>.md` y label `entity:plan`. |
| `/hs-code-loop` | A partir de un PR con label `entity:plan`, ejecuta el plan iterativamente delegando al agent `hs-code-executor` y al agent `hs-code-validator`. Auto-detecta si arrancar por exec o validate (labels primero, fallback a heuristica del diff). Aplica labels de estado y persiste el feedback como comment del PR. Default 5 iteraciones, configurable con `--max N`. |
| `/hs-code-exec` | Una sola pasada de implementacion sobre un PR con label `entity:plan`. Wrapper thin sobre el agent `hs-code-executor`. Aplica label `code:exec` al final. Sin validacion. |
| `/hs-code-validate` | Una sola pasada de validacion sobre un PR con label `entity:plan`. Wrapper thin sobre el agent `hs-code-validator`. Aplica label `code:passed` o `code:failed` y postea el feedback como comment del PR. Sirve para auditar PRs propios o ajenos sin tocar codigo. |
| `/hs-recover` | Adopta issues y PRs preexistentes al workflow harness: detecta el tipo de entidad, diagnostica labels y artefactos, genera `.harness/plans/<N>-<slug>.md` si falta, commitea y pushea al branch del PR, aplica `entity:spec` o `entity:plan`, y sugiere el siguiente comando del workflow. |
| `/hs-auto` | Pipeline end-to-end autonomo desde prompt, issue o PR hasta PR validado. No depende de otros skills `/hs-*` ni de labels: redacta spec, plan y orquesta el loop exec/validate inline, delegando solo en los agents `hs-code-executor` y `hs-code-validator`. En modo PR arranca siempre por validate y sintetiza `plan_text` in-memory si el branch no tiene `.harness/plans/*.md`. Aborta solo ante errores duros o prompts demasiado vagos para una spec minima. `--max N` (default 5). |

## Agents

### Primary (Tab-selectable)

| Agent | Que hace |
|-------|----------|
| `detach` | Antesala para tareas ruidosas. El usuario lo elige con Tab cuando quiere preservar el contexto del agente primario anterior. Bias fuerte a delegar trabajo pesado a subagents via Task tool y devolver solo un bloque `## Result` de 3 lineas (status / summary / artifacts). |

### Subagents (invocados via Task)

| Agent | Que hace |
|-------|----------|
| `hs-code-executor` | Implementa pasos de un plan (`.harness/plans/<N>-<slug>.md`) sobre la branch del PR. Antes de empezar carga contexto rico del PR (body, comments, reviews, review comments line-level, ultimo feedback del validator, spec issue body). Build/typecheck minimo + 1-3 unit tests acotados. Commit + push. Sin WebFetch/WebSearch. |
| `hs-code-validator` | Valida un PR de plan: carga contexto del PR (mismo set que el executor), espera `gh pr checks`, corre suite completa local, contrasta diff vs cada paso/archivo/riesgo del plan. Emite verdict PASS/FAIL + feedback accionable. Sin Edit/Write. |

## Modelo de ejecucion OpenCode

Los 6 workflows se ejecutan como **skills primarios**. Solo las partes autonomas de implementacion y validacion se delegan a subagents.

| Workflow | Entry point primario | Delegacion a subagent |
|----------|----------------------|------------------------|
| Clarify | `/clarify` | No. Interactivo multi-turno en el orquestador principal. |
| Spec | `/hs-spec` | No. Interactivo multi-turno en el orquestador principal (wrapper sobre `/clarify`). |
| Plan | `/hs-plan` | No. Interactivo multi-turno en el orquestador principal. |
| Exec | `/hs-code-exec` | Si, delega a `hs-code-executor`. |
| Validate | `/hs-code-validate` | Si, delega a `hs-code-validator`. |
| Loop | `/hs-code-loop` | Si, orquesta `hs-code-executor` y `hs-code-validator`. |
| Auto | `/hs-auto` | Si, solo para exec y validate; spec, plan y el loop se redactan/orquestan inline en el orquestador principal. No invoca a `/hs-spec`, `/hs-plan` ni a los wrappers `/hs-code-*`. |

No crear subagents separados para `/hs-spec` o `/hs-plan` salvo que el flujo deje de ser interactivo. Esos skills necesitan refinar asunciones con el usuario antes de persistir issue/PR.
`/hs-auto` es la excepcion no interactiva: acepta defaults seguros de spec/plan, no aplica labels harness y solo frena ante errores duros o un prompt demasiado vago para redactar una spec minima.

## Labels de estado (aplicados por los skills `/hs-code-*`)

| Label | Significado | Aplicado por |
|-------|-------------|--------------|
| `entity:spec` | Issue es una spec del workflow harness | `/hs-spec` |
| `entity:plan` | PR es un plan de implementacion | `/hs-plan` |
| `code:exec` | Ultima operacion fue exec sobre el PR; pendiente de validar | `/hs-code-loop`, `/hs-code-exec` |
| `code:passed` | Ultimo validate emitio PASS - PR listo para review/merge | `/hs-code-loop`, `/hs-code-validate` |
| `code:failed` | Ultimo validate emitio FAIL - feedback persistido como PR comment con marker `<!-- hs-code-validate:feedback ... -->` | `/hs-code-loop`, `/hs-code-validate` |

Los tres labels `code:*` son **mutuamente exclusivos** (cuando uno se aplica, los otros se quitan). Sirven como senal externa del estado del PR y como fallback de auto-detect para el loop.

## Reglas

- Sacar ambiguedades - si algo puede interpretarse de mas de una forma, clarificar antes de actuar
- Preguntas de desambiguacion SIEMPRE en formato multiple choice (opciones numeradas + opcion libre "otra")
- No agregar lo que no se pidio
- No especular sobre codigo sin leerlo
- Respuestas cortas y directas
- macOS - evitar flags GNU-only (`grep -P`). Usar `grep -E`

## Persistencia

- Skills persisten output en GitHub (issues con labels) cuando aplica.
- La copia desplegada de AGENTS.md (`~/.config/opencode/AGENTS.md` o `$OPENCODE_CONFIG_DIR/AGENTS.md`) NUNCA se modifica en runtime.
- Este archivo (`profiles/harness/opencode/AGENTS.md`) es la fuente del profile y se edita por PR.

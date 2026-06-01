# Harness Profile

Profile orientado a harness engineering: define specs y planes reutilizables a partir de historias de usuario, con un ciclo de refinamiento interactivo antes de persistirlos en GitHub.

## Skills

| Skill | Que hace |
|-------|----------|
| `/clarify` | Refina iterativamente asunciones sobre un issue de GitHub o un prompt libre. Lista asunciones tagueadas por temperatura (`[directa]/[media]/[especulativa]`), deja al usuario marcar cuales clarificar, pregunta una por una con multiple-choice (4 + "otra") y barra de progreso. **No depende de GitHub**: si hay repo gh, ofrece persistir al final (default: no) appendeando "Clarificaciones" al issue o creando uno nuevo; si no, muestra el resultado inline. En modo issue sin repo, pide pegar el body manualmente. Sin label propio. |
| `/hs-spec` | Wrapper sobre `/clarify` ejecutado dos veces (funcional + tecnica) para definir spec y plan desde una historia de usuario. Fase A: refina asunciones no-tecnicas y crea issue con label `entity:spec`. Tras un opt-out (default: seguir), Fase B refina asunciones tecnicas y Fase C crea branch `hs-plan/<N>`, archivo `.harness/plans/<N>-<slug>.md`, commit + push + PR con `Closes #<N>` y label `entity:plan`. Para refinar un issue ya existente, usar `/clarify` directo. |
| `/hs-auto` | Pipeline end-to-end autonomo desde prompt, issue o PR hasta PR validado. No depende de otros skills `/hs-*` ni de labels: redacta spec, plan y orquesta el loop exec/validate inline, delegando solo en los agents `hs-code-executor` y `hs-code-validator`. En modo PR arranca siempre por validate y sintetiza `plan_text` in-memory si el branch no tiene `.harness/plans/*.md`. Aborta solo ante errores duros o prompts demasiado vagos para una spec minima. `--max N` (default 5). |
| `/che-run` | Wrapper sobre `che run <slug> [prompt]` del CLI `che`. Hace preflight con `che doctor`, lista pipelines disponibles (`~/.che/pipelines/` + builtin `che-funnel`) si falta slug, corre el pipeline en foreground con timeout amplio, y reporta status leyendo el `manifest.yaml` del run mas reciente en `~/.che/runs/<slug>/`. Si el run fallo, vuelca las ultimas 30 lineas del stderr del primer step que fallo. Solo cubre `che run`; no toca `dash`, `upgrade` ni otros subcomandos. |
| `/new-skill` | Scaffolder de skills nuevos para el repo `cvm`. Pide descripcion en lenguaje natural, pregunta profile + harness(es) destino, redacta el `SKILL.md` siguiendo las convenciones de cada harness (frontmatter solo en opencode, `$ARGUMENTS` solo en claude), agrega la fila correspondiente en la tabla de skills del doc del profile (`CLAUDE.md` / `AGENTS.md`), y commitea + pushea a `main` sin preguntar. Aborta si no estas en la raiz del repo cvm o si hay cambios sin commitear. |
| `/diagnose` | Loop disciplinado para diagnosticar bugs duros y regresiones de performance: reproducir, minimizar, hipotetizar, instrumentar, fixear y dejar un regression test. La fase 1 ("construir feedback loop deterministico en menos de 30s") es la principal y bloquea el avance. Sanity check al inicio para evitar el loop en bugs obvios. Cierra con regression test verificado (falla con codigo viejo, pasa con el nuevo). |
| `/zoom-out` | Pide al orquestador subir una capa de abstraccion sobre un area de codigo: mapa de modulo, callers (entradas), dependencias (salidas), decisiones arquitectonicas y proximo paso sugerido. Usa el vocabulario del proyecto si hay glosario. Solo lectura, no modifica nada. Para entrar a codigo desconocido o entender como una pieza encaja en el sistema mayor antes de tocarla. |
| `/explain` | Explica un concepto, snippet, error o decision tecnica en 3 niveles de abstraccion crecientes (ELI5 / Dev junior / Experto) en una sola respuesta. Hibrido: usa el argumento si lo hay, si no toma lo ultimo relevante del contexto y lo cita. One-shot: la siguiente respuesta vuelve al estilo normal del thread. Cada nivel debe sumar info que el anterior no tenia; sin mezclar analogias en Nivel 3 ni trade-offs en Nivel 1. |
| `/handoff` | Compacta la conversacion actual en un documento de handoff (`$TMPDIR/cvm-handoff-<TS>.md`) para que otro agente o sesion la continue. Resume contexto, decisiones, trabajo en curso y proximos pasos. Referencia PRs/issues/plans/commits por URL o path en vez de duplicar su contenido. Redacta info sensible (keys, passwords, PII). Sugiere 1 a 3 skills del profile actual para el proximo agente. No commitea nada. |
| `/detach` | Ejecuta una tarea ruidosa en un subagent aislado (`general`) y devuelve solo un bloque `## Result` corto al main thread. Usar para preservar contexto ante busquedas, lecturas o comandos largos. |
| `/delegator` | Modo persistente de delegacion: deriva a subagents operaciones ruidosas y conserva inline solo conversacion, judgment, lecturas chicas y ediciones puntuales. Se apaga con `/delegator off` o modo normal. |
| `/arch-review` | Audita la codebase buscando deepening opportunities y shallow modules con deletion test. Genera un reporte HTML autocontenido en `$TMPDIR`, lo abre en browser y no persiste cambios en el repo salvo bootstrap vacio si el usuario lo elige. |
| `/herdr-detach` | Deriva un prompt a `claude`, `opencode` o `codex` en un pane de `herdr`, con modo fire-and-forget o `--wait`. Ancla `--here` al pane origen via `HERDR_PANE_ID` y deja el pane abierto. |
| `/herdr-pon` | Arma un loop ping-pong automatico entre el pane actual y un validador `claude` u `opencode` en `herdr`, con `state.json`, `output.md`, `ping.sh` y cap de rondas. |

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

Los workflows se ejecutan como **skills primarios**. Solo las partes autonomas de implementacion y validacion se delegan a subagents.

| Workflow | Entry point primario | Delegacion a subagent |
|----------|----------------------|------------------------|
| Clarify | `/clarify` | No. Interactivo multi-turno en el orquestador principal. |
| Spec + Plan | `/hs-spec` | No. Interactivo multi-turno en el orquestador principal (wrapper sobre `/clarify` ejecutado dos veces: funcional + tecnica). |
| Auto | `/hs-auto` | Si, solo para exec y validate; spec, plan y el loop se redactan/orquestan inline en el orquestador principal. No invoca a `/hs-spec`. |

No crear subagents separados para `/hs-spec` salvo que el flujo deje de ser interactivo. El skill necesita refinar asunciones con el usuario antes de persistir issue y PR.
`/hs-auto` es la excepcion no interactiva: acepta defaults seguros de spec/plan, no aplica labels harness y solo frena ante errores duros o un prompt demasiado vago para redactar una spec minima.

## Labels de estado

| Label | Significado | Aplicado por |
|-------|-------------|--------------|
| `entity:spec` | Issue es una spec del workflow harness | `/hs-spec` |
| `entity:plan` | PR es un plan de implementacion | `/hs-spec` |

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

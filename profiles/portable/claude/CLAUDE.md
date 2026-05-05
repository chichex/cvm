# Portable Profile

Profile orientado a definir specs portables y reutilizables a partir de historias de usuario, con un ciclo de refinamiento interactivo antes de persistirlas en GitHub.

## Skills

| Skill | Que hace |
|-------|----------|
| `/portable-spec` | A partir de una historia de usuario, redacta una spec, lista las asunciones no-tecnicas/funcionales, refina las que el usuario marca como incorrectas (preguntas one-by-one con barra de progreso), y crea un issue en GitHub con label `entity:spec`. |
| `/portable-plan` | A partir de un issue de spec (`entity:spec`), redacta un plan de implementacion, lista las asunciones tecnicas/de implementacion, refina las que el usuario marca como incorrectas, y crea un PR en GitHub con un `.md` en `.portable/plans/<N>-<slug>.md` y label `entity:plan`. |
| `/portable-code-loop` | (Claude Code only) A partir de un PR con label `entity:plan`, ejecuta el plan iterativamente delegando al subagent `portable-code-executor` (Sonnet) y al subagent `portable-code-validator` (Opus). Auto-detecta si arrancar por exec o validate (labels primero, fallback a heurística del diff). Aplica labels de estado y persiste el feedback como comment del PR. Default 5 iteraciones, configurable con `--max N`. |
| `/portable-code-exec` | (Claude Code only) Una sola pasada de implementacion sobre un PR con label `entity:plan`. Wrapper thin sobre el subagent `portable-code-executor` (Sonnet). Aplica label `code:exec` al final. Sin validacion. |
| `/portable-code-validate` | (Claude Code only) Una sola pasada de validacion sobre un PR con label `entity:plan`. Wrapper thin sobre el subagent `portable-code-validator` (Opus). Aplica label `code:passed` o `code:failed` y postea el feedback como comment del PR. Sirve para auditar PRs propios o ajenos sin tocar codigo. |
| `/portable-recover` | Adopta issues y PRs preexistentes al workflow portable: detecta el tipo de entidad, diagnostica labels y artefactos, genera `.portable/plans/<N>-<slug>.md` si falta, commitea y pushea al branch del PR, aplica `entity:spec` o `entity:plan`, y sugiere el siguiente comando del workflow. |

## Subagents (Claude Code only)

| Subagent | Que hace |
|----------|----------|
| `portable-code-executor` | Implementa pasos de un plan (`.portable/plans/<N>-<slug>.md`) sobre la branch del PR. Antes de empezar carga contexto rico del PR (body, comments, reviews, review comments line-level, ultimo feedback del validator, spec issue body). Build/typecheck minimo + 1-3 unit tests acotados. Commit + push. Modelo: Sonnet. Sin WebFetch/WebSearch. |
| `portable-code-validator` | Valida un PR de plan: carga contexto del PR (mismo set que el executor), espera `gh pr checks`, corre suite completa local, contrasta diff vs cada paso/archivo/riesgo del plan. Emite verdict PASS/FAIL + feedback accionable. Modelo: Opus. Sin Edit/Write. |

## Labels de estado (aplicados por los skills `/portable-code-*`)

| Label | Significado | Aplicado por |
|-------|-------------|--------------|
| `entity:spec` | Issue es una spec del workflow portable | `/portable-spec` |
| `entity:plan` | PR es un plan de implementacion | `/portable-plan` |
| `code:exec` | Ultima operacion fue exec sobre el PR; pendiente de validar | `/portable-code-loop`, `/portable-code-exec` |
| `code:passed` | Ultimo validate emitio PASS — PR listo para review/merge | `/portable-code-loop`, `/portable-code-validate` |
| `code:failed` | Ultimo validate emitio FAIL — feedback persistido como PR comment con marker `<!-- portable-code-validate:feedback ... -->` | `/portable-code-loop`, `/portable-code-validate` |

Los tres labels `code:*` son **mutuamente exclusivos** (cuando uno se aplica, los otros se quitan). Sirven como señal externa del estado del PR y como fallback de auto-detect para el loop.

## Reglas

- Sacar ambiguedades — si algo puede interpretarse de mas de una forma, clarificar antes de actuar
- Preguntas de desambiguacion SIEMPRE en formato multiple choice (opciones numeradas + opcion libre "otra")
- No agregar lo que no se pidio
- No especular sobre codigo sin leerlo
- Respuestas cortas y directas
- macOS — evitar flags GNU-only (`grep -P`). Usar `grep -E`

## Persistencia

- Skills persisten output en GitHub (issues con labels) cuando aplica.
- La copia desplegada de CLAUDE.md (`~/.claude/CLAUDE.md`) NUNCA se modifica en runtime.
- Este archivo (`profiles/portable/CLAUDE.md`) es la fuente del profile y se edita por PR.

# Plan: Agregar /portable-auto: pipeline end-to-end de spec a PR validado

Refs #68 · https://github.com/chichex/cvm/issues/68

## Contexto

`/portable-auto` es un nuevo skill del profile portable que orquesta el pipeline existente `/portable-spec` → `/portable-plan` → `/portable-code-loop` desde un único punto de entrada. Acepta como input un prompt libre, un issue normal, o un issue ya etiquetado con `entity:spec`. Detecta el tipo de input automáticamente, evalúa el "tamaño" del cambio en dos puntos del flow (post-spec y post-plan), emite advertencias bloqueantes cuando el cambio supera thresholds, y al final clasifica el resultado en categorías cualitativas. v1 es **Claude Code only** (asunción 26 del spec).

## Objetivo

Proveer un comando único que lleve a un usuario desde un prompt o issue hasta un PR validado, reusando los skills `/portable-*` ya desplegados, pausando solo en advertencias bloqueantes (cambio grande, input ambiguo, asunciones sin refinar) y emitiendo un veredicto cualitativo del fit al final.

## Approach

Implementar el skill como un único `SKILL.md` en `profiles/portable/claude/skills/portable-auto/`, escrito en la misma forma de prosa que el resto de skills `/portable-*` (sin frontmatter YAML, primera línea = description). El orquestador (Claude principal) parsea el input, compone llamadas a los skills hijos via Skill tool, y entre fases ejecuta dos chequeos heurísticos (post-spec y post-plan) que lee parseando los artefactos ya creados (issue y plan `.md`). Los thresholds viven inline en el SKILL.md como sección documentada, sin archivo de config externo. Para preservar la simetría del directorio paralelo `opencode/skills/`, se crea allí un stub que aborta de inmediato declarando "v1 Claude Code only".

## Pasos

- [ ] Crear `profiles/portable/claude/skills/portable-auto/SKILL.md` con la prosa completa del orquestador (sin frontmatter YAML, primera línea = descripción, siguiendo la convención del profile portable claude).
- [ ] Implementar dentro del SKILL.md el parser de input (regex `^#?[0-9]+$` para número, `github.com/.+/issues/[0-9]+` para URL, cualquier otra cosa = prompt) y la pregunta de desambiguación multiple-choice cuando el input es ambiguo.
- [ ] Implementar la rama por tipo de input: prompt → invocar `/portable-spec`; issue sin `entity:spec` → cargar body como historia y llamar `/portable-spec`; issue con `entity:spec` → saltar a `/portable-plan`.
- [ ] Definir los thresholds default (`>10 asunciones`, `>8 pasos`, `>15 archivos`, `>15 asunciones sin refinar`) y documentarlos como sección `## Thresholds` en el SKILL.md.
- [ ] Implementar los chequeos heurísticos post-spec y post-plan: parsear secciones del issue/plan (`## Asunciones validadas`, `## Pasos`, `## Archivos afectados`) y comparar contra los thresholds.
- [ ] Implementar la advertencia bloqueante con tres respuestas (`continuar` / `abortar` / `ajustar`) y la rama "ajustar" que sugiere alternativas en prosa libre (dividir en specs más chicas, agregar más detalle, etc.).
- [ ] Implementar el cómputo del veredicto final basado en señales cuantitativas: cantidad de iteraciones del loop, presencia de `code:failed` intermedio (timeline API), y si alguna fase superó threshold de tamaño.
- [ ] Documentar en el SKILL.md las cuatro categorías de fit (`encajó limpio` / `encajó con fricción` / `encajó con riesgo residual` / `no encajó`) con sus criterios de clasificación.
- [ ] Crear `profiles/portable/opencode/skills/portable-auto/SKILL.md` como stub con frontmatter YAML que aborta inmediatamente con mensaje "v1 Claude Code only — corre el skill desde Claude Code".
- [ ] Actualizar `profiles/portable/CLAUDE.md` agregando una fila en la tabla de skills con la descripción de `/portable-auto`.
- [ ] Probar manualmente el skill end-to-end con un caso simple durante el code-loop (prompt → spec → plan → loop) y verificar el formato del output final.

## Archivos afectados

- `profiles/portable/claude/skills/portable-auto/SKILL.md` — crear — skill principal con la lógica completa.
- `profiles/portable/opencode/skills/portable-auto/SKILL.md` — crear — stub que aborta (preserva simetría del directorio).
- `profiles/portable/CLAUDE.md` — modificar — agregar fila en la tabla de skills.

## Riesgos

- El skill compone llamadas a otros skills via Skill tool (`/portable-spec`, `/portable-plan`, `/portable-code-loop`). Si el runtime de Claude Code no permite Skills anidados o tiene restricciones desconocidas, hay que caer a fallback de prosa pidiendo al usuario invocar manualmente cada paso. Validar durante code-loop.
- Los thresholds heurísticos pueden ser molestos si están mal calibrados; sin telemetría es difícil afinarlos en v1. Documentar valores explícitos para que el usuario los entienda al ver la advertencia.
- El profile portable ya tiene versiones opencode reales de `/portable-code-loop`/`/portable-code-exec`/`/portable-code-validate` (delegan a `agents` en lugar de subagents). La asunción 26 del spec ("Claude Code only por subagents") es parcialmente incorrecta — OpenCode podría ser viable, pero v1 igual lo deja para después.
- Parseo de secciones del issue/plan via regex sobre Markdown es frágil: si el formato del issue de spec o del `.md` del plan cambia (los skills `/portable-spec` y `/portable-plan` cambian de output), los chequeos pueden romper. Definir los markers explícitamente (`## Asunciones validadas`, `## Pasos`, `## Archivos afectados`).
- "Encajó con riesgo residual" como categoría requiere que el skill recuerde si tiró advertencia de tamaño en alguna fase — implica state persistence dentro de la conversación del orquestador, no en disco.
- Detección de `code:failed` intermedio depende de la API de timeline de GitHub (`gh api repos/<repo>/issues/<pr>/timeline`); si la respuesta cambia o es incompleta, el feedback puede clasificar mal.

## Out of scope

- Soporte funcional real de OpenCode (queda como stub que aborta hasta v2).
- Flags adicionales: `--max`, `--auto`, `--no-confirm`, `--threshold` (todos fuera de v1).
- Auto-tuning de thresholds basado en historial.
- Cleanup automático de artefactos parciales (issue/PR) si el usuario aborta — el usuario los hereda.
- Persistencia de estado en disco — todo vive en la conversación.
- Tests automatizados del skill (sigue la convención del profile, los SKILL.md no tienen suite).
- Telemetría / métricas de uso.
- Nuevos labels propios — el skill se apoya en los que aplican los hijos (`entity:spec`, `entity:plan`, `code:exec`, `code:passed`, `code:failed`).

## Asunciones tecnicas validadas

1. La implementación es 100% prosa en Markdown dentro de un único `SKILL.md`, igual que el resto de skills del profile portable. No hay código Go ni binario nuevo.
2. La composición de skills hijos se hace via Skill tool (invocando `/portable-spec`, `/portable-plan`, `/portable-code-loop` como skills anidados desde el orquestador).
3. El parseo de input vive como pseudocódigo de prosa dentro del SKILL.md (regex / heurísticas en bash + `gh`), no en un módulo Go aparte.
4. Detección de issue: regex `^#?[0-9]+$` o URL `github.com/.+/issues/[0-9]+`. Cualquier otra cosa = prompt libre.
5. Para detectar si un issue tiene `entity:spec`: `gh issue view N --json labels --jq '.labels[].name' | grep -Fxq entity:spec`.
6. Los thresholds default son: `>10 asunciones`, `>8 pasos`, `>15 archivos`, `>15 asunciones sin refinar`. Se documentan como sección `## Thresholds` en SKILL.md, sin archivo de config externo.
7. Las "asunciones" se cuentan parseando el body del issue de spec (sección `## Asunciones validadas` → líneas `^[0-9]+\.`).
8. Los "pasos" se cuentan parseando el `.md` del plan (sección `## Pasos` → líneas `^- \[ \]`).
9. Los "archivos" se cuentan parseando el `.md` del plan (sección `## Archivos afectados` → líneas que arrancan con `` - ` ``).
10. La advertencia de tamaño se evalúa en dos puntos fijos: post-`/portable-spec` (antes de `/portable-plan`) y post-`/portable-plan` (antes de `/portable-code-loop`).
11. El feedback final cuantitativo se computa leyendo: cantidad de iteraciones del loop (history del PR + comments del validator), si hubo `code:failed` intermedio (event log de labels via `gh api .../timeline`), y si alguna fase tiró advertencia de tamaño (estado en conversación).
12. Categorías de fit final: `encajó limpio` (≤1 iteración, sin code:failed, sin advertencia de tamaño), `encajó con fricción` (>1 iter o code:failed presente, pero terminó en PASS), `encajó con riesgo residual` (PASS pero alguna fase superó threshold), `no encajó` (loop agotó iteraciones sin PASS).
13. v1 crea AMBOS `SKILL.md` (claude completo + opencode stub) para preservar la simetría del directorio del profile, pero el opencode aborta de entrada con mensaje "Claude Code only".
14. `/portable-auto` NO toca el branch ni el working tree directamente — eso queda 100% delegado a `/portable-plan` y `/portable-code-loop`.
15. La interacción con el usuario (multiple-choice, confirmaciones de "ajustar") es pseudocódigo de prosa, sin scripts bash interactivos.
16. Los outputs intermedios (URL del issue de spec, URL del PR de plan) se muestran via texto del orquestador apenas los devuelve el skill hijo.
17. La opción "ajustar" propone alternativas en prosa libre (no reglas formales) — el orquestador improvisa según contexto.
18. El SKILL.md de claude NO usa frontmatter YAML (primera línea de prosa = description); el de opencode SÍ usa frontmatter YAML (convención observada en el profile portable).
19. `/portable-auto` no instala nada — vive en `profiles/portable/claude/skills/portable-auto/SKILL.md` y se distribuye via `cvm install` con los demás (renderPortableCollection ya copia todo).
20. La documentación en `profiles/portable/CLAUDE.md` (fuente del profile, no runtime) se actualiza con una fila más en la tabla de skills. NO se toca `~/.claude/CLAUDE.md`.
21. No se agregan tests automatizados al repo. La verificación end-to-end es manual durante el code-loop.
22. Si el runtime de Claude Code no soporta Skills anidados, el SKILL.md cae a fallback: prosa pidiendo al usuario que ejecute manualmente cada `/portable-*` y vuelva a `/portable-auto` con el output. (Asunción a verificar en code-loop.)
23. El skill NO aplica labels propios — se apoya en los labels que aplican los skills hijos (`entity:spec`, `entity:plan`, `code:exec`, `code:passed`, `code:failed`).
24. El skill NO escribe a auto-memory ni a archivos persistentes fuera de los que generan los skills hijos.
25. Las llamadas a `gh` son siempre con `--json + --jq` para parseo robusto, nunca scraping de output texto.
26. Si `/portable-code-loop` agota iteraciones, `/portable-auto` toma el último estado (label del PR + último comment del validator) como input para el feedback final, sin reintentar.
27. Para detectar code:failed intermedio: `gh api repos/<repo>/issues/<pr>/timeline --paginate` y filtrar eventos `labeled` con `code:failed`. Asume que la timeline API expone history de labels.

---

_Plan generado por `/portable-plan` a partir de #68._

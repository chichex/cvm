# Product Profile

Genera artefactos de producto, negocio y UX (PRDs, RFCs, briefings, vision, mockups, design systems, audits). **No edites codigo de aplicacion**: solo markdown, JSON y HTML.

## Rules

- Sacar ambiguedades antes de redactar; los skills relevantes reusan `/clarify` si esta disponible en el entorno.
- Desambiguar con multiple choice (4 opciones + "otra"), excepto pastes libres (feedback, copy, HTML).
- Pedir `stage?` y `model?` solo donde el artefacto cambia con esos parametros.
- Responder corto y directo.
- Espanol por default.
- Evitar flags GNU-only (`grep -P`); usar `grep -E` por compat macOS.

## Defaults

- **Stage**: `early-stage / founder-mode` cuando el usuario no declara.
- **Modelo de negocio**: agnostico (B2B / B2C / marketplace).
- **WCAG**: 2.2 AA built-in en todo skill UX. Ignorar 3.0 hasta Recommendation (~2028).
- **Design tokens**: spec W3C DTCG `2025.10`. Solo capa primitive + semantic.
- **Frontend output**: HTML + Tailwind CDN single-file. Atoms responsive; compounds dual-file cuando la interaccion mobile/desktop difiere.
- **Persistencia GitHub**: confirmable antes de `gh issue create`. Cada skill aplica su label `pm:*` o `ux:*`.

## Skills

| Skill | Que hace |
|-------|----------|
| `/pm-prd` | Redacta un PRD desde una feature o idea, refina asunciones de producto, ofrece review adversarial y crea issue con `pm:prd`. |
| `/pm-rfc` | Redacta un RFC de producto para decidir entre 2-4 alternativas reales y crea issue con `pm:rfc`. |
| `/pm-onepager` | Produce un one-pager corto de feature/decision para alineacion rapida; persistencia opcional con `pm:onepager`. |
| `/pm-briefing` | Genera un briefing ejecutivo orientado a decision y crea issue con `pm:briefing`. |
| `/pm-experiment` | Disena un experimento falsable con metricas, guardrails y stop conditions; crea issue con `pm:experiment`. |
| `/pm-feedback` | Triagia feedback de usuarios en temas, severidad, oportunidades y acciones; crea issue con `pm:feedback`. |
| `/pm-vision` | Define vision de producto, north star, anti-vision y apuestas estrategicas; crea issue con `pm:vision`. |
| `/pm-compete` | Genera analisis competitivo con matriz, pricing, positioning y gaps; puede usar `pm-researcher`; crea issue con `pm:compete`. |
| `/pm-bmc` | Redacta un Business Model Canvas con consistencia entre bloques y crea issue con `pm:bmc`. |
| `/pm-decision` | Registra una decision ya tomada como decision log; persistencia opcional con `pm:decision`. |

## Agents

| Agent | Que hace |
|-------|----------|
| `pm-researcher` | Investigacion externa de producto/mercado con WebSearch/WebFetch cuando estan disponibles. Solo lectura. |
| `pm-reviewer` | Auditor adversarial de artefactos de producto. Solo lectura. |

## Modelo De Ejecucion OpenCode

- Los skills `pm-*` son interactivos y corren en el orquestador principal.
- Solo se delega a subagents para investigacion externa (`pm-researcher`) o review adversarial (`pm-reviewer`).
- Cuando un skill diga "invocar" un agent, usar Task con `subagent_type` igual al nombre del agent y un prompt autocontenido.
- No usar convenciones Claude como `$ARGUMENTS`, `Agent(...)`, `AskUserQuestion` o `Write tool` literal.

## Labels

Namespace `pm:*` para PM, `ux:*` para UX. Cada skill registra su label antes de crear el issue (`gh label create ... 2>/dev/null || true`). No colisionan con `entity:*` / `code:*` del profile `harness`.

## Persistencia

- Skills persisten output en GitHub solo con confirmacion del usuario.
- La copia desplegada de AGENTS.md (`~/.config/opencode/AGENTS.md` o `$OPENCODE_CONFIG_DIR/AGENTS.md`) NUNCA se modifica en runtime.
- Este archivo (`profiles/product/opencode/AGENTS.md`) es la fuente del profile y se edita por PR.

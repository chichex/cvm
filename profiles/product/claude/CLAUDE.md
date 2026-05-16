# Product Profile

Genera artefactos de producto, negocio y UX (PRDs, RFCs, briefings, vision, mockups, design systems, audits). **No edites codigo de aplicacion** — solo markdown, JSON y HTML.

## Rules

- Sacar dudas antes de redactar — cada skill incluye una fase de clarificación inline.
- Desambiguar con multiple choice (4 opciones + "otra"), excepto pastes libres (feedback, copy, HTML).
- Pedir `etapa?` y `tipo de negocio?` solo donde el artefacto cambia con esos parametros.
- Responder corto y directo.
- Lenguaje simple — evitar jerga de producto innecesaria (mantener nombres de artefactos: PRD, RFC, BMC).
- Español por default.
- Evitar flags GNU-only (`grep -P`); usar `grep -E` por compat macOS.

## Defaults

- **Etapa**: `etapa temprana` cuando el usuario no declara.
- **Tipo de negocio**: agnostico (para empresas / consumidor final / marketplace).
- **WCAG**: 2.2 AA built-in en todo skill UX. Ignorar 3.0 hasta Recommendation (~2028).
- **Design tokens**: spec W3C DTCG (Design Tokens Community Group) `2025.10`. Solo capa primitive + semantic.
- **Frontend output**: HTML + Tailwind CDN single-file. Atoms (atomos) responsive; compounds (compuestos) dual-file (archivo separado por viewport) cuando la interaccion mobile/desktop difiere (modal/sheet, nav/drawer, table/card-list).

## Output

Todos los skills guardan archivos locales — nunca crean issues ni dependen de GitHub.

- **PM skills** (`.md`): `.pm/<skill>/<slug>.md` (ej. `.pm/pm-prd/exportar-reportes-csv.md`).
- **UX skills** (`.md` o directorios con `.html`):
  - `/ux-critique` → `.ux/critique/<slug>.md`
  - `/ux-a11y-audit` → `.ux/a11y/<slug>.md`
  - `/ux-copy-review` → `.ux/copy/<slug>.md`
  - `/ux-propose` → `.ux/proposals/<slug>/variant-<N>-<nombre>.html` + `README.md`
  - `/ux-components` → `.ux/components/<slug>/<component>.html` + `README.md`
  - `/ux-extract` → `.ux/extract/<slug>/tokens.json` + `components.md`
  - `/ux-design-system` → `.ux/design-system/tokens.json` + `tailwind.config.js` + `README.md` (singleton, sin slug)
- **Slug**: derivado del titulo del artefacto, kebab-case, max 40 chars.
- **Confirmar antes de escribir**: cada skill pide confirmacion al usuario antes de guardar.
- **Crear directorio**: si el path no existe, crear con `mkdir -p` antes de escribir.

## Skills

Listados en el auto-listing del system prompt. Dos namespaces: `pm-*` (10 skills de producto/negocio) y `ux-*` (7 skills de UX). Detalles canonicos en cada `SKILL.md`.

## Subagents

- `pm-researcher` (Sonnet, WebSearch/WebFetch) — investigacion externa, solo lectura.
- `pm-reviewer` (Opus, solo lectura) — auditor critico de artefactos.

Reutilizables desde cualquier skill `pm-*` o `ux-*` cuando aplica.

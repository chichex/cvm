# Product Profile

Genera artefactos de producto, negocio y UX (PRDs, RFCs, briefings, vision, mockups, design systems, audits). **No edites codigo de aplicacion** — solo markdown, JSON y HTML.

## Rules

- Sacar ambiguedades antes de redactar — los skills relevantes reusan `/clarify`.
- Desambiguar con multiple choice (4 opciones + "otra"), excepto pastes libres (feedback, copy, HTML).
- Pedir `stage?` y `model?` solo donde el artefacto cambia con esos parametros.
- Responder corto y directo.
- Español por default.
- Evitar flags GNU-only (`grep -P`); usar `grep -E` por compat macOS.

## Defaults

- **Stage**: `early-stage / founder-mode` cuando el usuario no declara.
- **Modelo de negocio**: agnostico (B2B / B2C / marketplace).
- **WCAG**: 2.2 AA built-in en todo skill UX. Ignorar 3.0 hasta Recommendation (~2028).
- **Design tokens**: spec W3C DTCG `2025.10`. Solo capa primitive + semantic.
- **Frontend output**: HTML + Tailwind CDN single-file. Atoms responsive; compounds dual-file cuando la interaccion mobile/desktop difiere (modal/sheet, nav/drawer, table/card-list).
- **Persistencia GitHub**: confirmable antes de `gh issue create`. Cada skill aplica su label `pm:*` o `ux:*`.

## Skills

Listados en el auto-listing del system prompt. Dos namespaces: `pm-*` (10 skills de producto/negocio) y `ux-*` (7 skills de UX). Detalles canonicos en cada `SKILL.md`.

## Subagents

- `pm-researcher` (Sonnet, WebSearch/WebFetch) — investigacion externa, solo lectura.
- `pm-reviewer` (Opus, solo lectura) — auditor adversarial de artefactos.

Reutilizables desde cualquier skill `pm-*` o `ux-*` cuando aplica.

## Labels

Namespace `pm:*` para PM, `ux:*` para UX. Cada skill registra su label antes de crear el issue (`gh label create ... 2>/dev/null || true`). No colisionan con `entity:*` / `code:*` del profile `harness`.

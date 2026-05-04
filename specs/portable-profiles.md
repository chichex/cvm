# Portable Profile Contract v0.1 Experimental

## Goal

Define the small profile surface that `cvm` owns across harnesses. Portable assets are authored as `cvm` concepts first, then rendered or copied into Claude, OpenCode, Codex, or another harness when that harness has a compatible equivalent.

This is an experimental v0.1 contract. The current implementation covers manifest parsing plus activation-time rendering for portable instructions, skills, and agents. Treat the layout as subject to tightening while more non-Claude renderers consume it.

## Portable Rule

An asset is portable when all of these are true:

- `cvm` owns the concept, not a specific harness.
- The concept can be installed into more than one harness without guessing user intent.
- A missing or partial harness equivalent can be omitted or degraded explicitly.
- The mapping is 1:1 or close to 1:1.

Portable v0.1 is exactly the included set below. Anything outside that set is harness-specific by default.

If an asset requires semantic translation, behavior rewrites, or runtime-specific hooks, it is not portable v0.1.

## Included Assets

| Asset | Format | Notes |
|-------|--------|-------|
| `instructions` | `portable/instructions.md` | Base profile instructions in neutral Markdown. Harness renderers choose the target filename, such as `CLAUDE.md` or `AGENTS.md`. |
| `skills` | `portable/skills/<name>.md` | Prompt assets with minimal frontmatter. Harness renderers may wrap them in native `SKILL.md` layout. |
| `agents` | `portable/agents/<name>.md` | Instruction-only agent definitions. Native agent config remains harness-specific. |
| `settings` | `portable/settings.toml` | Reserved for conceptual settings with shared semantics, such as approval or sandbox policy. No settings keys are rendered yet. |

## Excluded Assets

These assets are harness-specific in portable v0.1:

- hooks
- plugins
- commands legacy
- MCP config with incompatible formats
- raw vendor settings files
- statusline commands
- keybindings
- output styles
- teams
- path-scoped rules
- runtime-specific memory, transcript, session, or cache data

A skill is portable only when it avoids harness-specific subagent invocation, harness-specific filesystem paths, and cross-skill contracts that depend on harness state or output conventions. Skills that assume those behaviors must stay under `<harness>/skills/`.

## Layout

```text
profiles/<name>/
  cvm.profile.toml
  portable/
    instructions.md
    skills/
      <name>.md
    agents/
      <name>.md
    settings.toml
  claude/
  codex/
  opencode/
```

## Manifest

```toml
name = "lite"
harnesses = ["claude", "opencode", "codex"]

[assets]
portable = "portable"
claude = "claude"
opencode = "opencode"
codex = "codex"
```

If a declared harness omits its harness-specific asset dir, `cvm` uses `[assets].portable` as the fallback. Profile authors are responsible for keeping that fallback directory limited to portable-shaped assets. If neither is present, legacy profiles default to the profile root and `harnesses = ["claude"]`.

## Merge Model

Activation order is:

1. Render assets from `portable/` into a temporary native asset tree for the target harness.
2. Apply the target harness override dir on top, if present.
3. Copy managed assets from that merged tree into the target harness.

Harness-specific assets win over portable assets for the same logical asset. `cvm` must not silently translate excluded assets; those belong in the harness override dir.

Implemented render mappings:

| Portable asset | Claude | OpenCode | Codex |
|----------------|--------|----------|-------|
| `instructions.md` | `CLAUDE.md` | `AGENTS.md` | `AGENTS.md` |
| `skills/<name>.md` | `skills/<name>/SKILL.md` | `skills/<name>/SKILL.md` | omitted |
| `agents/<name>.md` | `agents/<name>.md` | `agents/<name>.md` | omitted |
| `settings.toml` | omitted | omitted | omitted |

## Lite Profile Status

`profiles/lite` declares `claude`, `opencode`, and `codex` support. For OpenCode and Codex, only `portable/instructions.md` is rendered today; the current skills, statusline, MCP config, and memory rules depend on Claude Code behavior and remain Claude-only. OpenCode and Codex support for `lite` promises the neutral portable subset, not unsupported hooks, MCP, memory behavior, or Claude-only skills.

`lite` intentionally keeps Claude assets at the profile root with `claude = "."` for compatibility with the existing profile layout. New profiles may use the canonical sibling layout shown above when they do not need legacy root assets.

### Lite Skill Audit

Audit date: 2026-05-04. No `profiles/lite/skills` assets currently qualify as portable v0.1 skills, so no files are authored under `profiles/lite/portable/skills/` yet.

| Skill | Classification | Reason |
|-------|----------------|--------|
| `/go` | adaptable | The Codex/Gemini branches use external CLIs, but the default Opus branch invokes Claude Code `Agent` and then calls `/r`. |
| `/r` | adaptable | The review/persist concept is reusable, but the implementation writes Claude auto-memory under `~/.claude/projects/<path>/memory/`. |
| `/ux` | adaptable | The UX workflow and HTML output are reusable, but validation depends on an Opus `Agent` branch. |
| `/che-idea` | adaptable | The GitHub issue workflow is reusable, but issue enrichment is delegated to an Opus `Agent`. |
| `/che-explore` | adaptable | The GitHub planning workflow is reusable, but the default branch uses Opus `Agent` and Opus success invokes `/r` via Claude `Skill`. |
| `/che-execute` | adaptable | The GitHub/worktree workflow is reusable, but the default branch uses Opus `Agent` and Opus success invokes `/r` via Claude `Skill`. |
| `/che-validate` | adaptable | The GitHub review workflow is reusable, but Opus review uses Claude `Agent` and the skill exposes Claude-specific composition contracts. |
| `/che-iterate` | adaptable | The feedback application workflow is reusable, but it launches an Opus `Agent` and invokes `/r` via Claude `Skill`. |
| `/che-loop` | Claude-only | It is a pure orchestrator over `/che-validate` and `/che-iterate` through Claude `Skill`, so it has no standalone portable behavior today. |
| `/che-close` | adaptable | The GitHub close flow is reusable, but the merge/CI execution is delegated to an Opus `Agent`. |

These adaptable skills should stay in the Claude asset tree until a target harness has equivalent agent invocation, skill composition, and memory semantics, or until the portable portions are split into separate harness-neutral skills.

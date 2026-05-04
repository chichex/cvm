# Portable Profile Contract v0.1 Experimental

## Goal

Define the small profile surface that `cvm` owns across harnesses. Portable assets are authored as `cvm` concepts first, then rendered or copied into Claude, OpenCode, Codex, or another harness when that harness has a compatible equivalent.

This is an experimental v0.1 contract. The current implementation covers manifest parsing and portable asset-dir fallback only; renderer and merge-engine behavior is planned. Treat the layout as subject to tightening until at least one non-Claude renderer consumes it.

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
| `settings` | `portable/settings.toml` | Only conceptual settings with shared semantics, such as approval or sandbox policy. |

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

Rendering order is:

1. Load assets from `portable/`.
2. Apply the target harness override dir, if present.
3. Render or copy the final result into the target harness.

Harness-specific assets win over portable assets for the same logical asset. `cvm` must not silently translate excluded assets; those belong in the harness override dir.

## Lite Profile Status

`profiles/lite` now declares this contract and extracts neutral instructions into `portable/instructions.md`. It still declares only `claude` support because the current skills, statusline, MCP config, and memory rules depend on Claude Code behavior. OpenCode and Codex support for `lite` should be enabled only after renderers can map portable instructions and skills into native formats without promising unsupported hooks, MCP, or memory behavior.

`lite` intentionally keeps Claude assets at the profile root with `claude = "."` for compatibility with the existing profile layout. New profiles may use the canonical sibling layout shown above when they do not need legacy root assets.

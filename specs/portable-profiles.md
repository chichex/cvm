# Portable Profile Contract v1

## Goal

Define the small profile surface that `cvm` owns across harnesses. Portable assets are authored as `cvm` concepts first, then rendered or copied into Claude, OpenCode, Codex, or another harness when that harness has a compatible equivalent.

## Portable Rule

An asset is portable when all of these are true:

- `cvm` owns the concept, not a specific harness.
- The concept can be installed into more than one harness without guessing user intent.
- A missing or partial harness equivalent can be omitted or degraded explicitly.
- The mapping is 1:1 or close to 1:1.

If an asset requires semantic translation, behavior rewrites, or runtime-specific hooks, it is not portable v1.

## Included Assets

| Asset | Format | Notes |
|-------|--------|-------|
| `instructions` | `portable/instructions.md` | Base profile instructions in neutral Markdown. Harness renderers choose the target filename, such as `CLAUDE.md` or `AGENTS.md`. |
| `skills` | `portable/skills/<name>.md` | Prompt assets with minimal frontmatter. Harness renderers may wrap them in native `SKILL.md` layout. |
| `agents` | `portable/agents/<name>.md` | Instruction-only agent definitions. Native agent config remains harness-specific. |
| `settings` | `portable/settings.toml` | Only conceptual settings with shared semantics, such as approval or sandbox policy. |

## Excluded Assets

These assets are harness-specific in portable v1:

- hooks
- plugins
- commands legacy
- MCP config with incompatible formats
- raw vendor settings files
- runtime-specific memory, transcript, session, or cache data

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

If a harness-specific asset dir is omitted, `cvm` may use `[assets].portable` as the fallback. If neither is present, legacy profiles default to the profile root and `harnesses = ["claude"]`.

## Merge Model

Rendering order is:

1. Load assets from `portable/`.
2. Apply the target harness override dir, if present.
3. Render or copy the final result into the target harness.

Harness-specific assets win over portable assets for the same logical asset. `cvm` must not silently translate excluded assets; those belong in the harness override dir.

## Lite Profile Status

`profiles/lite` now declares this contract and extracts neutral instructions into `portable/instructions.md`. It still declares only `claude` support because the current skills, statusline, MCP config, and memory rules depend on Claude Code behavior. OpenCode and Codex support for `lite` should be enabled only after renderers can map portable instructions and skills into native formats without promising unsupported hooks, MCP, or memory behavior.

# cvm - Claude Version Manager

Profile manager for agent harnesses, starting with [Claude Code](https://claude.ai/code) and OpenCode. Switch configurations instantly, nuke everything, restore to vanilla. Like `nvm` but for your agent setup.

## Why

You've built the perfect Claude Code setup: custom skills, hooks, agents, rules, keybindings. But:

- You want **different configs for different contexts** (work vs personal vs experimental)
- You want to **clean everything instantly** without manually deleting files
- You want to **restore your original state** as if nothing happened
- You want to **share configs** across machines

`cvm` manages all of this with zero footprint in your projects.

## Install

```bash
# Homebrew (one command)
brew install chichex/tap/cvm

# One-liner (no brew needed)
curl -sL https://raw.githubusercontent.com/chichex/cvm/main/install.sh | sh

# From source
git clone https://github.com/chichex/cvm.git
cd cvm && make install
```

## Quick start

```bash
# Install a profile
cvm add lite git@github.com:chichex/cvm.git       # minimalist subagent orchestration
cvm use lite

# That's it. Update anytime:
cvm pull

# Go back to vanilla:
cvm use --none

# Nuke everything:
cvm nuke -f
```

## Commands

### Add profiles

```bash
cvm add work                                   # create empty profile
cvm add work --from chiche                     # copy from existing profile
cvm add work --local                           # create for current project only
cvm add chiche git@github.com:chichex/cvm.git  # add from GitHub repo
cvm add chiche chichex/cvm/profiles/chiche     # shorthand (any URL format works)
```

When adding from a repo without a path, cvm auto-discovers the profile:
1. Looks for `profiles/<name>/`
2. Looks for `<name>/` at root
3. If the repo root is a profile, uses that
4. If multiple profiles found, lists them for you to pick

### Switch profiles

```bash
cvm use work            # activate globally (~/.claude/)
cvm use work --local    # activate for current project (.claude/)
cvm use work --harness claude
cvm use work --harness opencode
cvm use --none          # back to vanilla
```

### List and remove

```bash
cvm ls                  # list all profiles (global + local, shows remote source)
cvm rm work             # remove a profile
cvm rm work --local     # remove a local profile
```

### Update and upgrade

```bash
cvm pull                # pull latest for all remote-linked profiles
cvm pull chiche         # pull a specific profile
cvm upgrade             # upgrade cvm itself to the latest version
```

### Clean up

```bash
cvm nuke                # remove ALL managed config (global + local)
cvm nuke --global       # only global
cvm nuke --local        # only local project
cvm nuke --harness claude
cvm nuke -f             # skip confirmation

cvm restore             # restore pre-cvm state from vanilla backup
cvm restore --global    # only global
cvm restore --local     # only local
cvm restore --harness claude
cvm restore --harness opencode
```

### Remote management

```bash
cvm remote ls          # list remote-linked profiles
cvm remote rm chiche   # unlink from remote (keeps local copy)
```

### Inspect

```bash
cvm status             # show active profiles by harness (global + local)
cvm status --harness claude
cvm status --harness opencode
cvm profile            # inspect active profile contents
cvm profile show work  # inspect a specific stored profile
```

### Bypass permissions

Toggle bypass mode on the active profile. Stored as an override, so it survives `cvm pull`.

```bash
cvm bypass on           # enable bypass on active global profile
cvm bypass off          # disable
cvm bypass status       # show current state
cvm bypass on --local   # affect the active local profile instead
```

### Overrides

User customizations that persist across `cvm pull`. Stored separately from the base profile and merged on top when applied.

```bash
cvm override ls                  # list overrides for the active profile
cvm override show                # structured inventory
cvm override add skill foo       # scaffold a new override file
cvm override set ~/.claude/...   # capture a live file as an override
cvm override edit                # open override dir in $EDITOR
cvm override apply               # re-apply active profile + overrides
cvm override rm skill foo        # remove an override file
```

## Two scopes

| Scope | Claude target | OpenCode target | Flag |
|-------|---------------|-----------------|------|
| **global** (default) | `~/.claude/` plus `~/.claude.json` | `~/.config/opencode/` or `$OPENCODE_CONFIG_DIR` | (none) |
| **local** | `.claude/` plus `.mcp.json` in current project | `.opencode/` in current project | `--local` |

For OpenCode, `opencode.json` lives inside the target dir and is user-owned; `cvm` only manages its `mcpServers` section.

## What cvm manages

### Portable profiles

Profiles can opt into `cvm.profile.toml` to describe supported harnesses and asset directories:

```toml
name = "lite"
harnesses = ["claude"]

[assets]
portable = "portable"
claude = "claude"
opencode = "opencode"
```

Portable v1 is intentionally small: `instructions`, `skills`, instruction-only `agents`, and conceptual `settings`. Hooks, plugins, MCP with incompatible formats, raw vendor settings, runtime memory, transcripts, sessions, and caches are not portable and must live in harness-specific override directories.

If a harness-specific asset dir is not declared, `cvm` can use `[assets].portable` as the fallback. Legacy profiles without a manifest still behave as Claude profiles rooted at the profile directory.

See `specs/portable-profiles.md` for the full contract and merge model.

### Claude

| Item | Description |
|------|-------------|
| `CLAUDE.md` | Global instructions |
| `settings.json` | Permissions, hooks config, plugins |
| `settings.local.json` | Local overrides |
| `.claude.json` | User-scoped MCP servers (managed as the `mcpServers` section only) |
| `.mcp.json` | Project-scoped MCP servers |
| `keybindings.json` | Keyboard shortcuts |
| `skills/` | Custom slash commands |
| `agents/` | Subagent definitions |
| `commands/` | Legacy commands |
| `hooks/` | Hook scripts |
| `rules/` | Path-scoped rules |
| `output-styles/` | Response format templates |
| `teams/` | Agent team definitions |
| `statusline-command.sh` | Status bar script |

Runtime data is **never** touched: `sessions/`, `cache/`, `history.jsonl`, `transcripts/`, `projects/` (auto-memory), `plugins/`.

### OpenCode

OpenCode support is intentionally limited to portable assets copied as-is into OpenCode's native config directories.

| Item | Description |
|------|-------------|
| `AGENTS.md` | Harness instructions |
| `opencode.json` | OpenCode configuration, managed only as the `mcpServers` section |
| `skills/` | OpenCode skills in native format |
| `agents/` | OpenCode agent definitions in native format |
| `commands/` | OpenCode commands in native format |

`cvm` does not translate Claude-specific assets for OpenCode. `CLAUDE.md`, Claude `settings.json`, hooks, plugins, non-MCP top-level `opencode.json` settings, and other non-portable behavior require profile-author adaptation and are not promised compatible.

OpenCode runtime storage is **never** touched, including `~/.local/share/opencode/`.

## How switching works

When you run `cvm use work`:

1. Backs up your original `~/.claude/` state (first time only, as "vanilla")
2. Saves current `~/.claude/`, `~/.claude.json`, and project MCP config to the previously active profile
3. Cleans all managed items from `~/.claude/`
4. Copies the "work" profile into `~/.claude/`
5. Updates `~/.cvm/state.json`

## The "lite" profile

A **minimalist profile** for subagent orchestration. No specs, no complex hooks — just skills and Claude Code's built-in auto-memory (`~/.claude/projects/<path>/memory/`).

`lite` declares the portable profile contract and includes neutral instructions in `profiles/lite/portable/instructions.md`, but it currently supports only Claude. Its skills, MCP config, statusline, and memory behavior are Claude-specific until OpenCode/Codex renderers can map the portable subset safely.

Skills:

| Skill | What it does |
|-------|--------------|
| `/go` | Unified subagent — default Opus; `--codex` / `--gemini` for external validation |
| `/r` | Session review + learnings persistence to project memory |
| `/ux` | UX iteration with multi-validator + HTML alternatives |
| `/che-idea` | Create a GitHub issue from a vague idea (auto-classified) |
| `/che-explore` | Enrich an issue with structured analysis + consolidated plan |
| `/che-execute` | Implement an issue in an isolated worktree + open draft PR |
| `/che-validate` | Review a PR/issue with parallel subagents (opus/codex/gemini) |
| `/che-iterate` | Apply comments/reviews on a PR or issue |
| `/che-loop` | Automate `che-validate → che-iterate → ...` until approved |
| `/che-close` | Ready-for-review → wait CI → merge → close linked issues |

The `che-*` skills mirror [che-cli](https://github.com/chichex/che-cli)'s state machine (`che:idea → planning → plan → executing → executed → validating → validated → closing → closed`) in lenient mode.

```bash
cvm add lite git@github.com:chichex/cvm.git
cvm use lite
```

## License

MIT

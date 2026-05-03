# cvm - Claude Version Manager

Profile manager for [Claude Code](https://claude.ai/code). Switch entire configurations instantly, nuke everything, restore to vanilla. Like `nvm` but for your Claude Code setup.

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
cvm nuke -f             # skip confirmation

cvm restore             # restore pre-cvm state from vanilla backup
cvm restore --global    # only global
cvm restore --local     # only local
```

### Remote management

```bash
cvm remote ls          # list remote-linked profiles
cvm remote rm chiche   # unlink from remote (keeps local copy)
```

## Two scopes

| Scope | What it manages | Flag |
|-------|----------------|------|
| **global** (default) | `~/.claude/` — applies to all projects | (none) |
| **local** | `.claude/` in current project | `--local` |

## What cvm manages

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

## How switching works

When you run `cvm use work`:

1. Backs up your original `~/.claude/` state (first time only, as "vanilla")
2. Saves current `~/.claude/`, `~/.claude.json`, and project MCP config to the previously active profile
3. Cleans all managed items from `~/.claude/`
4. Copies the "work" profile into `~/.claude/`
5. Updates `~/.cvm/state.json`

## The "lite" profile

A **minimalist profile** for users who want subagent orchestration without the full SDD workflow. No specs, no KB, no complex hooks — just skills and Claude Code's built-in auto-memory.

> **Note**: This profile does **not** use `cvm kb`. Persistence is handled entirely by Claude Code's native auto-memory system (`~/.claude/projects/<path>/memory/`).

- **6 skills**:
  - `/s` — Smart agent selector (menu with recommendations, multi-instance support)
  - `/o` — Unified subagent (default Opus; `--codex` / `--gemini` flags for external validation)
  - `/r` — Session review + learnings persistence to project memory
  - `/ux` — UX iteration with Opus+Gemini validation, generates HTML alternatives
  - `/issue` — GitHub issue creation with `ct:plan` label
  - `/pr` — Pull request creation with optional `/r`, waits for GitHub Actions
- **No hooks, no rules, no agents, no KB**
- **TDD encouraged** by default

```bash
cvm add lite git@github.com:chichex/cvm.git
cvm use lite
```

## License

MIT

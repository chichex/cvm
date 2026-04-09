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
# From source
git clone https://github.com/ayrtonmarini/cvm.git
cd cvm && make install

# Homebrew (once published)
brew tap ayrtonmarini/tap
brew install cvm
```

## Quick start

```bash
# Create a profile from your current ~/.claude/ config
cvm global init work

# You're done. Edit the profile anytime:
cvm global edit work

# Switch profiles
cvm global use work        # apply "work" profile
cvm global use --none      # back to vanilla

# See what's active
cvm status
```

## Concepts

### Two scopes

| Scope | What it manages | Storage |
|-------|----------------|---------|
| **global** | `~/.claude/` (applies to all projects) | `~/.cvm/global/profiles/` |
| **local** | `.claude/` in the current project | `~/.cvm/local/profiles/` |

### What cvm manages

These are the config files that cvm copies between profiles and `~/.claude/`:

| Item | Description |
|------|-------------|
| `CLAUDE.md` | Global instructions |
| `settings.json` | Permissions, hooks config, MCP servers |
| `settings.local.json` | Local overrides |
| `keybindings.json` | Keyboard shortcuts |
| `skills/` | Custom slash commands |
| `agents/` | Subagent definitions |
| `commands/` | Legacy commands |
| `hooks/` | Hook scripts |
| `rules/` | Path-scoped rules |
| `output-styles/` | Response format templates |
| `teams/` | Agent team definitions |
| `statusline-command.sh` | Status bar script |

### What cvm never touches

Runtime data stays untouched: `sessions/`, `cache/`, `history.jsonl`, `transcripts/`, `projects/` (auto-memory), `plugins/`.

## Commands

### Profiles

```bash
# Global profiles (~/.claude/)
cvm global init <name>              # create profile (copies current config)
cvm global init <name> --from other # create from existing profile
cvm global use <name>               # switch to profile
cvm global use --none               # switch to vanilla
cvm global ls                       # list profiles (* = active)
cvm global current                  # show active profile name
cvm global save                     # save current ~/.claude/ to active profile
cvm global edit [name]              # open profile in $EDITOR
cvm global rm <name>                # delete profile

# Local profiles (.claude/ in current project)
cvm local init [name]               # create (default name: directory name)
cvm local init --from backend       # create from existing
cvm local use <name>                # switch
cvm local use --none                # vanilla
cvm local ls / current / save / rm  # same as global
```

### Knowledge Base

```bash
cvm kb put <key> --body "..." --tag "a,b"   # create/update entry
cvm kb ls [--tag <tag>]                      # list entries
cvm kb show <key>                            # show entry content
cvm kb search <query>                        # search entries
cvm kb enable <key>                          # include in Claude context
cvm kb disable <key>                         # exclude without deleting
cvm kb rm <key>                              # delete entry

# All kb commands accept --local (default: global)
cvm kb ls --local
cvm kb put my-key --body "..." --local
```

### Lifecycle (used by hooks)

```bash
cvm lifecycle start    # session start: load context, detect tools
cvm lifecycle end      # session end: cleanup
cvm lifecycle status   # show current session info
```

### Diagnostics

```bash
cvm status    # show active profiles (global + local)
cvm health    # full system diagnostics
```

### Nuclear options

```bash
cvm nuke                # remove ALL managed config (global + local)
cvm nuke --global       # only global
cvm nuke --local        # only local project
cvm nuke -f             # skip confirmation

cvm restore             # restore pre-cvm state from vanilla backup
cvm restore --global    # only global
cvm restore --local     # only local
```

## How switching works

When you run `cvm global use work`:

1. Backs up your original `~/.claude/` state (first time only, as "vanilla")
2. Saves current `~/.claude/` config to the previously active profile
3. Cleans all managed items from `~/.claude/`
4. Copies the "work" profile into `~/.claude/`
5. Updates `~/.cvm/state.json`

Runtime files are **never** touched. Sessions, history, cache all stay intact.

## Storage layout

```
~/.cvm/
  ├── global/
  │   ├── profiles/
  │   │   ├── work/           # full ~/.claude/ config snapshot
  │   │   ├── personal/
  │   │   └── minimal/
  │   └── vanilla/            # original pre-cvm state
  ├── local/
  │   ├── profiles/
  │   │   ├── backend/
  │   │   └── frontend/
  │   └── vanilla/
  │       └── <project-hash>/ # per-project vanilla backup
  ├── state.json              # active profiles tracker
  └── session.json            # current session info (transient)
```

## Profile contents

A profile is just a directory that mirrors the managed parts of `~/.claude/`:

```
~/.cvm/global/profiles/work/
  ├── CLAUDE.md
  ├── settings.json
  ├── skills/
  │   ├── deploy/SKILL.md
  │   └── review/SKILL.md
  ├── hooks/
  │   └── slop-check.sh
  ├── rules/
  │   └── coding-standards.md
  └── agents/
      └── researcher/AGENT.md
```

## Homebrew distribution

To publish via Homebrew:

1. Create a GitHub repo `ayrtonmarini/homebrew-tap`
2. Install [GoReleaser](https://goreleaser.com/): `brew install goreleaser`
3. Tag a release: `git tag v0.1.0 && git push --tags`
4. Run: `goreleaser release --clean`

GoReleaser will:
- Cross-compile for macOS/Linux (amd64 + arm64)
- Create GitHub release with binaries
- Auto-update the Homebrew formula in your tap

Users install with:
```bash
brew tap ayrtonmarini/tap
brew install cvm
```

## License

MIT

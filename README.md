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
# Install the "chiche" profile (18 skills, auto-KB, adversarial debugging)
cvm add chiche git@github.com:chichex/cvm.git
cvm use chiche

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

### Diagnostics

```bash
cvm status    # show active profiles (global + local)
cvm health    # full system diagnostics
cvm profile   # inspect active profile contents (skills, agents, hooks, rules)
cvm bypass on # enable bypass permissions on active profile(s)
cvm bypass off
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

### Lifecycle (used by hooks)

```bash
cvm lifecycle start    # session start: load context, detect tools
cvm lifecycle end      # session end: cleanup + queue background retro + auto-run automation
cvm lifecycle status   # show current session info
cvm automation status  # queued candidates summary
cvm automation ls      # list candidate briefs
cvm automation show <id>  # inspect a materialized brief
cvm automation run     # process pending candidates now
cvm automation history # recent automation runs
cvm automation show-run <id> # inspect a recorded run
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

Runtime data is **never** touched: `sessions/`, `cache/`, `history.jsonl`, `transcripts/`, `projects/` (auto-memory), `plugins/`.

## How switching works

When you run `cvm use work`:

1. Backs up your original `~/.claude/` state (first time only, as "vanilla")
2. Saves current `~/.claude/` config to the previously active profile
3. Cleans all managed items from `~/.claude/`
4. Copies the "work" profile into `~/.claude/`
5. Updates `~/.cvm/state.json`

## The "chiche" profile

cvm ships with a built-in profile called **chiche** — a self-improving Claude Code configuration with:

- **18 skills**: learn, decide, gotcha, recall, retro, evolve, maintain, validate, orchestrate, checkpoint, quality-gate, spec, execute, fix, ux, higiene, skill-create, headless
- **5 rules**: model selection, context hygiene, cost awareness, scope guard, KB awareness
- **3 agents**: researcher (haiku), implementer (sonnet), reviewer (opus)
- **3 hooks**: tool detection, config protection, slop checking
- **Auto-KB**: learns from your sessions and persists insights between conversations
- **Low-latency automation loop**: prompt path stays light; retro runs in background on session end
- **Adversarial debugging**: launches competing agents to investigate bugs
- **Thresholded evolution**: maintain/evolve candidates are queued only when KB/session signals justify it
- **Automatic maintenance**: stale/duplicate KB entries are normalized and suppressed automatically
- **Automatic skill generation**: repeated KB patterns can install auto-generated skills into `.claude/skills/`

```bash
cvm add chiche git@github.com:chichex/cvm.git
cvm use chiche
```

## License

MIT

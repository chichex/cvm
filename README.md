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
cvm add sdd git@github.com:chichex/cvm.git           # spec-driven development
cvm add sdd-mem git@github.com:chichex/cvm.git       # sdd + persistent memory
cvm use sdd                                          # or: cvm use sdd-mem

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
cvm kb put <key> --body "..." --type learning # type: decision|learning|gotcha|discovery|session
cvm kb ls [--tag <tag>]                      # list entries
cvm kb show <key>                            # show entry content
cvm kb search <query>                        # search entries (ranked: exact > key > body)
cvm kb search <query> --sort recent          # sort by date instead of relevance
cvm kb search <query> --tag gotcha           # filter by tag
cvm kb search <query> --type learning        # filter by type
cvm kb search <query> --since 7d             # filter by age
cvm kb timeline [--days 7]                   # entries grouped by day
cvm kb stats                                 # token estimates and entry counts
cvm kb compact                               # compact index for context injection
cvm kb enable <key>                          # include in Claude context
cvm kb disable <key>                         # exclude without deleting
cvm kb rm <key>                              # delete entry
cvm kb clean [--force]                       # remove all entries

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
cvm lifecycle end      # session end: cleanup + auto-run automation
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

## The "sdd" profile

A **Spec-Driven Development** profile that enforces a spec-first workflow: every feature starts as a specification, implementation follows the spec, and verification checks compliance against it.

- **18 skills**: learn, decide, gotcha, recall, retro, evolve, maintain, orchestrate, checkpoint, quality-gate, spec, derive-tests, execute, fix, verify, spec-status, skill-create, headless
- **10 rules**: model selection, context hygiene, cost awareness, scope guard, KB awareness, agent routing, spec-first, no-spec-drift, traceability
- **5 agents**: researcher (haiku), implementer (sonnet), reviewer (opus), specifier (sonnet), verifier (opus)
- **MCP servers**: playwright, context7
- **Spec lifecycle**: specs live in `specs/`, are tracked with frontmatter status, and drive test derivation
- **Traceability**: every code change links back to a spec section
- **Drift protection**: implementation that deviates from spec is flagged automatically

```bash
cvm add sdd git@github.com:chichex/cvm.git
cvm use sdd
```

## The "sdd-mem" profile

Extends **sdd** with a persistent memory system inspired by [claude-mem](https://github.com/thedotmack/claude-mem) but without the infrastructure overhead (no daemon, no ChromaDB, no per-tool observer).

**Key additions over sdd:**
- **SQLite + FTS5 backend**: KB entries stored in SQLite with full-text search, porter stemming, and BM25 ranking. Flat files kept as automatic fallback. Set `CVM_KB_BACKEND=flat` to force flat files.
- **MCP KB tools**: Native `kb_search` and `kb_get` tools exposed via MCP — Claude queries the KB directly without shelling out. Requires `cvm-mcp-kb` binary in PATH (see install).
- **Context injection**: `SessionStart` hook injects a compact summary of recent KB entries (~2K tokens budget)
- **Auto session summary**: `SessionEnd` generates a structured summary via Haiku (~$0.001/session)
- **Tool observation**: `PostToolUse` hook captures Bash/Write/Edit events to enrich session summaries. Configurable via `CVM_OBSERVE_TOOLS`.
- **Progressive disclosure**: KB queries use 2-step pattern (search → show) to minimize token usage
- **Token awareness**: rule enforcing token budget consciousness for KB context
- **Content-hash dedup**: `cvm kb put` detects duplicate content and warns/skips
- **2 extra skills**: `/session-summary` (manual trigger) and all sdd skills

**Configuration** (env vars in settings.json):
| Variable | Default | Description |
|----------|---------|-------------|
| `CVM_KB_BACKEND` | sqlite | KB storage backend (`sqlite` or `flat`) |
| `CVM_OBSERVE_TOOLS` | Bash,Write,Edit | Tools captured by PostToolUse hook |
| `CVM_CONTEXT_ENTRY_COUNT` | 10 | Max entries in context injection |
| `CVM_CONTEXT_MAX_TOKENS` | 2000 | Token budget for context injection |
| `CVM_AUTOSUMMARY_ENABLED` | true | Enable auto session summary |
| `CVM_AUTOSUMMARY_MODEL` | haiku | Model for summary generation |

**Cost**: ~$0.001/session (vs claude-mem's ~$0.03 — 37x cheaper).

```bash
cvm add sdd-mem git@github.com:chichex/cvm.git
cvm use sdd-mem

# Optional: install MCP server for native KB tools
# (included in `make install`, or manually):
go build -o /usr/local/bin/cvm-mcp-kb ./cmd/mcp-kb/
```

## License

MIT

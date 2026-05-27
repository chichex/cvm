# cvm - Claude Version Manager

Profile manager for agent harnesses, starting with [Claude Code](https://claude.ai/code), OpenCode, and Codex. Switch configurations instantly, restore to vanilla. Like `nvm` but for your agent setup.

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
```

## Commands

### Add profiles

```bash
cvm add work                                   # create empty profile
cvm add work --from chiche                     # copy from existing profile
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
cvm use work            # activate user-level config
cvm use work --harness claude
cvm use work --harness opencode
cvm use work --harness codex
cvm use --none          # back to vanilla
```

### List and remove

```bash
cvm ls                  # list profiles, including remote source
cvm rm work             # remove a profile
```

### Update remote profiles

```bash
cvm pull                # pull latest for all remote-linked profiles
cvm pull chiche         # pull a specific profile
```

### Clean up

```bash
cvm restore             # restore pre-cvm state from vanilla backup
cvm restore --harness claude
cvm restore --harness opencode
```

### Upgrade cvm

```bash
cvm upgrade
```

Downloads the latest stable release from GitHub and atomically replaces the
running binary. Shows current vs latest version:

```
current: v0.23.0
latest:  v0.24.0
Downloading v0.24.0...
Upgraded to v0.24.0
```

If already on the latest version:

```
Already on latest version (v0.24.0)
```

If installed via Homebrew, the command exits with an instruction to use `brew upgrade chichex/tap/cvm` instead. The upgrade never touches profiles or harness configs.

### Bypass permissions

Toggle bypass mode on the active profile. Stored as an override, so it survives `cvm pull`.

```bash
cvm bypass on           # enable bypass on active profile
cvm bypass off          # disable
cvm bypass status       # show current state
```

## Targets

`cvm` manages only user-level harness configuration:

| Harness | Target |
|---------|--------|
| Claude | `~/.claude/` plus `~/.claude.json` |
| OpenCode | `~/.config/opencode/` or `$OPENCODE_CONFIG_DIR` |
| Codex | `~/.codex/` or `$CODEX_HOME` |

For OpenCode, `opencode.json` lives inside the target dir and is user-owned; `cvm` only manages its `mcpServers` section and the profile-owned `skills.paths` entry.

Project-local profiles were hard-deleted. `cvm local`, `cvm global`, `--local`, `--global`, project `.claude/`, project `.opencode/`, and project `.mcp.json` are no longer part of the model. Existing project-local files are left untouched on disk; remove them manually if you no longer want them, for example `rm -rf .claude .opencode .mcp.json` from the affected project.

## What cvm manages

### Profile manifest

Profiles can opt into `cvm.profile.toml` to declare supported harnesses and per-harness asset directories:

```toml
name = "example"
harnesses = ["claude", "opencode", "codex"]

[assets]
claude = "claude"
opencode = "opencode"
codex = "codex"
```

Legacy profiles without a manifest behave as Claude profiles rooted at the profile directory.

### Claude

| Item | Description |
|------|-------------|
| `CLAUDE.md` | Global instructions |
| `settings.json` | Permissions, hooks config, plugins |
| `settings.local.json` | Claude user overrides |
| `.claude.json` | User-scoped MCP servers (managed as the `mcpServers` section only) |
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

OpenCode support is intentionally limited to portable assets rendered into OpenCode's native config directories plus explicit OpenCode asset overrides.

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
2. Saves current `~/.claude/` and `~/.claude.json` to the previously active profile
3. Cleans all managed items from `~/.claude/`
4. Copies the "work" profile into `~/.claude/`
5. Updates `~/.cvm/state.json`

## The "lite" profile

A **minimalist profile** for subagent orchestration. No specs, no complex hooks — just skills and Claude Code's built-in auto-memory (`~/.claude/projects/<path>/memory/`).

`lite` is a Claude-only profile. Its skills depend on Claude Code's `Agent`/`Skill` tools and on the per-project auto-memory under `~/.claude/projects/<path>/memory/`, so they don't translate cleanly to OpenCode or Codex.

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
cvm use lite --harness claude
```

## Herdr integration

[Herdr](https://herdr.dev) es un multiplexor de TUIs para agentes CLI. El profile `harness` trae un skill y un bootstrap para integrarse con el:

### Skill `/herdr-detach`

Deriva un prompt a otro agente CLI (`claude`, `opencode`, `codex`) corriendo en un pane de `herdr`, opcionalmente esperando a que termine y devolviendo la respuesta inline.

```text
/herdr-detach [--wait] [--here|--new] <agente> <prompt>
```

- `<agente>`: `claude` | `opencode` | `codex`.
- `--wait`: bloquea hasta que el agente derivado termine (status `idle` o `done`) y devuelve la respuesta. Default: fire-and-forget — devuelve `pane_id` y sigue.
- `--here`: split del pane focused actual (default). `--new` crea un workspace nuevo dedicado.

Asume que la sesion actual ya corre dentro de `herdr` y que el binario del agente derivado esta en PATH. La integracion de `herdr` con el agente (`herdr integration install <agente>`) se auto-instala si falta.

Ver `profiles/harness/claude/skills/herdr-detach/SKILL.md` para el detalle completo.

### Bootstrap `scripts/bootstrap-herdr-override.sh`

`herdr integration install claude` instala un hook en `~/.claude/hooks/herdr-agent-state.sh` y registra eventos en `~/.claude/settings.json` (`PreToolUse`, `UserPromptSubmit`, `Stop`, `SessionEnd`, `PermissionRequest`) para reportar status al servidor de `herdr`. El problema: cada `cvm use` / `cvm pull` corre `CleanManagedItems` + `CopyManagedItems` + `ApplyOverrides`, que borra `~/.claude/hooks/` y `~/.claude/settings.json` antes de re-copiar el profile. Sin override layer, la integracion se pierde en cada apply.

`scripts/bootstrap-herdr-override.sh` deja la integracion en el override layer (`~/.cvm/global/overrides/harness/`) para que `ApplyOverrides` la restituya despues de cada limpieza:

```bash
bash scripts/bootstrap-herdr-override.sh           # default profile: harness
bash scripts/bootstrap-herdr-override.sh <profile> # override sobre otro profile
```

Que hace, en orden:

1. Verifica `cvm`, `herdr` y `python3` en PATH.
2. Corre `herdr integration install claude` para tener el hook fresco en `~/.claude/hooks/herdr-agent-state.sh`.
3. Copia ese hook a `~/.cvm/global/overrides/<profile>/hooks/herdr-agent-state.sh`.
4. Mergea el bloque `hooks` en `~/.cvm/global/overrides/<profile>/settings.json` (preserva lo que ya este ahi, ej `permissions`). Los `command` usan `"$HOME/.claude/hooks/herdr-agent-state.sh"` para portabilidad entre maquinas.
5. Re-aplica el profile (`cvm use <profile>`) y verifica que el hook y los eventos esten registrados en `~/.claude/settings.json`.

Idempotente: correlo una vez por maquina nueva con `herdr` instalado y olvidate. Despues de eso, los `cvm pull` / `cvm use` mantienen la integracion intacta.

## License

MIT

# cvm - Claude Version Manager

Profile manager for agent harnesses, starting with [Claude Code](https://claude.ai/code), OpenCode, and Codex. Switch configurations instantly, restore to vanilla. Like `nvm` but for your agent setup.

A profile is just a **git repo on disk**. Activating it **symlinks** its files into `~/.claude/` (and the OpenCode/Codex config dirs). Edits you make through the harness write straight back into the repo — live and already version-controlled. No copies, no merges, no hidden state.

## Why

You've built the perfect Claude Code setup: custom skills, hooks, agents, rules, keybindings. But:

- You want **different configs for different contexts** (work vs personal vs experimental)
- You want to **clean everything instantly** without manually deleting files
- You want to **restore your original state** as if nothing happened
- You want to **share configs** across machines
- You want edits to your active profile to be **live and versioned** — no "save back to profile" dance

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
# Clone a profile and activate it (symlinks into ~/.claude)
cvm add lite git@github.com:chichex/cvm.git
cvm use lite

# Update anytime (thin git pull --ff-only):
cvm pull

# Go back to vanilla (remove symlinks, restore your originals):
cvm off
```

## How it works

A profile's **source dir** is a real git working repo:

- For cloned profiles, that's `~/.cvm/profiles/<name>` (cloned with `.git` intact).
- For path-registered profiles (`cvm add <name> --path <dir>`), it's your own existing repo dir — cvm just records where it is. No second copy.

When you run `cvm use <name>`, cvm creates one **symlink per managed item**:

```
~/.claude/<item>  ->  <sourceDir>/<harnessSubdir>/<item>
```

`<harnessSubdir>` is `claude` / `opencode` / `codex` (or `.` when the profile is rooted at its repo, per manifest/auto-discovery).

Because the live config is a symlink into the repo, **editing a file through the harness writes into the source repo** — instantly live, and already under version control. That's the whole point.

### Vanilla stash

Before symlinking an item, if a **real** (non-symlink) file or dir already exists at the target, cvm moves it once to `~/.cvm/vanilla/<harness>/<item>` (only if nothing is stashed there yet). `cvm off` (or `cvm use --none`) removes the symlinks and restores that stash, leaving you exactly as you were before cvm.

## Commands

| Command | What it does |
|---------|--------------|
| `cvm ls` | List profiles, marking the active one per harness |
| `cvm use <name>` | Symlink all managed items for the profile's harnesses |
| `cvm off` (alias `cvm use --none`) | Remove symlinks, restore the vanilla stash |
| `cvm add <name> <giturl>` | `git clone` (keeping `.git`) into `~/.cvm/profiles/<name>` |
| `cvm add <name> --path <dir>` | Register an existing local repo as the source (no clone) |
| `cvm pull` | `git pull --ff-only` the active profile's source repo |
| `cvm push` | commit pending changes + `git push` the active profile's source repo |
| `cvm rm <name>` | Unregister a profile |
| `cvm harness` | Inspect harness targets/items |
| `cvm upgrade` | Update the cvm binary itself |

### Add profiles

```bash
cvm add work                                       # empty profile
cvm add work --from chiche                          # copy from existing profile
cvm add chiche git@github.com:chichex/cvm.git       # clone from GitHub (keeps .git)
cvm add chiche chichex/cvm/profiles/chiche          # shorthand (any URL format works)
cvm add mine --path ~/dev/my-profile                # register an existing local repo
```

With `--path`, cvm points at **your** working repo directly — edits flow straight into it and you commit/push from there as usual. With a URL, cvm clones into `~/.cvm/profiles/<name>`, keeping `.git` so `cvm pull` / `cvm push` are just thin git wrappers.

When adding from a repo without an explicit path, cvm auto-discovers the profile:

1. Looks for `profiles/<name>/`
2. Looks for `<name>/` at root
3. If the repo root is itself a profile, uses that
4. If multiple profiles are found, lists them for you to pick

### Switch profiles

```bash
cvm use work                  # activate every harness in the manifest
cvm use work --harness claude
cvm use work --harness opencode
cvm use work --harness codex
cvm off                       # remove symlinks, restore vanilla
cvm use --none                # same as cvm off
cvm off --harness claude      # revert a single harness
```

### List and remove

```bash
cvm ls                  # profiles, active markers, source dir/repo
cvm rm work             # unregister a profile
```

### Update and publish

`cvm pull` fast-forwards from git; `cvm push` commits any pending changes and pushes them in one step. cvm does **not** manage conflicts — git does.

```bash
cvm pull                # git -C <source> pull --ff-only
cvm pull chiche         # pull a specific profile
cvm push                # commit pending changes + git push (active profile)
cvm push -m "tweak"     # use a custom commit message
```

- `pull` uses `--ff-only`: if the branch has diverged it refuses cleanly, never auto-merges. If the working tree is dirty, the pull is skipped with a warning — resolve it with git in the source repo.
- `push` first commits a dirty tree (`git add -A` + commit, message from `-m` or a default `cvm: update <profile> profile`), then pushes. git's output — including non-fast-forward rejections — is surfaced verbatim. No merge smarts, no conflict markers — just git.

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

## Targets

`cvm` manages only user-level harness configuration:

| Harness | Target |
|---------|--------|
| Claude | `~/.claude/` |
| OpenCode | `~/.config/opencode/` or `$OPENCODE_CONFIG_DIR` |
| Codex | `~/.codex/` or `$CODEX_HOME` |

Each managed item is symlinked individually, so anything not in the managed list (runtime data, machine-local state) is left untouched.

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

Each item below is symlinked from the profile into `~/.claude/`. The profile fully owns it — there is no merging.

| Item | Description |
|------|-------------|
| `CLAUDE.md` | Global instructions |
| `settings.json` | Permissions, hooks config, plugins |
| `settings.local.json` | Claude user settings |
| `keybindings.json` | Keyboard shortcuts |
| `statusline-command.sh` | Status bar script |
| `commands/` | Slash commands |
| `skills/` | Custom skills |
| `agents/` | Subagent definitions |
| `hooks/` | Hook scripts |
| `rules/` | Path-scoped rules |
| `output-styles/` | Response format templates |
| `teams/` | Agent team definitions |

Runtime and machine-local data is **never** touched: `sessions/`, `cache/`, `history.jsonl`, `transcripts/`, `projects/` (auto-memory), `plugins/`, and `~/.claude.json`. MCP servers in `~/.claude.json` are machine-local mutable state and are not managed by cvm.

### OpenCode

OpenCode support symlinks the profile's portable assets into OpenCode's native config directory. The profile fully owns each item.

| Item | Description |
|------|-------------|
| `AGENTS.md` | Harness instructions |
| `opencode.json` | OpenCode configuration |
| `skills/` | OpenCode skills in native format |
| `agents/` | OpenCode agent definitions in native format |
| `commands/` | OpenCode commands in native format |

`cvm` does not translate Claude-specific assets for OpenCode. `CLAUDE.md`, Claude `settings.json`, hooks, plugins, and other Claude-specific behavior require profile-author adaptation and are not promised compatible. OpenCode runtime storage is **never** touched, including `~/.local/share/opencode/`.

### Codex

| Item | Description |
|------|-------------|
| `AGENTS.md` | Harness instructions |

Codex assets are symlinked into `~/.codex/` (or `$CODEX_HOME`).

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

[Herdr](https://herdr.dev) es un multiplexor de TUIs para agentes CLI. El profile `harness` trae un skill para integrarse con el:

### Skill `/herdr-detach`

Deriva un prompt a otro agente CLI (`claude`, `opencode`, `codex`) corriendo en un pane de `herdr`, opcionalmente esperando a que termine y devolviendo la respuesta inline.

```text
/herdr-detach [--wait] [--here|--new] <agente> <prompt>
```

- `<agente>`: `claude` | `opencode` | `codex`.
- `--wait`: bloquea hasta que el agente derivado termine (status `idle` o `done`) y devuelve la respuesta. Default: fire-and-forget — devuelve `pane_id` y sigue.
- `--here`: split del pane que origino la invocacion (anclado via `HERDR_PANE_ID`, no via focus state). Default. `--new`: crea un workspace nuevo dedicado.

Asume que la sesion actual ya corre dentro de `herdr` y que el binario del agente derivado esta en PATH. La integracion de `herdr` con el agente (`herdr integration install <agente>`) se auto-instala si falta.

Con el modelo de symlinks, la integracion de `herdr` se instala directamente en el repo del profile: `herdr integration install claude` edita `hooks/` y `settings.json` a traves de los symlinks, asi que los cambios quedan versionados en el profile y sobreviven a cada `cvm use` / `cvm pull` sin maquinaria extra.

Ver `profiles/harness/claude/skills/herdr-detach/SKILL.md` para el detalle completo.

## License

MIT

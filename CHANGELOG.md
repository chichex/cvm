# Changelog

All notable changes to cvm are documented here.

## v2.0.0 — 2026-06-08

### Breaking

cvm moved from a **copy+merge** model to a **pure-symlink** model. A profile is
now a real git repo on disk, and `cvm use` symlinks its managed items into your
harness config dirs instead of copying and merging them. Edits made through the
symlinks write straight back into the profile repo — live and version-controlled.
See [MIGRATION.md](MIGRATION.md) for the full upgrade guide.

- **Symlink activation.** `cvm use <name>` symlinks each managed item from the
  profile's source repo into the harness target dir; the profile fully owns its
  items (no merge, no override layering).
- **Profiles are git repos.** `cvm add <name> <giturl>` does a direct `git clone`
  (keeping `.git`) into `~/.cvm/profiles/<name>`. New `cvm add <name> --path <dir>`
  registers an existing local repo as the source without cloning.
- **Lightweight vanilla stash.** The first time a profile takes over a managed
  item, any real pre-existing file/dir is moved once to
  `~/.cvm/vanilla/<harness>/<item>`. `cvm off` (alias `cvm use --none`) removes
  the symlinks and restores the stash. Replaces the copy-based vanilla backup.
- **Transparent git pass-through.** `cvm pull` is `git pull --ff-only` (aborts
  cleanly on a dirty tree, never auto-merges); `cvm push` is `git push` with git's
  output — including non-fast-forward rejections — surfaced verbatim.

### Removed

- `cvm bypass` command and the `EnableBypass` / `DisableBypass` / `BypassStatus`
  harness methods.
- The override layer: `ApplyOverrides`, JSON/directory merging, `OverrideDir`,
  and `~/.cvm/global/overrides`.
- MCP / `.claude.json` management (`applyUserMCPServers`, `IsMCPPath`,
  `IsUserMCPPath`, `ExternalManagedPath`). cvm no longer touches `.claude.json`.
- `cvm restore` — folded into `cvm off`.
- Copy-based vanilla backup and the cache -> copy clone indirection.
- `SupportsPortableSkills` / `SupportsPortableAgents` harness methods.

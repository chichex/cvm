# Migration: copy+merge -> pure symlink

**This is a breaking change.** cvm no longer copies and merges profile contents
into your harness config dirs. A profile is now a real git repo on disk, and
activating it just creates **symlinks** from `~/.claude` (and the opencode/codex
config dirs) into that repo. Editing a managed file through the symlink writes
straight back into the profile repo — live, and already version-controlled.

If you upgraded from an older cvm, read the [Migrating an existing setup](#migrating-an-existing-setup)
section before running anything.

---

## What changed

### The model

**Before:** `cvm use` copied each managed item out of the profile into your
config dir, layering global overrides on top and merging JSON files / directories
where needed. Your live config was a derived snapshot; edits to `~/.claude/...`
did not flow back to the profile. A heavyweight copy-based "vanilla" backup
captured your original config so it could be restored.

**Now:** a profile **is** a git working repo on disk:

- Cloned profiles live at `~/.cvm/profiles/<name>` (a full `git clone`, `.git`
  kept).
- Path-registered profiles (`cvm add <name> --path <dir>`) point at an existing
  local repo you already own — cvm never copies or moves it; it only records the
  directory in `state.json`.

`cvm use <name>` symlinks each managed item:

```
~/.claude/<item> -> <sourceDir>/<harnessSubdir>/<item>
```

`<harnessSubdir>` is discovered exactly as before (`claude` / `opencode` /
`codex`, or `.` when the profile keeps assets at its root).

Because the target is a symlink into the repo, editing a file through it edits
the profile directly — instant and version-controlled. That is the whole point.

### Removed

- **`cvm bypass`** — the command, its tests, and `EnableBypass` /
  `DisableBypass` / `BypassStatus` on the `Harness` interface and all
  implementations. cvm no longer manages permission-bypass settings.
- **Override layer** — `ApplyOverrides`, `mergeJSONFiles`, `mergeDirectories`,
  the `OverrideDir` config, and `~/.cvm/global/overrides`. There is no longer any
  layering: a profile fully owns its managed items.
- **MCP / `.claude.json` management** — `applyUserMCPServers`, `IsMCPPath`,
  `IsUserMCPPath`, `ExternalManagedPath`, and all `.claude.json` handling.
  `.claude.json` is machine-local mutable state and is **not** symlinkable, so
  cvm no longer touches it. Manage your MCP servers directly with your harness.
- **Copy-based vanilla backup** — replaced by the lightweight stash below.
- **`cvm restore`** — folded into `cvm off` (alias `cvm use --none`).
- **Cache -> copy clone indirection** — `cvm add <url>` now does a direct
  `git clone` (keeping `.git`) into `~/.cvm/profiles/<name>`; profile contents
  are no longer copied out of a cache.
- `SupportsPortableSkills` / `SupportsPortableAgents` (only existed for the
  copy/merge path).

### Vanilla stash (new, lightweight)

Before symlinking an item, if a **real** (non-symlink) file or dir already exists
at the target, cvm moves it **once** to:

```
~/.cvm/vanilla/<harness>/<item>
```

It only stashes if nothing is already stashed there, and it never touches a real
file it doesn't own when removing links. `cvm off` (or `cvm use --none`) removes
the profile symlinks and moves the stashed vanilla items back into place.

This replaces the old copy-based backup entirely — no full-tree snapshots, just a
one-time move-aside of whatever was really there before cvm.

### Conflict policy: cvm does not manage conflicts, git does

- `cvm pull` runs `git -C <source-repo> pull --ff-only`. If the working tree is
  dirty, the pull is **skipped with a warning** and never auto-merges. Resolve in
  the repo with git directly.
- `cvm push` runs `git -C <source-repo> push` and surfaces git's output —
  including non-fast-forward rejections — **verbatim**. No merge smarts.

cvm is a transparent pass-through over git.

---

## Final command surface

| Command | Behavior |
|---------|----------|
| `cvm ls` | List profiles, mark the active one (per harness). |
| `cvm use <name>` | Symlink all managed items for the profile's harnesses. |
| `cvm off` (alias `cvm use --none`) | Remove profile symlinks, restore the vanilla stash. |
| `cvm add <name> <giturl>` | `git clone` (keep `.git`) into `~/.cvm/profiles/<name>`. |
| `cvm add <name> --path <dir>` | Register an existing local repo as the source (no clone). |
| `cvm pull` | `git pull --ff-only` the active profile's source repo; abort cleanly if dirty. |
| `cvm push` | `git push` the active profile's source repo; surface git's rejection verbatim. |
| `cvm rm <name>` | Unregister the profile (cloned copies are deleted; `--path` repos are never touched). |

`cvm harness` and `cvm upgrade` are unchanged (minus the removed bypass).

---

## Migrating an existing setup

1. **Upgrade cvm** to this version.

2. **Re-add your profiles.** Old override/MCP/bypass state is no longer read.

   - If your profile lives in a remote git repo, re-clone it:

     ```sh
     cvm add <name> github.com/you/repo/profiles/<name>
     ```

   - If you keep a profile as a local repo on disk (e.g. your own working copy),
     register it in place so edits flow straight into it:

     ```sh
     cvm add <name> --path ~/dev/my-profile
     ```

     cvm records the directory and symlinks out of it — it never copies or moves
     your repo.

3. **Activate it:**

   ```sh
   cvm use <name>
   ```

   The first time, any real (non-symlink) files already in `~/.claude` (or the
   opencode/codex config dirs) for a managed item are stashed once to
   `~/.cvm/vanilla/<harness>/<item>`, then replaced with symlinks into your
   profile repo.

4. **Edit live.** Open `~/.claude/<item>` in your editor as usual — you are
   editing the profile repo directly. Commit and `cvm push` when ready.

5. **Go back to vanilla** any time with `cvm off`; your pre-cvm files are moved
   back from the stash.

### Things to redo manually

- **MCP servers / `.claude.json`:** cvm no longer manages these. Configure them
  directly with your harness.
- **Global overrides:** removed. Fold whatever you kept in `~/.cvm/global/overrides`
  into the profile itself (the profile now fully owns its managed items).
- **Bypass settings:** removed. Manage permission settings directly in your
  profile's `settings.json`.

### Cleanup (optional)

Once you've confirmed your profiles work, you can remove leftover state from the
old model:

```sh
rm -rf ~/.cvm/global/overrides
```

Old `state.json` fields for overrides/bypass are simply ignored.

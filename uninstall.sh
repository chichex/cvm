#!/bin/sh
# uninstall.sh — cleanly remove cvm while keeping the ACTIVE profile's config.
#
# cvm activates a profile by symlinking the profile's files (which live in the
# profile's source repo, e.g. ~/.cvm/profiles/<name> or a path-registered dir)
# into each harness target dir (~/.claude, ~/.codex, ~/.config/opencode). If we
# deleted cvm's state first, those symlinks would dangle and you'd lose the
# config you're actually using.
#
# So the order here is:
#   1. MATERIALIZE — replace every cvm-managed symlink in the target dirs with a
#      real, dereferenced copy of its current contents. This freezes the active
#      profile's config in place as plain files, decoupled from cvm and the repo.
#   2. BACKUP (default on) — tar up cvm's own artifacts (state.json, vanilla
#      stash, cloned profiles) so nothing is irreversibly lost.
#   3. REMOVE — delete ONLY cvm's artifacts inside ~/.cvm (never the whole dir,
#      which may be shared with other tooling) and uninstall the cvm binary.
#   4. VERIFY — run verify-uninstall.sh to confirm a clean end state.
#
# This script does NOT depend on the cvm binary (works even if it's broken).
#
# Usage:
#   ./uninstall.sh [--yes] [--no-backup] [--dry-run]
#     --yes        skip the confirmation prompt
#     --no-backup  don't tar cvm's artifacts before deleting them
#     --dry-run    print what would happen, change nothing
set -eu

CVM_HOME="${HOME}/.cvm"
STATE_FILE="${CVM_HOME}/state.json"
VANILLA_DIR="${CVM_HOME}/vanilla"
PROFILES_DIR="${CVM_HOME}/profiles"
INSTALL_BIN="${HOME}/.local/bin/cvm"   # install.sh location

CLAUDE_DIR="${HOME}/.claude"
CODEX_DIR="${CODEX_HOME:-${HOME}/.codex}"
OPENCODE_DIR="${OPENCODE_CONFIG_DIR:-${HOME}/.config/opencode}"

# Managed items per harness (mirror the managed*DirItems slices in Go).
CLAUDE_ITEMS="CLAUDE.md settings.json settings.local.json keybindings.json statusline-command.sh commands skills agents hooks rules output-styles teams"
CODEX_ITEMS="AGENTS.md"
OPENCODE_ITEMS="AGENTS.md opencode.json skills agents commands plugin plugins"

YES=0; BACKUP=1; DRY=0
for arg in "$@"; do
  case "$arg" in
    --yes|-y) YES=1 ;;
    --no-backup) BACKUP=0 ;;
    --dry-run|-n) DRY=1 ;;
    -h|--help) sed -n '2,33p' "$0"; exit 0 ;;
    *) echo "unknown flag: $arg" >&2; exit 2 ;;
  esac
done

if [ -t 1 ]; then
  GREEN=$(printf '\033[32m'); RED=$(printf '\033[31m'); YEL=$(printf '\033[33m'); BLU=$(printf '\033[34m'); RST=$(printf '\033[0m')
else
  GREEN=; RED=; YEL=; BLU=; RST=
fi
info() { printf '%s::%s %s\n' "$BLU" "$RST" "$1"; }
ok()   { printf '%s ok%s %s\n' "$GREEN" "$RST" "$1"; }
warn() { printf '%swarn%s %s\n' "$YEL" "$RST" "$1"; }
act()  { if [ "$DRY" -eq 1 ]; then printf '%s[dry-run]%s would %s\n' "$YEL" "$RST" "$1"; else printf '       %s\n' "$1"; fi; }

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

# --- Report current state (best effort, never fatal) ---
info "cvm uninstall"
if [ -f "$STATE_FILE" ]; then
  # The harnesses block maps harness -> active profile; show those lines only.
  actives=$(grep -E '"(claude|opencode|codex)": *"' "$STATE_FILE" 2>/dev/null | sed -E 's/.*"([a-z]+)": *"([^"]*)".*/\1=\2/' | tr '\n' ' ' || true)
  [ -n "$actives" ] && info "active profile per harness: $actives"
fi
HAVE_BREW=0
if command -v brew >/dev/null 2>&1 && brew list cvm >/dev/null 2>&1; then HAVE_BREW=1; fi
if [ ! -e "$STATE_FILE" ] && [ ! -e "$VANILLA_DIR" ] && [ ! -e "$PROFILES_DIR" ] \
   && [ ! -e "$INSTALL_BIN" ] && [ "$HAVE_BREW" -eq 0 ] && ! command -v cvm >/dev/null 2>&1; then
  warn "nothing to uninstall — no cvm artifacts or binary found."
fi

# --- Confirmation ---
if [ "$DRY" -eq 0 ] && [ "$YES" -eq 0 ]; then
  printf '\nThis will materialize your ACTIVE profile config as real files, then remove\n'
  printf "cvm's state (state.json, vanilla, profiles) and the cvm binary.\n"
  printf 'Your repos and any non-cvm data under ~/.cvm are left untouched.\n'
  [ "$BACKUP" -eq 1 ] && printf "A backup tarball of cvm's artifacts will be created first.\n"
  printf 'Continue? [y/N] '
  read -r reply || reply=""
  case "$reply" in y|Y|yes|YES) ;; *) echo "aborted."; exit 1 ;; esac
fi

# --- Step 1: materialize cvm-managed symlinks into real files ---
# At a managed-item path, cvm is the only thing that creates symlinks (real files
# there are yours/vanilla and are left untouched). So: any symlink at such a path
# is cvm's — dereference it into a real copy in place. Dangling ones (profile
# source gone) are removed with a warning.
materialize() {
  path=$1; label=$2
  [ -L "$path" ] || return 0
  target=$(readlink "$path" 2>/dev/null || echo "?")
  if [ ! -e "$path" ]; then
    warn "$label: dangling symlink (target missing) $path -> $target"
    act "remove dangling symlink $path"
    [ "$DRY" -eq 1 ] || rm -f "$path"
    return 0
  fi
  act "materialize $path (was -> $target)"
  if [ "$DRY" -eq 0 ]; then
    tmp="${path}.cvm-uninstall.$$"
    rm -rf "$tmp"
    cp -RL "$path" "$tmp"   # -L dereferences the symlink and any nested links
    rm -f "$path"
    mv "$tmp" "$path"
  fi
}

materialize_dir() {
  dir=$1; items=$2; label=$3
  [ -d "$dir" ] || return 0
  for item in $items; do
    materialize "$dir/$item" "$label"
  done
}

info "materializing active-profile config (symlinks -> real files)"
materialize_dir "$CLAUDE_DIR" "$CLAUDE_ITEMS" "claude"
materialize_dir "$CODEX_DIR" "$CODEX_ITEMS" "codex"
materialize_dir "$OPENCODE_DIR" "$OPENCODE_ITEMS" "opencode"
ok "materialization done"

# --- Step 2: backup cvm's own artifacts ---
if [ "$BACKUP" -eq 1 ]; then
  set --
  [ -e "$STATE_FILE" ]   && set -- "$@" state.json
  [ -e "$VANILLA_DIR" ]  && set -- "$@" vanilla
  [ -e "$PROFILES_DIR" ] && set -- "$@" profiles
  if [ "$#" -gt 0 ]; then
    ts=$(date +%Y%m%d-%H%M%S 2>/dev/null || echo backup)
    backup="${HOME}/cvm-backup-${ts}.tar.gz"
    info "backing up cvm artifacts ($*) to $backup"
    act "create $backup"
    if [ "$DRY" -eq 0 ]; then
      tar -czf "$backup" -C "$CVM_HOME" "$@" && ok "backup written: $backup"
    fi
  fi
fi

# --- Step 3: remove cvm artifacts (surgical — ~/.cvm may be shared) ---
info "removing cvm artifacts under ~/.cvm"
for p in "$STATE_FILE" "$VANILLA_DIR" "$PROFILES_DIR"; do
  if [ -e "$p" ]; then
    act "rm -rf $p"
    [ "$DRY" -eq 1 ] || rm -rf "$p"
  fi
done
# If ~/.cvm now holds nothing but cvm ever created, and it's empty, drop it too.
if [ -d "$CVM_HOME" ] && [ -z "$(ls -A "$CVM_HOME" 2>/dev/null)" ]; then
  act "rmdir empty $CVM_HOME"
  [ "$DRY" -eq 1 ] || rmdir "$CVM_HOME"
fi

# --- Step 3b: remove the binary (detect install method) ---
info "removing cvm binary"
if [ "$HAVE_BREW" -eq 1 ]; then
  act "brew uninstall cvm"
  [ "$DRY" -eq 1 ] || brew uninstall cvm || warn "brew uninstall cvm failed"
fi
if [ -e "$INSTALL_BIN" ]; then
  act "rm -f $INSTALL_BIN"
  [ "$DRY" -eq 1 ] || rm -f "$INSTALL_BIN"
fi
# go install location, if any.
if command -v go >/dev/null 2>&1; then
  gobin="$(go env GOPATH 2>/dev/null)/bin/cvm"
  if [ -e "$gobin" ]; then
    act "rm -f $gobin"
    [ "$DRY" -eq 1 ] || rm -f "$gobin"
  fi
fi

if [ "$DRY" -eq 0 ] && command -v cvm >/dev/null 2>&1; then
  warn "a 'cvm' is still on PATH at $(command -v cvm) — installed by an unrecognized method; remove it manually."
fi

if [ "$DRY" -eq 1 ]; then
  echo; warn "dry-run complete — nothing was changed."
  exit 0
fi

# --- Step 4: verify ---
echo
info "verifying clean uninstall"
if [ -f "$SCRIPT_DIR/verify-uninstall.sh" ]; then
  sh "$SCRIPT_DIR/verify-uninstall.sh"
else
  warn "verify-uninstall.sh not found next to this script; skipping validation."
fi

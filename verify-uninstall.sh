#!/bin/sh
# verify-uninstall.sh — validates that cvm was uninstalled cleanly.
#
# Post-uninstall invariants:
#   1. cvm's own artifacts are gone: ~/.cvm/state.json, ~/.cvm/vanilla,
#      ~/.cvm/profiles. (~/.cvm itself may legitimately remain — it can be
#      shared with other tooling — so we do NOT require it to be absent.)
#   2. No cvm binary is resolvable: not on PATH, not at the install.sh path,
#      not brew-managed, not in GOPATH/bin.
#   3. No harness target dir contains a symlink for a managed item. After a
#      clean uninstall every managed item is either absent or a REAL file/dir
#      (the materialized active-profile config). A leftover symlink means a
#      cvm link survived; a dangling one means data was lost.
#
# Exit 0 = clean, non-zero = something is still off. Read-only; safe anytime.
set -eu

CVM_HOME="${HOME}/.cvm"
INSTALL_BIN="${HOME}/.local/bin/cvm"

CLAUDE_DIR="${HOME}/.claude"
CODEX_DIR="${CODEX_HOME:-${HOME}/.codex}"
OPENCODE_DIR="${OPENCODE_CONFIG_DIR:-${HOME}/.config/opencode}"

CLAUDE_ITEMS="CLAUDE.md settings.json settings.local.json keybindings.json statusline-command.sh commands skills agents hooks rules output-styles teams"
CODEX_ITEMS="AGENTS.md"
OPENCODE_ITEMS="AGENTS.md opencode.json skills agents commands plugin plugins"

if [ -t 1 ]; then
  GREEN=$(printf '\033[32m'); RED=$(printf '\033[31m'); YEL=$(printf '\033[33m'); RST=$(printf '\033[0m')
else
  GREEN=; RED=; YEL=; RST=
fi
FAILS=0
pass() { printf '%s  ok %s %s\n' "$GREEN" "$RST" "$1"; }
fail() { printf '%s fail%s %s\n' "$RED" "$RST" "$1"; FAILS=$((FAILS + 1)); }
note() { printf '%s note%s %s\n' "$YEL" "$RST" "$1"; }

# --- Check 1: cvm artifacts removed ---
for p in "$CVM_HOME/state.json" "$CVM_HOME/vanilla" "$CVM_HOME/profiles"; do
  if [ -e "$p" ]; then fail "cvm artifact still present: $p"; else pass "removed: $p"; fi
done
[ -d "$CVM_HOME" ] && note "~/.cvm still exists (expected if shared with other tooling)"

# --- Check 2: binary gone, every install method ---
if [ -e "$INSTALL_BIN" ]; then fail "binary still at $INSTALL_BIN"; else pass "no binary at install.sh path"; fi
if command -v brew >/dev/null 2>&1 && brew list cvm >/dev/null 2>&1; then
  fail "cvm still installed via Homebrew (brew uninstall cvm)"
else
  pass "not brew-managed"
fi
if command -v go >/dev/null 2>&1 && [ -e "$(go env GOPATH 2>/dev/null)/bin/cvm" ]; then
  fail "cvm still present in GOPATH/bin"
else
  pass "no cvm in GOPATH/bin"
fi
if command -v cvm >/dev/null 2>&1; then
  fail "a 'cvm' is still on PATH at $(command -v cvm)"
else
  pass "no cvm on PATH"
fi

# --- Check 3: no leftover symlinks among managed items ---
check_dir() {
  dir=$1; items=$2; label=$3
  [ -d "$dir" ] || { pass "$label dir absent ($dir)"; return; }
  local_fail=0
  for item in $items; do
    path="$dir/$item"
    if [ -L "$path" ]; then
      target=$(readlink "$path" 2>/dev/null || echo "?")
      if [ -e "$path" ]; then
        fail "$label: leftover symlink (should be a real file) $path -> $target"
      else
        fail "$label: dangling symlink (data lost) $path -> $target"
      fi
      local_fail=1
    fi
  done
  [ "$local_fail" -eq 0 ] && pass "$label: managed items are real files / absent (no symlinks)"
}
check_dir "$CLAUDE_DIR" "$CLAUDE_ITEMS" "claude"
check_dir "$CODEX_DIR" "$CODEX_ITEMS" "codex"
check_dir "$OPENCODE_DIR" "$OPENCODE_ITEMS" "opencode"

echo
if [ "$FAILS" -eq 0 ]; then
  printf '%sAll checks passed — cvm is uninstalled cleanly.%s\n' "$GREEN" "$RST"
  exit 0
else
  printf '%s%d check(s) failed.%s\n' "$RED" "$FAILS" "$RST"
  exit 1
fi

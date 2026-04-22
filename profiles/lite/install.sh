#!/usr/bin/env bash
# Instala/sincroniza los skills del profile lite en ~/.claude/skills/.
#
# Los skills viven en dos locations:
#   - profiles/lite/skills/  (versionado en el repo)
#   - ~/.claude/skills/      (leido por Claude Code al resolver slash-commands)
#
# Este script copia cada SKILL.md del repo al ~/.claude/skills/ correspondiente.
# Idempotente: re-correrlo sobrescribe con la version del repo.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC_DIR="$SCRIPT_DIR/skills"
DEST_DIR="$HOME/.claude/skills"

if [ ! -d "$SRC_DIR" ]; then
  echo "error: no existe $SRC_DIR" >&2
  exit 1
fi

mkdir -p "$DEST_DIR"

count=0
for src in "$SRC_DIR"/*/SKILL.md; do
  [ -f "$src" ] || continue
  name="$(basename "$(dirname "$src")")"
  dest_skill_dir="$DEST_DIR/$name"
  mkdir -p "$dest_skill_dir"
  cp "$src" "$dest_skill_dir/SKILL.md"
  echo "installed: /$name"
  count=$((count + 1))
done

echo ""
echo "done: $count skill(s) sincronizados a $DEST_DIR"

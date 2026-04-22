#!/usr/bin/env bash
# Instala/sincroniza los skills del profile lite en ~/.claude/skills/.
#
# Uso:
#   bash profiles/lite/install.sh
# (correr desde la raiz del repo o desde cualquier cwd — usa $BASH_SOURCE)
#
# Los skills viven en dos locations:
#   - profiles/lite/skills/  (versionado en el repo)
#   - ~/.claude/skills/      (leido por Claude Code al resolver slash-commands)
#
# Este script copia cada SKILL.md del repo al ~/.claude/skills/ correspondiente.
# Idempotente: re-correrlo sobrescribe con la version del repo.
#
# Limitacion conocida: solo copia SKILL.md de cada skill. Si un skill tiene
# archivos auxiliares (scripts, templates, fixtures) bajo profiles/lite/skills/<name>/,
# NO los sincroniza. Hoy ningun skill del profile lite los usa; si se agrega uno
# que los necesite, extender el loop con `cp -R` del directorio completo.

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

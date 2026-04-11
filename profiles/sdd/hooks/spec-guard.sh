#!/bin/bash
# PreToolUse hook: advisory warning when writing code files without a spec
# Does NOT block — just reminds

INPUT=$(cat)

FILE=""
if command -v jq >/dev/null 2>&1; then
  FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty' 2>/dev/null)
else
  FILE=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_input',{}).get('file_path',''))" 2>/dev/null)
fi

if [ -z "$FILE" ]; then
  exit 0
fi

# Only check source code files
case "$FILE" in
  */specs/*|*/test/*|*/tests/*|*/__tests__/*|*.spec.*|*.test.*|*.config.*|*.json|*.md|*.yml|*.yaml)
    exit 0
    ;;
  *.ts|*.tsx|*.js|*.jsx|*.py|*.go|*.rs|*.rb|*.java)
    ;;
  *)
    exit 0
    ;;
esac

# Find project root
PROJECT_DIR=$(git -C "$(dirname "$FILE" 2>/dev/null)" rev-parse --show-toplevel 2>/dev/null)
if [ -z "$PROJECT_DIR" ]; then
  exit 0
fi

SPECS_DIR="$PROJECT_DIR/specs"
if [ ! -d "$SPECS_DIR" ]; then
  exit 0
fi

# Check if there are specs pending implementation
REGISTRY="$SPECS_DIR/REGISTRY.md"
if [ -f "$REGISTRY" ]; then
  if grep -Fq -- 'draft' "$REGISTRY" 2>/dev/null || grep -Fq -- 'approved' "$REGISTRY" 2>/dev/null; then
    echo "[sdd-guard] Specs pending implementation exist. Ensure this change is spec-aligned." >&2
  fi
fi

exit 0

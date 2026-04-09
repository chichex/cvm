#!/bin/bash
# Detects common AI-generated code slop after Write/Edit
# Used by PostToolUse hook on Write|Edit

FILE="${CLAUDE_FILE_PATH:-}"

if [ -z "$FILE" ] || [ ! -f "$FILE" ]; then
  exit 0
fi

# Only check code files
case "$FILE" in
  *.ts|*.tsx|*.js|*.jsx|*.py|*.go|*.rs|*.rb)
    ;;
  *)
    exit 0
    ;;
esac

ISSUES=""

if grep -qn 'as any' "$FILE" 2>/dev/null; then
  ISSUES="${ISSUES}\n  - 'as any' found"
fi

if grep -qn '@ts-ignore' "$FILE" 2>/dev/null; then
  ISSUES="${ISSUES}\n  - '@ts-ignore' found"
fi

if grep -qn 'eslint-disable' "$FILE" 2>/dev/null; then
  ISSUES="${ISSUES}\n  - 'eslint-disable' found"
fi

if grep -qnE 'catch\s*\([^)]*\)\s*\{\s*\}' "$FILE" 2>/dev/null; then
  ISSUES="${ISSUES}\n  - Empty catch block found"
fi

if grep -qn 'TODO: implement' "$FILE" 2>/dev/null; then
  ISSUES="${ISSUES}\n  - 'TODO: implement' placeholder found"
fi

if [ -n "$ISSUES" ]; then
  echo "SLOP WARNING in $FILE:$ISSUES" >&2
  # Warning only, don't block
  exit 0
fi

exit 0

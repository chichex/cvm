#!/bin/bash
# Blocks edits to sensitive files (.env, credentials, secrets)
# Used by PreToolUse hook on Write|Edit

FILE="${CLAUDE_FILE_PATH:-}"

if [ -z "$FILE" ]; then
  exit 0
fi

if echo "$FILE" | grep -qE '\.(env|env\.local|env\.production)$|credentials|secrets|\.pem$|\.key$'; then
  echo "BLOCKED: will not modify sensitive file: $FILE" >&2
  exit 1
fi

exit 0

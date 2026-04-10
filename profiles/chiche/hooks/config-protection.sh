#!/bin/bash
# Blocks edits to sensitive files (.env, credentials, secrets)
# Used by PreToolUse hook on Write|Edit
# Hook input arrives via stdin as JSON:
# {"tool_name": "Write", "tool_input": {"file_path": "/path/to/file", "content": "..."}}

INPUT=$(cat)

# Extract file_path from tool_input using jq or python3 fallback
if command -v jq >/dev/null 2>&1; then
  FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty' 2>/dev/null)
else
  FILE=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_input',{}).get('file_path',''))" 2>/dev/null)
fi

if [ -z "$FILE" ]; then
  exit 0
fi

if echo "$FILE" | grep -qE '\.(env|env\.local|env\.production)$|credentials|secrets|\.pem$|\.key$'; then
  echo "BLOCKED: will not modify sensitive file: $FILE" >&2
  exit 2
fi

exit 0

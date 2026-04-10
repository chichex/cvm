#!/bin/bash
# SubagentStop hook: captura pasiva de learnings de subagents.
# Parsea el stdout del subagent buscando "## Key Learnings:" y
# persiste cada item en la KB automaticamente con cvm kb put.

# Read hook input from stdin
INPUT=$(cat)

# Extract the subagent's stdout from hook input
STDOUT=$(echo "$INPUT" | perl -0777 -ne 'if (/"stdout"\s*:\s*"((?:[^"\\]|\\.)*)"/s) { $s=$1; $s=~s/\\n/\n/g; $s=~s/\\"/"/g; $s=~s/\\\\/\\/g; print $s }')

if [ -z "$STDOUT" ]; then
  exit 0
fi

# Check if there's a Key Learnings section
if ! echo "$STDOUT" | grep -q "## Key Learnings:"; then
  exit 0
fi

# Extract lines after "## Key Learnings:" that start with "- " (macOS-compatible)
LEARNINGS=$(echo "$STDOUT" | perl -ne 'if (/^## Key Learnings:/..$found_end) { $found_end=1 if /^(##[^#]|$)/ and !/^## Key Learnings:/; print if /^- / and !$found_end }')

if [ -z "$LEARNINGS" ]; then
  exit 0
fi

# Persist each learning via cvm kb put
COUNT=0
while IFS= read -r line; do
  # Strip the leading "- " prefix
  BODY=$(echo "$line" | sed 's/^- //')

  if [ -z "$BODY" ]; then
    continue
  fi

  # Generate a key from first few words
  KEY=$(echo "$BODY" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9 ]//g' | awk '{for(i=1;i<=4&&i<=NF;i++) printf "%s-",$i; print ""}' | sed 's/-$//' | head -c 50)

  # Check for duplicates silently
  SEARCH_OUT=$(cvm kb search "$KEY" 2>/dev/null)
  if [ -n "$SEARCH_OUT" ] && ! echo "$SEARCH_OUT" | grep -qiE '(no matches|no results|0 results)'; then
    continue
  fi

  cvm kb put "$KEY" --body "$BODY" --tag "learning,auto-captured" 2>/dev/null
  COUNT=$((COUNT + 1))
done <<< "$LEARNINGS"

if [ "$COUNT" -gt 0 ]; then
  cat <<HOOK
{
  "hookSpecificOutput": {
    "hookEventName": "SubagentStop",
    "additionalContext": "[auto-captured] $COUNT learning(s) extraidos del subagent y guardados en KB."
  }
}
HOOK
fi

exit 0

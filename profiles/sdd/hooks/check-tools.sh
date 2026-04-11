#!/bin/bash
# SessionStart hook — detect available tools and write to ~/.cvm/available-tools.json

tools_file="$HOME/.cvm/available-tools.json"
mkdir -p "$(dirname "$tools_file")"

# Build JSON manually to avoid jq dependency
json="{"
first=true

for tool in codex gh docker node npm go python3 ruff cargo make; do
  if [ "$first" = true ]; then
    first=false
  else
    json="$json,"
  fi

  if command -v "$tool" > /dev/null 2>&1; then
    path=$(command -v "$tool")
    json="$json\"$tool\":{\"available\":true,\"path\":\"$path\"}"
  else
    json="$json\"$tool\":{\"available\":false}"
  fi
done

json="$json}"

echo "$json" > "$tools_file"

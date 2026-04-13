#!/bin/bash
# SessionStart hook — detect available tools and write to ~/.cvm/available-tools.json

tools_file="$HOME/.cvm/available-tools.json"
mkdir -p "$(dirname "$tools_file")"

# Build JSON manually to avoid jq dependency
json="{"
first=true

for tool in codex gemini gh docker node npm go python3 ruff cargo make; do
  if [ "$first" = true ]; then
    first=false
  else
    json="$json,"
  fi

  if command -v "$tool" > /dev/null 2>&1; then
    path=$(command -v "$tool")
    # Codex needs deeper validation: binary exists != configured
    if [ "$tool" = "codex" ]; then
      if codex exec "echo ok" > /dev/null 2>&1; then
        json="$json\"$tool\":{\"available\":true,\"path\":\"$path\",\"verified\":true}"
      else
        json="$json\"$tool\":{\"available\":false,\"path\":\"$path\",\"verified\":false,\"reason\":\"installed but not configured (license, auth, or sandbox issue)\"}"
      fi
    # Spec: S-012 | Req: B-001 | Gemini needs deeper validation: LLM prompt health check
    elif [ "$tool" = "gemini" ]; then
      gemini_output=$(gemini -p "reply with exactly: ok" 2>/dev/null) && echo "$gemini_output" | grep -q "ok"
      if [ $? -eq 0 ]; then
        json="$json\"$tool\":{\"available\":true,\"path\":\"$path\",\"verified\":true}"
      else
        json="$json\"$tool\":{\"available\":false,\"path\":\"$path\",\"verified\":false,\"reason\":\"installed but not configured\"}"
      fi
    else
      json="$json\"$tool\":{\"available\":true,\"path\":\"$path\"}"
    fi
  else
    json="$json\"$tool\":{\"available\":false}"
  fi
done

json="$json}"

echo "$json" > "$tools_file"

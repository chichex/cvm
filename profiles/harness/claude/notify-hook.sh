#!/usr/bin/env bash
# notify-send wrapper for Claude Code Stop/Notification hooks.
# Uso: notify-hook.sh <title> <urgency>
# Reporta el workspace de Hyprland donde corre la terminal y el cwd actual.

set -u

title="${1:-Claude Code}"
urgency="${2:-normal}"

workspace=""
if command -v hyprctl >/dev/null 2>&1 && command -v jq >/dev/null 2>&1; then
  clients_json="$(hyprctl clients -j 2>/dev/null || true)"
  pid=$$
  while [ -n "$pid" ] && [ "$pid" != "1" ]; do
    ws="$(printf '%s' "$clients_json" | jq -r --argjson pid "$pid" '.[] | select(.pid == $pid) | .workspace.name' 2>/dev/null | head -1)"
    if [ -n "$ws" ] && [ "$ws" != "null" ]; then
      workspace="$ws"
      break
    fi
    pid="$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ')"
  done
fi

[ -z "$workspace" ] && workspace="?"

body="ws ${workspace}
${PWD}"

notify-send -a 'Claude Code' -u "$urgency" "$title" "$body"

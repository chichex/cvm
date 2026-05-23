#!/usr/bin/env bash
# notify-send wrapper for Claude Code Stop/Notification hooks.
# Uso: notify-hook.sh <title> <urgency>
# Reporta el workspace de Hyprland donde corre la terminal y el cwd actual.
# Si detecta la ventana, adjunta una default action: al click izquierdo en la
# notificacion, Hyprland enfoca la ventana de origen (focuswindow address:...).

set -u

title="${1:-Claude Code}"
urgency="${2:-normal}"

workspace=""
address=""
if command -v hyprctl >/dev/null 2>&1 && command -v jq >/dev/null 2>&1; then
  clients_json="$(hyprctl clients -j 2>/dev/null || true)"
  pid=$$
  while [ -n "$pid" ] && [ "$pid" != "1" ]; do
    match="$(printf '%s' "$clients_json" \
      | jq -r --argjson pid "$pid" \
        '.[] | select(.pid == $pid) | "\(.workspace.name)\t\(.address)"' \
        2>/dev/null | head -1)"
    if [ -n "$match" ] && [ "$match" != "null"$'\t'"null" ]; then
      workspace="${match%%$'\t'*}"
      address="${match##*$'\t'}"
      break
    fi
    pid="$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ')"
  done
fi

[ -z "$workspace" ] && workspace="?"

body="ws ${workspace}
${PWD}"

if [ -n "$address" ] || [ "$workspace" != "?" ]; then
  # notify-send -A implica --wait: queda vivo hasta el click o expiracion.
  # Lo corremos en background + disown para no bloquear el hook de Claude.
  (
    action="$(notify-send -a 'Claude Code' -u "$urgency" \
      -A 'default=Ir' "$title" "$body" 2>/dev/null)"
    if [ "$action" = "default" ]; then
      if [ -n "$address" ] \
        && hyprctl dispatch focuswindow "address:${address}" >/dev/null 2>&1; then
        :
      elif [ "$workspace" != "?" ]; then
        hyprctl dispatch workspace "name:${workspace}" >/dev/null 2>&1 || true
      fi
    fi
  ) >/dev/null 2>&1 &
  disown 2>/dev/null || true
else
  notify-send -a 'Claude Code' -u "$urgency" "$title" "$body"
fi

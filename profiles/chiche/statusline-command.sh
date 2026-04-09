#!/usr/bin/env bash
input=$(cat)
cwd=$(echo "$input" | jq -r '.workspace.current_dir // .cwd')
used=$(echo "$input" | jq -r '.context_window.used_percentage // empty')

# Colors
CYAN='\033[36m'
GREEN='\033[32m'
RED='\033[31m'
BLUE='\033[1;34m'
YELLOW='\033[33m'
MAGENTA='\033[35m'
DIM='\033[2m'
RESET='\033[0m'

# Shorten home directory to ~
home="$HOME"
short_cwd="${cwd/#$home/~}"

# Git branch and dirty status
git_info=""
if git_branch=$(GIT_OPTIONAL_LOCKS=0 git -C "$cwd" symbolic-ref --short HEAD 2>/dev/null); then
  if ! GIT_OPTIONAL_LOCKS=0 git -C "$cwd" diff --quiet 2>/dev/null || ! GIT_OPTIONAL_LOCKS=0 git -C "$cwd" diff --cached --quiet 2>/dev/null; then
    dirty=" ${YELLOW}*${RESET}"
  else
    dirty=""
  fi
  git_info=" ${BLUE}(${RED}${git_branch}${BLUE})${RESET}${dirty}"
fi

# Context usage with color based on percentage
ctx_info=""
if [ -n "$used" ]; then
  pct=$(echo "$used" | awk '{printf "%d", $1+0.5}')
  if [ "$pct" -ge 75 ]; then
    ctx_color="$RED"
  elif [ "$pct" -ge 50 ]; then
    ctx_color="$YELLOW"
  else
    ctx_color="$DIM"
  fi
  ctx_info=" ${ctx_color}[ctx:${pct}%]${RESET}"
fi

# CVM active profile
cvm_info=""
if command -v cvm &>/dev/null; then
  profile=$(cvm global current 2>/dev/null)
  if [ -n "$profile" ] && [ "$profile" != "(vanilla)" ]; then
    cvm_info=" ${MAGENTA}[${profile}]${RESET}"
  fi
fi

printf '%b' "${GREEN}>${RESET} ${CYAN}${short_cwd}${RESET}${git_info}${cvm_info}${ctx_info}\n"

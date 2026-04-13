#!/usr/bin/env bash
# Spec: S-010 | Req: B-003 | Session digest extraction
# Extracts a structured digest from the session transcript JSONL.
# Pure shell + python3 — no LLM calls (I-002). Idempotent (I-005).

set -euo pipefail

MIN_TOOLS="${CVM_AUTOSUMMARY_MIN_TOOLS:-3}"

# --- Locate the transcript JSONL ---
CLAUDE_DIR="${HOME}/.claude"

# Derive project dir name (same as Claude Code: replace / and _ with -, keep leading -)
cwd="$(pwd)"
project_dir=$(echo "$cwd" | sed 's|/|-|g; s|_|-|g')

# Use project_dir for digest filename so auto-summary.sh can find it (NOT $$, which differs per process)
DIGEST_FILE="/tmp/cvm-session-digest-${project_dir}.txt"
project_path="${CLAUDE_DIR}/projects/${project_dir}"

if [ ! -d "$project_path" ]; then
  echo "transcript dir not found, skipping auto-summary" >&2
  exit 0
fi

# Find most recent .jsonl file (the current session's transcript)
transcript=$(ls -t "${project_path}"/*.jsonl 2>/dev/null | head -1)

if [ -z "$transcript" ] || [ ! -f "$transcript" ]; then
  echo "transcript not found, skipping auto-summary" >&2
  exit 0
fi

# --- Extract digest using python3 ---
python3 << 'PYEOF' "$transcript" "$MIN_TOOLS" "$DIGEST_FILE"
import json
import sys
import os
from datetime import datetime

transcript_path = sys.argv[1]
min_tools = int(sys.argv[2])
digest_file = sys.argv[3]

# Parse JSONL
messages = []
with open(transcript_path, 'r') as f:
    for line in f:
        line = line.strip()
        if not line:
            continue
        try:
            messages.append(json.loads(line))
        except json.JSONDecodeError:
            continue

if not messages:
    print("empty transcript, skipping auto-summary", file=sys.stderr)
    sys.exit(0)

# Count tool uses
tool_uses = []
user_prompts = []
assistant_texts = []
files_modified = set()
files_read = set()
tool_counts = {}

for msg in messages:
    msg_type = msg.get('type', '')

    # User messages (human)
    if msg_type == 'human':
        content = msg.get('message', {}).get('content', '')
        if isinstance(content, str) and content.strip():
            user_prompts.append(content.strip()[:300])
        elif isinstance(content, list):
            for block in content:
                if isinstance(block, dict) and block.get('type') == 'text':
                    text = block.get('text', '').strip()
                    if text:
                        user_prompts.append(text[:300])

    # Assistant messages
    elif msg_type == 'assistant':
        content = msg.get('message', {}).get('content', [])
        if isinstance(content, list):
            for block in content:
                if isinstance(block, dict):
                    if block.get('type') == 'text':
                        text = block.get('text', '').strip()
                        if text:
                            assistant_texts.append(text[:200])
                    elif block.get('type') == 'tool_use':
                        tool_name = block.get('name', 'unknown')
                        tool_counts[tool_name] = tool_counts.get(tool_name, 0) + 1
                        tool_input = block.get('input', {})

                        # Track files
                        if tool_name in ('Write', 'Edit', 'NotebookEdit'):
                            fp = tool_input.get('file_path', '')
                            if fp:
                                files_modified.add(fp)
                        elif tool_name == 'Read':
                            fp = tool_input.get('file_path', '')
                            if fp:
                                files_read.add(fp)
                        elif tool_name == 'Bash':
                            cmd = tool_input.get('command', '')
                            # Simple heuristic for file modifications via bash
                            if cmd:
                                for token in cmd.split():
                                    if '/' in token and not token.startswith('-'):
                                        pass  # Too noisy to track

total_tool_uses = sum(tool_counts.values())

# Check threshold
if total_tool_uses < min_tools:
    print("session too short, skipping auto-summary", file=sys.stderr)
    sys.exit(0)

# Compute session duration
first_ts = None
last_ts = None
for msg in messages:
    ts = msg.get('timestamp', '')
    if ts:
        try:
            if isinstance(ts, str):
                dt = datetime.fromisoformat(ts)
            elif isinstance(ts, (int, float)):
                dt = datetime.fromtimestamp(ts / 1000 if ts > 1e12 else ts)
            else:
                continue
            if first_ts is None or dt < first_ts:
                first_ts = dt
            if last_ts is None or dt > last_ts:
                last_ts = dt
        except (ValueError, OSError):
            continue

duration_str = "unknown"
if first_ts and last_ts:
    delta = last_ts - first_ts
    minutes = int(delta.total_seconds() / 60)
    if minutes >= 60:
        duration_str = f"{minutes // 60}h {minutes % 60}m"
    else:
        duration_str = f"{minutes}m"

# --- Build digest (cap at ~6000 chars) ---
lines = []
lines.append(f"# Session Digest")
lines.append(f"Duration: {duration_str}")
lines.append(f"Tool uses: {total_tool_uses}")
lines.append("")

# User prompts (max 10)
lines.append("## User Prompts")
for i, p in enumerate(user_prompts[:10]):
    lines.append(f"  {i+1}. {p}")
lines.append("")

# Assistant reasoning (max 15)
lines.append("## Assistant Reasoning")
for i, t in enumerate(assistant_texts[:15]):
    lines.append(f"  {i+1}. {t}")
lines.append("")

# Tool counts
lines.append("## Tools Used")
for tool, count in sorted(tool_counts.items(), key=lambda x: -x[1]):
    lines.append(f"  - {tool}: {count}")
lines.append("")

# Files modified (max 20)
if files_modified:
    lines.append("## Files Modified")
    for f in sorted(files_modified)[:20]:
        lines.append(f"  - {f}")
    lines.append("")

# Files read (max 15)
if files_read:
    lines.append("## Files Read")
    for f in sorted(files_read)[:15]:
        lines.append(f"  - {f}")

digest = "\n".join(lines)

# Cap at 6000 chars
if len(digest) > 6000:
    digest = digest[:6000] + "\n... (truncated)"

# Write digest
with open(digest_file, 'w') as f:
    f.write(digest)

print(f"digest written to {digest_file} ({len(digest)} chars, ~{len(digest)//4} tokens)", file=sys.stderr)
PYEOF

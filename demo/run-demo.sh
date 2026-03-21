#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export TERM="xterm-256color"
export COLORTERM="truecolor"
export CLICOLOR_FORCE="1"

"$repo_root/scripts/seed-demo-sessions.sh"

if command -v tmux-all-the-time >/dev/null 2>&1; then
  exec "$(command -v tmux-all-the-time)"
fi

if [[ -x "$repo_root/tmux-all-the-time" ]]; then
  exec "$repo_root/tmux-all-the-time"
fi

exec go run "$repo_root"

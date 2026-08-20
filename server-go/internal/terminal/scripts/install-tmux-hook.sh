#!/usr/bin/env bash
# Installs cardputerme's tmux session-created hook — the thing that makes a
# brand-new tmux session expose itself with zero manual commands.
#
# Two effects, both required:
#   1. applied live, to the tmux server already running (so it takes effect
#      immediately, with nothing to restart)
#   2. persisted to ~/.tmux.conf, so it survives a `tmux kill-server` — the
#      Go server only re-applies (1) on its own startup, not on tmux's.
#
# Exits non-zero — with a reason on stderr — if either step fails. The Go
# server treats that as fatal: an exposed terminal whose new tmux sessions
# silently never expose themselves is a worse failure mode than not starting.
set -euo pipefail

if ! command -v tmux >/dev/null 2>&1; then
  echo "install-tmux-hook: tmux is not installed (brew install tmux / apt install tmux)" >&2
  exit 1
fi

HOOK_CMD='run-shell -b "cd '"'"'#{pane_current_path}'"'"' && SESSION='"'"'#{session_name}'"'"' cardputerme '"'"'#{session_name}'"'"' >>'"$HOME"'/.cardputerme/hook.log 2>&1"'
HOOK_LINE='set-hook -g session-created "run-shell -b \"cd '"'"'#{pane_current_path}'"'"' && SESSION='"'"'#{session_name}'"'"' cardputerme '"'"'#{session_name}'"'"' >>'"$HOME"'/.cardputerme/hook.log 2>&1\""'
TMUX_CONF="$HOME/.tmux.conf"

if [ ! -f "$TMUX_CONF" ] || ! grep -qF 'set-hook -g session-created' "$TMUX_CONF" 2>/dev/null; then
  printf '\n# cardputerme: expose every new tmux session automatically\n%s\n' "$HOOK_LINE" >>"$TMUX_CONF"
fi

if ! tmux info >/dev/null 2>&1; then
  echo "install-tmux-hook: no tmux server is running to apply the hook to" >&2
  exit 1
fi

if ! tmux set-hook -g session-created "$HOOK_CMD"; then
  echo "install-tmux-hook: tmux set-hook failed" >&2
  exit 1
fi

echo "install-tmux-hook: session-created hook applied and persisted to $TMUX_CONF"

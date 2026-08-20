#!/bin/sh
set -eu

REPO="${CARDPUTERME_REPO:-brutalzinn/cardputerme}"
VERSION="${CARDPUTERME_VERSION:-latest}"

say() { printf 'cardputerme — %s\n' "$1"; }
die() { printf 'cardputerme — %s\n' "$1" >&2; exit 1; }

detect_os() {
  case "$(uname -s)" in
    Darwin) echo darwin ;;
    Linux) echo linux ;;
    *) die "unsupported OS $(uname -s) — macOS and Linux only" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    arm64|aarch64) echo arm64 ;;
    x86_64|amd64) echo amd64 ;;
    *) die "unsupported CPU $(uname -m) — amd64 (Intel) and arm64 only" ;;
  esac
}

pick_bin_dir() {
  if [ -n "${CARDPUTERME_BIN_DIR:-}" ]; then
    echo "$CARDPUTERME_BIN_DIR"
    return
  fi
  if [ -w /usr/local/bin ]; then
    echo /usr/local/bin
    return
  fi
  echo "$HOME/.local/bin"
}

asset_url() {
  if [ "$VERSION" = latest ]; then
    echo "https://github.com/$REPO/releases/latest/download/$1"
    return
  fi
  echo "https://github.com/$REPO/releases/download/$VERSION/$1"
}

fetch() {
  curl -fsSL "$(asset_url "$1")" -o "$2" || die "download failed: $1 ($VERSION) — check the release exists"
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
    return
  fi
  echo ""
}

verify() {
  got="$(sha256_of "$1")"
  if [ -z "$got" ]; then
    say "no sha256 tool found — skipping checksum verification"
    return 0
  fi
  want="$(awk -v n="$2" '$2 == n { print $1 }' "$3")"
  if [ -z "$want" ]; then
    die "$2 is missing from checksums.txt"
  fi
  if [ "$want" != "$got" ]; then
    die "checksum mismatch for $2 — refusing to install"
  fi
}

command -v curl >/dev/null 2>&1 || die "curl is required"

if ! command -v tmux >/dev/null 2>&1 && [ -z "${CARDPUTERME_SKIP_TMUX_CHECK:-}" ]; then
  printf 'cardputerme — tmux is not installed.\n' >&2
  printf '  cardputerme captures your terminal through tmux and does nothing without it.\n' >&2
  printf '  install it first:  brew install tmux   (macOS)   |   sudo apt install tmux   (Ubuntu)\n' >&2
  printf '  then re-run this installer (or set CARDPUTERME_SKIP_TMUX_CHECK=1 to install anyway).\n' >&2
  exit 1
fi

OS="$(detect_os)"
ARCH="$(detect_arch)"
BIN_DIR="$(pick_bin_dir)"
ASSET="cardputerme-$OS-$ARCH"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

say "installing $ASSET ($VERSION) into $BIN_DIR"
fetch "$ASSET" "$TMP/$ASSET"
fetch cardputerme "$TMP/cardputerme"
fetch checksums.txt "$TMP/checksums.txt"

verify "$TMP/$ASSET" "$ASSET" "$TMP/checksums.txt"
verify "$TMP/cardputerme" cardputerme "$TMP/checksums.txt"

mkdir -p "$BIN_DIR"
install -m 755 "$TMP/$ASSET" "$BIN_DIR/cardputerme-server"
install -m 755 "$TMP/cardputerme" "$BIN_DIR/cardputerme"

say "installed $BIN_DIR/cardputerme"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) say "add it to your PATH:  echo 'export PATH=\"$BIN_DIR:\$PATH\"' >> ~/.zshrc" ;;
esac

# --- auto-exposure (#46): every tmux session exposes itself, no command needed ---

HOOK_CMD='run-shell -b "cd '"'"'#{pane_current_path}'"'"' && SESSION='"'"'#{session_name}'"'"' cardputerme '"'"'#{session_name}'"'"' >>$HOME/.cardputerme/hook.log 2>&1"'
TMUX_CONF="$HOME/.tmux.conf"
# NOTE: wrapping HOOK_CMD in single quotes here would collide with the single
# quotes already inside it (around each #{...}) and truncate the parsed
# argument the moment tmux's OWN config-file tokenizer hit the first one —
# verified empirically (`tmux show-hooks -g` came back empty). Double quotes
# with the inner ones escaped is the form tmux's parser actually accepts;
# the direct `tmux set-hook` argv call below needs no such escaping since it
# is never re-tokenized as text.
HOOK_LINE='set-hook -g session-created "run-shell -b \"cd '"'"'#{pane_current_path}'"'"' && SESSION='"'"'#{session_name}'"'"' cardputerme '"'"'#{session_name}'"'"' >>$HOME/.cardputerme/hook.log 2>&1\""'

if [ -f "$TMUX_CONF" ] && grep -qF 'set-hook -g session-created' "$TMUX_CONF" 2>/dev/null; then
  say "tmux auto-expose hook already present in $TMUX_CONF"
else
  printf '\n# cardputerme: expose every new tmux session automatically\n%s\n' "$HOOK_LINE" >>"$TMUX_CONF"
  say "added the auto-expose hook to $TMUX_CONF"
fi
if command -v tmux >/dev/null 2>&1 && tmux info >/dev/null 2>&1; then
  tmux set-hook -g session-created "$HOOK_CMD" 2>/dev/null && say "applied the hook to the tmux server already running"
fi

# A "boot" tmux session is what makes cardputerme usable before you ever open a
# terminal: it keeps the tmux server alive (so the hook above has something to
# attach to) and gives the Cardputer something to show with zero terminals open.
#
# This doubles as the watchdog: `cardputerme boot` alone only recreates the
# SERVER if it died (EnsureSession only runs on a fresh spawn, never over the
# attach-via-HTTP path) — it does nothing if "boot" itself was killed while the
# server keeps running. Recreating the tmux session first, unconditionally,
# covers both failures with one idempotent line; the OS scheduler re-runs it
# periodically (StartInterval/timer below) rather than anything in Go polling.
BOOT_CMD="tmux has-session -t boot >/dev/null 2>&1 || tmux new-session -d -s boot -c \"\$HOME\"; $BIN_DIR/cardputerme boot"
case "$OS" in
  darwin)
    AGENT_DIR="$HOME/Library/LaunchAgents"
    AGENT_FILE="$AGENT_DIR/com.cardputerme.boot.plist"
    mkdir -p "$AGENT_DIR"
    cat >"$AGENT_FILE" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.cardputerme.boot</string>
	<key>ProgramArguments</key>
	<array>
		<string>/bin/sh</string>
		<string>-lc</string>
		<string>$BOOT_CMD</string>
	</array>
	<key>WorkingDirectory</key>
	<string>$HOME</string>
	<key>RunAtLoad</key>
	<true/>
	<key>StartInterval</key>
	<integer>60</integer>
	<key>StandardOutPath</key>
	<string>$HOME/.cardputerme/boot.log</string>
	<key>StandardErrorPath</key>
	<string>$HOME/.cardputerme/boot.log</string>
</dict>
</plist>
PLIST
    mkdir -p "$HOME/.cardputerme"
    launchctl unload "$AGENT_FILE" >/dev/null 2>&1 || true
    if launchctl load -w "$AGENT_FILE" >/dev/null 2>&1; then
      say "boot session will start at login (and starting now): $AGENT_FILE"
    else
      say "wrote $AGENT_FILE but could not load it — run: launchctl load -w $AGENT_FILE"
    fi
    ;;
  linux)
    UNIT_DIR="$HOME/.config/systemd/user"
    SERVICE_FILE="$UNIT_DIR/cardputerme-boot.service"
    TIMER_FILE="$UNIT_DIR/cardputerme-boot.timer"
    mkdir -p "$UNIT_DIR" "$HOME/.cardputerme"
    cat >"$SERVICE_FILE" <<UNIT
[Unit]
Description=cardputerme boot session (keeps tmux + cardputerme alive)

[Service]
Type=oneshot
WorkingDirectory=$HOME
ExecStart=/bin/sh -lc '$BOOT_CMD'
UNIT
    cat >"$TIMER_FILE" <<TIMER
[Unit]
Description=Run cardputerme-boot at login and every 60s (watchdog)

[Timer]
OnStartupSec=5
OnUnitActiveSec=60
Unit=cardputerme-boot.service

[Install]
WantedBy=timers.target
TIMER
    if command -v systemctl >/dev/null 2>&1 && systemctl --user daemon-reload >/dev/null 2>&1 && systemctl --user enable --now cardputerme-boot.timer >/dev/null 2>&1; then
      say "boot session watchdog running (every 60s): $TIMER_FILE"
    else
      say "wrote $SERVICE_FILE + $TIMER_FILE but could not enable them — run: systemctl --user enable --now cardputerme-boot.timer"
    fi
    ;;
esac

say "every tmux session now exposes itself automatically — nothing left to run"

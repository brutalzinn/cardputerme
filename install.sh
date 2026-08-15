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

command -v tmux >/dev/null 2>&1 || say "tmux is required at runtime — install it (brew install tmux / apt install tmux)"

say "run 'cardputerme' inside a tmux terminal, then pick it on the Cardputer"

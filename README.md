# cardputerme

**Expose any terminal to an [M5Stack Cardputer ADV](https://shop.m5stack.com/products/m5stack-cardputer-kit-w-m5stamps3) and drive it from your pocket.** Run one command on your computer, pick it on the device, and you're typing into that terminal over Wi-Fi. The only difference from an ssh session is the small screen.

```
cd ~/my-project
cardputerme            # expose a terminal for this directory
```

Then on the Cardputer: pick the exposure from the list and start typing. That's the whole workflow.

---

## How it works

Each `cardputerme` run starts its **own background server** for that one terminal on a free port (8001–8255) and broadcasts a tiny **UDP beacon** every 2 seconds. The Cardputer listens for beacons, shows every exposed terminal on the network (name + your computer's IP:port), and connects to the one you pick over a WebSocket. **Nothing is configured on the device** — no IP, no port, no names to type.

```
Cardputer ADV  ──UDP beacons──►  discovers every exposure on the LAN
      │
      └──WebSocket──►  one server per exposed terminal  ──►  your shell
```

The device is a thin renderer: the server owns the input buffer, history, viewport, colors and layout, and mirrors the terminal's own screen. Any behavior change is a server change — the firmware rarely needs re-flashing.

---

## Install

**Requirement:** `tmux` (`brew install tmux` / `apt install tmux`) — the invisible capture backend; you never interact with it directly. It is not optional: the installer, the launcher and the server each refuse to run without it. Run `cardputerme` **from inside a tmux terminal** — outside one there is nothing to mirror, so it says so and exposes a new empty terminal instead.

```
curl -fsSL https://raw.githubusercontent.com/brutalzinn/cardputerme/main/install.sh | sh
```

Grabs the matching binary from the latest [GitHub Release](https://github.com/brutalzinn/cardputerme/releases), verifies its checksum and installs `cardputerme` into `/usr/local/bin` (or `~/.local/bin` when that is not writable). Builds are native per platform — **macOS Intel and Apple Silicon, Linux amd64 and arm64** — and static (CGO off), so they run on any Ubuntu/glibc version. Pin a version with `CARDPUTERME_VERSION=v0.0.2`, or choose the target with `CARDPUTERME_BIN_DIR=~/bin`.

**From source** (needs Go ≥ 1.26 — the launcher builds the server on first run):

```
git clone <repo> && cd cardputerme
make setup     # fetch Go deps + create firmware/.env from the template
echo "alias cardputerme=\"$PWD/bin/cardputer-server\"" >> ~/.zshrc && source ~/.zshrc
```

**Flash the Cardputer once.** Set your Wi-Fi in the `.env` that `make setup` created, then upload:

```
# edit firmware/cardputer/.env → set WIFI_SSID and WIFI_PASS
make flash     # build + upload the firmware to the Cardputer
```

The Cardputer and your computer must be on the **same Wi-Fi network** (the beacon is a LAN broadcast).

---

## Using it

### Expose a terminal (on your computer)

```
cardputerme               # name = current directory, started in this dir
cardputerme myproject     # or give it an explicit name
```

Re-running the same name just says "already exposed". Each exposure prints a log path under `~/.cardputerme/<name>.log`. A server shuts itself down when its terminal goes away.

### Connect (on the Cardputer)

1. It joins Wi-Fi, then shows the **server list** — each row is `name` + `IP:port`. If there's only one, it auto-connects.
2. Press the **number** next to an exposure to connect.
3. You now see that terminal mirrored live. Type — it's a real shell.

### Keys

| Key | Action |
|-----|--------|
| letters / numbers | type into the command buffer |
| **Enter** | send the line (or confirm) · **Shift+Enter** newline |
| **digits** (at a prompt) | answer an on-screen `1./2./3.` menu |
| **esc** | clear the buffer, or send a real Escape if empty |
| **Shift+esc** | interrupt (stops an agent / sends Escape to the app) |
| **fn + arrows** | scroll/read around the screen (pan) |
| **opt + arrows** | send real arrow keys (drive a TUI's own selector) |
| **ctrl + letter** | control key (Ctrl+C, etc.) |
| **ctrl + up/down** | recall previous / next command |
| **Tab** | accept the dim autosuggested command (else passes to the terminal) |
| **ctrl + = / ctrl + _** | zoom text in / out · **ctrl + space** reset zoom |
| **fn + esc** | back to the server list |

---

## Running Claude Code (or any long-lived program)

Every exposure is a **persistent tmux session**, so anything you start in it keeps running when you close your laptop lid or walk away — and you can watch and steer it from the Cardputer. This is the point of the project: kick off [Claude Code](https://claude.com/claude-code) (or an agent, a long build, a REPL) at your desk, then monitor and nudge it from your pocket.

**Start it from the Cardputer** — the simplest way:

```
cd ~/my-project
cardputerme                # expose this dir
```

Connect on the device, type `claude`, press Enter. Claude Code now runs in the exposed session and you drive it entirely from the Cardputer — answer its permission prompts with the **number keys**, interrupt with **Shift+esc**, scroll its output with **fn+arrows**.

**Start it at your desk, then follow it from your pocket** — the power move. Because the exposure is a real tmux session, you can attach to the *same* session on your computer:

```
cd ~/my-project
cardputerme                # exposes tmux session "my-project"
tmux attach -t my-project  # attach on your computer (name = the exposure name)
claude                     # run Claude Code here
# ...work at your desk, then Ctrl-b d to detach — it keeps running
```

Now your desk screen **and** the Cardputer mirror the same live session — type from either. Detach at your desk (`Ctrl-b d`), pick up your Cardputer, and keep answering Claude's questions and watching its progress while you're away from the keyboard. (`Ctrl-b` is tmux's prefix; `d` detaches.)

> You never *need* tmux for basic use — `cardputerme` manages the session for you and the Cardputer never shows it. Attaching is only for the dual-drive "desk + pocket" workflow above.

### The `/cardputer` skill (expose without leaving Claude Code)

This repo ships a **Claude Code plugin** with a `/cardputer` skill — run it inside any Claude Code session (that's in tmux) and it exposes *that* session to the device for you.

Install as a plugin:

```
/plugin marketplace add brutalzinn/cardputerme
/plugin install cardputerme@cardputerme
```

Or symlink the skill straight from your clone into your Claude config (works across accounts):

```
ln -sfn "$PWD/skills/cardputer" ~/.claude/skills/cardputer
```

Then in Claude Code: **`/cardputer`** → pick the exposure on the device → you're driving Claude from your pocket.

---

## Layout

```
├── bin/cardputer-server   # the CLI (aliased as `cardputerme`)
├── server-go/             # Go server — one process per exposed terminal, event-driven
│   ├── cmd/cardputerme/   #   entry point
│   └── internal/          #   screen · input · discovery · terminal · server
└── firmware/cardputer/    # flashed once to the Cardputer ADV
```

## Develop

A `Makefile` at the repo root covers setup — run `make` to list targets:

```
make setup      # install Go deps + create firmware/.env
make test       # run the server tests
make flash      # upload firmware to the Cardputer
make release    # cross-compile CLI binaries into dist/ (macOS + Linux)
```

`make release` builds static binaries for macOS and Linux (amd64 + arm64) into `dist/`. On a machine with no Go toolchain, drop the matching `dist/cardputerme-<os>-<arch>` binary in place and `bin/cardputer-server` uses it instead of building.

**Cutting a release.** Push a tag, then manually run the **Release** GitHub Action with that tag:

```
git tag v0.1.0 && git push origin v0.1.0
# then: Actions → Release → Run workflow → tag: v0.1.0
```

The workflow (`.github/workflows/release.yml`) checks out the tag, cross-compiles, and publishes a GitHub Release with the four binaries. (`make release-publish VERSION=vX.Y.Z` does the same locally if you'd rather use `gh`.)

Or drive the server directly:

```
cd server-go && go test ./...     # 72 tests, 4 packages
go vet ./...
```

The tmux backend lives only in `internal/terminal/` — a PTY backend can drop in behind the same interface. The device does no content logic, so most changes are server-side and need no re-flash.

## Troubleshooting

- **Nothing in the server list** — confirm the device and computer are on the same Wi-Fi (not a guest/isolated network), and that a `cardputerme` is running. The beacon is UDP broadcast on port 8000.
- **Reconnecting…** on the device — the server for that exposure stopped (its terminal closed). Run `cardputerme` again.
- **Wrong Wi-Fi** — edit `firmware/cardputer/.env` and re-flash.

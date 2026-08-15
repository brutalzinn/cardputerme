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

**Requirements:** Go ≥ 1.22 (the launcher builds the server on first run) and `tmux` (`brew install tmux`) — the invisible capture backend; you never interact with it directly.

```
git clone <repo> && cd cardputerme
echo "alias cardputerme=\"$PWD/bin/cardputer-server\"" >> ~/.zshrc && source ~/.zshrc
```

**Flash the Cardputer once.** Copy the env template, add your Wi-Fi, and upload:

```
cp firmware/cardputer/.env.example firmware/cardputer/.env
# edit firmware/cardputer/.env → set WIFI_SSID and WIFI_PASS
cd firmware/cardputer
pio run -e cardputer-adv -t upload
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

## Layout

```
├── bin/cardputer-server   # the CLI (aliased as `cardputerme`)
├── server-go/             # Go server — one process per exposed terminal, event-driven
│   ├── cmd/cardputerme/   #   entry point
│   └── internal/          #   screen · input · discovery · terminal · server
└── firmware/cardputer/    # flashed once to the Cardputer ADV
```

## Develop

```
cd server-go && go test ./...     # 71 tests, 4 packages
go vet ./...
```

The tmux backend lives only in `internal/terminal/` — a PTY backend can drop in behind the same interface. The device does no content logic, so most changes are server-side and need no re-flash.

## Troubleshooting

- **Nothing in the server list** — confirm the device and computer are on the same Wi-Fi (not a guest/isolated network), and that a `cardputerme` is running. The beacon is UDP broadcast on port 8000.
- **Reconnecting…** on the device — the server for that exposure stopped (its terminal closed). Run `cardputerme` again.
- **Wrong Wi-Fi** — edit `firmware/cardputer/.env` and re-flash.

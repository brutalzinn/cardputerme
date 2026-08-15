# cardputerme

Expose any terminal to an **M5Stack Cardputer ADV** and drive it from your pocket.
The only difference from an ssh session is the small screen.

```
cd ~/my-project
cardputerme            # exposes a terminal for this dir
cardputerme <name>     # or with an explicit name
```

Each run spawns its own background WebSocket server on a free port (8001–8255)
and broadcasts a UDP beacon (port 8000) every 2 seconds with its name and port.
The Cardputer listens for beacons, lists every exposed terminal with the
computer's IPv4 and port, and connects to the one you pick. Nothing is
configured on the device — no IP, no port, no names.

```
Cardputer ADV  ──UDP beacons──  finds every exposure on the network
      │
      └──WebSocket──►  one server per exposed terminal  ──►  your shell
```

The device is a pure renderer: the server owns the input buffer, history,
viewport, colors and layout, and mirrors the terminal's own screen. Any UI
change is a server change — the firmware rarely needs a re-flash.

## Layout

```
├── bin/cardputer-server   # the CLI (aliased as `cardputerme`)
├── server-go/             # Go server — one process per exposed terminal, event-driven (no polling)
└── firmware/cardputer/    # flashed once to the Cardputer ADV
```

## Setup

Requirements: Go ≥ 1.22 (the launcher builds the server binary on first run).

```
git clone <repo> && cd cardputerme
echo 'alias cardputerme="$PWD/bin/cardputer-server"' >> ~/.zshrc
```

Firmware (once): set your Wi-Fi credentials in `firmware/cardputer/.env`, then

```
cd firmware/cardputer
pio run -e cardputer-adv -t upload
```

## Tests

```
cd server-go && go test ./...
```

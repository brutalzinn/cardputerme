# Cardputer ⇄ Claude Code Remote Control

Drive **Claude Code** on your Mac Mini from an **M5Stack Cardputer ADV** — read
Claude's latest output on the 240×135 color screen as legible paged "cards", and
type commands back on the physical keyboard.

Claude Code has no remote API, so a tiny Node.js server bridges the two using
**tmux**: it reads Claude's terminal with `capture-pane` and injects your typed
commands with `send-keys`.

```
Cardputer ADV  ──HTTP (Wi-Fi)──►  Node.js server  ──tmux──►  Claude Code
  keyboard/screen                    (Mac Mini)         (running in a tmux session)
                          capture-pane = read   ·   send-keys = write
```

```
cardputerme/
├── server/                        # runs on the Mac Mini
│   ├── server.js
│   ├── package.json
│   └── .env.example
└── firmware/cardputer-claude/     # flashed to the Cardputer ADV
    ├── platformio.ini
    └── src/main.cpp
```

---

## 1. Server (Mac Mini)

**Requirements:** Node ≥ 18 and `tmux` (`brew install tmux`).

```bash
cd server
npm install
# optional: cp .env.example .env  and edit it
npm start
```

You should see:

```
Cardputer<->Claude bridge on http://0.0.0.0:4711
  tmux session : claude
  cards        : 20 cols x 5 lines (scrollback 200)
```

### Config (env vars, all optional)

| Var                | Default  | Meaning                                            |
| ------------------ | -------- | -------------------------------------------------- |
| `PORT`             | `4711`   | Port the Cardputer connects to                     |
| `TMUX_SESSION`     | `claude` | tmux session name where Claude Code runs           |
| `WRAP_COLS`        | `20`     | Chars per line — **must match** firmware `WRAP_COLS`|
| `LINES_PER_CARD`   | `5`      | Lines per card                                     |
| `SCROLLBACK_LINES` | `200`    | tmux history lines to include (0 = visible only)   |
| `MAX_CARDS`        | `40`     | Cap on cards returned (keeps device JSON small)    |

To load `.env` inline: `set -a; source .env; set +a; npm start`.

### Find your Mac's LAN IP (put it in the firmware)

```bash
ipconfig getifaddr en0    # Wi-Fi;  try en1 if that's empty
```

---

## 2. Start Claude Code inside tmux

The session name must match `TMUX_SESSION` (default `claude`):

```bash
tmux new -s claude        # create + attach
claude                    # start Claude Code inside it
# detach with:  Ctrl-b  then  d      (Claude keeps running)
# reattach:     tmux attach -t claude
```

---

## 3. Firmware (Cardputer ADV)

Edit the config block at the top of `firmware/cardputer-claude/src/main.cpp`:

```cpp
const char* WIFI_SSID = "YOUR_WIFI_SSID";
const char* WIFI_PASS = "YOUR_WIFI_PASSWORD";
const char* SERVER    = "http://192.168.1.50:4711";  // your Mac's LAN IP + port
const int   WRAP_COLS = 20;   // keep equal to the server's WRAP_COLS
```

The Cardputer must be on the **same Wi-Fi/subnet** as the Mac Mini.

### Option A — PlatformIO (recommended)

```bash
cd firmware/cardputer-claude
pio run -t upload
pio device monitor      # 115200 baud
```

`platformio.ini` targets an ESP32-S3 with 8 MB flash; the `M5Cardputer`
library auto-detects the Cardputer ADV via M5Unified.

### Option B — Arduino IDE

1. Install ESP32 boards (Boards Manager → "esp32" by Espressif).
2. Library Manager → install **M5Cardputer** and **ArduinoJson**.
3. Board: **ESP32S3 Dev Module**; USB CDC On Boot: **Enabled**; Flash Size: **8MB**.
4. Copy `src/main.cpp` into a sketch named `cardputer-claude.ino` (rename the file).
5. Upload.

---

## 4. Using it

On the Cardputer:

- **VIEW mode** (default) — shows one card of Claude's output.
  - `;` previous card &nbsp;·&nbsp; `.` next card
  - `` ` `` refresh &nbsp;·&nbsp; start typing any letter to enter a command
- **INPUT mode** — type a command.
  - **Enter** sends it to Claude and auto-refreshes
  - `` ` `` cancels &nbsp;·&nbsp; `del` backspaces

---

## 5. Quick test & troubleshooting

Test the server directly with `curl` (no Cardputer needed):

```bash
curl localhost:4711/health
# {"ok":true,"session":"claude","exists":true}

curl 'localhost:4711/cards?cols=20&lines=5'
# {"total":N,"cards":[["line","line",...], ...]}

curl -X POST localhost:4711/command \
  -H 'Content-Type: application/json' -d '{"text":"hello claude"}'
# then check the session:  tmux attach -t claude
```

| Symptom                        | Fix                                                             |
| ------------------------------ | -------------------------------------------------------------- |
| `exists:false` / "No session"  | Session name ≠ `TMUX_SESSION`; run `tmux new -s claude`        |
| Cardputer shows `GET -1`/`NoWiFi` | Wrong IP, different subnet, or macOS firewall blocking port 4711 |
| Command doesn't reach Claude   | `tmux attach -t claude` and confirm the pane is the input line |
| Text wraps oddly               | Make firmware `WRAP_COLS` equal to the server's `WRAP_COLS`    |

> **Note on output:** `capture-pane` returns Claude Code's rendered TUI, so cards
> may include some box-drawing chrome. The server strips ANSI codes and trailing
> blanks. If it's ever too noisy, a future tweak is to read Claude's transcript
> JSONL (`~/.claude/projects/<proj>/<uuid>.jsonl`) for the *read* path while
> keeping `send-keys` for the *write* path.

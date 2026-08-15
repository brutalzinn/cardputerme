# cardputerme — Roadmap
> Updated 2026-08-15. Focus: **terminal fidelity (#14)** — mirror the terminal's exact colors + layout. Terse; `git log` holds detail. **#8 E2E user-verified on hardware — the whole system works.**

## What is cardputerme
**Expose ANY terminal to an M5Cardputer with one command.** `cardputerme [name]` (zsh alias → `bin/cardputer-server`) builds+execs a **Go binary** that runs a background WS server for that ONE terminal on a free port (8001–8255) and broadcasts a **UDP beacon** (port 8000, 2s: `{app,v,name,port}`); the device listens, lists every live exposure (**IPv4:port + name**), and connects to the one you pick. NO sessions concept, NO mDNS, NO baked IP. Backend (tmux) invisible & swappable inside `internal/terminal/` only; the whole system = **our Go server + our firmware**. Thin device: draws the server-described display, forwards raw keys; server owns everything (input, history, viewport). House rules: no hooks, no regex, no `else`, no model tokens, no code comments, KISS, TDD, **event-driven (no polling)**; device E2E is the user's. North star: **SSH-parity** — only diff vs ssh is the small screen; mirror the terminal's own colors + layout.

## Focus — terminal fidelity (SSH-parity)
The whole system is verified working on hardware (E2E user-confirmed): discover, connect, mirror, drive, answer prompts, history, autosuggest, marquee, zoom. The last north-star gap is **exactness** — the device should show the terminal's OWN colors and layout, indistinguishable from ssh but for screen size. "Done" = colors match a real terminal side-by-side across ≥2 CLIs.

## 🎯 Now
Current: **#14 terminal-fidelity pass** — audit colors + layout vs a real terminal; extend `ansi.go` where needed.
> **Sync rule (WIP=1):** exactly one task is 🟡 at a time (mirrors the one started tasks.roblab.app task). On finish: mark ✅, log under Done (dated), promote next to 🟡, update this block. Never two 🟡. *(OpenTogg sync stopped.)*

## Plan — 3 days (deploy at end of each day)

**📅 Day 1 — 2026-08-15 · Fidelity (SSH-parity)**
14. 🟡 **Terminal-fidelity pass.** *(current)* #14a bold→bright done (`ansi.go`). Remaining: side-by-side color/layout audit vs a real terminal with ≥2 CLIs (agent CLI + plain bash `ls --color`); extend `internal/screen/ansi.go` coverage where colors diverge; decide if bg/reverse map to anything (device is per-line fg only) or document out-of-scope; README truth-pass. Server-only, TDD.
> 🚀 **Deploy (end of Day 1)** — colors faithful side-by-side; layout mirrors the terminal; docs true.

**📅 Day 2 — 2026-08-16 · Parked items, as chosen**
- ⬜ Pull one item from Parked only if it becomes the focus (WIP=1). Otherwise the roadmap is essentially complete.
> 🚀 **Deploy (end of Day 2)** — n/a until a task is promoted.

## Done 2026-08-15 (Go era)
- **#8 Device E2E — user-verified on hardware.** Full checklist confirmed working on the Cardputer: beacon discovery → WS connect → mirror → run commands → drive live Claude Code → answer prompts → history recall → autosuggest ghost + Tab → marquee → fn+esc back → zoom. 3 bugs E2E caught+fixed: beacon subnet-broadcast (`652d4f7`), grid-based prompt detection, awaiting-on-connect.
- **#11 Zoom (server-owned text size) — verified on device.** Display msg carries `size` (one font, `setTextSize` scales); **ctrl+= / ctrl+_** (Cardputer +/-) zoom in/out, **ctrl+space** resets; viewport `cols()`/`rows()` derive inversely from size (1↔3, size 2 baseline). History recall stays on ctrl+up/down; nav (fn/opt+arrows) unchanged. Firmware flashed; zoom confirmed working on hardware.
- **#16 Conventional Go CLI layout + on-device fixes.** `server-go/` → `cmd/cardputerme/main.go` + `internal/{screen,input,discovery,terminal,server}`; `Server` struct replaces globals; JS `server/` removed; dead code purged (`EndsWithQuestion`, `hub.sendTo`), `deadcode`+`go vet` clean; 67 tests across packages. On-device fixes: grid-based prompt detection, awaiting-on-connect, beacon subnet-broadcast, WS connect logging.
- **#15 Go rewrite (very fast, event-driven, no polling).** tmux `pipe-pane`→fifo change-signal (session death on stream close, zero timers); regex-free; gorilla/websocket; wire integration test (real tmux + gorilla client). JS retired (−658 files).
- **#13 History autosuggest core.** `Suggest()` newest prefix match; Tab accepts (else passes through); dim ghost preview line. *(Ghost UX tuning → #8 on device.)*
- **#12 Processing status.** `splitScreen` prefers the "esc to interrupt" tail row as status, so an agent's live spinner rides the marquee.
- **#14a bold→bright fidelity.** `ansi.go` tracks bold, brightens 30-37→90-97 (matches how terminals render bold+color).
- **#10 Server-per-exposure + UDP beacon discovery.** `cardputerme [name]` = its own WS server (free port, pidfile+log `~/.cardputerme/`, cwd terminal, self-exit); device learns IPv4:port from the beacon packet; sessions concept removed; esc = real Escape; `firmware/cardputer` beacon listener + local picker + dynamic connect.

## Done 2026-08-14 (server-driven everything — JS era, superseded by Go)
- **#1–#7:** agnostic prompt detection · terminal adapter · named sessions · display protocol + ANSI→RGB565 colors · server-side input FSM (esc/ctrl/history/selector/pan) · server-rendered numbered picker · thin firmware + marquee. See `git log`.

## Security note (acknowledged — no action needed)
Wi-Fi creds were once committed to `firmware/.env` (untracked + gitignored 2026-08-15; still in git history). **User decided NOT to rotate (2026-08-15) — accepted.** History scrub (`git filter-repo`/BFG + force-push) remains available if ever wanted; `firmware/cardputer/.pio/` (~344 MB) is also in history. No open action.

## Parked (post-E2E — not now)
PTY backend (drop-in via the adapter → "expose ANY window", no tmux; user chose to keep tmux for now); cursor-anchored prompt detection; command snippets; session peek in picker.

---
## Reference
- **Run:** `cardputerme [name]` (alias in ~/.zshrc) — background Go server per exposure, free port 8001–8255, pidfile+log `~/.cardputerme/<name>.{pid,log}`; terminal named after the cwd (or `<name>`), created IN it; re-run = "already exposed"; self-exits when its terminal dies. Beacon: UDP 8000 every 2s (subnet-directed + limited broadcast).
- **Tests:** `cd server-go && go test ./...` (71, 4 packages) · `go vet ./...` · `go run golang.org/x/tools/cmd/deadcode@latest -test ./...`. **Flash:** `cd firmware/cardputer && $PIO run -e cardputer-adv -t upload` (device `/dev/cu.usbmodem21201`; E2E is the user's; only Wi-Fi creds in `firmware/.env`).
- **Keys:** chars type · Enter send/confirm · Shift+Enter newline · esc clear / real Escape · shift+esc interrupt · Tab accept-autosuggest-else-passthrough · **fn+arrows read (pan) · digits choose** · opt+arrows real arrows (drive TUI selector) · ctrl+letter control-key · **ctrl+up/down history recall** · **ctrl+= / ctrl+_ zoom in/out · ctrl+space reset zoom** · (device) digits pick a server, fn+esc back to the list.
- **Method:** WIP=1; one 🟡 = one started tasks.roblab.app task. Quiet work — user watches via the device.

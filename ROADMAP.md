# cardputerme — Roadmap
> Updated 2026-08-15. **All 19 planned tasks shipped** — the whole system is E2E-verified on hardware, the Go server is audited + race-clean, and releases cut via a manual tag-driven GitHub Action publishing macOS + Ubuntu binaries. Terse; `git log` holds detail. New work starts from ## Parked.

## What is cardputerme
**Expose ANY terminal to an M5Cardputer with one command.** `cardputerme [name]` (zsh alias → `bin/cardputer-server`) builds+execs a **Go binary** that runs a background WS server for that ONE terminal on a free port (8001–8255) and broadcasts a **UDP beacon** (port 8000, 2s: `{app,v,name,port}`); the device listens, lists every live exposure (**IPv4:port + name**), and connects to the one you pick. NO sessions concept, NO mDNS, NO baked IP. Backend (tmux) invisible & swappable inside `internal/terminal/` only; the whole system = **our Go server + our firmware**. Thin device: draws the server-described display, forwards raw keys; server owns everything (input, history, viewport). House rules: no hooks, no regex, no `else`, no model tokens, no code comments, KISS, TDD, **event-driven (no polling)**; device E2E is the user's. North star: **SSH-parity** — only diff vs ssh is the small screen; mirror the terminal's own colors + layout.

## Focus — ✅ ALL SHIPPED
Every planned task (#1–#19) is delivered. The whole job is done: expose any terminal with `cardputerme`, discover it on the Cardputer over a UDP beacon, connect over WebSocket, and drive it with full SSH-parity — mirror, keys, prompts, history, autosuggest, marquee, zoom, faithful colors. Go server is unit-tested (72), 4-agent-audited, and race-clean. Install needs no Go: prebuilt macOS + Ubuntu binaries cross-compile via `make release` and ship through a manual tag-driven GitHub Action. `v0.0.1` tagged. Next north star lives in ## Parked — pull one in to open a new cycle.

## 🎯 Now
Nothing queued — the plan is empty (all 19 ✅, project shipped). New work starts by promoting an item from ## Parked to a fresh 🟡.
> **Sync rule (WIP=1):** exactly one task is 🟡 at a time (mirrors the one started tasks.roblab.app task). On finish: mark ✅, log under Done (dated), promote next to 🟡, update this block. Never two 🟡. *(OpenTogg sync stopped.)*

## Plan — 3 days (deploy at end of each day)
_Empty — all planned tasks shipped. Populate from ## Parked when the next cycle begins._

## Done 2026-08-15 (Go era)
- **#19 Release via manual tag-driven GitHub Action.** `.github/workflows/release.yml` — `workflow_dispatch` with a required `tag` input; checks out that tag (fails if unpushed → enforces tag-first), sets up Go from `server-go/go.mod`, runs `make release-publish VERSION=<tag>` with `contents:write` + `GH_TOKEN`. Process: push a tag, then Actions → Release → Run workflow. Reuses the #18 Makefile targets; README documents it. YAML validated.
- **#18 Publish CLI binaries for macOS + Ubuntu.** `make release` cross-compiles `server-go/cmd/cardputerme` (CGO off, `-s -w`; GOOS=darwin,linux × GOARCH=arm64,amd64) → `dist/cardputerme-<os>-<arch>` — verified all 4 (Mach-O + static ELF). `make release-publish VERSION=vX.Y.Z` ships `dist/*` to a GitHub Release via `gh` (run when authed + tagged — the outward-facing publish is the user's). `bin/cardputer-server` now prefers a matching prebuilt `dist/` binary, falls back to `go build`, else errors with guidance — so a no-Go machine runs `cardputerme` from a download (`bee3ea5`).
- **#14 Terminal-fidelity pass (SSH-parity).** Colors mirror the terminal: #14a bold→bright, plus bold basic colors now brighten regardless of SGR order (`6fa5b0d`, tracks the basic-color index through SGR state). bg/reverse documented out-of-scope (device is per-line fg only). 4-agent audit verified key handling, action dispatch, and pick-server end to end; fixed the one real defect — a data race on `terminal.Subscribe`'s stop flag → `atomic.Bool`, `go test -race` clean (`5679e67`). Dev ergonomics: `Makefile` (setup/test/build/flash) + gitignore polish.
- **#17 Claude Code plugin — `/cardputer` skill.** Repo ships a Claude Code plugin (`.claude-plugin/{plugin,marketplace}.json` + `skills/cardputer/SKILL.md`); `/cardputer` detects the current tmux session and runs `cardputerme <session>` to expose it. Symlinked into both accounts' `skills/`. Mechanism verified on a fresh session (own port 8002 + beacon). Agnostic — only calls the CLI.
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

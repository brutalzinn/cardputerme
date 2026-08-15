# cardputerme — Roadmap
> Updated 2026-08-15. Focus: **land the server-per-exposure redesign (#10)** — expose = own WS server, device auto-finds via UDP beacons — then retest on-device. Terse; `git log` holds detail.

## What is cardputerme
**Expose ANY terminal to an M5Cardputer with one command.** `cardputerme [name]` (zsh alias → `bin/cardputer-server`) spawns a **background WS server for that ONE terminal** on a free port (8001–8255) and broadcasts a **UDP beacon** (port 8000, every 2s: `{app,v,name,port}`); the device listens, shows every live exposure (**IPv4:port + name**), and connects to the picked one. NO sessions concept, NO mDNS/DNS, NO baked IP. Backend invisible & swappable inside `lib/terminal.js` only; the whole system = **our server + our firmware**. Thin device: draws the server-described display, forwards raw keys; server owns everything (input buffer, history, viewport). House rules: no hooks, no regex, no `else`, no model tokens, no code comments, KISS, TDD; device E2E is the user's. North star: **SSH-parity** — only diff vs ssh is the small screen; mirror the terminal's own colors + layout.

## 🎯 Now
Current: **#10 server-per-exposure redesign** — server side LANDED; next: firmware beacon listener + local server picker + dynamic WS connect (main.cpp, then flash + on-device check).
> **Sync rule (WIP=1):** exactly one task is 🟡 at a time (mirrors the one started tasks.roblab.app task). On finish: mark ✅, log under Done (dated), promote next to 🟡, update this block. *(OpenTogg sync stopped.)*

## Plan — 3 days (deploy at end of each day)

**📅 Day 1 — 2026-08-14/15 · Prove it works**
8. ⏸️ **E2E verification (user-run).** *(paused 2026-08-15 for the #10 redesign — the checklist changed: no session picker, esc = real Escape; retest AFTER the new firmware is flashed)* Checklist: type/send · esc (clear / real Escape) · marquee · Ctrl+C · history (ctrl+fn-↑/↓) · shift+esc interrupt · choice panel (digits, `>` pointer) · read-while-choosing (opt+arrows) · pan · server picker (beacons). Fix each failure immediately (server-side preferred; flash only if unavoidable).
   - ✅ R1 (2026-08-15): question text unreadable while choosing (view orbited the wrapping pointer) → viewport now ANCHORS the pointer near the view bottom so the question above fills the screen (`anchorRow`, lib/viewport.js; 88/88). Server restart picks it up.
   - ✅ R2 (2026-08-15): battery % in the top header (green >20%, orange ≤20%, refresh 15s) — flashed.
   - ✅ R3 (2026-08-15, user-decided): **arrows read, numbers choose** — bare arrows ALWAYS pan (read anything, even during questions); digits answer menu options; opt+arrows now send real arrow keys (drive the terminal's own selector); Enter confirms. lib/input.js FSM swap; 86/86.
   - ✅ R4 (2026-08-15): **session lifecycle DX** — idle unattached shells auto-clear after `IDLE_MINUTES` (default 30; `lib/idle.js` `sessionsToClear` + `killTmuxSession`, 60s sweep in server.js, never the active/attached session); `registry.prune` drops dead sessions from the picker; `cardputerme` with no arg names the session after the cwd + reuses an already-running server (`bin/cardputer-server`). 99/99. **Uncommitted** — protect in the checkpoint commit.
   - ✅ R4 checkpoint-committed (`54e95d5`); idle sweep later removed with the sessions concept (#10).
> 🚀 **Deploy (end of Day 1)** — all 10 pass on the physical device.

**📅 Day 2 — 2026-08-15 · Server-per-exposure redesign**
10. 🟡 **CLI + discovery redesign — server-per-exposure + UDP beacons.** *(current)* Contract: `cardputerme [name]` = a **background WS server for that ONE terminal** on a free port 8001–8255; discovery = **UDP broadcast beacon** (port 8000, 2s, `{app:'cardputerme',v:1,name,port}` — receiver takes the IPv4 from the packet source, Syncthing-style); the device server list IS the picker. NO sessions, NO mDNS (rejected: complexity + ESP32 `queryService` one-instance bug), NO baked IP.
   - ✅ S1 (2026-08-15): server side landed (`c20c41d`) — launcher backgrounds `node server.js` (pidfile+log in `~/.cardputerme/`, dedup per name, terminal created IN the cwd), port scan `lib/discovery.js`, beacon `lib/beacon.js`, self-exit when the terminal dies. Verified live: 2 exposures on 8001/8002, beacon received with correct IPv4.
   - ✅ S2 (2026-08-15): sessions concept KILLED (`0ef7dc3`, −471 LOC) — registry/picker/idle-sweep/sessions-wire removed; esc with empty input now reaches the terminal as a real Escape (ssh-parity).
   - ✅ S3 (2026-08-15): tmux/claude mention sweep + full comment strip (no-regex char-scan) across server/; adapter export renamed `createBackend`; README rewritten truthful; `firmware/cardputer-claude` → `firmware/cardputer`; `.env` cleaned (PORT/TMUX_SESSION/READ_MODE gone). 80/80.
   - ⬜ S4: **firmware** — UDP beacon listener, local server picker (`N. name ip:port`, digits pick, auto-connect when only one, fn+esc back to picker), dynamic WS connect; drop baked `WS_HOST/PORT`. Flash + on-device check (user).
> 🚀 **Deploy (end of Day 2)** — `cardputerme` in two dirs; both appear on the device; pick + drive each from the couch.

**📅 Day 3 — 2026-08-16 · Cleanup + polish**
9. ⬜ **Cleanup queue (remaining /simplify findings).** (a) Status bar = the pane's bottom chrome block (bypass + status rows) joined for the marquee, dropped from the body. (b) Dead code purge (card pipeline remnants, `detectChoice`, unreachable color branches, unused `cwd`/`run`). (c) Reuse homes (`keepTail`/`isDigit`/tab-in-`toAscii`/native `trimEnd`). (d) Efficiency: pane-identity early-out, drop `exists()` from tick, parse rows once + cap first, ASCII fast-paths, one echo frame for legacy `cmd`. (e) Altitude: extract tested `lib/screen.js` (pure `composeMirror`), one state object. *(Comment strip + `createBackend` + picker/dead-session purge already done in #10 S2/S3.)* All TDD; suite green each step.
> 🚀 **Deploy (end of Day 3)** — restart exposures; payload ≪15KB; idle CPU visibly down; behavior unchanged on device.

**📅 Day 4 — 2026-08-17 · Comfort**
11. ⬜ **Zoom (server-owned font).** Display message gains `font`; device `setTextSize`; a ctrl-chord zooms; viewport dims derive from zoom.
12. ⬜ **Processing status.** Prefer the tail row containing "esc to interrupt" (plain `.includes`) as status so Claude's live spinner/token line rides the marquee.
13. ⬜ **History autosuggest + Tab accept.** While typing, ghost-render the newest history command matching the prefix (dim color); **Tab accepts** it (no suggestion → Tab passes to the terminal, server-decided). Avoids retyping repeated commands. Server-only, TDD.
14. ⬜ **Terminal-fidelity pass (SSH-parity).** North star: Cardputer = ssh session, only difference is screen size. Audit against the REAL terminal side-by-side: (a) colors — device must show exactly the colors the terminal displays (extend `lib/ansi.js` coverage: bold/bright, 24-bit `38;2`, bg colors, reverse video); (b) layout — mirror the terminal's own structure (row order, spacing, input at bottom) rather than reshaping it. Verify with at least two different CLIs (e.g. an agent CLI + plain bash `ls --color`). Server-only, TDD.
> 🚀 **Deploy (end of Day 4)** — zoom live; working-state always visible; colors faithful.

## Done 2026-08-14 (server-driven everything)
- **#7 Thin firmware.** Renders the generic display (per-line server colors + status bar + **marquee ticker** for long status); forwards every raw key (uniform `<mod>+<arrow>` rule, one `arrowFor` table); zero content logic. Flashed + connected.
- **#6 Server-rendered picker.** Numbered text menu (Telegram-style), digit selects, esc cancels — composed server-side.
- **#5 Input engine (server-side FSM).** `lib/input.js` pure FSM + `applyKey`: server-owned compose buffer (echo line), esc=clear→picker, shift+esc=interrupt, Enter/Shift+Enter, Tab, digits answer prompts, **fn+↑/↓ drive TUI selectors** (viewport tracks the `>` pointer, recenters only on move), **opt+arrows always pan**, **Ctrl combos** (Ctrl+C…), **history recall** (ctrl+↑/↓, editable). Generic key names end-to-end; tmux spellings ONLY in the adapter. Typing/pan fast-path from cached grid.
- **#4 Display protocol + terminal colors.** `{type:'display', body:[{text,color}], status}` screen-sized (~4KB); terminal's OWN ANSI colors mirrored to RGB565 (`lib/ansi.js`, no regex); bottom status bar = pane's last row. User-confirmed colors + status on device.
- **Session exposure by name.** `cardputerme <name>` creates+exposes a session (`ensureSession`, adapter-side); zsh alias installed; launcher script backend-neutral. Auto-discovery + `/select`; 86/86 tests.
- **#1–#3** (earlier): agnostic prompt detection · terminal adapter · named sessions/one server. See git log (`05be6b6`, `7f6ffc4`).

## Parked (post-focus — not now)
PTY backend (drop-in via adapter; enables "expose ANY window" fully — run `cardputerme` inside a window to capture it); cursor-anchored prompt detection (tmux `#{cursor_y}` + styled-row); command snippets; session peek in picker; README truth-pass.

---
## Reference
- **Run:** `cardputerme [name]` (alias in ~/.zshrc) — background server per exposure, free port 8001–8255, pidfile+log in `~/.cardputerme/<name>.{pid,log}`; terminal named after the cwd (or `<name>`), created IN the cwd; re-run = "already exposed"; server self-exits ~60s after its terminal dies. Beacon: UDP 8000 broadcast every 2s.
- **Tests:** `cd server && npm test` (80 pass). **Flash:** `cd firmware/cardputer && $PIO run -e cardputer-adv -t upload` (device `/dev/cu.usbmodem21201`; E2E is the user's; only Wi-Fi creds in firmware/.env).
- **Keys:** chars type · Enter send/confirm · Shift+Enter newline · esc clear / real Escape · shift+esc interrupt · Tab · **arrows read (pan) · numbers choose** · opt+arrows real arrow keys (drive TUI selector) · ctrl+letter control-key · ctrl+fn-↑/↓ history · (device) digits pick a server, fn+esc back to the server list.
- **Method:** WIP=1; one 🟡 = one started tasks.roblab.app task. Quiet work (no agent fan-out spam — user watches via the device).

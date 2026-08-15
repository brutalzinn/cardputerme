# cardputerme — Roadmap
> Updated 2026-08-15. Focus: **land the server-per-exposure redesign (#10)** — expose = own WS server, device auto-finds via UDP beacons — then retest on-device. Terse; `git log` holds detail.

## What is cardputerme
**Expose ANY terminal to an M5Cardputer with one command.** `cardputerme [name]` (zsh alias → `bin/cardputer-server`) spawns a **background WS server for that ONE terminal** on a free port (8001–8255) and broadcasts a **UDP beacon** (port 8000, every 2s: `{app,v,name,port}`); the device listens, shows every live exposure (**IPv4:port + name**), and connects to the picked one. NO sessions concept, NO mDNS/DNS, NO baked IP. Backend invisible & swappable inside `lib/terminal.js` only; the whole system = **our server + our firmware**. Thin device: draws the server-described display, forwards raw keys; server owns everything (input buffer, history, viewport). House rules: no hooks, no regex, no `else`, no model tokens, no code comments, KISS, TDD; device E2E is the user's. North star: **SSH-parity** — only diff vs ssh is the small screen; mirror the terminal's own colors + layout.

## 🎯 Now
Current: **#9 cleanup queue** — remaining /simplify findings (bottom-chrome status join, dead-code purge, reuse homes, efficiency, lib/screen.js extraction). #10 closed code-side (user waived the test gate 2026-08-15); flash = `cd firmware/cardputer && $PIO run -e cardputer-adv -t upload` whenever the device is plugged.
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

**📅 Day 2 — 2026-08-15/16 · Cleanup + polish**
9. 🟡 **Cleanup queue (remaining /simplify findings).** *(current)* (a) Status bar = the pane's bottom chrome block (bypass + status rows) joined for the marquee, dropped from the body. (b) ✅ Dead code purge — `detectChoice` + unused `cwd()` removed (75/75). Remaining: card-pipeline remnants (`sliceIntoCards` used only for fallback text), unreachable color branches. (c) ✅ Reuse homes — tab expansion homed in `toAscii` (split/join, no regex; the two call-site expansions were dead no-ops since `toAscii` stripped tabs), custom `trimEnd` replaced by native everywhere (fixed a latent ReferenceError in `buildState`). *(isDigit/isDigits left as-is — genuinely different char-vs-string helpers in different modules; unifying would couple the pure FSM to detect for no gain.)* (d) Efficiency: drop `exists()` from tick, parse rows once + cap first, ASCII fast-paths, one echo frame for legacy `cmd`. (e) Altitude: extract tested `lib/screen.js` (pure `composeMirror`), one state object. All TDD; suite green each step.
> 🚀 **Deploy (end of Day 2)** — restart exposures; payload ≪15KB; idle CPU visibly down; behavior unchanged over the wire (dry-run script).
**📅 Day 3 — 2026-08-16/17 · Comfort**
11. ⬜ **Zoom (server-owned font).** Display message gains `font`; device `setTextSize`; a ctrl-chord zooms; viewport dims derive from zoom.
12. ⬜ **Processing status.** Prefer the tail row containing "esc to interrupt" (plain `.includes`) as status so Claude's live spinner/token line rides the marquee.
13. ⬜ **History autosuggest + Tab accept.** While typing, ghost-render the newest history command matching the prefix (dim color); **Tab accepts** it (no suggestion → Tab passes to the terminal, server-decided). Avoids retyping repeated commands. Server-only, TDD.
14. ⬜ **Terminal-fidelity pass (SSH-parity).** North star: Cardputer = ssh session, only difference is screen size. Audit against the REAL terminal side-by-side: (a) colors — device must show exactly the colors the terminal displays (extend `lib/ansi.js` coverage: bold/bright, 24-bit `38;2`, bg colors, reverse video); (b) layout — mirror the terminal's own structure (row order, spacing, input at bottom) rather than reshaping it. Verify with at least two different CLIs (e.g. an agent CLI + plain bash `ls --color`). Server-only, TDD.
> 🚀 **Deploy (end of Day 3)** — zoom live; working-state always visible; colors faithful.

## Done 2026-08-15 (server-per-exposure redesign — #10)
- **#10 CLI + discovery redesign.** `cardputerme [name]` = background WS server per terminal (free port 8001–8255, pidfile+log `~/.cardputerme/`, cwd terminal, self-exit); UDP beacon discovery (port 8000, 2s, Syncthing-pattern — device learns IPv4:port from the packet); sessions concept removed (−471 LOC); esc = real Escape (ssh-parity); comment strip + tmux/claude purge (`createBackend`, README rewrite, `firmware/cardputer` rename); firmware beacon listener + local server picker + dynamic WS connect (compiles, RAM 15%/flash 32%). Wire contract verified end-to-end by fake-device dry run (beacon→connect→type→mirror). 80/80. Commits `c20c41d`…`943b1ae`. **Flash pending (user, waived as gate).**

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

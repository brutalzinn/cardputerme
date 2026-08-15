# cardputerme — Roadmap
> Updated 2026-08-15. Focus: **prove the integration really works** (user-run untethered E2E, live fix loop), then land the reviewed cleanup. Terse; `git log` holds detail.

## What is cardputerme
**Expose ANY terminal window/session to an M5Cardputer.** One command (`cardputerme [name]` — zsh alias → `bin/cardputer-server`) exposes this machine; the device auto-lists sessions and drives whichever you pick over ONE WebSocket. Fully agnostic: the user NEVER knows how a session is hosted (tmux is an invisible, swappable backend inside `lib/terminal.js` only); the whole system is exactly **our server + our firmware**. Thin device: draws the server-described display (text+color per line + status bar w/ marquee), forwards raw keys; the **server owns everything** (input buffer, esc flows, pickers, history, viewport). House rules: no hooks, no regex, no `else`, no model tokens, no code comments (the code is the documentation), KISS, TDD; device E2E is the user's. North star: **SSH-parity** — the only difference vs ssh is the small screen; mirror the terminal's own colors + layout.

## 🎯 Now
Current: **#8 E2E verification** — run the 10-item device checklist; fix failures one at a time.
> **Sync rule (WIP=1):** exactly one task is 🟡 at a time (mirrors the one started tasks.roblab.app task). On finish: mark ✅, log under Done (dated), promote next to 🟡, update this block. *(OpenTogg sync stopped.)*

## Plan — 3 days (deploy at end of each day)

**📅 Day 1 — 2026-08-14/15 · Prove it works**
8. 🟡 **E2E verification (user-run).** *(current)* 10-item checklist: type/send · esc-clear · session picker · marquee · Ctrl+C · history (ctrl+fn-↑/↓) · shift+esc interrupt · choice panel (fn+↑/↓ + Enter, `>` pointer) · read-while-choosing (opt+arrows) · pan. Fix each failure immediately (server-side preferred; flash only if unavoidable).
   - ✅ R1 (2026-08-15): question text unreadable while choosing (view orbited the wrapping pointer) → viewport now ANCHORS the pointer near the view bottom so the question above fills the screen (`anchorRow`, lib/viewport.js; 88/88). Server restart picks it up.
   - ✅ R2 (2026-08-15): battery % in the top header (green >20%, orange ≤20%, refresh 15s) — flashed.
   - ✅ R3 (2026-08-15, user-decided): **arrows read, numbers choose** — bare arrows ALWAYS pan (read anything, even during questions); digits answer menu options; opt+arrows now send real arrow keys (drive the terminal's own selector); Enter confirms. lib/input.js FSM swap; 86/86.
   - ✅ R4 (2026-08-15): **session lifecycle DX** — idle unattached shells auto-clear after `IDLE_MINUTES` (default 30; `lib/idle.js` `sessionsToClear` + `killTmuxSession`, 60s sweep in server.js, never the active/attached session); `registry.prune` drops dead sessions from the picker; `cardputerme` with no arg names the session after the cwd + reuses an already-running server (`bin/cardputer-server`). 99/99. **Uncommitted** — protect in the checkpoint commit.
   - ⏭ Next: user checkpoint-commits (now incl. R4 idle-sweep + launcher work), then the comment-strip sweep (see #9f), then retest questions on-device.
> 🚀 **Deploy (end of Day 1)** — all 10 pass on the physical device.

**📅 Day 2 — 2026-08-15 · Reviewed cleanup (4-agent /simplify findings, approved)**
9. ⬜ **Bypass/status bottom-chrome fix + cleanup queue.** (a) Status bar = the pane's bottom chrome block (bypass + status rows) joined for the marquee, dropped from the body. (b) Dead code purge (card pipeline, `detectChoice`, unread prompt-text assembly → `hasChoicePrompt`, unreachable color branches, unused `cwd`/`remove`/`run`, inert picker actions). (c) Reuse homes (`keepTail`/`isDigit`/tab-in-`toAscii`/native `trimEnd`). (d) Efficiency: pane-identity early-out, drop `exists()` from tick, parse rows once + cap first, ASCII fast-paths, one echo frame for legacy `cmd`, TTL on `refreshSessions`. (e) Altitude: extract tested `lib/screen.js` (pure `composeMirror`), unify `selectSession`/`submitText` across FSM/HTTP/WS, one state object, backend-neutral adapter exports (`createBackend`). (f) **Comment strip (user rule: the code is the documentation)** — remove ALL code comments across server/ + firmware via the no-regex char-scan stripper (scratchpad script ready); AFTER a user checkpoint commit; verify: full suite green + firmware compiles + `git diff` eyeball. All TDD; suite green each step.
> 🚀 **Deploy (end of Day 2)** — restart server; payload ≪15KB; idle CPU visibly down; behavior unchanged on device.

**📅 Day 3 — 2026-08-16 · Any instance, comfortably**
10. ⬜ **CLI + discovery redesign (user-spec 2026-08-15).** Contract: `cardputerme [name]` is the whole story — **exposing a terminal = creating the WS server** (no separate start step, no "add session" command); device **auto-finds servers** via mDNS `_cardputerme._tcp` (bonjour-service), picker on-device, **no IP baked in firmware**. Settle first: (a) one server PER EXPOSURE, own port, dies with its terminal — recommended, flips the one-server invariant — vs (b) one daemon/machine + client invocations (`POST /select`, server.js:273). Fix the launcher gaps: foreground-hostage server, detached-sibling spawn, `tmux attach` hint leak. Full spec in tracker task #10.
11. ⬜ **Zoom (server-owned font).** Display message gains `font`; device `setTextSize`; a ctrl-chord zooms; viewport dims derive from zoom.
12. ⬜ **Processing status.** Prefer the tail row containing "esc to interrupt" (plain `.includes`) as status so Claude's live spinner/token line rides the marquee.
13. ⬜ **History autosuggest + Tab accept.** While typing, ghost-render the newest history command matching the prefix (dim color); **Tab accepts** it (no suggestion → Tab passes to the terminal, server-decided). Avoids retyping repeated commands. Server-only, TDD.
14. ⬜ **Terminal-fidelity pass (SSH-parity).** North star: Cardputer = ssh session, only difference is screen size. Audit against the REAL terminal side-by-side: (a) colors — device must show exactly the colors the terminal displays (extend `lib/ansi.js` coverage: bold/bright, 24-bit `38;2`, bg colors, reverse video); (b) layout — mirror the terminal's own structure (row order, spacing, input at bottom) rather than reshaping it. Verify with Claude Code AND at least one other CLI (e.g. plain bash + `ls --color`). Server-only, TDD.
> 🚀 **Deploy (end of Day 3)** — two servers picked from the couch; zoom live; working-state always visible.

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
- **Run:** `cardputerme [name]` (alias in ~/.zshrc) or `./bin/cardputer-server`; no arg → session named after the cwd, started IN it; if the server already runs on `:4711` the session is just created (auto-discovered). ONE port, ONE WS. Device: `ws://192.168.0.149:4711/ws` (baked in firmware/.env). Idle unattached shells auto-clear (`IDLE_MINUTES`, default 30; 0 disables).
- **Tests:** `cd server && npm test` (99 pass). **Flash:** `cd firmware/cardputer-claude && $PIO run -e cardputer-adv -t upload` (device `/dev/cu.usbmodem21201`; E2E is the user's).
- **Keys:** chars type · Enter send/confirm · Shift+Enter newline · esc clear→picker · shift+esc interrupt · Tab · **arrows read (pan) · numbers choose** · opt+arrows real arrow keys (drive TUI selector) · ctrl+letter control-key · ctrl+fn-↑/↓ history.
- **Method:** WIP=1; one 🟡 = one started tasks.roblab.app task. Quiet work (no agent fan-out spam — user watches via the device).

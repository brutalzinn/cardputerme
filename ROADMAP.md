# cardputerme — Roadmap
> Updated 2026-08-14. Focus: an **agnostic** CLI that exposes a terminal session over WebSocket to a Cardputer — independent of tmux, Claude Code, hooks, regex, and model tokens. Terse; `git log` holds detail.

## What is cardputerme
A **CLI app** you run in a terminal: it exposes **one WebSocket server** so an **M5Cardputer** can drive terminal sessions. **ONE server exposes MULTIPLE named sessions** over a **single WebSocket / single port** — the device picks a session by name (custom name or a random UUID). One port, no multi-port handling. Fully **agnostic** — works with plain `bash`, Claude Code, or any other agent/CLI; it doesn't matter whether the user is in tmux or bash. Stack: Node `express`+`ws`, ESP32-S3 firmware (M5Cardputer). House rules: **no hooks, no regex, no model tokens, independent of any external application** — deterministic local algorithms only. **Thin frontend**: the **server holds ALL rules and sends exactly what to display** (cards, menus, colors-by-role); the device just renders + forwards keys — so a color/rule tweak is a backend edit, **never a device re-flash**.

## Focus — the generic protocol (server describes, device renders)
The Cardputer is a **pure client**: it draws a generic **display message** (a list of lines/segments, each `text + font + color + fit`) and **forwards raw keypresses**. It holds **no state and no logic**. The **server owns everything** — it composes every screen (mirror, prompt, menu/picker *with the highlight*, toast) and runs the whole **interaction engine** (paging, mode, input buffer, selection, gestures like **Esc twice → server picker**). No semantic roles on the wire; the device never knows if content is an agent, a response, a question, or a menu. Why: all evolution happens server-side, so the system grows more generic and complete over time and the **firmware never needs re-flashing**. "Done" = every screen *and* interaction is server-driven; the firmware is just forward-keys + draw.

## 🎯 Now
Current: **#4 Display-message protocol** — server emits a generic `{lines:[{text,color,font,fit}]}` display message; the device renders it verbatim.
> **Sync rule (WIP=1):** exactly one task is 🟡 at a time (mirrors the one started tasks.roblab.app task). On finish: mark ✅, log under Done (dated), promote next to 🟡, update this block. Never two 🟡; never lag. *(OpenTogg timer sync stopped for this project.)*

## Plan — 3 days (deploy at end of each day)

**📅 Day 1 — 2026-08-14 · Agnostic core (server)**
1. ✅ **Agnostic prompt detection.** (See Done.)
2. ✅ **Terminal adapter (tmux first).** (See Done.)
> 🚀 **Deploy (end of Day 1)** — done: server restarted on the adapter; `npm test` 30/30; `/cards` mirrors session `generic`.

**📅 Day 2 — 2026-08-15 · Generic protocol (server describes, device renders)**
3. ✅ **Named sessions (one server, many).** (See Done.)
4. 🟡 **Display-message protocol.** *(current)* Replace the ad-hoc `cards` push with ONE generic **display message** — `{ type:'display', lines:[{ text, color, font, fit }] }` — composed entirely server-side (a pure display-model builder over the screen state). No roles on the wire; move the firmware's `"> "` prompt inference here (server sets the color). TDD the pure builder (screen state → display message).
5. ⬜ **Input engine (server-side FSM).** Device forwards every **raw keypress** (`{type:'key', key}`) and holds no state; the **server** owns the interaction FSM — paging, input buffer, session/menu selection, and **gestures (Esc twice → server picker)** — plus key-press simulation to the session (`Tab`/`Enter`/`Escape`/arrows via `terminal.sendKey`, lone digit answers a menu). TDD the FSM + key map (pure; fake runner).
> 🚀 **Deploy (end of Day 2)** — server emits display messages + drives all nav; `npm test` green.

**📅 Day 3 — 2026-08-16 · Server-rendered menus + thin firmware + prove**
6. ⬜ **Server-rendered menus/picker (numbered text, Telegram-style).** The server composes menus as **plain numbered text** display messages — **server picker** (by computer/host name) → **session picker** (by name) — and the user **selects by pressing the number** (reuses the digit-select path; no arrows/highlight). The device just shows the text + forwards the digit; the server maps number→session and switches. Same pattern as a CLI's own `1./2.` option menu.
7. ⬜ **Thin-render firmware + prove.** Shrink `main.cpp` to "forward keys + draw the display message" (drop local paging/mode/input-buffer). User flashes **once** (their E2E); thereafter every display/interaction change is server-only.
> 🚀 **Deploy (end of Day 3)** — a prompt AND a menu selection round-trip from any CLI; the firmware never needs another flash for UI changes.

## Done 2026-08-14 (agnostic pivot)
- **#3 Named sessions (one server, many) + full-generic cleanup.** `server/lib/sessions.js` registry (name→adapter, UUID fallback) + `listTmuxSessions` auto-discovery; `server.js` routes reads/writes through the **selected** session (`activeTerminal`), with `listSessions`/`selectSession` over the WS and `/sessions`+`/select` HTTP. **Removed modes entirely** — deleted the Claude transcript machinery (`lib/transcript.js`, `readClaudeLatest`, discovery, `READ_MODE`, `fs.watch`): the bridge just mirrors the screen + detects prompts, for any CLI. User never names a session — all auto-discovered, picked on the device. TDD: `test/sessions.test.js` (7) + enumerator tests. **34/34 pass.** Verified: auto-discovered `claude/generic/rchat/tesseract`, `/select` switches live.
- **#2 Terminal adapter (tmux first).** Extracted terminal I/O into `server/lib/terminal.js` (`exists`/`capture`/`cwd`/`sendText`/`sendKey`), tmux backend built from a session name with an injectable runner. `server.js` core makes **zero** direct tmux calls — all via `terminal.*`. TDD: `test/terminal.test.js` (7 contract tests with a fake runner). **30/30 pass.** Live server restarted on the adapter (session `generic`); control unaffected. PTY backend can now drop in unchanged.
- **#1 Agnostic prompt detection — no hooks, no regex, no Claude coupling.** Added `server/lib/detect.js` (pure char-scan: ends-with-`?` + `N.`/`N)` option lists) and reworked `detectPrompt` to use it (always shows the question line). Removed ALL hook infra (`.claude/settings.json`, `bin/cardputerme-hook`, `/hook` route, `lib/hook.js`) and the `isQuestion` regex; default `READ_MODE=raw` mirrors any terminal, `claude` transcript-clean is opt-in; `detectPrompt` runs for every mode so any CLI's menu is caught. Core is regex-free.

## Parked (post-focus — not now)
PTY backend (drop-in via the Day-1 adapter, removes tmux entirely); command-history reuse UX; full README truth-pass; self-drive USB harness.

---
## Reference
- **Server:** `node server.js` (or `./bin/cardputer-server`); ONE port `:4711` (`PORT`), ONE WebSocket for all sessions. Sessions **auto-discovered** — no session config needed; optional `SESSION=<name>` pre-selects one. Debug routes: `/health`, `/sessions`, `/select`, `/cards`, `/command`.
- **Tests:** `cd server && node --test test/*.test.js` (34 pass). `npm test` wired.
- **Firmware:** `PIO=~/.platformio/penv/bin/pio`; `$PIO run -t upload` (in `firmware/cardputer-claude/`). Device E2E is the user's.
- **Detection rule:** choose-prompt = ends-with-`?` and/or ≥2 `N.`/`N)` options; always show the question. No hooks, no regex, no tokens.
- **Method:** WIP=1; one 🟡 = one tasks.roblab.app started task. OpenTogg timer sync stopped for this project.

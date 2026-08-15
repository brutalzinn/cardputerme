---
name: cardputer
description: Expose the CURRENT Claude Code session to an M5Cardputer over Wi-Fi so the user can watch and drive Claude from the pocket device. Runs the cardputerme CLI on this terminal's tmux session — it becomes discoverable (UDP beacon) and drivable (WebSocket) on the Cardputer. Use when the user asks to "expose to cardputer", "send this session to the cardputer", or drive Claude from the device. Global; works from any Claude Code account.
---

# cardputer — expose this session to the device

Exposes the terminal Claude Code is running in to an **M5Cardputer**. The device discovers it over a UDP beacon and drives it over WebSocket, so the user watches and controls **this exact session** from the Cardputer (answer prompts with digits, Shift+esc to interrupt, fn+arrows to scroll).

The whole system stays agnostic: this skill only calls the `cardputerme` CLI on the current session. tmux is an invisible backend — never surface tmux details beyond the "start inside tmux" hint below.

## Steps

1. **Find the current session.** Run:
   ```
   tmux display-message -p '#{session_name}'
   ```
   If it errors (Claude Code is not inside tmux), tell the user: *"cardputerme needs a tmux session — launch Claude Code inside `tmux` first, then rerun /cardputer."* Then stop.

2. **Expose it.** Run the cardputerme launcher for that session name — use the first that resolves:
   - `cardputerme "<session>"` — the shell alias (installed by the cardputerme repo).
   - `"$CARDPUTERME_HOME/bin/cardputer-server" "<session>"` — if `$CARDPUTERME_HOME` is set.
   - Otherwise locate the cardputerme repo and run `<repo>/bin/cardputer-server "<session>"`.

   The launcher backgrounds a WebSocket server on a free port (8001–8255) and starts broadcasting a beacon. Re-running for an already-exposed session just prints "already exposed".

3. **Tell the user what to do on the device.** Report the exposure name and: *"On the Cardputer, pick `<session>` from the server list (it auto-connects if it's the only one). You'll see and drive this Claude Code session live."*

Do nothing else — this skill only exposes the session you're already in; it never switches sessions or runs other commands.

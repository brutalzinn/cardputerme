---
name: cardputer
description: Expose the CURRENT Claude Code session to an M5Cardputer over Wi-Fi so the user can watch and drive Claude from the pocket device. Runs the cardputerme CLI on this terminal — it becomes discoverable (UDP beacon) and drivable (WebSocket) on the Cardputer, named after the current project. Use when the user asks to "expose to cardputer", "send this session to the cardputer", or drive Claude from the device. Global; works from any Claude Code account.
---

# cardputer — expose this session to the device

Exposes the terminal Claude Code is running in to an **M5Cardputer**. The device discovers it over a UDP beacon and drives it over WebSocket, so the user watches and controls **this exact session** from the Cardputer (answer prompts with digits, Shift+esc to interrupt, fn+arrows to scroll).

Every exposure is named after **the project directory Claude Code is running in**, so several terminals on the same computer show up as distinct entries on the device and the user picks the one they want. The whole system stays agnostic: this skill only invokes the `cardputerme` CLI, which resolves the name and the capture backend itself. tmux is an invisible backend — never surface tmux details beyond the "start inside tmux" hint below.

## Steps

1. **Check this terminal is inside tmux.** Run:
   ```
   tmux display-message -p '#{session_name}'
   ```
   If it errors (Claude Code is not inside tmux), tell the user: *"cardputerme needs a tmux session — launch Claude Code inside `tmux` first, then rerun /cardputer."* Then stop. Do not use the printed name for anything else — the CLI reads it on its own.

2. **Expose it.** Run the launcher **from the project directory, with no arguments** — use the first that resolves:
   - `cardputerme` — installed on PATH, or the shell alias.
   - `"$CARDPUTERME_HOME/bin/cardputer-server"` — if `$CARDPUTERME_HOME` is set.
   - Otherwise locate the cardputerme repo and run `<repo>/bin/cardputer-server`.

   It names the exposure after the current directory, attaches to this terminal's tmux session, backgrounds a WebSocket server on a free port (8001–8255) and starts broadcasting a beacon. Re-running in the same project just prints "already exposed". Pass a name explicitly (`cardputerme <name>`) only when the user asks for a specific label.

3. **Tell the user what to do on the device.** Report the exposure name the launcher printed and: *"On the Cardputer, pick `<name>` from the server list (it auto-connects if it's the only one). You'll see and drive this Claude Code session live."*

Do nothing else — this skill only exposes the session you're already in; it never switches sessions or runs other commands.

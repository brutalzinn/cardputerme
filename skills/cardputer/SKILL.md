---
name: cardputer
description: Link the CURRENT Claude Code session to an M5Cardputer over Wi-Fi so the user can watch and drive Claude from the pocket device. Registers this terminal as a session on the machine's cardputerme server (small HTTP API), which the device discovers over a UDP beacon and drives over WebSocket. Use when the user asks to "expose to cardputer", "link this session", "send this session to the cardputer", or drive Claude from the device. Global; works from any Claude Code account.
---

# cardputer — link this session to the device

Links the terminal Claude Code is running in to an **M5Cardputer**. The device drives **this exact session** over WebSocket (answer prompts with digits, Shift+esc to interrupt, fn+arrows to scroll, `fn+esc` to switch session).

**ONE server per machine owns MANY sessions.** Linking a terminal means registering it as a session on that server — not starting a server of its own. Each session is named after **the project directory**, so the device lists them all and the user picks one. Switching between sessions on the same machine happens on the device with no reconnect.

The system stays agnostic: this skill only talks to the CLI and its small HTTP API. tmux is an invisible backend — never surface tmux details beyond the "start inside tmux" hint below.

## The API (the server's whole surface)

The server publishes its port in `~/.cardputerme/server.port` (a bare number, so no JSON tooling is needed to read it). Base URL is `http://127.0.0.1:<port>`.

| Call | Purpose |
|---|---|
| `GET /health` | Is a server running here? Returns `{ok, machine, current, sessions[], awaiting, notify}`. |
| `GET /sessions` | `{current, sessions[]}` — what this machine exposes. |
| `POST /sessions` | Link a terminal: `{"name":…,"session":…,"cwd":…}`. Idempotent by name. |
| `DELETE /sessions?name=…` | Unlink one session. |

Prefer the CLI (step 3) — it performs exactly this registration. Use the API directly only to **inspect** state (step 4) or when the user asks for something the CLI does not cover, such as unlinking.

## Steps

1. **Check tmux is installed.** Run `command -v tmux`. If missing, stop and tell the user:
   *"cardputerme captures your terminal through tmux and cannot run without it — install it with `brew install tmux` (macOS) or `sudo apt install tmux` (Ubuntu), then rerun /cardputer."* Nothing works without it.

2. **Check this session is inside tmux.** Run:
   ```
   tmux display-message -p '#{session_name}'
   ```
   Do not use the printed name for anything else — the CLI resolves it itself.

   **If it errors, Claude Code is not inside tmux and must be restarted inside it.** A running process cannot be moved into tmux, so tell the user to relaunch — the conversation is resumed, nothing is lost:
   ```
   tmux new -s $(basename "$PWD") 'claude --continue'
   ```
   Say: *"This session isn't inside tmux, so there's no terminal for the Cardputer to mirror. Quit Claude Code and start it again with the command above — `--continue` picks this conversation back up inside tmux, then rerun /cardputer."* Then stop. Never kill the current instance yourself: it would end the user's session mid-turn, and a second instance started while this one is alive can clash over the same conversation history.

3. **Link it.** Run the launcher **from the project directory, with no arguments** — first that resolves:
   - `cardputerme` — on PATH, or the shell alias.
   - `"$CARDPUTERME_HOME/bin/cardputer-server"` — if `$CARDPUTERME_HOME` is set.
   - Otherwise locate the cardputerme repo and run `<repo>/bin/cardputer-server`.

   It names the session after the current directory and **attaches to the machine's running server**, starting one only if none is listening. It prints either *"attached to the server on port N"* or *"started the server and exposed …"*. Both are success. Pass a name explicitly (`cardputerme <name>`) only when the user asks for a specific label.

4. **Confirm and report.** Read back what the machine now exposes:
   ```
   curl -sS "http://127.0.0.1:$(tr -dc '0-9' <~/.cardputerme/server.port)/sessions"
   ```
   Then tell the user the session name and: *"On the Cardputer, press `fn+esc` and pick `<name>` from the list. Sessions on this machine switch instantly; other machines appear below them prefixed `@`."*

   If the list does **not** contain the expected name, say so plainly and show the server log (`~/.cardputerme/server.log`) rather than claiming success.

Do nothing else — this skill only links the session you are already in. It never switches sessions, kills servers, or runs other commands.

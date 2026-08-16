---
name: cardputer
description: Link the CURRENT Claude Code session to an M5Cardputer over Wi-Fi so the user can watch and drive Claude from the pocket device, and send the user a notification on that device. Registers this terminal as a session on the machine's cardputerme server (small HTTP API), which the device discovers over a UDP beacon and drives over WebSocket. Use when the user asks to "expose to cardputer", "link this session", "send this session to the cardputer", drive Claude from the device, or to notify/alert/ping the user on the Cardputer when a long task finishes or is stuck. Global; works from any Claude Code account.
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
| `POST /notify` | **Get the user's attention on the device**: `{"session":…,"text":…}`. See below. |

Prefer the CLI (step 3) — it performs exactly this registration. Use the API directly only to **inspect** state (step 4), to **notify** (below), or when the user asks for something the CLI does not cover, such as unlinking.

## Notifying the user on the device

The point of the Cardputer is that the user does **not** have to watch the screen. `POST /notify` is how you reach them: it plays a sound, lights the LED, wakes the screen, and puts your text in the device header until they press a key.

```
port=$(tr -dc '0-9' <~/.cardputerme/server.port)
curl -sS -X POST "http://127.0.0.1:$port/notify" \
  -H 'Content-Type: application/json' \
  -d '{"session":"'"$(basename "$PWD")"'","text":"tests still running after 5m"}'
```

The reply is `{"delivered":…,"queued":true,"waiting":N,"clients":N,"reason":"…"}`.

**`delivered:false` is not an error, and the alert is never lost** — `queued` is always true, and it will reach the device as soon as one is looking. Read `reason` to know why it did not go out now:

- `silenced by ;notify 0` — the user asked for silence. Respect it; do not retry or route around it.
- `no device connected` — nothing is listening yet. The alert is waiting in the inbox.

`waiting` is how many alerts are still unanswered. If it is climbing, you are paging too often — stop.

**Levels.** Add `"level"` to say how much attention you are asking for. Absent or unknown means `attention`, so you never have to set it.

| level | use it for | on the device |
|---|---|---|
| `info` | done, nothing needed from them | blue LED, one quiet note |
| `attention` (default) | finished and waiting on them | orange pulsing LED, the notification sound |
| `urgent` | broken and stopping the work | red LED, a rising three-note burst |

Use `urgent` sparingly. It is the one that will make someone put down a coffee; spend it on "the deploy is failing", not "the tests finished".

**When to notify — the bar is "the user would want to be interrupted":**

- A long task finished and you need their input or approval.
- You are **stuck**: blocked on something you cannot resolve, or a step has run far longer than expected.
- A build, test run or deploy failed in a way that stops the work.

**When NOT to:** routine progress, each step of a task, anything they will see the moment they look at the terminal, or more than once for the same event. A device that cries wolf gets ignored, and every alert costs the user's attention.

Keep `text` short — the header shows about 26 characters before clipping. Say what happened, not what you did (`"tests failed: 3 red"` beats `"I have finished running the test suite"`). Set `session` to the project name so the user knows **which** terminal wants them; both fields are optional.

Notifying does not require the session to be linked first, but it is far more useful when it is — otherwise the user gets an alert with nothing to switch to.

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

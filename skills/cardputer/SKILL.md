---
name: cardputer
description: Notify the user on their M5Cardputer when Claude Code needs their attention — finished a task, is stuck, or hit a failure that stops the work. Every tmux session is now exposed to the device automatically, so this skill covers ONLY paging the user via the cardputerme server's `POST /notify` API — it does not link or expose sessions (that happens by itself). Use when Claude Code needs the user's input/approval, is blocked, or a build/test/deploy just failed. Global; works from any Claude Code account.
---

# cardputer — notify the user on the device

The point of the Cardputer is that the user does **not** have to watch the screen. Every tmux
session on the machine is exposed to it automatically now — there is nothing to link, and
nothing else in this skill applies. The one thing left for Claude Code to do deliberately is
**ask for the user's attention** when it actually needs it: `POST /notify` plays a sound,
lights the LED, wakes the screen, and puts your text in the device header until the user
presses a key.

## The call

The server publishes its port in `~/.cardputerme/server.port` (a bare number). If that file is
missing, no cardputerme server is running yet on this machine — say so and stop; there is
nothing to notify.

```
port=$(tr -dc '0-9' <~/.cardputerme/server.port)
curl -sS -X POST "http://127.0.0.1:$port/notify" \
  -H 'Content-Type: application/json' \
  -d '{"session":"'"$(basename "$PWD")"'","text":"tests still running after 5m"}'
```

The reply is `{"delivered":…,"queued":true,"waiting":N,"clients":N,"reason":"…"}`.

**`delivered:false` is not an error, and the alert is never lost** — `queued` is always true,
and it will reach the device as soon as one is looking. Read `reason` to know why it did not go
out now:

- `silenced by ;notify 0` — the user asked for silence. Respect it; do not retry or route around it.
- `no device connected` — nothing is listening yet. The alert is waiting in the inbox.

`waiting` is how many alerts are still unanswered. If it is climbing, you are paging too often — stop.

**Levels.** Add `"level"` to say how much attention you are asking for. Absent or unknown means
`attention`, so you never have to set it.

| level | use it for | on the device |
|---|---|---|
| `info` | done, nothing needed from them | blue LED, one quiet note |
| `attention` (default) | finished and waiting on them | orange pulsing LED, the notification sound |
| `urgent` | broken and stopping the work | red LED, a rising three-note burst |

Use `urgent` sparingly. It is the one that will make someone put down a coffee; spend it on "the
deploy is failing", not "the tests finished".

## When to notify — the bar is "the user would want to be interrupted"

- A long task finished and you need their input or approval.
- You are **stuck**: blocked on something you cannot resolve, or a step has run far longer than expected.
- A build, test run or deploy failed in a way that stops the work.

**When NOT to:** routine progress, each step of a task, anything they will see the moment they
look at the terminal, or more than once for the same event. A device that cries wolf gets
ignored, and every alert costs the user's attention.

Keep `text` short — the header shows about 26 characters before clipping. Say what happened,
not what you did (`"tests failed: 3 red"` beats `"I have finished running the test suite"`). Set
`session` to the project name so the user knows **which** terminal wants them; both fields are
optional.

Do nothing else — this skill only pages the user. It never links, unlinks, or inspects sessions;
exposure is automatic and this skill has no part in it.

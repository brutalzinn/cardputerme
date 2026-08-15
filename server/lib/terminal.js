'use strict';

// Terminal backend adapter — the ONE place that knows how to read/write a
// terminal session. The bridge core (server.js) talks to this interface only, so
// it never depends on tmux: today the backend is tmux, later a PTY-owned shell
// can drop in with the same shape. An adapter exposes:
//
//   exists()        -> Promise<boolean>   is the session live?
//   capture()       -> Promise<string|null>  the visible screen text
//   cwd()           -> Promise<string>    the session's working dir ('' if none)
//   sendText(text)  -> Promise<boolean>   type text, then submit (Enter)
//   sendKey(key)    -> Promise<boolean>   one literal keypress, no Enter
//
// (Named keys — Tab / Enter / Esc / arrows — are task #4; sendKey handles a
// literal char today, e.g. answering a menu with its digit.)

const { execFile } = require('child_process');

// Keys sent by NAME (not literally) — tmux `send-keys <Name>`. Anything else is
// sent as a literal character with `-l`.
const NAMED_KEYS = new Set(['Enter', 'Escape', 'Tab', 'Up', 'Down', 'Left', 'Right', 'BSpace', 'Space']);

// Default process runner -> { code, stdout, stderr }. Injectable so the backend
// is unit-testable without spawning anything (tests pass a fake runner).
function run(cmd, args) {
  return new Promise((resolve) => {
    execFile(cmd, args, { maxBuffer: 4 * 1024 * 1024 }, (err, stdout, stderr) => {
      resolve({
        code: err && typeof err.code === 'number' ? err.code : err ? 1 : 0,
        stdout: stdout || '',
        stderr: stderr || '',
      });
    });
  });
}

// A terminal backend bound to a tmux session. tmux is just one implementation.
function tmuxBackend({ session, scrollbackLines = 200, runner = run, typeDelayMs = 120 } = {}) {
  return {
    async exists() {
      return (await runner('tmux', ['has-session', '-t', session])).code === 0;
    },
    async capture() {
      // -e keeps ANSI colour escapes so the server can mirror the terminal's own
      // colours to the device (parsed by lib/ansi).
      const args = ['capture-pane', '-p', '-e', '-t', session];
      if (scrollbackLines > 0) args.push('-S', `-${scrollbackLines}`);
      const { code, stdout } = await runner('tmux', args);
      return code === 0 ? stdout : null;
    },
    async cwd() {
      const { code, stdout } = await runner('tmux', ['display-message', '-p', '-t', session, '#{pane_current_path}']);
      return code === 0 ? stdout.trim() : '';
    },
    // Type text then submit — a small gap so the TUI registers the text before Enter.
    async sendText(text) {
      if ((await runner('tmux', ['send-keys', '-t', session, '-l', text])).code !== 0) return false;
      if (typeDelayMs > 0) await new Promise((r) => setTimeout(r, typeDelayMs));
      return (await runner('tmux', ['send-keys', '-t', session, 'Enter'])).code === 0;
    },
    // One keypress. Named keys (Escape/Enter/Tab/arrows…) go by name; any other
    // key is a literal character (e.g. a lone digit answering a menu).
    async sendKey(key) {
      const args = NAMED_KEYS.has(key)
        ? ['send-keys', '-t', session, key]
        : ['send-keys', '-t', session, '-l', key];
      return (await runner('tmux', args)).code === 0;
    },
  };
}

// Enumerate the live tmux sessions by name — feeds the session registry so the
// device can pick any running session. tmux-specific (a PTY backend would list
// its own sessions differently); kept here in the backend layer, not the core.
async function listTmuxSessions(runner = run) {
  const { code, stdout } = await runner('tmux', ['list-sessions', '-F', '#{session_name}']);
  if (code !== 0) return [];
  return stdout.split('\n').map((s) => s.trim()).filter((s) => s.length > 0);
}

module.exports = { tmuxBackend, listTmuxSessions, run };

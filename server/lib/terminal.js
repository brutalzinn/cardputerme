'use strict';

// Terminal backend adapter — the ONE place that knows how to read/write a
// terminal session AND the only place that knows the backend's key spellings.
// The core speaks GENERIC key names ('enter', 'escape', 'up', 'ctrl+c', '2');
// this adapter translates them to tmux. Swap in another backend (a PTY) and
// nothing outside this file changes. An adapter exposes:
//
//   exists()        -> Promise<boolean>   is the session live?
//   capture()       -> Promise<string|null>  the visible screen text (with ANSI)
//   cwd()           -> Promise<string>    the session's working dir ('' if none)
//   sendText(text)  -> Promise<boolean>   type text, then submit (Enter)
//   sendKey(key)    -> Promise<boolean>   one GENERIC keypress, no Enter

const { execFile } = require('child_process');

// Generic key name -> tmux named-key spelling. 'ctrl+<x>' maps to tmux 'C-<x>';
// any other single character is sent literally (-l).
const KEY_TO_TMUX = {
  enter: 'Enter',
  escape: 'Escape',
  tab: 'Tab',
  up: 'Up',
  down: 'Down',
  left: 'Left',
  right: 'Right',
  backspace: 'BSpace',
  space: 'Space',
};

function tmuxKey(key) {
  if (KEY_TO_TMUX[key]) return KEY_TO_TMUX[key];
  if (key.startsWith('ctrl+') && key.length === 6) return 'C-' + key[5];
  return null;   // literal character
}

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
    // Make sure the named session EXISTS (create it detached if missing) — this
    // is what lets `cardputerme <name>` expose a fresh session by name.
    async ensureSession(cwd) {
      if ((await runner('tmux', ['has-session', '-t', session])).code === 0) return true;
      const args = ['new-session', '-d', '-s', session];
      if (cwd) args.push('-c', cwd);
      return (await runner('tmux', args)).code === 0;
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
    // One GENERIC keypress ('enter', 'up', 'ctrl+c', a literal char) — translated
    // to the tmux spelling here and nowhere else.
    async sendKey(key) {
      const named = tmuxKey(key);
      const args = named
        ? ['send-keys', '-t', session, named]
        : ['send-keys', '-t', session, '-l', key];
      return (await runner('tmux', args)).code === 0;
    },
  };
}

module.exports = { tmuxBackend, run };

'use strict';

const { execFile } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const KEY_NAMES = {
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

function namedKey(key) {
  if (KEY_NAMES[key]) return KEY_NAMES[key];
  if (key.startsWith('ctrl+') && key.length === 6) return 'C-' + key[5];
  return null;
}

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

function createBackend({ session, scrollbackLines = 200, runner = run, typeDelayMs = 120 } = {}) {
  return {
    async exists() {
      return (await runner('tmux', ['has-session', '-t', session])).code === 0;
    },

    async ensureSession(cwd) {
      if ((await runner('tmux', ['has-session', '-t', session])).code === 0) return true;
      const args = ['new-session', '-d', '-s', session];
      if (cwd) args.push('-c', cwd);
      return (await runner('tmux', args)).code === 0;
    },
    async capture() {

      const args = ['capture-pane', '-p', '-e', '-t', session];
      if (scrollbackLines > 0) args.push('-S', `-${scrollbackLines}`);
      const { code, stdout } = await runner('tmux', args);
      return code === 0 ? stdout : null;
    },

    async sendText(text) {
      if ((await runner('tmux', ['send-keys', '-t', session, '-l', text])).code !== 0) return false;
      if (typeDelayMs > 0) await new Promise((r) => setTimeout(r, typeDelayMs));
      return (await runner('tmux', ['send-keys', '-t', session, 'Enter'])).code === 0;
    },

    async sendKey(key) {
      const named = namedKey(key);
      const args = named
        ? ['send-keys', '-t', session, named]
        : ['send-keys', '-t', session, '-l', key];
      return (await runner('tmux', args)).code === 0;
    },

    subscribe({ onChange, onGone }) {
      const fifo = path.join(os.tmpdir(), `cardputerme-${session}-${process.pid}.fifo`);
      let stream = null;
      let stopped = false;
      const open = () => {
        if (stopped) return;
        stream = fs.createReadStream(fifo);
        stream.on('data', () => onChange());
        stream.on('error', () => {});
        stream.on('close', async () => {
          if (stopped) return;
          if ((await runner('tmux', ['has-session', '-t', session])).code !== 0) { onGone(); return; }
          open();
        });
      };
      try { fs.unlinkSync(fifo); } catch { /* absent */ }
      execFile('mkfifo', [fifo], () => {
        runner('tmux', ['pipe-pane', '-o', '-t', session, `cat > '${fifo}'`]);
        open();
      });
      return () => {
        stopped = true;
        if (stream) stream.destroy();
        runner('tmux', ['pipe-pane', '-t', session]);
        try { fs.unlinkSync(fifo); } catch { /* absent */ }
      };
    },
  };
}

module.exports = { createBackend, run };


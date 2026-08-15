'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { tmuxBackend, listTmuxSessions } = require('../lib/terminal');

// A fake runner records the (cmd, args) issued and returns canned results in
// order, so we test the adapter CONTRACT without touching real tmux. This is
// what lets the core depend on `terminal`, not tmux — swap the backend freely.
function fakeRunner(results) {
  let i = 0;
  const runner = async (cmd, args) => {
    runner.calls.push({ cmd, args });
    const r = (results && results[i++]) || {};
    return { code: 0, stdout: '', stderr: '', ...r };
  };
  runner.calls = [];
  return runner;
}

test('exists() runs has-session and maps exit code to boolean', async () => {
  const ok = fakeRunner([{ code: 0 }]);
  assert.equal(await tmuxBackend({ session: 'S', runner: ok }).exists(), true);
  assert.deepEqual(ok.calls[0], { cmd: 'tmux', args: ['has-session', '-t', 'S'] });

  const no = fakeRunner([{ code: 1 }]);
  assert.equal(await tmuxBackend({ session: 'S', runner: no }).exists(), false);
});

test('capture() runs capture-pane with scrollback and returns stdout (null on failure)', async () => {
  const ok = fakeRunner([{ code: 0, stdout: 'PANE TEXT' }]);
  const t = tmuxBackend({ session: 'S', scrollbackLines: 200, runner: ok });
  assert.equal(await t.capture(), 'PANE TEXT');
  assert.deepEqual(ok.calls[0].args, ['capture-pane', '-p', '-e', '-t', 'S', '-S', '-200']);

  const fail = fakeRunner([{ code: 1, stdout: 'ignored' }]);
  assert.equal(await tmuxBackend({ session: 'S', runner: fail }).capture(), null);
});

test('capture() omits the -S flag when scrollback is 0', async () => {
  const ok = fakeRunner([{ code: 0, stdout: 'x' }]);
  await tmuxBackend({ session: 'S', scrollbackLines: 0, runner: ok }).capture();
  assert.deepEqual(ok.calls[0].args, ['capture-pane', '-p', '-e', '-t', 'S']);
});

test('cwd() runs display-message and returns the trimmed path', async () => {
  const ok = fakeRunner([{ code: 0, stdout: '/Users/x/proj\n' }]);
  const t = tmuxBackend({ session: 'S', runner: ok });
  assert.equal(await t.cwd(), '/Users/x/proj');
  assert.deepEqual(ok.calls[0].args, ['display-message', '-p', '-t', 'S', '#{pane_current_path}']);

  const fail = fakeRunner([{ code: 1, stdout: '/nope' }]);
  assert.equal(await tmuxBackend({ session: 'S', runner: fail }).cwd(), '');
});

test('sendText() types the literal text then submits with Enter', async () => {
  const ok = fakeRunner([{ code: 0 }, { code: 0 }]);
  const t = tmuxBackend({ session: 'S', runner: ok, typeDelayMs: 0 });
  assert.equal(await t.sendText('git status'), true);
  assert.deepEqual(ok.calls[0].args, ['send-keys', '-t', 'S', '-l', 'git status']);
  assert.deepEqual(ok.calls[1].args, ['send-keys', '-t', 'S', 'Enter']);
});

test('sendText() returns false and does NOT press Enter when typing fails', async () => {
  const fail = fakeRunner([{ code: 1 }]);
  const t = tmuxBackend({ session: 'S', runner: fail, typeDelayMs: 0 });
  assert.equal(await t.sendText('x'), false);
  assert.equal(fail.calls.length, 1); // no Enter after a failed type
});

test('sendKey() sends a literal char with -l (e.g. a menu digit)', async () => {
  const ok = fakeRunner([{ code: 0 }]);
  const t = tmuxBackend({ session: 'S', runner: ok });
  assert.equal(await t.sendKey('2'), true);
  assert.deepEqual(ok.calls[0].args, ['send-keys', '-t', 'S', '-l', '2']);
});

test('sendKey() sends a NAMED key (Escape) by name, without -l', async () => {
  const ok = fakeRunner([{ code: 0 }]);
  const t = tmuxBackend({ session: 'S', runner: ok });
  assert.equal(await t.sendKey('Escape'), true);
  assert.deepEqual(ok.calls[0].args, ['send-keys', '-t', 'S', 'Escape']);
});

test('listTmuxSessions() lists session names, dropping blanks', async () => {
  const ok = fakeRunner([{ code: 0, stdout: 'claude\ngeneric\n\nrchat\n' }]);
  assert.deepEqual(await listTmuxSessions(ok), ['claude', 'generic', 'rchat']);
  assert.deepEqual(ok.calls[0].args, ['list-sessions', '-F', '#{session_name}']);
});

test('listTmuxSessions() returns [] when tmux has no server / errors', async () => {
  const fail = fakeRunner([{ code: 1, stdout: 'no server running' }]);
  assert.deepEqual(await listTmuxSessions(fail), []);
});

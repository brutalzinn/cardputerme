'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { tmuxBackend, listTmuxSessions, listTmuxSessionsInfo, killTmuxSession } = require('../lib/terminal');

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

// The core speaks GENERIC key names; ONLY this adapter knows tmux spellings.
test('sendKey() translates generic ctrl+c to tmux C-c, without -l', async () => {
  const ok = fakeRunner([{ code: 0 }]);
  const t = tmuxBackend({ session: 'S', runner: ok });
  assert.equal(await t.sendKey('ctrl+c'), true);
  assert.deepEqual(ok.calls[0].args, ['send-keys', '-t', 'S', 'C-c']);
});

test('sendKey() translates generic named keys to tmux spellings, without -l', async () => {
  const cases = [['escape', 'Escape'], ['enter', 'Enter'], ['tab', 'Tab'], ['up', 'Up']];
  for (const [generic, tmuxName] of cases) {
    const ok = fakeRunner([{ code: 0 }]);
    const t = tmuxBackend({ session: 'S', runner: ok });
    assert.equal(await t.sendKey(generic), true);
    assert.deepEqual(ok.calls[0].args, ['send-keys', '-t', 'S', tmuxName]);
  }
});

test('ensureSession() creates a detached session when it does not exist', async () => {
  const r = fakeRunner([{ code: 1 }, { code: 0 }]);           // has-session: no -> create
  const t = tmuxBackend({ session: 'test', runner: r });
  assert.equal(await t.ensureSession(), true);
  assert.deepEqual(r.calls[0].args, ['has-session', '-t', 'test']);
  assert.deepEqual(r.calls[1].args, ['new-session', '-d', '-s', 'test']);
});

test('ensureSession() is a no-op when the session already exists', async () => {
  const r = fakeRunner([{ code: 0 }]);                        // has-session: yes
  const t = tmuxBackend({ session: 'test', runner: r });
  assert.equal(await t.ensureSession(), true);
  assert.equal(r.calls.length, 1);                            // no create call
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

test('listTmuxSessionsInfo() parses name, activity, attached and pane command', async () => {
  const ok = fakeRunner([{ code: 0, stdout: 'claude\t1755200000\t1\tnode\nold\t1755100000\t0\tzsh\n\n' }]);
  assert.deepEqual(await listTmuxSessionsInfo(ok), [
    { name: 'claude', activity: 1755200000, attached: 1, command: 'node' },
    { name: 'old', activity: 1755100000, attached: 0, command: 'zsh' },
  ]);
  assert.deepEqual(ok.calls[0].args, [
    'list-sessions', '-F', '#{session_name}\t#{session_activity}\t#{session_attached}\t#{pane_current_command}',
  ]);
});

test('listTmuxSessionsInfo() returns [] when tmux has no server / errors', async () => {
  const fail = fakeRunner([{ code: 1, stdout: 'no server running' }]);
  assert.deepEqual(await listTmuxSessionsInfo(fail), []);
});

test('killTmuxSession() kills by EXACT name (=) so prefixes never match another session', async () => {
  const ok = fakeRunner([{ code: 0 }]);
  assert.equal(await killTmuxSession('old', ok), true);
  assert.deepEqual(ok.calls[0], { cmd: 'tmux', args: ['kill-session', '-t', '=old'] });

  const fail = fakeRunner([{ code: 1 }]);
  assert.equal(await killTmuxSession('old', fail), false);
});

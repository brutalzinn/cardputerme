'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { createBackend } = require('../lib/terminal');

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
  assert.equal(await createBackend({ session: 'S', runner: ok }).exists(), true);
  assert.deepEqual(ok.calls[0], { cmd: 'tmux', args: ['has-session', '-t', 'S'] });

  const no = fakeRunner([{ code: 1 }]);
  assert.equal(await createBackend({ session: 'S', runner: no }).exists(), false);
});

test('capture() runs capture-pane with scrollback and returns stdout (null on failure)', async () => {
  const ok = fakeRunner([{ code: 0, stdout: 'PANE TEXT' }]);
  const t = createBackend({ session: 'S', scrollbackLines: 200, runner: ok });
  assert.equal(await t.capture(), 'PANE TEXT');
  assert.deepEqual(ok.calls[0].args, ['capture-pane', '-p', '-e', '-t', 'S', '-S', '-200']);

  const fail = fakeRunner([{ code: 1, stdout: 'ignored' }]);
  assert.equal(await createBackend({ session: 'S', runner: fail }).capture(), null);
});

test('capture() omits the -S flag when scrollback is 0', async () => {
  const ok = fakeRunner([{ code: 0, stdout: 'x' }]);
  await createBackend({ session: 'S', scrollbackLines: 0, runner: ok }).capture();
  assert.deepEqual(ok.calls[0].args, ['capture-pane', '-p', '-e', '-t', 'S']);
});

test('sendText() types the literal text then submits with Enter', async () => {
  const ok = fakeRunner([{ code: 0 }, { code: 0 }]);
  const t = createBackend({ session: 'S', runner: ok, typeDelayMs: 0 });
  assert.equal(await t.sendText('git status'), true);
  assert.deepEqual(ok.calls[0].args, ['send-keys', '-t', 'S', '-l', 'git status']);
  assert.deepEqual(ok.calls[1].args, ['send-keys', '-t', 'S', 'Enter']);
});

test('sendText() returns false and does NOT press Enter when typing fails', async () => {
  const fail = fakeRunner([{ code: 1 }]);
  const t = createBackend({ session: 'S', runner: fail, typeDelayMs: 0 });
  assert.equal(await t.sendText('x'), false);
  assert.equal(fail.calls.length, 1);
});

test('sendKey() sends a literal char with -l (e.g. a menu digit)', async () => {
  const ok = fakeRunner([{ code: 0 }]);
  const t = createBackend({ session: 'S', runner: ok });
  assert.equal(await t.sendKey('2'), true);
  assert.deepEqual(ok.calls[0].args, ['send-keys', '-t', 'S', '-l', '2']);
});

test('sendKey() translates generic ctrl+c to the backend C-c, without -l', async () => {
  const ok = fakeRunner([{ code: 0 }]);
  const t = createBackend({ session: 'S', runner: ok });
  assert.equal(await t.sendKey('ctrl+c'), true);
  assert.deepEqual(ok.calls[0].args, ['send-keys', '-t', 'S', 'C-c']);
});

test('sendKey() translates generic named keys to backend spellings, without -l', async () => {
  const cases = [['escape', 'Escape'], ['enter', 'Enter'], ['tab', 'Tab'], ['up', 'Up']];
  for (const [generic, spelling] of cases) {
    const ok = fakeRunner([{ code: 0 }]);
    const t = createBackend({ session: 'S', runner: ok });
    assert.equal(await t.sendKey(generic), true);
    assert.deepEqual(ok.calls[0].args, ['send-keys', '-t', 'S', spelling]);
  }
});

test('ensureSession() creates a detached session when it does not exist', async () => {
  const r = fakeRunner([{ code: 1 }, { code: 0 }]);
  const t = createBackend({ session: 'test', runner: r });
  assert.equal(await t.ensureSession(), true);
  assert.deepEqual(r.calls[0].args, ['has-session', '-t', 'test']);
  assert.deepEqual(r.calls[1].args, ['new-session', '-d', '-s', 'test']);
});

test('ensureSession(cwd) creates the session in the given working dir', async () => {
  const r = fakeRunner([{ code: 1 }, { code: 0 }]);
  const t = createBackend({ session: 'test', runner: r });
  assert.equal(await t.ensureSession('/Users/x/proj'), true);
  assert.deepEqual(r.calls[1].args, ['new-session', '-d', '-s', 'test', '-c', '/Users/x/proj']);
});

test('ensureSession() is a no-op when the session already exists', async () => {
  const r = fakeRunner([{ code: 0 }]);
  const t = createBackend({ session: 'test', runner: r });
  assert.equal(await t.ensureSession(), true);
  assert.equal(r.calls.length, 1);
});


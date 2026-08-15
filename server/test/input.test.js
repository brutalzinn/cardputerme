'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { interpretKey, isDigits, pickIndex } = require('../lib/input');

// The LAZY-DEVICE contract: the device forwards every raw key; the SERVER owns
// the input buffer, all special functions (esc clears / opens the picker), and
// what to send to the terminal. State: { mode:'mirror'|'picker', input:string }.

const MIRROR = (input) => ({ mode: 'mirror', input: input || '' });
const PICKER = () => ({ mode: 'picker', input: '' });
const SESS = ['claude', 'generic', 'rchat'];
const ctx = (over) => Object.assign({ sessions: SESS, awaiting: false }, over);

test('helpers scan chars (no regex)', () => {
  assert.equal(isDigits('42'), true);
  assert.equal(isDigits('4a'), false);
  assert.equal(pickIndex('2', 3), 2);
  assert.equal(pickIndex('9', 3), 0);
  assert.equal(pickIndex('x', 3), 0);
});

// --- typing into the server-owned buffer
test('a char appends to the input buffer (no terminal send)', () => {
  const r = interpretKey(MIRROR(''), 'h', ctx());
  assert.equal(r.state.input, 'h');
  assert.equal(r.action.kind, 'none');
});

test('backspace removes the last char', () => {
  const r = interpretKey(MIRROR('hi'), 'backspace', ctx());
  assert.equal(r.state.input, 'h');
});

test('shift+enter appends a newline, still composing', () => {
  const r = interpretKey(MIRROR('line1'), 'shift+enter', ctx());
  assert.equal(r.state.input, 'line1\n');
  assert.equal(r.action.kind, 'none');
});

test('enter with text sends it and clears the buffer', () => {
  const r = interpretKey(MIRROR('git status'), 'enter', ctx());
  assert.deepEqual(r.action, { kind: 'send', text: 'git status' });
  assert.equal(r.state.input, '');
});

test('enter with an empty buffer presses Enter in the terminal', () => {
  const r = interpretKey(MIRROR(''), 'enter', ctx());
  assert.deepEqual(r.action, { kind: 'pressKey', key: 'enter' });
});

// --- esc: the special function key, fully server-side
test('esc while typing CLEARS the input (cancel current sending)', () => {
  const r = interpretKey(MIRROR('half a mess'), 'esc', ctx());
  assert.equal(r.state.input, '');
  assert.equal(r.state.mode, 'mirror');
  assert.equal(r.action.kind, 'none');
});

test('esc with nothing typed opens the picker', () => {
  const r = interpretKey(MIRROR(''), 'esc', ctx());
  assert.equal(r.state.mode, 'picker');
  assert.equal(r.action.kind, 'openPicker');
});

test('esc in the picker cancels back to mirror', () => {
  const r = interpretKey(PICKER(), 'esc', ctx());
  assert.equal(r.state.mode, 'mirror');
  assert.equal(r.action.kind, 'closePicker');
});

// --- prompts and the picker use digits
test('digit answers an on-screen menu when awaiting and not typing', () => {
  const r = interpretKey(MIRROR(''), '2', ctx({ awaiting: true }));
  assert.deepEqual(r.action, { kind: 'pressKey', key: '2' });
  assert.equal(r.state.input, '');
});

test('digit while typing just appends (no menu answer)', () => {
  const r = interpretKey(MIRROR('port '), '2', ctx({ awaiting: true }));
  assert.equal(r.state.input, 'port 2');
  assert.equal(r.action.kind, 'none');
});

test('picker + number selects that session, back to mirror', () => {
  const r = interpretKey(PICKER(), '2', ctx());
  assert.deepEqual(r.action, { kind: 'select', name: 'generic' });
  assert.equal(r.state.mode, 'mirror');
});

test('picker ignores junk and out-of-range numbers', () => {
  assert.equal(interpretKey(PICKER(), '9', ctx()).action.kind, 'none');
  assert.equal(interpretKey(PICKER(), 'a', ctx()).state.mode, 'picker');
});

test('arrows produce a pan action in mirror mode', () => {
  const r = interpretKey(MIRROR('typing kept'), 'up', ctx());
  assert.deepEqual(r.action, { kind: 'pan', key: 'up' });
  assert.equal(r.state.input, 'typing kept');
});

test('arrows PAN even while awaiting a prompt (arrows read, numbers choose)', () => {
  assert.deepEqual(interpretKey(MIRROR(''), 'down', ctx({ awaiting: true })).action, { kind: 'pan', key: 'down' });
  assert.deepEqual(interpretKey(MIRROR(''), 'up', ctx({ awaiting: true })).action, { kind: 'pan', key: 'up' });
  assert.deepEqual(interpretKey(MIRROR(''), 'left', ctx({ awaiting: true })).action, { kind: 'pan', key: 'left' });
  assert.deepEqual(interpretKey(MIRROR('draft'), 'up', ctx({ awaiting: true })).action, { kind: 'pan', key: 'up' });
});

test('tab presses Tab in the terminal (selector auto-advance)', () => {
  const r = interpretKey(MIRROR(''), 'tab', ctx());
  assert.deepEqual(r.action, { kind: 'pressKey', key: 'tab' });
});

test('arrows are ignored in the picker', () => {
  assert.equal(interpretKey(PICKER(), 'down', ctx()).action.kind, 'none');
});

test('opt+arrows send real arrow keys to the terminal (drive its selector)', () => {
  const r = interpretKey(MIRROR(''), 'opt+up', ctx({ awaiting: true }));
  assert.deepEqual(r.action, { kind: 'pressKey', key: 'up' });
  assert.deepEqual(interpretKey(MIRROR(''), 'opt+down', ctx()).action, { kind: 'pressKey', key: 'down' });
  assert.deepEqual(interpretKey(MIRROR(''), 'opt+left', ctx()).action, { kind: 'pressKey', key: 'left' });
});

// --- shift+esc: STOP the agent — a real Escape into the terminal (Claude
// Code's own "esc to interrupt"), without touching the user's draft.
test('shift+esc interrupts the terminal activity and keeps the input', () => {
  const r = interpretKey(MIRROR('my draft'), 'shift+esc', ctx());
  assert.deepEqual(r.action, { kind: 'pressKey', key: 'escape' });
  assert.equal(r.state.input, 'my draft');
});

test('shift+esc works with an empty input too (never opens the picker)', () => {
  const r = interpretKey(MIRROR(''), 'shift+esc', ctx({ awaiting: true }));
  assert.deepEqual(r.action, { kind: 'pressKey', key: 'escape' });
  assert.equal(r.state.mode, 'mirror');
});

// --- Ctrl combos: real control keys reach the terminal (feel-at-home).
test('ctrl+<letter> sends the control key to the terminal', () => {
  const r = interpretKey(MIRROR(''), 'ctrl+c', ctx());
  assert.deepEqual(r.action, { kind: 'pressKey', key: 'ctrl+c' });
});

test('ctrl combos do not disturb the input buffer', () => {
  const r = interpretKey(MIRROR('keep me'), 'ctrl+c', ctx());
  assert.equal(r.state.input, 'keep me');
});

// --- Command history recall (ctrl+up = prev, ctrl+down = next — shell idiom).
const HIST = ['first cmd', 'second cmd', 'third cmd'];   // oldest -> newest

test('ctrl+up recalls the newest command into the input', () => {
  const r = interpretKey(MIRROR(''), 'ctrl+up', ctx({ history: HIST }));
  assert.equal(r.state.input, 'third cmd');
  assert.equal(r.state.hist, 2);
  assert.equal(r.action.kind, 'none');
});

test('ctrl+up again steps further back (and floors at the oldest)', () => {
  let s = interpretKey(MIRROR(''), 'ctrl+up', ctx({ history: HIST })).state;
  s = interpretKey(s, 'ctrl+up', ctx({ history: HIST })).state;
  assert.equal(s.input, 'second cmd');
  s = interpretKey(s, 'ctrl+up', ctx({ history: HIST })).state;
  s = interpretKey(s, 'ctrl+up', ctx({ history: HIST })).state;   // extra press
  assert.equal(s.input, 'first cmd');                            // floored
});

test('ctrl+down steps forward; past the newest clears the input', () => {
  let s = interpretKey(MIRROR(''), 'ctrl+up', ctx({ history: HIST })).state;   // third
  s = interpretKey(s, 'ctrl+up', ctx({ history: HIST })).state;                // second
  s = interpretKey(s, 'ctrl+down', ctx({ history: HIST })).state;                // third again
  assert.equal(s.input, 'third cmd');
  s = interpretKey(s, 'ctrl+down', ctx({ history: HIST })).state;                // past newest
  assert.equal(s.input, '');
  assert.equal(s.hist, null);
});

test('a recalled command is editable and Enter sends the edited text', () => {
  let s = interpretKey(MIRROR(''), 'ctrl+up', ctx({ history: HIST })).state;
  s = interpretKey(s, '!', ctx({ history: HIST })).state;
  const r = interpretKey(s, 'enter', ctx({ history: HIST }));
  assert.deepEqual(r.action, { kind: 'send', text: 'third cmd!' });
  assert.equal(r.state.hist, null);                              // send resets recall
});

test('ctrl+up with no history does nothing', () => {
  const r = interpretKey(MIRROR(''), 'ctrl+up', ctx({ history: [] }));
  assert.equal(r.state.input, '');
  assert.equal(r.action.kind, 'none');
});

test('esc on a recalled draft clears input and recall state', () => {
  let s = interpretKey(MIRROR(''), 'ctrl+up', ctx({ history: HIST })).state;
  const r = interpretKey(s, 'esc', ctx({ history: HIST }));
  assert.equal(r.state.input, '');
  assert.equal(r.state.hist, null);
  assert.equal(r.state.mode, 'mirror');                          // clear, not picker
});

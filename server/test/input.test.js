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
  assert.deepEqual(r.action, { kind: 'pressEnter' });
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
  assert.deepEqual(r.action, { kind: 'answerMenu', key: '2' });
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

// --- arrows: pan the viewport while reading, but DRIVE the terminal's own
// selector (AskUserQuestion & co.) when a prompt is up and nothing is typed.
test('arrows produce a pan action in mirror mode', () => {
  const r = interpretKey(MIRROR('typing kept'), 'up', ctx());
  assert.deepEqual(r.action, { kind: 'pan', key: 'up' });
  assert.equal(r.state.input, 'typing kept');   // panning does not lose input
});

test('up/down go INTO the terminal when awaiting a prompt and not typing', () => {
  const r = interpretKey(MIRROR(''), 'down', ctx({ awaiting: true }));
  assert.deepEqual(r.action, { kind: 'sendArrow', key: 'Down' });
  assert.deepEqual(interpretKey(MIRROR(''), 'up', ctx({ awaiting: true })).action, { kind: 'sendArrow', key: 'Up' });
});

test('left/right still PAN while awaiting (horizontal reading stays possible)', () => {
  assert.equal(interpretKey(MIRROR(''), 'left', ctx({ awaiting: true })).action.kind, 'pan');
  assert.equal(interpretKey(MIRROR(''), 'right', ctx({ awaiting: true })).action.kind, 'pan');
});

test('arrows still pan while typing, even when awaiting', () => {
  const r = interpretKey(MIRROR('draft'), 'up', ctx({ awaiting: true }));
  assert.equal(r.action.kind, 'pan');
});

test('tab presses Tab in the terminal (selector auto-advance)', () => {
  const r = interpretKey(MIRROR(''), 'tab', ctx());
  assert.deepEqual(r.action, { kind: 'pressTab' });
});

test('arrows are ignored in the picker', () => {
  assert.equal(interpretKey(PICKER(), 'down', ctx()).action.kind, 'none');
});

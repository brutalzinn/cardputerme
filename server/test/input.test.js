'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { interpretKey, isAllBackticks, isDigits, pickIndex } = require('../lib/input');

const MIRROR = { mode: 'mirror' };
const PICKER = { mode: 'picker' };
const SESS = ['claude', 'generic', 'rchat'];

test('helpers scan chars (no regex)', () => {
  assert.equal(isAllBackticks('`'), true);
  assert.equal(isAllBackticks('``'), true);
  assert.equal(isAllBackticks('`a'), false);
  assert.equal(isAllBackticks(''), false);
  assert.equal(isDigits('42'), true);
  assert.equal(isDigits('4a'), false);
  assert.equal(pickIndex('2', 3), 2);
  assert.equal(pickIndex('9', 3), 0);   // out of range
  assert.equal(pickIndex('x', 3), 0);
});

// --- mirror mode: the Cardputer Esc button emits `; the server interprets it.
test('mirror + single backtick -> send a real Escape to the terminal (never typed)', () => {
  const r = interpretKey(MIRROR, '`', { sessions: SESS });
  assert.deepEqual(r, { state: { mode: 'mirror' }, action: { kind: 'escape' } });
});

test('mirror + double backtick -> open the session picker', () => {
  const r = interpretKey(MIRROR, '``', { sessions: SESS });
  assert.deepEqual(r.state, { mode: 'picker' });
  assert.equal(r.action.kind, 'openPicker');
});

test('mirror + digit while awaiting a menu -> answer that menu', () => {
  const r = interpretKey(MIRROR, '2', { sessions: SESS, awaiting: true });
  assert.deepEqual(r.action, { kind: 'answerMenu', key: '2' });
});

test('mirror + digit when NOT awaiting -> just type it', () => {
  const r = interpretKey(MIRROR, '2', { sessions: SESS, awaiting: false });
  assert.deepEqual(r.action, { kind: 'type', text: '2' });
});

test('mirror + normal text -> type it', () => {
  const r = interpretKey(MIRROR, 'git status', { sessions: SESS });
  assert.deepEqual(r.action, { kind: 'type', text: 'git status' });
});

// --- picker mode: numbered text, pick by number (Telegram-style).
test('picker + number -> select that session, back to mirror', () => {
  const r = interpretKey(PICKER, '2', { sessions: SESS });
  assert.deepEqual(r, { state: { mode: 'mirror' }, action: { kind: 'select', name: 'generic' } });
});

test('picker + backtick -> cancel back to mirror', () => {
  const r = interpretKey(PICKER, '`', { sessions: SESS });
  assert.deepEqual(r, { state: { mode: 'mirror' }, action: { kind: 'closePicker' } });
});

test('picker + out-of-range or junk -> ignored, stay in picker', () => {
  assert.deepEqual(interpretKey(PICKER, '9', { sessions: SESS }).state, { mode: 'picker' });
  assert.equal(interpretKey(PICKER, '9', { sessions: SESS }).action.kind, 'none');
  assert.equal(interpretKey(PICKER, 'abc', { sessions: SESS }).action.kind, 'none');
});

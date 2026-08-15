'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { sessionsToClear, SHELLS } = require('../lib/idle');

const info = (name, over) => ({ name, activity: 1000, attached: 0, command: 'zsh', ...over });
const opts = (over) => ({ nowSec: 1000 + 1800, timeoutSec: 1800, active: 'picked', ...over });

test('clears a detached shell session idle past the timeout', () => {
  assert.deepEqual(sessionsToClear([info('old')], opts()), ['old']);
});

test('keeps a session still within the timeout', () => {
  assert.deepEqual(sessionsToClear([info('fresh', { activity: 1001 })], opts()), []);
});

test('keeps the active (device-selected) session no matter how idle', () => {
  assert.deepEqual(sessionsToClear([info('picked')], opts()), []);
});

test('keeps an attached session', () => {
  assert.deepEqual(sessionsToClear([info('mine', { attached: 1 })], opts()), []);
});

test('keeps a session running a program (not a bare shell prompt)', () => {
  assert.deepEqual(sessionsToClear([info('busy', { command: 'node' })], opts()), []);
});

test('clears every shell it knows as a bare prompt', () => {
  for (const shell of SHELLS) {
    assert.deepEqual(sessionsToClear([info('s', { command: shell })], opts()), ['s']);
  }
});

test('mixed list: returns only the clearable names', () => {
  const infos = [
    info('old'),
    info('picked'),
    info('fresh', { activity: 1001 }),
    info('busy', { command: 'vim' }),
    info('old2'),
  ];
  assert.deepEqual(sessionsToClear(infos, opts()), ['old', 'old2']);
});

test('timeout of zero clears nothing (feature off)', () => {
  assert.deepEqual(sessionsToClear([info('old')], opts({ timeoutSec: 0 })), []);
});

'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { buildDisplay, COLORS } = require('../lib/display');

test('buildDisplay wraps body lines into {text,color} and adds a status bar', () => {
  const msg = buildDisplay(['hello world'], '[generic] ready', {});
  assert.equal(msg.type, 'display');
  assert.deepEqual(msg.body, [{ text: 'hello world', color: COLORS.text }]);
  assert.deepEqual(msg.status, { text: '[generic] ready', color: COLORS.status });
});

test('buildDisplay colors a "> " user-prompt line with the prompt color', () => {
  const msg = buildDisplay(['> run the plan', 'ok, running'], '', {});
  assert.equal(msg.body[0].color, COLORS.prompt);
  assert.equal(msg.body[1].color, COLORS.text);
});

test('buildDisplay colors every body line with the ask color while awaiting', () => {
  const msg = buildDisplay(['Proceed?', '1. Yes', '2. No'], '', { awaiting: true });
  for (const l of msg.body) assert.equal(l.color, COLORS.ask);
});

test('buildDisplay on empty body yields an empty body list + empty status', () => {
  const msg = buildDisplay([], undefined, undefined);
  assert.deepEqual(msg.body, []);
  assert.deepEqual(msg.status, { text: '', color: COLORS.status });
});

test('buildDisplay coerces non-string cells + status to strings', () => {
  const msg = buildDisplay([42], 7, {});
  assert.equal(msg.body[0].text, '42');
  assert.equal(msg.status.text, '7');
});

test('COLORS are explicit RGB565 numbers incl. a status color', () => {
  for (const k of ['text', 'prompt', 'ask', 'status']) assert.equal(typeof COLORS[k], 'number');
});


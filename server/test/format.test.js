'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { toAscii, wrapLine, sliceIntoCards } = require('../lib/format');

test('toAscii transliterates common typography', () => {
  assert.equal(toAscii('“hi” — it’s a test…'), '"hi" - it\'s a test...');
});

test('toAscii drops emoji and box-drawing so the ASCII font stays clean', () => {
  assert.equal(toAscii('Pong! 🏓'), 'Pong! ');
  assert.equal(toAscii('╭─ box ─╮'), ' box ');
});

test('toAscii keeps newlines', () => {
  assert.equal(toAscii('a\nb'), 'a\nb');
});

test('wrapLine breaks at word boundaries within cols', () => {
  assert.deepEqual(wrapLine('the quick brown fox', 10), ['the quick', 'brown fox']);
});

test('wrapLine hard-breaks a word longer than cols', () => {
  assert.deepEqual(wrapLine('supercalifragilistic', 10), ['supercalif', 'ragilistic']);
});

test('sliceIntoCards chunks wrapped lines into cards of linesPerCard', () => {
  const cards = sliceIntoCards('one two three four five six', 100, 2);
  // one logical line -> 1 wrapped line -> 1 card of up to 2 lines
  assert.equal(cards.length, 1);
  assert.equal(cards[0][0], 'one two three four five six');
});

test('sliceIntoCards splits many lines across cards and caps at maxCards', () => {
  const text = Array.from({ length: 20 }, (_, i) => 'line' + i).join('\n');
  const cards = sliceIntoCards(text, 20, 5, 3); // maxCards=3 keeps the newest
  assert.equal(cards.length, 3);
  // newest tail retained: last line should be line19
  assert.equal(cards[cards.length - 1].at(-1), 'line19');
});

test('sliceIntoCards sanitizes unicode before slicing', () => {
  const cards = sliceIntoCards('Pong! 🏓', 20, 5);
  assert.equal(cards[0][0], 'Pong!');
});

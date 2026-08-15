'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { rgb565, xterm256, parseLine, stripAnsi } = require('../lib/ansi');

const DEF = 0xFFFF;
const ESC = '\x1b';

test('rgb565 packs r,g,b into 16-bit 565', () => {
  assert.equal(rgb565(255, 255, 255), 0xFFFF);
  assert.equal(rgb565(0, 0, 0), 0x0000);
  assert.equal(rgb565(255, 0, 0), 0xF800);
  assert.equal(rgb565(0, 255, 0), 0x07E0);
  assert.equal(rgb565(0, 0, 255), 0x001F);
});

test('xterm256 maps cube + greyscale indices', () => {
  assert.equal(xterm256(196), rgb565(255, 0, 0));   // cube bright red
  assert.equal(xterm256(231), rgb565(255, 255, 255)); // cube white
  assert.equal(xterm256(16), rgb565(0, 0, 0));      // cube black
  assert.equal(xterm256(255), rgb565(238, 238, 238)); // top greyscale
});

test('stripAnsi removes escape sequences, keeping text', () => {
  assert.equal(stripAnsi(`${ESC}[31mhi${ESC}[0m`), 'hi');
  assert.equal(stripAnsi(`plain`), 'plain');
  assert.equal(stripAnsi(`${ESC}[38;5;246m x ${ESC}[39m`), ' x ');
});

test('parseLine returns clean text + the color at the first visible char', () => {
  const r = parseLine(`${ESC}[38;5;231muser typed this${ESC}[39m`, DEF);
  assert.equal(r.text, 'user typed this');
  assert.equal(r.color, xterm256(231));
});

test('parseLine uses the default color when no color is set', () => {
  const r = parseLine('just text', DEF);
  assert.equal(r.text, 'just text');
  assert.equal(r.color, DEF);
});

test('parseLine honors 24-bit truecolor (38;2;r;g;b)', () => {
  const r = parseLine(`${ESC}[38;2;255;0;0mred`, DEF);
  assert.equal(r.color, rgb565(255, 0, 0));
});

test('parseLine ignores leading spaces when picking the line color', () => {
  const r = parseLine(`   ${ESC}[38;5;196mX`, DEF);
  assert.equal(r.text, '   X');
  assert.equal(r.color, xterm256(196));
});

'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { endsWithQuestion, parseChoices, detectChoice } = require('../lib/detect');

// --- endsWithQuestion: the ONLY question rule — last non-space char is '?'.
// Plain string scan, no regex, no hooks, no model call. Agnostic to what wrote
// the text (Claude Code, a shell prompt, anything in the terminal).
test('endsWithQuestion is true when text ends with a question mark', () => {
  assert.equal(endsWithQuestion('Do you want to proceed?'), true);
});

test('endsWithQuestion ignores trailing whitespace/newlines', () => {
  assert.equal(endsWithQuestion('Proceed?  \n\t'), true);
});

test('endsWithQuestion is false without a trailing question mark', () => {
  assert.equal(endsWithQuestion('All done.'), false);
  assert.equal(endsWithQuestion('a? and more'), false);
  assert.equal(endsWithQuestion(''), false);
});

// --- parseChoices: an option list is "N. label" or "N) label" lines. Detected
// by scanning characters (leading digits, a '.'/')' separator, a space), never
// by a regex pattern.
test('parseChoices reads a numbered menu with dot separators', () => {
  const opts = parseChoices('Pick one:\n1. Yes\n2. No, keep it');
  assert.deepEqual(opts, [
    { n: 1, label: 'Yes' },
    { n: 2, label: 'No, keep it' },
  ]);
});

test('parseChoices accepts ")" separators and leading indentation', () => {
  const opts = parseChoices('  1) Alpha\n  2) Beta');
  assert.deepEqual(opts, [
    { n: 1, label: 'Alpha' },
    { n: 2, label: 'Beta' },
  ]);
});

test('parseChoices ignores lines that are not options', () => {
  assert.deepEqual(parseChoices('3.14 is pi\nno number here\n1.no space'), []);
});

// --- detectChoice: the combined, agnostic signal the bridge acts on.
test('detectChoice flags awaiting on a 2+ option menu', () => {
  const d = detectChoice('Choose:\n1. Yes\n2. No');
  assert.equal(d.awaiting, true);
  assert.equal(d.options.length, 2);
});

test('detectChoice flags awaiting on a trailing question with no menu', () => {
  const d = detectChoice('Should I continue?');
  assert.equal(d.awaiting, true);
  assert.equal(d.question, true);
  assert.equal(d.options.length, 0);
});

test('detectChoice is not awaiting on a plain statement', () => {
  const d = detectChoice('Finished the refactor.');
  assert.equal(d.awaiting, false);
  assert.equal(d.question, false);
  assert.equal(d.options.length, 0);
});

test('detectChoice does not treat a single stray option as a menu', () => {
  const d = detectChoice('Step 1. do the thing');
  assert.equal(d.awaiting, false);
});

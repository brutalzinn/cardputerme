'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { endsWithQuestion, parseChoices } = require('../lib/detect');

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



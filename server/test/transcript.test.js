'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const {
  textFromMessage,
  isUserPrompt,
  isQuestion,
  extractLatest,
} = require('../lib/transcript');

test('textFromMessage reads a string content', () => {
  assert.equal(textFromMessage({ content: 'hello' }), 'hello');
});

test('textFromMessage joins text blocks and ignores tool_use', () => {
  const msg = { content: [{ type: 'text', text: 'a' }, { type: 'tool_use', name: 'x' }, { type: 'text', text: 'b' }] };
  assert.equal(textFromMessage(msg), 'a\nb');
});

test('isUserPrompt is true for a real prompt, false for a tool_result carrier', () => {
  assert.equal(isUserPrompt({ type: 'user', message: { content: 'hi' } }), true);
  assert.equal(isUserPrompt({ type: 'user', message: { content: [{ type: 'tool_result', content: 'x' }] } }), false);
  assert.equal(isUserPrompt({ type: 'assistant', message: { content: 'x' } }), false);
});

test('isQuestion detects a trailing question mark', () => {
  assert.equal(isQuestion('Want me to proceed?'), true);
  assert.equal(isQuestion('All done.'), false);
});

test('isQuestion detects approval-style prompts without a question mark', () => {
  assert.equal(isQuestion('Which option do you want'), true);
  assert.equal(isQuestion('Should I continue and deploy'), true);
});

test('extractLatest returns the last prompt + following assistant reply', () => {
  const objs = [
    { type: 'user', message: { content: 'ping' } },
    { type: 'assistant', message: { content: 'Pong!' } },
    { type: 'user', message: { content: 'run plan' } },
    { type: 'assistant', message: { content: 'Here is the plan. Approve?' } },
  ];
  const r = extractLatest(objs);
  assert.equal(r.prompt, 'run plan');
  assert.equal(r.reply, 'Here is the plan. Approve?');
  assert.equal(r.assistantCount, 2);
  assert.equal(r.question, true);
  assert.match(r.text, /^> run plan/);
});

test('extractLatest reports no-reply-yet when assistant has not answered', () => {
  const objs = [{ type: 'user', message: { content: 'ping' } }];
  const r = extractLatest(objs);
  assert.equal(r.reply, '');
  assert.equal(r.assistantCount, 0);
  assert.match(r.text, /no text reply yet/);
});

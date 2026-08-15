'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { createRegistry } = require('../lib/sessions');

// stand-in terminal adapter (real one is lib/terminal). The registry only holds
// it — it never calls into it — so a tagged object is enough to prove routing.
const fakeBackend = (tag) => ({ tag });
const includes = (needle) => (e) => e.message.includes(needle);

test('add() with a name stores an entry retrievable by name', () => {
  const r = createRegistry();
  const e = r.add('work', fakeBackend('w'));
  assert.equal(e.name, 'work');
  assert.equal(r.has('work'), true);
  assert.equal(r.get('work').backend.tag, 'w');
});

test('add() without a name uses a generated id (UUID fallback)', () => {
  let n = 0;
  const r = createRegistry({ newId: () => `uuid-${++n}` });
  const e = r.add('', fakeBackend('x'));
  assert.equal(e.name, 'uuid-1');
  assert.equal(r.has('uuid-1'), true);
});

test('add() rejects a duplicate name', () => {
  const r = createRegistry();
  r.add('dup', fakeBackend('a'));
  assert.throws(() => r.add('dup', fakeBackend('b')), includes('already exists'));
});

test('list() exposes names only (no backend leak)', () => {
  const r = createRegistry();
  r.add('a', fakeBackend('a'));
  r.add('b', fakeBackend('b'));
  assert.deepEqual(r.list(), [{ name: 'a' }, { name: 'b' }]);
});

test('remove() drops the session', () => {
  const r = createRegistry();
  r.add('gone', fakeBackend('g'));
  assert.equal(r.remove('gone'), true);
  assert.equal(r.has('gone'), false);
  assert.equal(r.get('gone'), null);
});

test('names() returns the registered names in insertion order', () => {
  const r = createRegistry();
  r.add('one', fakeBackend('1'));
  r.add('two', fakeBackend('2'));
  assert.deepEqual(r.names(), ['one', 'two']);
});

test('get() returns null for an unknown session', () => {
  const r = createRegistry();
  assert.equal(r.get('nope'), null);
});

test('prune() drops entries not in the live list and returns the removed names', () => {
  const r = createRegistry();
  r.add('alive', fakeBackend('a'));
  r.add('dead', fakeBackend('d'));
  r.add('gone', fakeBackend('g'));
  assert.deepEqual(r.prune(['alive']), ['dead', 'gone']);
  assert.deepEqual(r.names(), ['alive']);
});

test('prune() with everything live removes nothing', () => {
  const r = createRegistry();
  r.add('a', fakeBackend('a'));
  assert.deepEqual(r.prune(['a']), []);
  assert.deepEqual(r.names(), ['a']);
});

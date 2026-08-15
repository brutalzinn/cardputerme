'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { pickPort } = require('../lib/discovery');

test('pickPort returns the first free port from the start', async () => {
  const probed = [];
  const isFree = async (p) => { probed.push(p); return p >= 4713; };
  assert.equal(await pickPort(isFree, { start: 4711, tries: 10 }), 4713);
  assert.deepEqual(probed, [4711, 4712, 4713]);
});

test('pickPort returns 0 when no port in range is free', async () => {
  const isFree = async () => false;
  assert.equal(await pickPort(isFree, { start: 4711, tries: 3 }), 0);
});

test('pickPort takes the start port immediately when free', async () => {
  const isFree = async () => true;
  assert.equal(await pickPort(isFree, { start: 4711, tries: 3 }), 4711);
});

test('pickPort defaults to the 8001-8255 exposure range', async () => {
  const probed = [];
  const isFree = async (p) => { probed.push(p); return false; };
  assert.equal(await pickPort(isFree), 0);
  assert.equal(probed[0], 8001);
  assert.equal(probed[probed.length - 1], 8255);
  assert.equal(probed.length, 255);
});


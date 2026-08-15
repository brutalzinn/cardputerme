'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { beaconMessage, BEACON_PORT, BEACON_ADDR, BEACON_INTERVAL_MS } = require('../lib/beacon');

test('beaconMessage carries app, session and websocket port as JSON', () => {
  const msg = JSON.parse(beaconMessage('myproj', 8003));
  assert.deepEqual(msg, { app: 'cardputerme', session: 'myproj', port: 8003 });
});

test('beacon constants: fixed UDP port 8000, subnet broadcast, ~2s cadence', () => {
  assert.equal(BEACON_PORT, 8000);
  assert.equal(BEACON_ADDR, '255.255.255.255');
  assert.equal(BEACON_INTERVAL_MS, 2000);
});

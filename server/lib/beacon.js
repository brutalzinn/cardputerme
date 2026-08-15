'use strict';

const BEACON_PORT = 8000;
const BEACON_ADDR = '255.255.255.255';
const BEACON_INTERVAL_MS = 2000;

function beaconMessage(session, port) {
  return JSON.stringify({ app: 'cardputerme', session, port });
}

module.exports = { beaconMessage, BEACON_PORT, BEACON_ADDR, BEACON_INTERVAL_MS };

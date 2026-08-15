'use strict';

const SHELLS = ['zsh', 'bash', 'sh', 'fish'];

function sessionsToClear(infos, { nowSec, timeoutSec, active }) {
  if (timeoutSec <= 0) return [];
  const out = [];
  for (const s of infos) {
    if (s.name === active) continue;
    if (s.attached > 0) continue;
    if (!SHELLS.includes(s.command)) continue;
    if (nowSec - s.activity < timeoutSec) continue;
    out.push(s.name);
  }
  return out;
}

module.exports = { sessionsToClear, SHELLS };

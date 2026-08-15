'use strict';

const COLORS = {
  text: 0xFFFF,
  prompt: 0xFFE0,
  ask: 0xFD20,
  status: 0x07FF,
};

function lineColor(text, awaiting) {
  if (awaiting) return COLORS.ask;
  if (text.startsWith('> ')) return COLORS.prompt;
  return COLORS.text;
}

function buildDisplay(bodyLines, statusText, opts) {
  const awaiting = !!(opts && opts.awaiting);
  const body = [];
  for (const cell of (bodyLines || [])) {
    if (cell && typeof cell === 'object') {
      body.push({ text: String(cell.text), color: cell.color });
      continue;
    }
    const text = String(cell);
    body.push({ text, color: lineColor(text, awaiting) });
  }
  const status = { text: String(statusText == null ? '' : statusText), color: COLORS.status };
  return { type: 'display', body, status };
}

module.exports = { buildDisplay, COLORS };


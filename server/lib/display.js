'use strict';

// Generic display protocol. The server decides text + color for every line and
// composes a bottom STATUS bar; the device just draws. No semantic roles cross
// the wire — meaning is encoded as color. This is where the old device-side
// "> " prompt inference now lives (server picks the color).
//
// A display message is:
//   { type:'display',
//     body:   [ {text, color} ],   // scrollable body lines
//     status: {text, color} }      // one-line bottom status bar (clipped)
//
// `color` is an explicit RGB565 value (the device's native format) — the device
// needs no color table and never a re-flash to add a color.

const COLORS = {
  text: 0xFFFF,    // white  — normal output
  prompt: 0xFFE0,  // yellow — a "> " user-prompt line
  ask: 0xFD20,     // orange — shown while awaiting a choice
  status: 0x07FF,  // cyan   — the bottom status bar
};

// Decide a body line's color server-side. Only the RESULT (a color) is sent.
function lineColor(text, awaiting) {
  if (awaiting) return COLORS.ask;
  if (text.startsWith('> ')) return COLORS.prompt;
  return COLORS.text;
}

// Pure: body lines + a status string -> a generic display message. A body line
// may be a plain string (server picks the color via the "> "/awaiting rule) OR
// an already-coloured {text,color} object (e.g. mirrored from the terminal's own
// ANSI colours) — used as-is.
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

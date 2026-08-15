'use strict';

// Deterministic ANSI-color parsing — pure, NO regex. Translates the terminal's
// OWN colors (from `tmux capture-pane -e`) into the Cardputer's RGB565, so the
// device shows text in the same colors the terminal already uses (user vs agent,
// etc.) without any color logic of its own — we only translate the color.

// Pack 8-bit r,g,b into RGB565 (the device's native format).
function rgb565(r, g, b) {
  return ((r & 0xf8) << 8) | ((g & 0xfc) << 3) | (b >> 3);
}

const CUBE = [0, 95, 135, 175, 215, 255];

// The 16 system colors (standard xterm) as RGB565.
const SYS16 = [
  [0, 0, 0], [128, 0, 0], [0, 128, 0], [128, 128, 0],
  [0, 0, 128], [128, 0, 128], [0, 128, 128], [192, 192, 192],
  [128, 128, 128], [255, 0, 0], [0, 255, 0], [255, 255, 0],
  [0, 0, 255], [255, 0, 255], [0, 255, 255], [255, 255, 255],
].map(([r, g, b]) => rgb565(r, g, b));

// xterm 256-color index -> RGB565.
function xterm256(n) {
  if (n < 16) return SYS16[n];
  if (n < 232) {
    const i = n - 16;
    return rgb565(CUBE[Math.floor(i / 36) % 6], CUBE[Math.floor(i / 6) % 6], CUBE[i % 6]);
  }
  const level = 8 + (n - 232) * 10;
  return rgb565(level, level, level);
}

// Apply one SGR parameter list to the current fg; return the new fg color.
function applySGR(params, curFg, def) {
  const parts = params.length ? params.split(';') : ['0'];
  let fg = curFg;
  let k = 0;
  while (k < parts.length) {
    const n = parseInt(parts[k] || '0', 10);
    if (n === 0 || n === 39) { fg = def; k += 1; continue; }
    if (n >= 30 && n <= 37) { fg = SYS16[n - 30]; k += 1; continue; }
    if (n >= 90 && n <= 97) { fg = SYS16[n - 90 + 8]; k += 1; continue; }
    if (n === 38) {
      const mode = parseInt(parts[k + 1] || '0', 10);
      if (mode === 5) { fg = xterm256(parseInt(parts[k + 2] || '0', 10)); k += 3; continue; }
      if (mode === 2) { fg = rgb565(parseInt(parts[k + 2] || '0', 10), parseInt(parts[k + 3] || '0', 10), parseInt(parts[k + 4] || '0', 10)); k += 5; continue; }
      k += 1; continue;
    }
    k += 1;  // ignore bold/italic/background/etc.
  }
  return fg;
}

function isFinalByte(c) {
  return c >= '@' && c <= '~';
}

// Parse a line with ANSI escapes -> { text (clean), color (fg at 1st visible char) }.
function parseLine(raw, defaultColor) {
  const s = String(raw == null ? '' : raw);
  let text = '';
  let curFg = defaultColor;
  let lineColor = defaultColor;
  let colorSet = false;
  let i = 0;
  while (i < s.length) {
    const c = s[i];
    if (c === '\x1b' && s[i + 1] === '[') {
      let j = i + 2;
      while (j < s.length && !isFinalByte(s[j])) j += 1;
      if (s[j] === 'm') curFg = applySGR(s.slice(i + 2, j), curFg, defaultColor);
      i = j + 1;
      continue;
    }
    if (c === '\x1b') { i += 1; continue; }   // stray / non-CSI escape
    text += c;
    if (!colorSet && c !== ' ') { lineColor = curFg; colorSet = true; }
    i += 1;
  }
  return { text, color: lineColor };
}

function stripAnsi(s) {
  return parseLine(s, 0).text;
}

module.exports = { rgb565, xterm256, parseLine, stripAnsi };

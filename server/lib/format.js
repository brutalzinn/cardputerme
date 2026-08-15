'use strict';

// Text formatting for the Cardputer's ASCII, monospace, 20-col screen.
// Pure functions — no I/O — so they are unit-testable.

const ANSI = /\x1b\[[0-9;?]*[ -/]*[@-~]/g;

// The Cardputer's built-in font is ASCII-only. Transliterate common
// typography, drop everything else (emoji, box-drawing) that would render
// as garbage. Newlines are preserved.
function toAscii(s) {
  return String(s)
    .replace(/[‘’‛]/g, "'") // ' ' ‛  -> '
    .replace(/[“”]/g, '"')        // " "     -> "
    .replace(/[–—−]/g, '-')  // – — −   -> -
    .replace(/…/g, '...')              // …       -> ...
    .replace(/[•·]/g, '*')        // • ·     -> *
    .replace(/[→⇒]/g, '->')       // → ⇒     -> ->
    .replace(/[←⇐]/g, '<-')       // ← ⇐     -> <-
    .replace(/[✓✔]/g, 'v')        // ✓ ✔     -> v
    .replace(/ /g, ' ')                // nbsp    -> space
    .replace(/[^\x0A\x20-\x7E]/g, '');      // drop remaining non-ASCII
}

// Word-wrap one logical line to `cols`; hard-break words longer than a line.
function wrapLine(line, cols) {
  if (line.length <= cols) return [line];
  const out = [];
  const words = line.split(/\s+/);
  let cur = '';
  for (let word of words) {
    while (word.length > cols) {
      if (cur) { out.push(cur); cur = ''; }
      out.push(word.slice(0, cols));
      word = word.slice(cols);
    }
    if (!cur) { cur = word; continue; }
    if (cur.length + 1 + word.length <= cols) { cur += ' ' + word; continue; }
    out.push(cur);
    cur = word;
  }
  if (cur) out.push(cur);
  return out.length ? out : [''];
}

// Turn arbitrary text into cards (array of arrays of <=linesPerCard lines).
// maxCards keeps the newest cards (the tail = what the user just saw).
function sliceIntoCards(text, cols, linesPerCard, maxCards = 40) {
  const cleaned = toAscii(String(text).replace(ANSI, '').replace(/\r/g, ''));
  let rawLines = cleaned.split('\n');
  while (rawLines.length && rawLines[rawLines.length - 1].trim() === '') rawLines.pop();

  const wrapped = [];
  for (const line of rawLines) {
    for (const w of wrapLine(line.replace(/\t/g, '  '), cols)) {
      wrapped.push(w.replace(/\s+$/, '')); // trim trailing space for clean display
    }
  }

  const cards = [];
  for (let i = 0; i < wrapped.length; i += linesPerCard) {
    cards.push(wrapped.slice(i, i + linesPerCard));
  }
  if (cards.length > maxCards) return cards.slice(cards.length - maxCards);
  return cards;
}

module.exports = { toAscii, wrapLine, sliceIntoCards };

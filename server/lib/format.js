'use strict';

const ANSI = /\x1b\[[0-9;?]*[ -/]*[@-~]/g;

function toAscii(s) {
  return String(s)
    .split('\t').join('  ')
    .replace(/[‘’‛]/g, "'")
    .replace(/[“”]/g, '"')
    .replace(/[–—−]/g, '-')
    .replace(/…/g, '...')
    .replace(/[•·]/g, '*')
    .replace(/[❯▶›]/g, '>')
    .replace(/[→⇒]/g, '->')
    .replace(/[←⇐]/g, '<-')
    .replace(/[✓✔]/g, 'v')
    .replace(/ /g, ' ')
    .replace(/[^\x0A\x20-\x7E]/g, '');
}

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

function sliceIntoCards(text, cols, linesPerCard, maxCards = 40) {
  const cleaned = toAscii(String(text).replace(ANSI, '').replace(/\r/g, ''));
  let rawLines = cleaned.split('\n');
  while (rawLines.length && rawLines[rawLines.length - 1].trim() === '') rawLines.pop();

  const wrapped = [];
  for (const line of rawLines) {
    for (const w of wrapLine(line, cols)) {
      wrapped.push(w.replace(/\s+$/, ''));
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


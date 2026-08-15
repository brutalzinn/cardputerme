'use strict';

function endsWithQuestion(text) {
  const t = String(text == null ? '' : text).trimEnd();
  return t.length > 0 && t[t.length - 1] === '?';
}

function isDigit(c) {
  return c >= '0' && c <= '9';
}

function parseChoices(text) {
  const out = [];
  for (const raw of String(text == null ? '' : text).split('\n')) {
    const line = raw.trim();
    let i = 0;
    while (i < line.length && isDigit(line[i])) i++;
    if (i === 0) continue;
    if (i >= line.length) continue;
    const sep = line[i];
    if (sep !== '.' && sep !== ')') continue;
    let j = i + 1;
    if (j >= line.length || line[j] !== ' ') continue;
    while (j < line.length && line[j] === ' ') j++;
    const label = line.slice(j).trim();
    if (!label) continue;
    out.push({ n: parseInt(line.slice(0, i), 10), label });
  }
  return out;
}

module.exports = { endsWithQuestion, parseChoices };


'use strict';

function trimEnd(s) {
  let e = s.length;
  while (e > 0) {
    const c = s[e - 1];
    const isSpace = c === ' ' || c === '\n' || c === '\t' || c === '\r';
    if (!isSpace) break;
    e--;
  }
  return s.slice(0, e);
}

function endsWithQuestion(text) {
  const t = trimEnd(String(text == null ? '' : text));
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

function detectChoice(text) {
  const options = parseChoices(text);
  const question = endsWithQuestion(text);
  return { awaiting: options.length >= 2 || question, question, options };
}

module.exports = { endsWithQuestion, parseChoices, detectChoice, trimEnd };


'use strict';

// Agnostic question detection — pure, deterministic, no regex, no hooks, no
// model call. cardputerme is a generic terminal remote; it must not depend on
// tmux internals or Claude Code's transcript/hook format. So it decides "is the
// program waiting for me?" from the visible text alone, with two simple rules:
//
//   1. the text ends with '?'            -> a question
//   2. it lists 2+ numbered options      -> a menu to choose from
//
// Either one means the user is being asked to answer. Nothing here matches
// syntax patterns; it scans characters directly (see memory: no-regex).

// Trim trailing whitespace WITHOUT a regex.
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

// Rule 1: last non-space character is a question mark.
function endsWithQuestion(text) {
  const t = trimEnd(String(text == null ? '' : text));
  return t.length > 0 && t[t.length - 1] === '?';
}

function isDigit(c) {
  return c >= '0' && c <= '9';
}

// Rule 2: parse "N. label" / "N) label" option lines by scanning characters.
// A line qualifies when it is: [spaces] digits ('.'|')') space+ label.
function parseChoices(text) {
  const out = [];
  for (const raw of String(text == null ? '' : text).split('\n')) {
    const line = raw.trim();
    let i = 0;
    while (i < line.length && isDigit(line[i])) i++;
    if (i === 0) continue;                       // no leading number
    if (i >= line.length) continue;              // number with nothing after
    const sep = line[i];
    if (sep !== '.' && sep !== ')') continue;    // needs a '.'/')' separator
    let j = i + 1;
    if (j >= line.length || line[j] !== ' ') continue; // needs a space after it
    while (j < line.length && line[j] === ' ') j++;
    const label = line.slice(j).trim();
    if (!label) continue;                        // needs a non-empty label
    out.push({ n: parseInt(line.slice(0, i), 10), label });
  }
  return out;
}

// The combined signal the bridge acts on. `awaiting` = the user needs to answer.
function detectChoice(text) {
  const options = parseChoices(text);
  const question = endsWithQuestion(text);
  return { awaiting: options.length >= 2 || question, question, options };
}

module.exports = { endsWithQuestion, parseChoices, detectChoice, trimEnd };

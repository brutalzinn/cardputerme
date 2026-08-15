'use strict';

// Pure interaction FSM — the LAZY-DEVICE contract. The device forwards every raw
// key; the SERVER owns the input buffer, all special functions and what reaches
// the terminal. No regex, no else.
//
// State: { mode: 'mirror' | 'picker', input: string }
// Keys:  single chars, plus named: enter, shift+enter, backspace, esc,
//        up, down, left, right.
//
//   mirror:
//     char        -> append to the input buffer (digit answers a menu when
//                    awaiting and the buffer is empty)
//     backspace   -> delete last char
//     shift+enter -> append '\n' (compose multi-line)
//     enter       -> send the buffer (empty buffer -> press Enter in terminal)
//     esc         -> typing? CLEAR the input : open the session picker
//     arrows      -> pan the viewport (input kept)
//   picker:
//     number      -> select that session, back to mirror
//     esc         -> cancel back to mirror
//     anything else ignored

function isDigits(text) {
  if (!text) return false;
  for (const c of text) {
    if (c < '0' || c > '9') return false;
  }
  return true;
}

// 1-based menu index within [1, n], or 0 if not a valid pick.
function pickIndex(text, n) {
  if (!isDigits(text)) return 0;
  const k = parseInt(text, 10);
  if (k >= 1 && k <= n) return k;
  return 0;
}

const ARROWS = new Set(['up', 'down', 'left', 'right']);
// tmux named-key spelling for each arrow (sent INTO the terminal when a prompt
// selector is up — e.g. Claude Code's AskUserQuestion panel).
const ARROW_NAME = { up: 'Up', down: 'Down', left: 'Left', right: 'Right' };

function interpretKey(state, key, ctx) {
  const mode = (state && state.mode) || 'mirror';
  const input = (state && state.input) || '';
  const k = String(key == null ? '' : key);
  const sessions = (ctx && ctx.sessions) || [];
  const awaiting = !!(ctx && ctx.awaiting);

  if (mode === 'picker') {
    if (k === 'esc') return { state: { mode: 'mirror', input }, action: { kind: 'closePicker' } };
    const n = pickIndex(k, sessions.length);
    if (n > 0) return { state: { mode: 'mirror', input }, action: { kind: 'select', name: sessions[n - 1] } };
    return { state, action: { kind: 'none' } };
  }

  // mirror
  if (k === 'esc') {
    if (input.length > 0) return { state: { mode: 'mirror', input: '' }, action: { kind: 'none' } };
    return { state: { mode: 'picker', input }, action: { kind: 'openPicker' } };
  }
  if (k === 'enter') {
    if (input.length > 0) return { state: { mode: 'mirror', input: '' }, action: { kind: 'send', text: input } };
    return { state, action: { kind: 'pressEnter' } };
  }
  if (k === 'shift+enter') return { state: { mode: 'mirror', input: input + '\n' }, action: { kind: 'none' } };
  if (k === 'backspace') return { state: { mode: 'mirror', input: input.slice(0, -1) }, action: { kind: 'none' } };
  if (k === 'tab') return { state, action: { kind: 'pressTab' } };
  if (ARROWS.has(k)) {
    // A prompt selector is on screen and the user isn't typing: up/down DRIVE it;
    // left/right keep panning so wide options stay readable.
    const vertical = k === 'up' || k === 'down';
    if (awaiting && vertical && input.length === 0) return { state, action: { kind: 'sendArrow', key: ARROW_NAME[k] } };
    return { state, action: { kind: 'pan', key: k } };
  }
  if (k.length === 1) {
    if (awaiting && input.length === 0 && isDigits(k)) return { state, action: { kind: 'answerMenu', key: k } };
    return { state: { mode: 'mirror', input: input + k }, action: { kind: 'none' } };
  }
  return { state, action: { kind: 'none' } };
}

module.exports = { interpretKey, isDigits, pickIndex };

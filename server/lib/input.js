'use strict';

// Pure interaction FSM. The server interprets EVERY key the device forwards (as
// {type:'cmd',text}); the device special-cases nothing, so NO firmware change is
// needed. Backtick is the Cardputer's Esc button. No regex, no else.
//
//   mirror mode (default): a terminal session is on screen.
//     `        -> send a real Escape to the terminal (never typed)
//     `` (2+)  -> open the session picker
//     digit while awaiting a menu -> answer that menu
//     anything else -> type it into the terminal
//   picker mode: the numbered session menu is on screen.
//     N        -> select session N, back to mirror
//     `        -> cancel, back to mirror
//     anything else -> ignored (stay in picker)

function isAllBackticks(text) {
  if (!text) return false;
  for (const c of text) {
    if (c !== '`') return false;
  }
  return true;
}

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

function interpretKey(state, rawText, ctx) {
  const mode = (state && state.mode) || 'mirror';
  const text = String(rawText == null ? '' : rawText).trim();
  const sessions = (ctx && ctx.sessions) || [];
  const awaiting = !!(ctx && ctx.awaiting);

  if (mode === 'picker') {
    if (isAllBackticks(text)) return { state: { mode: 'mirror' }, action: { kind: 'closePicker' } };
    const k = pickIndex(text, sessions.length);
    if (k > 0) return { state: { mode: 'mirror' }, action: { kind: 'select', name: sessions[k - 1] } };
    return { state: { mode: 'picker' }, action: { kind: 'none' } };
  }

  // mirror mode
  if (isAllBackticks(text)) {
    if (text.length >= 2) return { state: { mode: 'picker' }, action: { kind: 'openPicker' } };
    return { state: { mode: 'mirror' }, action: { kind: 'escape' } };
  }
  if (awaiting && text.length === 1 && isDigits(text)) {
    return { state: { mode: 'mirror' }, action: { kind: 'answerMenu', key: text } };
  }
  return { state: { mode: 'mirror' }, action: { kind: 'type', text } };
}

module.exports = { interpretKey, isAllBackticks, isDigits, pickIndex };

'use strict';

// Pure interaction FSM — the LAZY-DEVICE contract. The device forwards every raw
// key; the SERVER owns the input buffer, all special functions and what reaches
// the terminal. Keys and actions use GENERIC names only — no backend (tmux)
// spelling here; the terminal adapter translates. No regex, no else.
//
// State: { mode: 'mirror', input: string, hist: number|null }
// Keys:  single chars, arrows (up/down/left/right), enter, backspace, tab, esc,
//        and modifier chords as '<mod>+<base>' (shift+enter, shift+esc,
//        ctrl+<char>, ctrl+up/down, opt+<arrow>).
//
// Actions: none | send{text} | pressKey{key} | pan{key}
//
//   char        -> append to the input buffer (digit answers a menu when
//                  awaiting and the buffer is empty -> pressKey digit)
//   backspace   -> delete last char
//   shift+enter -> append '\n' (compose multi-line)
//   enter       -> send the buffer (empty buffer -> pressKey enter)
//   tab         -> pressKey tab (selector auto-advance / completion)
//   esc         -> typing? CLEAR the input : real Escape into the terminal
//   shift+esc   -> pressKey escape (STOP the agent; draft kept)
//   ctrl+up/dn  -> history recall prev / next (shell idiom)
//   ctrl+<char> -> pressKey ctrl+<char> (Ctrl+C & co.)
//   arrows      -> pan (read around; numbers choose menu options)
//   opt+<arrow> -> pressKey arrow (drive the terminal's own selector)

function isDigits(text) {
  if (!text) return false;
  for (const c of text) {
    if (c < '0' || c > '9') return false;
  }
  return true;
}

const ARROWS = new Set(['up', 'down', 'left', 'right']);

// Split '<mod>+<base>' once — the single modifier parse for the whole FSM.
function splitMod(k) {
  const i = k.indexOf('+');
  if (i <= 0) return { mod: '', base: k };
  return { mod: k.slice(0, i), base: k.slice(i + 1) };
}

const press = (state, key) => ({ state, action: { kind: 'pressKey', key } });
const quiet = (state) => ({ state, action: { kind: 'none' } });

function interpretKey(state, key, ctx) {
  const input = (state && state.input) || '';
  const hist = state && typeof state.hist === 'number' ? state.hist : null;
  const k = String(key == null ? '' : key);
  const awaiting = !!(ctx && ctx.awaiting);
  const history = (ctx && ctx.history) || [];
  const mirror = (newInput, newHist = null) => ({ mode: 'mirror', input: newInput, hist: newHist });

  // Parse any modifier chord exactly once.
  const { mod, base } = splitMod(k);

  if (mod === 'shift') {
    if (base === 'esc') return press(state, 'escape');                    // STOP the agent; draft kept
    if (base === 'enter') return quiet(mirror(input + '\n', hist));      // compose multi-line
    return quiet(state);
  }
  if (mod === 'ctrl') {
    if (base === 'up') {                                                  // history prev (shell idiom)
      if (history.length === 0) return quiet(state);
      const idx = hist === null ? history.length - 1 : Math.max(0, hist - 1);
      return quiet(mirror(history[idx], idx));
    }
    if (base === 'down') {                                                // history next
      if (hist === null) return quiet(state);
      const idx = hist + 1;
      if (idx >= history.length) return quiet(mirror(''));
      return quiet(mirror(history[idx], idx));
    }
    if (base.length === 1) return press(state, 'ctrl+' + base);           // Ctrl+C & co.
    return quiet(state);
  }
  if (mod === 'opt') {
    if (ARROWS.has(base)) return press(state, base);
    return quiet(state);
  }

  if (k === 'esc') {
    if (input.length > 0) return quiet(mirror(''));                       // clear (also drops recall)
    return press(state, 'escape');                                        // real Escape (ssh-parity)
  }
  if (k === 'enter') {
    if (input.length > 0) return { state: mirror(''), action: { kind: 'send', text: input } };
    return press(state, 'enter');
  }
  if (k === 'backspace') return quiet(mirror(input.slice(0, -1), hist));
  if (k === 'tab') return press(state, 'tab');
  if (ARROWS.has(k)) return { state, action: { kind: 'pan', key: k } };
  if (k.length === 1) {
    if (awaiting && input.length === 0 && isDigits(k)) return press(state, k);   // answer a menu
    return quiet(mirror(input + k, hist));
  }
  return quiet(state);
}

module.exports = { interpretKey, isDigits };

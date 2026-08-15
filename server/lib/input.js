'use strict';

function isDigits(text) {
  if (!text) return false;
  for (const c of text) {
    if (c < '0' || c > '9') return false;
  }
  return true;
}

const ARROWS = new Set(['up', 'down', 'left', 'right']);

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

  const { mod, base } = splitMod(k);

  if (mod === 'shift') {
    if (base === 'esc') return press(state, 'escape');
    if (base === 'enter') return quiet(mirror(input + '\n', hist));
    return quiet(state);
  }
  if (mod === 'ctrl') {
    if (base === 'up') {
      if (history.length === 0) return quiet(state);
      const idx = hist === null ? history.length - 1 : Math.max(0, hist - 1);
      return quiet(mirror(history[idx], idx));
    }
    if (base === 'down') {
      if (hist === null) return quiet(state);
      const idx = hist + 1;
      if (idx >= history.length) return quiet(mirror(''));
      return quiet(mirror(history[idx], idx));
    }
    if (base.length === 1) return press(state, 'ctrl+' + base);
    return quiet(state);
  }
  if (mod === 'opt') {
    if (ARROWS.has(base)) return press(state, base);
    return quiet(state);
  }

  if (k === 'esc') {
    if (input.length > 0) return quiet(mirror(''));
    return press(state, 'escape');
  }
  if (k === 'enter') {
    if (input.length > 0) return { state: mirror(''), action: { kind: 'send', text: input } };
    return press(state, 'enter');
  }
  if (k === 'backspace') return quiet(mirror(input.slice(0, -1), hist));
  if (k === 'tab') return press(state, 'tab');
  if (ARROWS.has(k)) return { state, action: { kind: 'pan', key: k } };
  if (k.length === 1) {
    if (awaiting && input.length === 0 && isDigits(k)) return press(state, k);
    return quiet(mirror(input + k, hist));
  }
  return quiet(state);
}

module.exports = { interpretKey, isDigits };


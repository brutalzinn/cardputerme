'use strict';

/*
 * cardputerme — a generic terminal remote for the M5Cardputer.
 *
 * ONE server exposes MANY named terminal sessions over ONE WebSocket. Fully
 * agnostic: no Claude, no hooks, no regex, no modes. It doesn't matter what runs
 * in a session (bash, an agent, anything).
 *   READ : mirror the session's screen (terminal adapter, lib/terminal).
 *   WRITE: inject keypresses/text (adapter sendText/sendKey).
 *   DETECT: a "choose one" prompt via a pure algorithm (lib/detect) — the device
 *           always shows the question and beeps.
 * The device is a thin renderer: it lists sessions, picks one, renders cards,
 * and sends keys. The server decides everything shown.
 *
 * HTTP routes (/health, /sessions, /select, /cards, /command) are for debugging.
 */

const fs = require('fs');
const path = require('path');
const http = require('http');
const express = require('express');
const { WebSocketServer } = require('ws');
const { sliceIntoCards, toAscii, wrapLine } = require('./lib/format');
const { parseChoices, endsWithQuestion, trimEnd } = require('./lib/detect');
const { tmuxBackend } = require('./lib/terminal');
const { pickPort, freePortProbe } = require('./lib/discovery');
const { beaconMessage, BEACON_PORT, BEACON_ADDR, BEACON_INTERVAL_MS } = require('./lib/beacon');
const net = require('net');
const dgram = require('dgram');
const { buildDisplay, COLORS } = require('./lib/display');
const { interpretKey } = require('./lib/input');
const { parseLine, stripAnsi } = require('./lib/ansi');
const { panViewport, windowLines, anchorRow } = require('./lib/viewport');

// ---- tiny .env loader (no dependency) --------------------------------------
(function loadEnv() {
  try {
    const p = path.join(__dirname, '.env');
    if (!fs.existsSync(p)) return;
    for (const line of fs.readFileSync(p, 'utf8').split('\n')) {
      const s = line.trim();
      if (!s || s.startsWith('#') || !s.includes('=')) continue;
      const i = s.indexOf('=');
      const k = s.slice(0, i).trim();
      let v = s.slice(i + 1).trim();
      if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) v = v.slice(1, -1);
      if (!(k in process.env)) process.env[k] = v;
    }
  } catch { /* ignore */ }
})();

// ---- config ----------------------------------------------------------------
const PORT_START = 8001;
const PORT_TRIES = 255;
// Optional default session to pre-select. The user need NOT know any session
// name — sessions are auto-discovered and picked on the device. Empty = none.
const NAME = process.env.SESSION || 'cardputerme';
const SESSION_CWD = process.env.SESSION_CWD || '';
const WRAP_COLS = parseInt(process.env.WRAP_COLS || '20', 10);
const LINES_PER_CARD = parseInt(process.env.LINES_PER_CARD || '7', 10);
const SCROLLBACK_LINES = parseInt(process.env.SCROLLBACK_LINES || '200', 10);
const MAX_CARDS = parseInt(process.env.MAX_CARDS || '40', 10);
const NOTIFY = (process.env.NOTIFY || '1') !== '0';

// ---- the ONE terminal this server exposes -----------------------------------
// Server-per-exposure: `cardputerme [name]` spawns this server for exactly one
// terminal. tmux is today's backend; a PTY drops in with the same interface.
const terminal = tmuxBackend({ session: NAME, scrollbackLines: SCROLLBACK_LINES });
// UI interaction state, driven entirely server-side by the input FSM (lib/input).
// `input` is the SERVER-owned compose buffer (the lazy device only forwards keys).
let uiState = { mode: 'mirror', input: '', hist: null };
// Sent-command history (newest last) — recalled with ctrl+;/ctrl+. like a shell.
const history = [];
const HISTORY_MAX = 50;

// Server-owned omnidirectional viewport over the terminal's native-width grid.
// The device forwards arrow keys; the server pans and sends only this window.
const VIEW_ROWS = 6;             // fits the device body between header + status bar
const VIEW_COLS = WRAP_COLS;     // device columns
const PROMPT_TAIL_ROWS = 16;     // prompt detection scans ONLY the pane's tail
let view = { row: 0, col: 0, follow: true, selRow: -1 };

function clamp(v, lo, hi) {
  if (v < lo) return lo;
  if (v > hi) return hi;
  return v;
}

function activeTerminal() {
  return terminal;
}

// ---- build cards -----------------------------------------------------------
const NO_SESSION = 'Terminal is gone.\nRun cardputerme on\nthe computer to\nexpose it again.';

// Reinterpret the terminal screen for the Cardputer: find a "choose one" prompt
// (create file? run plan? proceed?) that lives in the TUI, not the transcript.
// Pure algorithm over the captured text — NO regex, NO hooks, agnostic to what
// program is running. An option list is 2+ "N. label" / "N) label" lines
// (lib/detect.parseChoices); the question is the line above them ending in '?'.
// We ALWAYS include that question line, so the device shows the question, not
// just the bare options.
function detectPrompt(pane) {
  if (!pane) return null;
  // ASCII-clean first so the `❯` pointer / box chars don't hide "1. Yes".
  const lines = toAscii(pane).split('\n').map((l) => trimEnd(l));
  const optIdx = [];
  for (let i = 0; i < lines.length; i++) if (parseChoices(lines[i]).length > 0) optIdx.push(i);
  if (optIdx.length < 2) return null;               // need a real option list
  let start = optIdx[0];
  // Walk up to the question: prefer the nearest line ending in '?', else the
  // nearest non-empty line. Either way the device always shows the question.
  for (let i = start - 1; i >= 0 && i >= start - 6; i--) {
    if (!lines[i].trim()) continue;
    start = i;
    if (endsWithQuestion(lines[i])) break;          // reached the question line
  }
  let end = optIdx[optIdx.length - 1];
  // Include a trailing hint line (esc / enter / cancel / tab) if present.
  const hint = lines[end + 1] ? lines[end + 1].toLowerCase() : '';
  if (hint.includes('esc') || hint.includes('enter') || hint.includes('cancel') || hint.includes('tab')) end += 1;
  const text = lines.slice(start, end + 1).filter((l) => l.trim()).join('\n');
  return text.trim() ? text : null;
}

// Flatten wrapped text into a capped list of screen lines (newest tail kept) —
// small enough for the device's WS buffer (large frames crash the ESP32).
const MAX_LINES = 90;
function screenLines(text) {
  const lines = [].concat(...sliceIntoCards(text, WRAP_COLS, LINES_PER_CARD, MAX_CARDS));
  if (lines.length > MAX_LINES) return lines.slice(lines.length - MAX_LINES);
  return lines;
}

// Turn ANSI-coloured pane rows into a NATIVE-WIDTH coloured grid (no wrapping) —
// the viewport pans over this so the layout stays terminal-like. Each row keeps
// its terminal colour (mirrored, translated to RGB565).
function gridLines(rows) {
  const out = [];
  for (const raw of rows) {
    const { text, color } = parseLine(raw, COLORS.text);
    out.push({ text: toAscii(text).split('\t').join('  '), color });
  }
  if (out.length > MAX_LINES) return out.slice(out.length - MAX_LINES);
  return out;
}

// Split a captured (ANSI) pane into a coloured grid + a one-line STATUS (the
// terminal's bottom status row = its last non-empty line, like Claude Code's bar).
function splitScreen(pane) {
  const rows = String(pane || '').split('\n');
  let last = -1;
  for (let i = rows.length - 1; i >= 0; i--) {
    if (stripAnsi(rows[i]).trim()) { last = i; break; }
  }
  if (last < 0) return { grid: [], status: '' };
  const status = toAscii(stripAnsi(rows[last])).trim();
  return { grid: gridLines(rows.slice(0, last)), status };
}

// Build the current screen: body lines + a status string. NO modes/transcript;
// same generic path for every CLI. The device renders this via lib/display.
async function buildState() {
  const term = activeTerminal();
  if (!(await term.exists())) {
    return { lines: screenLines(NO_SESSION), status: 'terminal gone', sessionExists: false, awaiting: false };
  }
  const pane = await term.capture();          // includes ANSI colour escapes (-e)

  // Prompt detection — TAIL ONLY, and it never takes over the screen. A real
  // interactive prompt sits at the BOTTOM of a terminal; numbered lists in older
  // output must not trigger it (false positives). `awaiting` only means: beep,
  // status hint, and a lone digit answers — the mirror itself stays untouched,
  // with the terminal's own colours. Nice, unbreakable Claude Code integration.
  const plain = stripAnsi(pane);
  const tail = plain.split('\n').slice(-PROMPT_TAIL_ROWS).join('\n');
  const awaiting = detectPrompt(tail) !== null;

  // Mirror: a native-width coloured grid + the status. Cache it so buffer-only
  // keystrokes can re-render instantly without another capture (typing fast-path).
  const { grid, status } = splitScreen(pane);
  cachedMirror = { grid, status, awaiting };
  return composeMirror(grid, status, awaiting);
}

// Compose the mirror display from a grid + status: apply the server-owned
// viewport (pan/follow) and render the compose buffer. Pure w.r.t. the pane —
// usable from the cache for instant keystroke echo.
let cachedMirror = null;

// The selector row of an on-screen menu: the line whose text starts with the
// selection pointer ('>' — toAscii maps ❯/▶ to it) followed by a numbered
// option. Lets the viewport TRACK the highlight while the user navigates.
function findSelectorRow(grid) {
  for (let i = grid.length - 1; i >= 0; i--) {
    const t = grid[i].text.trim();
    if (t.startsWith('>') && parseChoices(t.slice(1)).length > 0) return i;
  }
  return -1;
}

function composeMirror(grid, status, awaiting) {
  let maxLen = 0;
  for (const l of grid) { if (l.text.length > maxLen) maxLen = l.text.length; }
  const maxRow = Math.max(0, grid.length - VIEW_ROWS);
  const maxCol = Math.max(0, maxLen - VIEW_COLS);
  const selRow = awaiting ? findSelectorRow(grid) : -1;
  const selMoved = selRow >= 0 && selRow !== view.selRow;
  view.selRow = selRow;
  if (selMoved) { view.row = anchorRow(selRow, VIEW_ROWS); view.follow = false; }
  if (selRow < 0 && view.follow) view.row = maxRow;
  view.row = clamp(view.row, 0, maxRow);
  view.col = clamp(view.col, 0, maxCol);
  if (selRow < 0 && view.row >= maxRow) view.follow = true;   // back at the bottom -> follow again
  const lines = windowLines(grid, view, { rows: VIEW_ROWS, cols: VIEW_COLS });
  // The SERVER-owned compose buffer, rendered into the display (lazy device):
  // the last wrapped pieces of what the user is typing, in the prompt colour.
  if (uiState.input.length > 0) {
    const composed = wrapLine('> ' + uiState.input.split('\n').join(' | '), VIEW_COLS);
    for (const piece of composed.slice(-2)) lines.push({ text: piece, color: COLORS.prompt });
  }
  const hint = awaiting ? 'PROMPT: press a number' : status;
  const bar = `${hint}  r${view.row}/${maxRow} c${view.col}`;
  return { lines: lines.length ? lines : screenLines('(empty)'), status: `[${NAME}] ${bar}`, sessionExists: true, awaiting };
}

// ---- HTTP (debugging only) -------------------------------------------------
const app = express();
app.use(express.json({ limit: '64kb' }));
app.get('/health', async (_req, res) => {
  res.json({ ok: true, name: NAME, exists: await activeTerminal().exists(), notify: NOTIFY, awaiting: lastAwaiting });
});
app.get('/cards', async (_req, res) => {
  const st = await buildState();
  res.json({ lines: st.lines.length, status: st.status, sessionExists: st.sessionExists, awaiting: st.awaiting, body: st.lines });
});
app.post('/command', async (req, res) => {
  const text = req.body && typeof req.body.text === 'string' ? req.body.text : '';
  if (!text) return res.status(400).json({ ok: false, error: 'missing text' });
  const term = activeTerminal();
  if (!(await term.exists())) return res.status(409).json({ ok: false, error: 'no active session' });
  res.json({ ok: await term.sendText(text), sent: text });
});

// ---- WebSocket (the device's only channel) ---------------------------------
const server = http.createServer(app);
const wss = new WebSocketServer({ server, path: '/ws' });

let lastSig = '';
let lastAwaiting = false;

// The generic display message (lib/display): body lines with server-chosen
// colors + a bottom status bar. This is the ONLY screen message the device
// renders — kept screen-sized so it never overloads the ESP32's WS buffer.
function displayMessage(st) {
  return JSON.stringify(buildDisplay(st.lines, st.status, { awaiting: st.awaiting }));
}
function broadcast(str) {
  for (const c of wss.clients) if (c.readyState === 1) c.send(str);
}

// Push the screen when it changes; beep once when a choose-one prompt appears.
async function pushIfChanged(force) {
  const st = await buildState();
  const sig = JSON.stringify(st.lines) + '' + st.status;
  if (sig !== lastSig || force) {
    lastSig = sig;
    broadcast(displayMessage(st));
  }
  const freshQuestion = st.awaiting && !lastAwaiting; // a choose-one prompt appeared on screen
  if (NOTIFY && freshQuestion) broadcast(JSON.stringify({ type: 'notify', reason: 'question' }));
  if (!st.awaiting && lastAwaiting) view.follow = true; // prompt answered -> resume following output
  lastAwaiting = st.awaiting;
}

// Route ONE raw key through the pure FSM (lib/input) and execute its action.
// This is the whole lazy-device contract: the server owns the input buffer and
// every special function; the device only reported a keypress.
async function applyKey(key) {
  const { state, action } = interpretKey(uiState, key, { awaiting: lastAwaiting, history });
  uiState = state;
  if (action.kind === 'pan') view = Object.assign({}, view, panViewport(view, action.key));
  if (action.kind === 'send') {
    history.push(action.text);                                   // shell-style recall (ctrl+up)
    if (history.length > HISTORY_MAX) history.shift();
    view.follow = true;
    await activeTerminal().sendText(action.text);                // sending re-sticks auto-scroll
  }
  // ONE keypress path: the FSM emits a GENERIC key; the adapter spells it.
  if (action.kind === 'pressKey') {
    view.follow = true;
    await activeTerminal().sendKey(action.key);
  }
  // Fast-path: buffer-only changes AND pans re-render from the cached grid —
  // instant echo, no capture-pane per keystroke (pans are the hottest keys).
  const cacheOnly = action.kind === 'none' || action.kind === 'pan';
  if (cacheOnly && uiState.mode === 'mirror' && cachedMirror) {
    broadcast(displayMessage(composeMirror(cachedMirror.grid, cachedMirror.status, cachedMirror.awaiting)));
    return;
  }
  const touchedTerminal = action.kind === 'send' || action.kind === 'pressKey';
  setTimeout(() => pushIfChanged(true).catch(() => {}), touchedTerminal ? 250 : 0);
}

wss.on('connection', async (ws, req) => {
  const who = (req && req.socket && req.socket.remoteAddress) || '?';
  console.log(`[ws] connect ${who} (clients=${wss.clients.size})`);
  ws.on('close', (code, reason) => console.log(`[ws] close ${who} code=${code} ${String(reason || '')}`));
  ws.on('error', (e) => console.log(`[ws] error ${who} ${e && e.message}`));
  // Send the current screen immediately to the newcomer.
  const st = await buildState();
  ws.send(displayMessage(st));

  ws.on('message', async (data) => {
    let m; try { m = JSON.parse(data.toString()); } catch { return; }
    if (!m || typeof m.type !== 'string') return;

    // Every raw key goes through the FSM (lazy device: the server owns input).
    if (m.type === 'key' && typeof m.key === 'string') {
      await applyKey(m.key);
      return;
    }
    // Legacy whole-text command (older firmware's local input mode): feed each
    // char through the FSM, then press enter — same one path, no special case.
    if (m.type === 'cmd' && typeof m.text === 'string') {
      for (const c of m.text) await applyKey(c === '\n' ? 'shift+enter' : c);
      await applyKey('enter');
      return;
    }
  });
});

setInterval(async () => {
  if (await terminal.exists()) return;
  console.log(`[expose] terminal '${NAME}' is gone — shutting down`);
  process.exit(0);
}, 60000);

// Server-side tick (NOT device polling): catches screen changes that emit no
// event — prompts, menus, live streaming. The Cardputer just receives pushes.
setInterval(() => { if (wss.clients.size) pushIfChanged(false).catch(() => {}); }, 800);
// Keep sockets alive with a periodic ping.
setInterval(() => { for (const c of wss.clients) if (c.readyState === 1) c.ping(); }, 20000);

function startBeacon(port) {
  const sock = dgram.createSocket('udp4');
  sock.on('error', () => {});
  sock.bind(() => sock.setBroadcast(true));
  const msg = Buffer.from(beaconMessage(NAME, port));
  setInterval(() => sock.send(msg, 0, msg.length, BEACON_PORT, BEACON_ADDR), BEACON_INTERVAL_MS);
  console.log(`  beacon : udp ${BEACON_ADDR}:${BEACON_PORT} every ${BEACON_INTERVAL_MS}ms`);
}

async function start() {
  const port = await pickPort(freePortProbe(net), { start: PORT_START, tries: PORT_TRIES });
  if (port === 0) {
    console.log(`cardputerme — no free port between ${PORT_START} and ${PORT_START + PORT_TRIES - 1}`);
    process.exit(1);
  }
  server.listen(port, '0.0.0.0', async () => {
    console.log(`cardputerme — exposing '${NAME}' on http://0.0.0.0:${port}  (ws://…/ws)`);
    const created = await terminal.ensureSession(SESSION_CWD);
    if (!created) console.log(`  ! could not create terminal '${NAME}'`);
    startBeacon(port);
  });
}
start();

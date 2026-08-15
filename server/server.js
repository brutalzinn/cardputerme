'use strict';

const fs = require('fs');
const path = require('path');
const http = require('http');
const express = require('express');
const { WebSocketServer } = require('ws');
const { sliceIntoCards, toAscii, wrapLine } = require('./lib/format');
const { parseChoices, endsWithQuestion, trimEnd } = require('./lib/detect');
const { createBackend } = require('./lib/terminal');
const { pickPort, freePortProbe } = require('./lib/discovery');
const { beaconMessage, BEACON_PORT, BEACON_ADDR, BEACON_INTERVAL_MS } = require('./lib/beacon');
const net = require('net');
const dgram = require('dgram');
const { buildDisplay, COLORS } = require('./lib/display');
const { interpretKey } = require('./lib/input');
const { parseLine, stripAnsi } = require('./lib/ansi');
const { panViewport, windowLines, anchorRow } = require('./lib/viewport');

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
  } catch {  }
})();

const PORT_START = 8001;
const PORT_TRIES = 255;

const NAME = process.env.SESSION || 'cardputerme';
const SESSION_CWD = process.env.SESSION_CWD || '';
const WRAP_COLS = parseInt(process.env.WRAP_COLS || '20', 10);
const LINES_PER_CARD = parseInt(process.env.LINES_PER_CARD || '7', 10);
const SCROLLBACK_LINES = parseInt(process.env.SCROLLBACK_LINES || '200', 10);
const MAX_CARDS = parseInt(process.env.MAX_CARDS || '40', 10);
const NOTIFY = (process.env.NOTIFY || '1') !== '0';

const terminal = createBackend({ session: NAME, scrollbackLines: SCROLLBACK_LINES });

let uiState = { mode: 'mirror', input: '', hist: null };

const history = [];
const HISTORY_MAX = 50;

const VIEW_ROWS = 6;
const VIEW_COLS = WRAP_COLS;
const PROMPT_TAIL_ROWS = 16;
let view = { row: 0, col: 0, follow: true, selRow: -1 };

function clamp(v, lo, hi) {
  if (v < lo) return lo;
  if (v > hi) return hi;
  return v;
}

function activeTerminal() {
  return terminal;
}

const NO_SESSION = 'Terminal is gone.\nRun cardputerme on\nthe computer to\nexpose it again.';

function detectPrompt(pane) {
  if (!pane) return null;

  const lines = toAscii(pane).split('\n').map((l) => trimEnd(l));
  const optIdx = [];
  for (let i = 0; i < lines.length; i++) if (parseChoices(lines[i]).length > 0) optIdx.push(i);
  if (optIdx.length < 2) return null;
  let start = optIdx[0];

  for (let i = start - 1; i >= 0 && i >= start - 6; i--) {
    if (!lines[i].trim()) continue;
    start = i;
    if (endsWithQuestion(lines[i])) break;
  }
  let end = optIdx[optIdx.length - 1];

  const hint = lines[end + 1] ? lines[end + 1].toLowerCase() : '';
  if (hint.includes('esc') || hint.includes('enter') || hint.includes('cancel') || hint.includes('tab')) end += 1;
  const text = lines.slice(start, end + 1).filter((l) => l.trim()).join('\n');
  return text.trim() ? text : null;
}

const MAX_LINES = 90;
function screenLines(text) {
  const lines = [].concat(...sliceIntoCards(text, WRAP_COLS, LINES_PER_CARD, MAX_CARDS));
  if (lines.length > MAX_LINES) return lines.slice(lines.length - MAX_LINES);
  return lines;
}

function gridLines(rows) {
  const out = [];
  for (const raw of rows) {
    const { text, color } = parseLine(raw, COLORS.text);
    out.push({ text: toAscii(text).split('\t').join('  '), color });
  }
  if (out.length > MAX_LINES) return out.slice(out.length - MAX_LINES);
  return out;
}

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

async function buildState() {
  const term = activeTerminal();
  if (!(await term.exists())) {
    return { lines: screenLines(NO_SESSION), status: 'terminal gone', sessionExists: false, awaiting: false };
  }
  const pane = await term.capture();

  const plain = stripAnsi(pane);
  const tail = plain.split('\n').slice(-PROMPT_TAIL_ROWS).join('\n');
  const awaiting = detectPrompt(tail) !== null;

  const { grid, status } = splitScreen(pane);
  cachedMirror = { grid, status, awaiting };
  return composeMirror(grid, status, awaiting);
}

let cachedMirror = null;

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
  if (selRow < 0 && view.row >= maxRow) view.follow = true;
  const lines = windowLines(grid, view, { rows: VIEW_ROWS, cols: VIEW_COLS });

  if (uiState.input.length > 0) {
    const composed = wrapLine('> ' + uiState.input.split('\n').join(' | '), VIEW_COLS);
    for (const piece of composed.slice(-2)) lines.push({ text: piece, color: COLORS.prompt });
  }
  const hint = awaiting ? 'PROMPT: press a number' : status;
  const bar = `${hint}  r${view.row}/${maxRow} c${view.col}`;
  return { lines: lines.length ? lines : screenLines('(empty)'), status: `[${NAME}] ${bar}`, sessionExists: true, awaiting };
}

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

const server = http.createServer(app);
const wss = new WebSocketServer({ server, path: '/ws' });

let lastSig = '';
let lastAwaiting = false;

function displayMessage(st) {
  return JSON.stringify(buildDisplay(st.lines, st.status, { awaiting: st.awaiting }));
}
function broadcast(str) {
  for (const c of wss.clients) if (c.readyState === 1) c.send(str);
}

async function pushIfChanged(force) {
  const st = await buildState();
  const sig = JSON.stringify(st.lines) + '' + st.status;
  if (sig !== lastSig || force) {
    lastSig = sig;
    broadcast(displayMessage(st));
  }
  const freshQuestion = st.awaiting && !lastAwaiting;
  if (NOTIFY && freshQuestion) broadcast(JSON.stringify({ type: 'notify', reason: 'question' }));
  if (!st.awaiting && lastAwaiting) view.follow = true;
  lastAwaiting = st.awaiting;
}

async function applyKey(key) {
  const { state, action } = interpretKey(uiState, key, { awaiting: lastAwaiting, history });
  uiState = state;
  if (action.kind === 'pan') view = Object.assign({}, view, panViewport(view, action.key));
  if (action.kind === 'send') {
    history.push(action.text);
    if (history.length > HISTORY_MAX) history.shift();
    view.follow = true;
    await activeTerminal().sendText(action.text);
  }

  if (action.kind === 'pressKey') {
    view.follow = true;
    await activeTerminal().sendKey(action.key);
  }

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

  const st = await buildState();
  ws.send(displayMessage(st));

  ws.on('message', async (data) => {
    let m; try { m = JSON.parse(data.toString()); } catch { return; }
    if (!m || typeof m.type !== 'string') return;

    if (m.type === 'key' && typeof m.key === 'string') {
      await applyKey(m.key);
      return;
    }

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

setInterval(() => { if (wss.clients.size) pushIfChanged(false).catch(() => {}); }, 800);

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


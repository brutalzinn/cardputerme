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
const { tmuxBackend, listTmuxSessions } = require('./lib/terminal');
const { createRegistry } = require('./lib/sessions');
const { buildDisplay, COLORS } = require('./lib/display');
const { interpretKey } = require('./lib/input');
const { parseLine, stripAnsi } = require('./lib/ansi');

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
const PORT = parseInt(process.env.PORT || '4711', 10);
// Optional default session to pre-select. The user need NOT know any session
// name — sessions are auto-discovered and picked on the device. Empty = none.
const DEFAULT_SESSION = process.env.SESSION || process.env.TMUX_SESSION || '';
const WRAP_COLS = parseInt(process.env.WRAP_COLS || '20', 10);
const LINES_PER_CARD = parseInt(process.env.LINES_PER_CARD || '7', 10);
const SCROLLBACK_LINES = parseInt(process.env.SCROLLBACK_LINES || '200', 10);
const MAX_CARDS = parseInt(process.env.MAX_CARDS || '40', 10);
const NOTIFY = (process.env.NOTIFY || '1') !== '0';

// ---- session registry (one server, many named sessions) --------------------
// The core reads/writes through the SELECTED session's adapter, never a single
// hard-wired one. Live tmux sessions are auto-registered by name; the device
// lists them and selects one over the ONE WebSocket (no per-session port).
// tmux is today's backend; a PTY-owned shell drops in with the same interface.
const registry = createRegistry();
function makeBackend(session) {
  return tmuxBackend({ session, scrollbackLines: SCROLLBACK_LINES });
}
let activeSessionName = DEFAULT_SESSION;
// UI interaction state, driven entirely server-side by the input FSM (lib/input).
// 'mirror' = showing a session; 'picker' = showing the numbered session menu.
let uiState = { mode: 'mirror' };

// Auto-discover live sessions and register any we don't know yet; keep the
// active selection valid. The user never types a session name — sessions are
// found here and picked on the device. Cheap; safe to call often.
async function refreshSessions() {
  const live = await listTmuxSessions();
  for (const name of live) {
    if (registry.has(name)) continue;
    registry.add(name, makeBackend(name));
  }
  // Honor an optional pre-selected default even if it isn't live yet.
  if (DEFAULT_SESSION && !registry.has(DEFAULT_SESSION)) registry.add(DEFAULT_SESSION, makeBackend(DEFAULT_SESSION));
  if (!activeSessionName || !registry.has(activeSessionName)) activeSessionName = registry.names()[0] || '';
}

// The adapter for the currently-selected session (always defined).
function activeTerminal() {
  const entry = registry.get(activeSessionName);
  if (entry) return entry.backend;
  return makeBackend(activeSessionName);
}

// ---- build cards -----------------------------------------------------------
const NO_SESSION = 'No active session.\nOpen a terminal session\nand pick it on the\ndevice.';

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

// Wrap ANSI-coloured pane rows into capped {text,color} display lines — each
// wrapped piece keeps its row's terminal colour (mirrored, translated to RGB565).
function coloredLines(rows) {
  const out = [];
  for (const raw of rows) {
    const { text, color } = parseLine(raw, COLORS.text);
    for (const piece of wrapLine(toAscii(text).split('\t').join('  '), WRAP_COLS)) {
      out.push({ text: trimEnd(piece), color });
    }
  }
  if (out.length > MAX_LINES) return out.slice(out.length - MAX_LINES);
  return out;
}

// Split a captured (ANSI) pane into coloured body lines + a one-line STATUS (the
// terminal's bottom status row = its last non-empty line, like Claude Code's bar).
function splitScreen(pane) {
  const rows = String(pane || '').split('\n');
  let last = -1;
  for (let i = rows.length - 1; i >= 0; i--) {
    if (stripAnsi(rows[i]).trim()) { last = i; break; }
  }
  if (last < 0) return { lines: [], status: '' };
  const status = toAscii(stripAnsi(rows[last])).trim();
  return { lines: coloredLines(rows.slice(0, last)), status };
}

// Build the current screen: body lines + a status string. NO modes/transcript;
// same generic path for every CLI. The device renders this via lib/display.
async function buildState() {
  // Picker mode: a plain NUMBERED TEXT menu (Telegram-style); pick by number.
  if (uiState.mode === 'picker') {
    const names = registry.names();
    const menu = ['Pick a session:', ''].concat(names.map((n, i) => `${i + 1}. ${n}`));
    return { lines: screenLines(menu.join('\n')), status: '` cancel | press a number', sessionExists: true, awaiting: false };
  }

  const term = activeTerminal();
  if (!(await term.exists())) {
    return { lines: screenLines(NO_SESSION), status: 'no active session', sessionExists: false, awaiting: false };
  }
  const pane = await term.capture();          // includes ANSI colour escapes (-e)
  const prompt = detectPrompt(stripAnsi(pane)); // detection runs on plain text
  if (prompt) {
    return { lines: screenLines(prompt), status: `[${activeSessionName}] answer the prompt`, sessionExists: true, awaiting: true };
  }
  // Mirror: colour body lines from the terminal's OWN colours; extract the status.
  const { lines, status } = splitScreen(pane);
  return { lines: lines.length ? lines : screenLines('(empty)'), status: `[${activeSessionName}] ${status}`, sessionExists: true, awaiting: false };
}

// ---- HTTP (debugging only) -------------------------------------------------
const app = express();
app.use(express.json({ limit: '64kb' }));
app.get('/health', async (_req, res) => {
  await refreshSessions();
  res.json({ ok: true, active: activeSessionName, sessions: registry.list(), exists: await activeTerminal().exists(), notify: NOTIFY, awaiting: lastAwaiting });
});
// The session list + current selection (the device's picker reads this).
app.get('/sessions', async (_req, res) => {
  await refreshSessions();
  res.json({ active: activeSessionName, sessions: registry.list() });
});
// Select the active session by name — the user never types a tmux name; they
// pick from the auto-discovered list.
app.post('/select', async (req, res) => {
  const name = req.body && typeof req.body.name === 'string' ? req.body.name : '';
  await refreshSessions();
  if (!registry.has(name)) return res.status(404).json({ ok: false, error: `unknown session '${name}'` });
  activeSessionName = name;
  pushIfChanged(true).catch(() => {});
  res.json({ ok: true, active: activeSessionName });
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
// The session list + current selection — the device's picker renders this.
function sessionsMessage() {
  return JSON.stringify({ type: 'sessions', active: activeSessionName, sessions: registry.list() });
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
  lastAwaiting = st.awaiting;
}

wss.on('connection', async (ws, req) => {
  const who = (req && req.socket && req.socket.remoteAddress) || '?';
  console.log(`[ws] connect ${who} (clients=${wss.clients.size})`);
  ws.on('close', (code, reason) => console.log(`[ws] close ${who} code=${code} ${String(reason || '')}`));
  ws.on('error', (e) => console.log(`[ws] error ${who} ${e && e.message}`));
  // Send the session list + current screen immediately to the newcomer.
  await refreshSessions();
  const st = await buildState();
  ws.send(sessionsMessage());
  ws.send(displayMessage(st));

  ws.on('message', async (data) => {
    let m; try { m = JSON.parse(data.toString()); } catch { return; }
    if (!m || typeof m.type !== 'string') return;

    // Pick a session by name (from the device's picker).
    if (m.type === 'selectSession' && typeof m.name === 'string') {
      await refreshSessions();
      if (!registry.has(m.name)) return;
      activeSessionName = m.name;
      ws.send(sessionsMessage());
      pushIfChanged(true).catch(() => {});
      return;
    }
    // Ask for the current session list.
    if (m.type === 'listSessions') {
      await refreshSessions();
      ws.send(sessionsMessage());
      return;
    }
    // Send input to the active session. A lone digit picks a menu option (no
    // Enter — the CLI's selector acts on the keypress); anything else is typed +
    // submitted. Single-digit test is a plain char check (no regex).
    // Every key the device forwards is interpreted HERE (server-side) by the
    // pure FSM (lib/input) — the device special-cases nothing. Backtick is Esc:
    // `=Escape to the terminal, ``=open picker; in the picker a number selects a
    // session. Otherwise a digit answers an on-screen menu, or text is typed.
    if (m.type === 'cmd' && typeof m.text === 'string') {
      const { state, action } = interpretKey(uiState, m.text, { sessions: registry.names(), awaiting: lastAwaiting });
      uiState = state;
      const term = activeTerminal();
      if (action.kind === 'select') activeSessionName = action.name;
      if (action.kind === 'escape') await term.sendKey('Escape');
      if (action.kind === 'answerMenu') await term.sendKey(action.key);
      if (action.kind === 'type') await term.sendText(action.text);
      // openPicker / closePicker / select / none: buildState reflects uiState below.
      setTimeout(() => pushIfChanged(true).catch(() => {}), 300);
      return;
    }
  });
});

// Server-side tick (NOT device polling): catches screen changes that emit no
// event — prompts, menus, live streaming. The Cardputer just receives pushes.
setInterval(() => { if (wss.clients.size) pushIfChanged(false).catch(() => {}); }, 800);
// Keep sockets alive with a periodic ping.
setInterval(() => { for (const c of wss.clients) if (c.readyState === 1) c.ping(); }, 20000);

server.listen(PORT, '0.0.0.0', async () => {
  console.log(`cardputerme — terminal remote on http://0.0.0.0:${PORT}  (ws://…/ws)`);
  console.log(`  cards  : ${WRAP_COLS} cols x ${LINES_PER_CARD} lines | notify: ${NOTIFY ? 'on' : 'off'}`);
  await refreshSessions();
  console.log(`  sessions: ${registry.names().join(', ') || '(none yet)'} | active: ${activeSessionName || '(none)'}`);
});

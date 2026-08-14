'use strict';

/*
 * Cardputer <-> Claude Code bridge — pure WebSocket for the device.
 *
 * Claude Code runs inside a tmux session (started in any project folder).
 *   READ : parse Claude's transcript JSONL (clean reply) or raw tmux screen.
 *   WRITE: inject typed commands with `tmux send-keys` (like typing).
 * The Cardputer keeps ONE WebSocket open: cards are pushed down, commands
 * come up, and a notify event fires a sound when Claude replies / asks.
 *
 * HTTP routes (/health, /cards, /command) remain for curl debugging only.
 */

const fs = require('fs');
const os = require('os');
const path = require('path');
const http = require('http');
const { execFile } = require('child_process');
const express = require('express');
const { WebSocketServer } = require('ws');
const { sliceIntoCards, toAscii } = require('./lib/format');
const { parseJsonl, extractLatest } = require('./lib/transcript');

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
const TMUX_SESSION = process.env.TMUX_SESSION || 'claude';
const WRAP_COLS = parseInt(process.env.WRAP_COLS || '20', 10);
const LINES_PER_CARD = parseInt(process.env.LINES_PER_CARD || '7', 10);
const SCROLLBACK_LINES = parseInt(process.env.SCROLLBACK_LINES || '200', 10);
const MAX_CARDS = parseInt(process.env.MAX_CARDS || '40', 10);
const READ_MODE = (process.env.READ_MODE || 'claude').toLowerCase();
const NOTIFY = (process.env.NOTIFY || '1') !== '0';
const CLAUDE_PROJECTS_DIR = process.env.CLAUDE_PROJECTS_DIR || '';
const CLAUDE_PROJECT = process.env.CLAUDE_PROJECT || '';

function expandTilde(p) {
  return p.startsWith('~') ? path.join(os.homedir(), p.slice(1)) : p;
}

// ---- tmux helpers ----------------------------------------------------------
function run(cmd, args) {
  return new Promise((resolve) => {
    execFile(cmd, args, { maxBuffer: 4 * 1024 * 1024 }, (err, stdout, stderr) => {
      resolve({ code: err && typeof err.code === 'number' ? err.code : err ? 1 : 0, stdout: stdout || '', stderr: stderr || '' });
    });
  });
}
async function sessionExists() {
  return (await run('tmux', ['has-session', '-t', TMUX_SESSION])).code === 0;
}
async function capturePane() {
  const args = ['capture-pane', '-p', '-t', TMUX_SESSION];
  if (SCROLLBACK_LINES > 0) args.push('-S', `-${SCROLLBACK_LINES}`);
  const { code, stdout } = await run('tmux', args);
  return code === 0 ? stdout : null;
}
// Type text then submit — small gap so the TUI registers the text before Enter.
async function sendToTmux(text) {
  if ((await run('tmux', ['send-keys', '-t', TMUX_SESSION, '-l', text])).code !== 0) return false;
  await new Promise((r) => setTimeout(r, 120));
  return (await run('tmux', ['send-keys', '-t', TMUX_SESSION, 'Enter'])).code === 0;
}

// ---- transcript discovery (auto-follow newest across all ~/.claude* accounts)
function candidateProjectRoots() {
  if (CLAUDE_PROJECTS_DIR) return [expandTilde(CLAUDE_PROJECTS_DIR)];
  const home = os.homedir();
  const roots = [];
  try {
    for (const name of fs.readdirSync(home)) {
      if (/^\.claude(-.*)?$/.test(name)) {
        const p = path.join(home, name, 'projects');
        try { if (fs.statSync(p).isDirectory()) roots.push(p); } catch { /* skip */ }
      }
    }
  } catch { /* skip */ }
  return roots;
}
// Encode a filesystem path the way Claude Code names its project dir (/ -> -).
function encodeProject(cwd) {
  return cwd.replace(/\//g, '-');
}
// The cwd of the tmux session the Cardputer controls, so we follow THAT
// Claude session specifically (not some other active account/session).
async function tmuxCwd() {
  const { code, stdout } = await run('tmux', ['display-message', '-p', '-t', TMUX_SESSION, '#{pane_current_path}']);
  return code === 0 ? stdout.trim() : '';
}

function findLatestTranscript(preferProject) {
  const roots = candidateProjectRoots();
  const pinned = CLAUDE_PROJECT || preferProject || '';
  // First try the pinned/preferred project; fall back to newest-anywhere.
  if (pinned) {
    const hit = newestIn(roots.map((r) => path.join(r, pinned)));
    if (hit) return hit;
  }
  let best = null, bestMtime = -1;
  for (const root of roots) {
    const projDirs = safeSubdirs(root);
    for (const dir of projDirs) {
      let entries;
      try { entries = fs.readdirSync(dir); } catch { continue; }
      for (const name of entries) {
        if (!name.endsWith('.jsonl')) continue;
        const full = path.join(dir, name);
        try {
          const st = fs.statSync(full);
          if (st.mtimeMs > bestMtime) { bestMtime = st.mtimeMs; best = full; }
        } catch { /* skip */ }
      }
    }
  }
  return best;
}
function safeSubdirs(root) {
  try {
    return fs.readdirSync(root, { withFileTypes: true }).filter((d) => d.isDirectory()).map((d) => path.join(root, d.name));
  } catch { return []; }
}
// Newest *.jsonl among the given directories (or null).
function newestIn(dirs) {
  let best = null, bestMtime = -1;
  for (const dir of dirs) {
    let entries;
    try { entries = fs.readdirSync(dir); } catch { continue; }
    for (const name of entries) {
      if (!name.endsWith('.jsonl')) continue;
      const full = path.join(dir, name);
      try {
        const st = fs.statSync(full);
        if (st.mtimeMs > bestMtime) { bestMtime = st.mtimeMs; best = full; }
      } catch { /* skip */ }
    }
  }
  return best;
}
// Returns { text, question, assistantCount } or null.
function readClaudeLatest(preferProject) {
  const file = findLatestTranscript(preferProject);
  if (!file) return null;
  let raw;
  try { raw = fs.readFileSync(file, 'utf8'); } catch { return null; }
  const meta = extractLatest(parseJsonl(raw));
  if (!meta.prompt && !meta.reply) return null;
  return meta;
}

// ---- build cards -----------------------------------------------------------
const NO_SESSION = `No claude session.\nStart it with:\ntmux new -s ${TMUX_SESSION}\nthen run claude inside\nthat project folder.`;

// Claude Code approval / selection prompts (create file? run plan? proceed?)
// live in the TUI, not the transcript. Detect them from the captured screen so
// the Cardputer can SEE and ANSWER them (type the option number).
function detectPrompt(pane) {
  if (!pane) return null;
  // ASCII-clean first so the `❯` pointer / box chars don't hide "1. Yes".
  const lines = toAscii(pane).replace(/\r/g, '').split('\n').map((l) => l.replace(/\s+$/, ''));
  const optIdx = [];
  for (let i = 0; i < lines.length; i++) if (/^\s*\d+\.\s+\S/.test(lines[i])) optIdx.push(i);
  if (optIdx.length < 2) return null;               // need a real option list
  let start = optIdx[0];
  // pull in a question line just above the options, if present
  for (let i = start - 1; i >= 0 && i >= start - 4; i--) {
    if (!lines[i].trim()) continue;
    if (/\?|do you want|would you like|select|choose|proceed/i.test(lines[i])) start = i;
    break;
  }
  let end = optIdx[optIdx.length - 1];
  if (lines[end + 1] && /esc|tab|enter|cancel/i.test(lines[end + 1])) end += 1;
  const text = lines.slice(start, end + 1).filter((l) => l.trim()).join('\n');
  return text.trim() ? text : null;
}

async function buildState(mode) {
  const exists = await sessionExists();
  if (mode === 'claude') {
    // Interactive prompt on screen? Surface it (so it can be answered) + flag it.
    if (exists) {
      const pane = await capturePane();
      const prompt = detectPrompt(pane);
      if (prompt) {
        const cards = sliceIntoCards(prompt, WRAP_COLS, LINES_PER_CARD, MAX_CARDS);
        return { cards, sessionExists: true, mode: 'prompt', assistantCount: 0, question: true, awaiting: true };
      }
    }
    const cwd = exists ? await tmuxCwd() : '';
    const meta = readClaudeLatest(cwd ? encodeProject(cwd) : '');
    if (meta) {
      const cards = sliceIntoCards(meta.text, WRAP_COLS, LINES_PER_CARD, MAX_CARDS);
      return { cards: cards.length ? cards : [['(empty)']], sessionExists: exists, mode: 'claude', assistantCount: meta.assistantCount, question: meta.question, awaiting: false };
    }
  }
  if (!exists) {
    return { cards: sliceIntoCards(NO_SESSION, WRAP_COLS, LINES_PER_CARD, MAX_CARDS), sessionExists: false, mode: 'raw', assistantCount: 0, question: false, awaiting: false };
  }
  const pane = await capturePane();
  const cards = sliceIntoCards(pane || '(empty)', WRAP_COLS, LINES_PER_CARD, MAX_CARDS);
  return { cards: cards.length ? cards : [['(empty)']], sessionExists: exists, mode: 'raw', assistantCount: 0, question: false, awaiting: false };
}

// ---- HTTP (debugging only) -------------------------------------------------
const app = express();
app.use(express.json({ limit: '64kb' }));
app.get('/health', async (_req, res) => res.json({ ok: true, session: TMUX_SESSION, exists: await sessionExists(), readMode: READ_MODE, notify: NOTIFY }));
app.get('/cards', async (req, res) => {
  const st = await buildState((req.query.mode || READ_MODE).toLowerCase());
  res.json({ total: st.cards.length, mode: st.mode, sessionExists: st.sessionExists, cards: st.cards });
});
app.post('/command', async (req, res) => {
  const text = req.body && typeof req.body.text === 'string' ? req.body.text : '';
  if (!text) return res.status(400).json({ ok: false, error: 'missing text' });
  if (!(await sessionExists())) return res.status(409).json({ ok: false, error: `no tmux session '${TMUX_SESSION}'` });
  res.json({ ok: await sendToTmux(text), sent: text });
});

// ---- WebSocket (the device's only channel) ---------------------------------
const server = http.createServer(app);
const wss = new WebSocketServer({ server, path: '/ws' });

let lastCardsSig = '';
let lastAssistantCount = 0;
let lastAwaiting = false;

function cardsMessage(st) {
  return JSON.stringify({ type: 'cards', total: st.cards.length, mode: st.mode, sessionExists: st.sessionExists, cards: st.cards });
}
function broadcast(str) {
  for (const c of wss.clients) if (c.readyState === 1) c.send(str);
}

// Push new state to all clients when it changes; fire notify on a fresh reply
// OR when Claude puts up an interactive prompt (needs the user to answer).
async function pushIfChanged(force) {
  const st = await buildState(READ_MODE);
  const sig = JSON.stringify(st.cards);
  if (sig !== lastCardsSig || force) {
    lastCardsSig = sig;
    broadcast(cardsMessage(st));
  }
  if (NOTIFY) {
    if (st.awaiting && !lastAwaiting) {
      broadcast(JSON.stringify({ type: 'notify', reason: 'question' })); // approval/selection appeared
    } else if (st.assistantCount > lastAssistantCount) {
      broadcast(JSON.stringify({ type: 'notify', reason: st.question ? 'question' : 'reply' }));
    }
  }
  lastAssistantCount = Math.max(lastAssistantCount, st.assistantCount);
  lastAwaiting = st.awaiting;
}

wss.on('connection', async (ws) => {
  // send current state immediately to the newcomer
  const st = await buildState(READ_MODE);
  lastAssistantCount = Math.max(lastAssistantCount, st.assistantCount);
  ws.send(cardsMessage(st));

  ws.on('message', async (data) => {
    let m; try { m = JSON.parse(data.toString()); } catch { return; }
    if (m && m.type === 'cmd' && typeof m.text === 'string') {
      // Answering an on-screen menu: a lone digit picks the option (no Enter,
      // Claude's selector acts on the keypress). Otherwise type + submit.
      if (lastAwaiting && /^\d$/.test(m.text.trim())) {
        await run('tmux', ['send-keys', '-t', TMUX_SESSION, '-l', m.text.trim()]);
      } else {
        await sendToTmux(m.text);
      }
      setTimeout(() => pushIfChanged(true).catch(() => {}), 500);
    }
  });
});

// Event-driven: watch each transcript root; debounce a burst of writes.
let debounce = null;
function scheduledPush() {
  clearTimeout(debounce);
  debounce = setTimeout(() => pushIfChanged(false).catch(() => {}), 150);
}
for (const root of candidateProjectRoots()) {
  try { fs.watch(root, { recursive: true }, scheduledPush); } catch { /* skip */ }
}
// Server-side tick (NOT device polling): catches TUI-only changes that emit no
// file event — approval prompts, selection menus, live streaming. Only the
// server touches tmux; the Cardputer just receives pushes over its socket.
setInterval(() => { if (wss.clients.size) pushIfChanged(false).catch(() => {}); }, 800);
// Keep sockets alive with a periodic ping.
setInterval(() => { for (const c of wss.clients) if (c.readyState === 1) c.ping(); }, 20000);

server.listen(PORT, '0.0.0.0', () => {
  console.log(`Cardputer<->Claude bridge on http://0.0.0.0:${PORT}  (ws://…/ws)`);
  console.log(`  tmux session : ${TMUX_SESSION}`);
  console.log(`  cards        : ${WRAP_COLS} cols x ${LINES_PER_CARD} lines`);
  console.log(`  read mode    : ${READ_MODE} | notify: ${NOTIFY ? 'on' : 'off'}`);
  const roots = candidateProjectRoots();
  console.log(`  transcripts  : ${CLAUDE_PROJECTS_DIR || 'auto (' + roots.length + ' account dir(s))'}`);
});

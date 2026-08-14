'use strict';

// Claude Code transcript parsing — pure functions over already-parsed JSONL
// objects, so they are unit-testable without touching the filesystem.

// Pull plain text out of a message.content (string or content-block array).
// Only 'text' blocks count; tool_use/tool_result/thinking are ignored.
function textFromMessage(msg) {
  if (!msg) return '';
  const c = msg.content;
  if (typeof c === 'string') return c;
  if (Array.isArray(c)) {
    return c
      .filter((b) => b && b.type === 'text' && typeof b.text === 'string')
      .map((b) => b.text)
      .join('\n');
  }
  return '';
}

// A "real" user turn (a prompt), not a tool_result carrier.
function isUserPrompt(obj) {
  if (!obj || obj.type !== 'user') return false;
  const c = obj.message && obj.message.content;
  if (typeof c === 'string') return true;
  if (Array.isArray(c)) return c.some((b) => b && b.type === 'text');
  return false;
}

// Does Claude's reply look like it's waiting on the user? Drives the
// notification sound (e.g. after a plan, "Do you want to proceed?").
function isQuestion(text) {
  const t = String(text).trim();
  if (!t) return false;
  if (t.endsWith('?')) return true;
  return /\b(do you want|would you like|should i|shall i|which option|proceed|approve this|confirm)\b/i.test(
    t
  );
}

// Given the parsed transcript objects, return the latest exchange:
//   { prompt, reply, text, assistantCount, question }
function extractLatest(objs) {
  const assistantCount = objs.filter((o) => o.type === 'assistant').length;

  let lastPromptIdx = -1;
  for (let i = objs.length - 1; i >= 0; i--) {
    if (isUserPrompt(objs[i])) { lastPromptIdx = i; break; }
  }
  const prompt = lastPromptIdx >= 0 ? textFromMessage(objs[lastPromptIdx].message).trim() : '';

  const replyParts = [];
  for (let i = lastPromptIdx + 1; i < objs.length; i++) {
    if (objs[i].type === 'assistant') {
      const t = textFromMessage(objs[i].message).trim();
      if (t) replyParts.push(t);
    }
  }
  const reply = replyParts.join('\n\n').trim();

  let text = '';
  if (prompt) text += '> ' + prompt + '\n\n';
  text += reply || '(thinking / no text reply yet)';

  return { prompt, reply, text, assistantCount, question: isQuestion(reply) };
}

// Parse raw JSONL text into objects (skipping partial/non-JSON lines).
function parseJsonl(raw) {
  const objs = [];
  for (const line of String(raw).trim().split('\n')) {
    try { objs.push(JSON.parse(line)); } catch { /* skip */ }
  }
  return objs;
}

module.exports = { textFromMessage, isUserPrompt, isQuestion, extractLatest, parseJsonl };

'use strict';

const crypto = require('crypto');

// One server, many named sessions. A registry maps a session NAME -> an entry
// { name, backend }, where `backend` is a terminal adapter (lib/terminal). The
// name is user-given, or a random UUID when omitted. Names are unique. The
// registry only stores adapters — it never calls into them; the core reads/writes
// through the selected session's backend. This is what lets ONE server + ONE
// WebSocket drive many sessions without per-session ports.
function createRegistry({ newId = () => crypto.randomUUID() } = {}) {
  const byName = new Map(); // insertion-ordered

  function add(name, backend) {
    const finalName = name || newId();
    if (byName.has(finalName)) throw new Error(`session '${finalName}' already exists`);
    const entry = { name: finalName, backend };
    byName.set(finalName, entry);
    return entry;
  }

  function has(name) {
    return byName.has(name);
  }

  function get(name) {
    return byName.get(name) || null;
  }

  function remove(name) {
    return byName.delete(name);
  }

  // Picker metadata only — never leak the backend adapter over the wire.
  function list() {
    return Array.from(byName.values()).map((e) => ({ name: e.name }));
  }

  function names() {
    return Array.from(byName.keys());
  }

  function prune(liveNames) {
    const keep = new Set(liveNames);
    const removed = [];
    for (const name of byName.keys()) {
      if (keep.has(name)) continue;
      removed.push(name);
    }
    for (const name of removed) byName.delete(name);
    return removed;
  }

  return { add, has, get, remove, list, names, prune };
}

module.exports = { createRegistry };

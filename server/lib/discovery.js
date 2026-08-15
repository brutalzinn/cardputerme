'use strict';

async function pickPort(isFree, { start = 8001, tries = 255 } = {}) {
  for (let port = start; port < start + tries; port++) {
    if (await isFree(port)) return port;
  }
  return 0;
}

function freePortProbe(net) {
  return (port) => new Promise((resolve) => {
    const probe = net.createServer();
    probe.once('error', () => resolve(false));
    probe.listen(port, '0.0.0.0', () => probe.close(() => resolve(true)));
  });
}

module.exports = { pickPort, freePortProbe };

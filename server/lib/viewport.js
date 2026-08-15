'use strict';

// Server-owned omnidirectional viewport over the terminal's NATIVE-width grid.
// The device forwards arrow keys; the server pans and sends only the visible
// window (so payloads stay tiny). Pure, no regex, no else.

const ROW_STEP = 1;   // rows per up/down
const COL_STEP = 8;   // cols per left/right

// Move the viewport by one arrow press. `follow` (stick to the bottom) turns off
// on any vertical move; buildState re-enables it when we're back at the bottom.
function panViewport(view, key) {
  const { row, col, follow } = view;
  if (key === 'up') return { row: Math.max(0, row - ROW_STEP), col, follow: false };
  if (key === 'down') return { row: row + ROW_STEP, col, follow: false };
  if (key === 'left') return { row, col: Math.max(0, col - COL_STEP), follow };
  if (key === 'right') return { row, col: col + COL_STEP, follow };
  return view;
}

function anchorRow(selRow, viewRows) {
  return Math.max(0, selRow - (viewRows - 2));
}

// Slice a rows x cols window out of the grid at the viewport offset, keeping each
// line's colour. No padding rows past the end of the grid.
function windowLines(grid, view, dims) {
  const out = [];
  for (let r = view.row; r < view.row + dims.rows && r < grid.length; r += 1) {
    const ln = grid[r];
    out.push({ text: String(ln.text).slice(view.col, view.col + dims.cols), color: ln.color });
  }
  return out;
}

module.exports = { panViewport, windowLines, anchorRow, ROW_STEP, COL_STEP };

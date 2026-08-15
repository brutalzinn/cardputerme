'use strict';

const ROW_STEP = 1;
const COL_STEP = 8;

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

function windowLines(grid, view, dims) {
  const out = [];
  for (let r = view.row; r < view.row + dims.rows && r < grid.length; r += 1) {
    const ln = grid[r];
    out.push({ text: String(ln.text).slice(view.col, view.col + dims.cols), color: ln.color });
  }
  return out;
}

module.exports = { panViewport, windowLines, anchorRow, ROW_STEP, COL_STEP };


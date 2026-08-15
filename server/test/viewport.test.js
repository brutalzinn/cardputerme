'use strict';
const { test } = require('node:test');
const assert = require('node:assert');
const { panViewport, windowLines, anchorRow, ROW_STEP, COL_STEP } = require('../lib/viewport');

// The server owns an omnidirectional viewport over the terminal's NATIVE-width
// grid; the device forwards arrow keys and renders the window the server sends.

test('panViewport up/down moves rows and unsticks follow', () => {
  assert.deepEqual(panViewport({ row: 5, col: 0, follow: true }, 'up'),
    { row: 5 - ROW_STEP, col: 0, follow: false });
  assert.deepEqual(panViewport({ row: 5, col: 0, follow: false }, 'down'),
    { row: 5 + ROW_STEP, col: 0, follow: false });
});

test('panViewport left/right moves columns, floored at 0', () => {
  assert.equal(panViewport({ row: 0, col: 10, follow: false }, 'left').col, 10 - COL_STEP);
  assert.equal(panViewport({ row: 0, col: 0, follow: false }, 'left').col, 0);   // floored
  assert.equal(panViewport({ row: 0, col: 0, follow: false }, 'right').col, COL_STEP);
});

test('panViewport floors row at 0 and ignores unknown keys', () => {
  assert.equal(panViewport({ row: 0, col: 0, follow: false }, 'up').row, 0);
  assert.deepEqual(panViewport({ row: 2, col: 2, follow: true }, 'zzz'), { row: 2, col: 2, follow: true });
});

test('windowLines slices rows x cols at the viewport offset, keeping colors', () => {
  const grid = [
    { text: 'aaaabbbbcccc', color: 1 },
    { text: 'ddddeeeeffff', color: 2 },
    { text: 'gggghhhhiiii', color: 3 },
  ];
  const win = windowLines(grid, { row: 1, col: 4 }, { rows: 2, cols: 4 });
  assert.deepEqual(win, [
    { text: 'eeee', color: 2 },
    { text: 'hhhh', color: 3 },
  ]);
});

test('anchorRow places the pointer near the bottom so the question above is visible', () => {
  assert.equal(anchorRow(10, 8), 10 - (8 - 2));  // pointer on view row 6 of 8
});

test('anchorRow floors at 0 when the pointer is near the top of the grid', () => {
  assert.equal(anchorRow(1, 8), 0);
  assert.equal(anchorRow(0, 8), 0);
});

test('windowLines stops at the end of the grid (no padding rows)', () => {
  const grid = [{ text: 'x', color: 9 }];
  const win = windowLines(grid, { row: 0, col: 0 }, { rows: 5, cols: 10 });
  assert.equal(win.length, 1);
});

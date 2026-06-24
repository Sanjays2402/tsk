import { test } from "node:test";
import assert from "node:assert/strict";
import {
  MOVE_SLOP,
  LONG_PRESS_MS,
  exceededSlop,
  shouldLongPress,
  trackMove,
  type PressState,
} from "../src/touch.ts";

test("constants are sane", () => {
  assert.ok(MOVE_SLOP > 0);
  assert.ok(LONG_PRESS_MS >= 300);
});

test("exceededSlop is false for a still finger", () => {
  assert.equal(exceededSlop({ x: 100, y: 100 }, { x: 103, y: 102 }), false);
});

test("exceededSlop is true once movement passes the radius", () => {
  assert.equal(exceededSlop({ x: 100, y: 100 }, { x: 120, y: 100 }), true);
});

test("exceededSlop is symmetric in both axes", () => {
  assert.equal(exceededSlop({ x: 0, y: 0 }, { x: 0, y: MOVE_SLOP + 1 }), true);
  assert.equal(exceededSlop({ x: 0, y: 0 }, { x: 0, y: MOVE_SLOP - 1 }), false);
});

test("shouldLongPress fires only when still AND held past threshold", () => {
  assert.equal(shouldLongPress(LONG_PRESS_MS, false), true);
  assert.equal(shouldLongPress(LONG_PRESS_MS + 50, false), true);
});

test("shouldLongPress: a moved press never fires (it's a scroll)", () => {
  assert.equal(shouldLongPress(LONG_PRESS_MS + 1000, true), false);
});

test("shouldLongPress: an early release is a tap, not a long-press", () => {
  assert.equal(shouldLongPress(LONG_PRESS_MS - 1, false), false);
});

test("trackMove latches moved once exceeded (a pause back in radius stays moved)", () => {
  const state: PressState = {
    id: 1,
    start: { x: 0, y: 0 },
    moved: false,
    timer: 0,
  };
  // Big move -> moved becomes true.
  state.moved = trackMove(state, { x: 50, y: 0 });
  assert.equal(state.moved, true);
  // Finger drifts back near the start, but moved stays latched.
  assert.equal(trackMove(state, { x: 1, y: 1 }), true);
});

test("trackMove stays false for tiny jitter", () => {
  const state: PressState = {
    id: 2,
    start: { x: 200, y: 200 },
    moved: false,
    timer: 0,
  };
  assert.equal(trackMove(state, { x: 202, y: 201 }), false);
});

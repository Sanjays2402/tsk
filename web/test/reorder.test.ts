import { test } from "node:test";
import assert from "node:assert/strict";
import {
  computeReorder,
  computeSectionReorder,
  arraysEqual,
  dropPosForY,
  type DropPos,
} from "../src/reorder.ts";

const ORDER = [1, 2, 3, 4, 5];

test("drag #4 before #2 -> [1 4 2 3 5], before=2", () => {
  const r = computeReorder(ORDER, 4, 2, "before");
  assert.deepEqual(r.order, [1, 4, 2, 3, 5]);
  assert.equal(r.before, 2);
  assert.equal(r.changed, true);
});

test("drag #2 after #4 -> [1 3 4 2 5], before=5", () => {
  const r = computeReorder(ORDER, 2, 4, "after");
  assert.deepEqual(r.order, [1, 3, 4, 2, 5]);
  assert.equal(r.before, 5); // lands in front of whatever followed #4
  assert.equal(r.changed, true);
});

test("drag #1 after the last row -> moved to end, before=0", () => {
  const r = computeReorder(ORDER, 1, 5, "after");
  assert.deepEqual(r.order, [2, 3, 4, 5, 1]);
  assert.equal(r.before, 0);
  assert.equal(r.changed, true);
});

test("drag #5 before #1 -> [5 1 2 3 4], before=1", () => {
  const r = computeReorder(ORDER, 5, 1, "before");
  assert.deepEqual(r.order, [5, 1, 2, 3, 4]);
  assert.equal(r.before, 1);
});

test("dropping onto self is a no-op", () => {
  const r = computeReorder(ORDER, 3, 3, "before");
  assert.equal(r.changed, false);
  assert.deepEqual(r.order, ORDER);
});

test("drag #2 after #1 is a no-op (already there)", () => {
  // #2 already directly follows #1, so before resolves to #2 (itself) -> no-op.
  const r = computeReorder(ORDER, 2, 1, "after");
  assert.equal(r.changed, false);
  assert.deepEqual(r.order, ORDER);
});

test("drag #2 before #3 is a no-op (already there)", () => {
  // #2 already sits directly before #3 -> dropping before #3 changes nothing.
  const r = computeReorder(ORDER, 2, 3, "before");
  assert.equal(r.changed, false);
});

test("unknown moved id is a no-op", () => {
  const r = computeReorder(ORDER, 99, 2, "before");
  assert.equal(r.changed, false);
  assert.deepEqual(r.order, ORDER);
});

test("unknown target id is a no-op", () => {
  const r = computeReorder(ORDER, 2, 99, "after");
  assert.equal(r.changed, false);
});

test("computeReorder does not mutate the input order", () => {
  const input = [1, 2, 3];
  computeReorder(input, 3, 1, "before");
  assert.deepEqual(input, [1, 2, 3]);
});

test("adjacent forward drag lands correctly", () => {
  // [1 2 3]: drag #1 after #2 -> [2 1 3], before=3
  const r = computeReorder([1, 2, 3], 1, 2, "after");
  assert.deepEqual(r.order, [2, 1, 3]);
  assert.equal(r.before, 3);
});

test("arraysEqual", () => {
  assert.equal(arraysEqual([1, 2, 3], [1, 2, 3]), true);
  assert.equal(arraysEqual([1, 2], [1, 2, 3]), false);
  assert.equal(arraysEqual([1, 2, 3], [1, 3, 2]), false);
});

test("dropPosForY splits a row at its midpoint", () => {
  // Row spans y=100..140 (top=100, height=40), midpoint=120.
  assert.equal(dropPosForY(100, 40, 105), "before");
  assert.equal(dropPosForY(100, 40, 119), "before");
  assert.equal(dropPosForY(100, 40, 120), "after");
  assert.equal(dropPosForY(100, 40, 135), "after");
});

test("DropPos type round-trips both literals", () => {
  const a: DropPos = "before";
  const b: DropPos = "after";
  assert.equal(a, "before");
  assert.equal(b, "after");
});

// --- F40: section-constrained reorder (Pinned drag) ------------------------

// Global file order: 1(pin) 2(pin) 3(unpinned) 4(pin) 5(unpinned)
// Pinned section visible order: [1, 2, 4]
const GLOBAL = [1, 2, 3, 4, 5];
const PINNED = [1, 2, 4];

test("section reorder: drag pinned #4 before pinned #1", () => {
  // Within the section #4 should land first; globally it must move in front of
  // #1 so the file reads 4,1,2,3,5.
  const r = computeSectionReorder(GLOBAL, PINNED, 4, 1, "before");
  assert.equal(r.changed, true);
  assert.equal(r.before, 1);
  assert.deepEqual(r.order, [4, 1, 2, 3, 5]);
});

test("section reorder: drag pinned #1 after pinned #2", () => {
  // Section becomes [2, 1, 4] -> #1 now sits in front of #4, so before=4.
  const r = computeSectionReorder(GLOBAL, PINNED, 1, 2, "after");
  assert.equal(r.changed, true);
  assert.equal(r.before, 4);
  // Global: remove 1, insert before 4 -> [2, 3, 1, 4, 5]
  assert.deepEqual(r.order, [2, 3, 1, 4, 5]);
});

test("section reorder: drag pinned #1 to the end of the section", () => {
  // Section becomes [2, 4, 1]. #1 lands after the last pin #4; globally that's
  // before whatever follows #4 in the file (#5).
  const r = computeSectionReorder(GLOBAL, PINNED, 1, 4, "after");
  assert.equal(r.changed, true);
  assert.equal(r.before, 5);
  assert.deepEqual(r.order, [2, 3, 4, 1, 5]);
});

test("section reorder: dragging onto self is a no-op", () => {
  const r = computeSectionReorder(GLOBAL, PINNED, 2, 2, "before");
  assert.equal(r.changed, false);
  assert.deepEqual(r.order, GLOBAL);
});

test("section reorder: a target outside the section is a no-op", () => {
  // #3 isn't in PINNED -> the gesture can't resolve, no change.
  const r = computeSectionReorder(GLOBAL, PINNED, 1, 3, "before");
  assert.equal(r.changed, false);
});

test("section reorder: drag to section end when section IS the file tail -> before 0", () => {
  // All three are pinned and contiguous at the end.
  const r = computeSectionReorder([7, 1, 2, 4], [1, 2, 4], 1, 4, "after");
  assert.equal(r.before, 0); // nothing follows #4 -> file end
  assert.deepEqual(r.order, [7, 2, 4, 1]);
});

test("section reorder does not mutate inputs", () => {
  const g = [1, 2, 3];
  const s = [1, 2];
  computeSectionReorder(g, s, 2, 1, "before");
  assert.deepEqual(g, [1, 2, 3]);
  assert.deepEqual(s, [1, 2]);
});

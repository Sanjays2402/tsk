import { test } from "node:test";
import assert from "node:assert/strict";
import {
  emptyBulk,
  isBulkActive,
  isSelected,
  toggleOne,
  selectRange,
  clearBulk,
  reconcileBulk,
  selectedInOrder,
  bulkSummary,
  renderBulkBar,
} from "../src/bulkselect.ts";

const VISIBLE = [10, 20, 30, 40, 50];

test("emptyBulk is inactive and selects nothing", () => {
  const b = emptyBulk();
  assert.equal(isBulkActive(b), false);
  assert.equal(b.ids.size, 0);
  assert.equal(b.anchor, null);
  assert.equal(isSelected(b, 10), false);
});

test("toggleOne adds, re-toggles removes, and moves the anchor", () => {
  let b = toggleOne(emptyBulk(), 30);
  assert.equal(isSelected(b, 30), true);
  assert.equal(b.anchor, 30);
  assert.equal(isBulkActive(b), true);
  b = toggleOne(b, 30);
  assert.equal(isSelected(b, 30), false);
  assert.equal(b.anchor, 30); // anchor stays put even when deselecting
  assert.equal(isBulkActive(b), false);
});

test("toggleOne does not mutate the input state", () => {
  const a = emptyBulk();
  const b = toggleOne(a, 10);
  assert.equal(a.ids.size, 0);
  assert.equal(b.ids.size, 1);
});

test("selectRange selects an inclusive forward range from the anchor", () => {
  const anchored = toggleOne(emptyBulk(), 20); // anchor = 20
  const ranged = selectRange(anchored, VISIBLE, 40);
  assert.deepEqual(selectedInOrder(ranged, VISIBLE), [20, 30, 40]);
  assert.equal(ranged.anchor, 20); // anchor preserved for re-ranging
});

test("selectRange works backward too (anchor below target)", () => {
  const anchored = toggleOne(emptyBulk(), 40); // anchor = 40
  const ranged = selectRange(anchored, VISIBLE, 10);
  assert.deepEqual(selectedInOrder(ranged, VISIBLE), [10, 20, 30, 40]);
});

test("selectRange unions onto an existing selection", () => {
  let b = toggleOne(emptyBulk(), 10); // {10}, anchor 10
  b = { ids: b.ids, anchor: 40 }; // pretend anchor moved to 40
  const ranged = selectRange(b, VISIBLE, 50); // add 40..50
  assert.deepEqual(selectedInOrder(ranged, VISIBLE), [10, 40, 50]);
});

test("selectRange with no anchor selects just the target", () => {
  const ranged = selectRange(emptyBulk(), VISIBLE, 30);
  assert.deepEqual(selectedInOrder(ranged, VISIBLE), [30]);
  assert.equal(ranged.anchor, 30);
});

test("selectRange ignores a target that isn't visible", () => {
  const anchored = toggleOne(emptyBulk(), 20);
  const ranged = selectRange(anchored, VISIBLE, 999);
  assert.equal(ranged, anchored); // unchanged reference
});

test("clearBulk empties everything", () => {
  const b = clearBulk();
  assert.equal(b.ids.size, 0);
  assert.equal(b.anchor, null);
});

test("reconcileBulk drops ids that are no longer visible", () => {
  let b = toggleOne(emptyBulk(), 20);
  b = selectRange(b, VISIBLE, 50); // {20,30,40,50}
  const next = reconcileBulk(b, [20, 40]); // 30,50 vanished
  assert.deepEqual([...next.ids].sort((x, y) => x - y), [20, 40]);
});

test("reconcileBulk clears a vanished anchor", () => {
  const b = toggleOne(emptyBulk(), 30); // anchor 30
  const next = reconcileBulk(b, [10, 20]); // 30 gone
  assert.equal(next.anchor, null);
});

test("reconcileBulk returns same reference when nothing changed", () => {
  let b = toggleOne(emptyBulk(), 20);
  b = selectRange(b, VISIBLE, 30); // {20,30}, anchor 20
  const next = reconcileBulk(b, VISIBLE);
  assert.equal(next, b);
});

test("reconcileBulk on empty selection is a no-op", () => {
  const b = emptyBulk();
  assert.equal(reconcileBulk(b, VISIBLE), b);
});

test("selectedInOrder yields visible top-to-bottom order", () => {
  let b = toggleOne(emptyBulk(), 50);
  b = toggleOne(b, 10);
  b = toggleOne(b, 30);
  assert.deepEqual(selectedInOrder(b, VISIBLE), [10, 30, 50]);
});

test("bulkSummary reads naturally", () => {
  assert.equal(bulkSummary(1), "1 selected");
  assert.equal(bulkSummary(7), "7 selected");
});

test("renderBulkBar collapses when empty, shows actions otherwise", () => {
  assert.equal(renderBulkBar(0), "");
  const html = renderBulkBar(3);
  assert.match(html, /3 selected/);
  assert.match(html, /data-bulk-toggle/);
  assert.match(html, /data-bulk-delete/);
  assert.match(html, /data-bulk-clear/);
});

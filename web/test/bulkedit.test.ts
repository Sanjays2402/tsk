import { test } from "node:test";
import assert from "node:assert/strict";
import {
  parseTagOps,
  isNoopTagOps,
  applyTagOps,
  priorityGlyph,
  renderBulkEditCluster,
  renderBulkPriorityMenu,
  renderBulkPinMenu,
  renderBulkTagEditor,
  renderBulkDueEditor,
  BULK_PRIORITIES,
} from "../src/bulkedit.ts";

test("parseTagOps splits adds and removes by sigil", () => {
  const ops = parseTagOps("+dev -home #urgent plain");
  assert.deepEqual(ops.add, ["dev", "urgent", "plain"]);
  assert.deepEqual(ops.remove, ["home"]);
});

test("parseTagOps lower-cases and de-dupes within each bucket", () => {
  const ops = parseTagOps("+Dev +dev -Home -home");
  assert.deepEqual(ops.add, ["dev"]);
  assert.deepEqual(ops.remove, ["home"]);
});

test("parseTagOps: an explicit remove overrides a same-token add", () => {
  const ops = parseTagOps("+x -x");
  assert.deepEqual(ops.add, []);
  assert.deepEqual(ops.remove, ["x"]);
});

test("parseTagOps ignores bare sigils and empty input", () => {
  assert.deepEqual(parseTagOps("+ - #"), { add: [], remove: [] });
  assert.deepEqual(parseTagOps("   "), { add: [], remove: [] });
});

test("isNoopTagOps detects an empty op", () => {
  assert.equal(isNoopTagOps({ add: [], remove: [] }), true);
  assert.equal(isNoopTagOps({ add: ["x"], remove: [] }), false);
  assert.equal(isNoopTagOps({ add: [], remove: ["y"] }), false);
});

test("applyTagOps removes then unions adds, preserving order", () => {
  const next = applyTagOps(["work", "home"], { add: ["dev"], remove: ["home"] });
  assert.deepEqual(next, ["work", "dev"]);
});

test("applyTagOps de-dupes an add that already exists", () => {
  const next = applyTagOps(["work"], { add: ["work", "dev"], remove: [] });
  assert.deepEqual(next, ["work", "dev"]);
});

test("applyTagOps normalizes existing tag case for comparison", () => {
  const next = applyTagOps(["Work"], { add: ["work"], remove: [] });
  assert.deepEqual(next, ["work"]); // not duplicated
});

test("applyTagOps does not mutate the input array", () => {
  const input = ["a", "b"];
  applyTagOps(input, { add: ["c"], remove: ["a"] });
  assert.deepEqual(input, ["a", "b"]);
});

test("applyTagOps removing a missing tag is a no-op on the list", () => {
  assert.deepEqual(applyTagOps(["a"], { add: [], remove: ["zzz"] }), ["a"]);
});

test("priorityGlyph maps each level to its letter", () => {
  assert.equal(priorityGlyph("low"), "L");
  assert.equal(priorityGlyph("medium"), "M");
  assert.equal(priorityGlyph("high"), "H");
  assert.equal(priorityGlyph("urgent"), "U");
});

test("BULK_PRIORITIES is the full ascending ladder", () => {
  assert.deepEqual([...BULK_PRIORITIES], ["low", "medium", "high", "urgent"]);
});

test("renderBulkEditCluster carries the four openers", () => {
  const html = renderBulkEditCluster();
  assert.match(html, /data-bulk-edit="priority"/);
  assert.match(html, /data-bulk-edit="tag"/);
  assert.match(html, /data-bulk-edit="due"/);
  // F47: the pin opener joins the cluster
  assert.match(html, /data-bulk-edit="pin"/);
});

test("renderBulkPriorityMenu has a button per priority with set hooks", () => {
  const html = renderBulkPriorityMenu();
  for (const p of BULK_PRIORITIES) {
    assert.ok(html.includes(`data-bulk-set-prio="${p}"`), `missing ${p}`);
  }
});

test("tag + due editors expose their inputs", () => {
  assert.match(renderBulkTagEditor(), /data-bulk-tag-input/);
  assert.match(renderBulkDueEditor(), /data-bulk-due-input/);
});

// --- F47: bulk pin menu + live due preview ---------------------------------

test("renderBulkPinMenu has pin-all (1) and unpin-all (0) actions", () => {
  const html = renderBulkPinMenu();
  assert.match(html, /data-bulk-set-pin="1"/);
  assert.match(html, /data-bulk-set-pin="0"/);
  assert.match(html, /Pin all/);
  assert.match(html, /Unpin all/);
});

test("renderBulkDueEditor exposes a live-preview slot", () => {
  const html = renderBulkDueEditor();
  // F47: main.ts fills this from /api/parse-date as you type
  assert.match(html, /data-bulk-due-preview/);
  // the input + the hint are still present
  assert.match(html, /data-bulk-due-input/);
  assert.match(html, /empty \+ Enter clears/);
});

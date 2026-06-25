import { test } from "node:test";
import assert from "node:assert/strict";
import {
  longestChainPath,
  renderChainDrill,
  renderUnblockedPicker,
  type DepStatsTask,
  type ChainNode,
} from "../src/deps.ts";

test("longestChainPath returns the head-to-root path of the deepest chain", () => {
  // #4 -> #3 -> #2 -> #1 (a straight chain of open blockers). #1 is the root.
  const tasks: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1] },
    { id: 3, done: false, depends_on: [2] },
    { id: 4, done: false, depends_on: [3] },
  ];
  assert.deepEqual(longestChainPath(tasks), [4, 3, 2, 1]);
});

test("longestChainPath is empty for a flat graph", () => {
  const tasks: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false },
  ];
  assert.deepEqual(longestChainPath(tasks), []);
});

test("longestChainPath skips done blockers (they don't extend depth)", () => {
  // #3 depends on done #2 -> not blocked; the only open chain is #4 -> #1.
  const tasks: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: true },
    { id: 3, done: false, depends_on: [2] },
    { id: 4, done: false, depends_on: [1] },
  ];
  assert.deepEqual(longestChainPath(tasks), [4, 1]);
});

test("longestChainPath picks the deeper branch at a fork", () => {
  // #5 depends on #4 (deep: ->#3->#2) and #1 (shallow). Deeper branch wins.
  const tasks: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false },
    { id: 3, done: false, depends_on: [2] },
    { id: 4, done: false, depends_on: [3] },
    { id: 5, done: false, depends_on: [1, 4] },
  ];
  assert.deepEqual(longestChainPath(tasks), [5, 4, 3, 2]);
});

test("longestChainPath terminates on a cycle", () => {
  // #1 <-> #2 cycle, both undone. Must not loop forever; returns a bounded path.
  const tasks: DepStatsTask[] = [
    { id: 1, done: false, depends_on: [2] },
    { id: 2, done: false, depends_on: [1] },
  ];
  const path = longestChainPath(tasks);
  assert.ok(path.length <= 2);
  // no id repeats
  assert.equal(new Set(path).size, path.length);
});

test("renderChainDrill emits a jump button per node with the right ids", () => {
  const nodes: ChainNode[] = [
    { id: 4, title: "ship release" },
    { id: 3, title: "write changelog" },
    { id: 1, title: "fix the bug" },
  ];
  const html = renderChainDrill(nodes);
  for (const id of [4, 3, 1]) {
    assert.match(html, new RegExp(`data-chain-jump="${id}"`));
  }
  // the last node is tagged as the root blocker
  assert.match(html, /is-root/);
  // two arrows between three nodes
  assert.equal((html.match(/chain-arrow/g) ?? []).length, 2);
});

test("renderChainDrill escapes titles", () => {
  const html = renderChainDrill([{ id: 1, title: "<script>" }]);
  assert.doesNotMatch(html, /<script>/);
  assert.match(html, /&lt;script&gt;/);
});

test("renderChainDrill is empty for no nodes", () => {
  assert.equal(renderChainDrill([]), "");
});

// --- F65: search highlight in the chain drill ------------------------------

test("renderChainDrill marks the matched query in node titles", () => {
  const nodes: ChainNode[] = [
    { id: 4, title: "ship release" },
    { id: 1, title: "fix the bug" },
  ];
  const html = renderChainDrill(nodes, "bug");
  // the matched subsequence is wrapped in <mark>
  assert.match(html, /<mark>bug<\/mark>/);
  // a non-matching node title carries no mark
  assert.match(html, /ship release/);
});

test("renderChainDrill with no query leaves titles unmarked (plain escaped)", () => {
  const html = renderChainDrill([{ id: 1, title: "fix the bug" }]);
  assert.doesNotMatch(html, /<mark>/);
  assert.match(html, /fix the bug/);
});

test("renderChainDrill still escapes while highlighting", () => {
  // a query that matches inside a dangerous title must not break escaping
  const html = renderChainDrill([{ id: 1, title: "<b>x</b>" }], "x");
  assert.doesNotMatch(html, /<b>/);
  assert.match(html, /&lt;b&gt;/);
  assert.match(html, /<mark>x<\/mark>/);
});

// --- F62: newly-unblocked picker -------------------------------------------

test("renderUnblockedPicker emits a jump button per unblocked task", () => {
  const nodes: ChainNode[] = [
    { id: 2, title: "write the docs" },
    { id: 5, title: "ship it" },
  ];
  const html = renderUnblockedPicker(nodes);
  assert.match(html, /data-unblock-jump="2"/);
  assert.match(html, /data-unblock-jump="5"/);
  assert.match(html, /write the docs/);
  // it's a flat list — no chain arrows between rows
  assert.doesNotMatch(html, /chain-arrow/);
});

test("renderUnblockedPicker escapes titles", () => {
  const html = renderUnblockedPicker([{ id: 1, title: "<img src=x>" }]);
  assert.doesNotMatch(html, /<img/);
  assert.match(html, /&lt;img/);
});

test("renderUnblockedPicker is empty for no nodes", () => {
  assert.equal(renderUnblockedPicker([]), "");
});

test("renderUnblockedPicker marks the matched query in titles", () => {
  const html = renderUnblockedPicker([{ id: 2, title: "write the docs" }], "docs");
  assert.match(html, /<mark>docs<\/mark>/);
});

test("renderUnblockedPicker with no query leaves titles unmarked", () => {
  const html = renderUnblockedPicker([{ id: 2, title: "write the docs" }]);
  assert.doesNotMatch(html, /<mark>/);
  assert.match(html, /write the docs/);
});

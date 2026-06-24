import { test } from "node:test";
import assert from "node:assert/strict";
import {
  buildAdjacency,
  canReach,
  validateAddDep,
  currentDeps,
  withDepAdded,
  withDepRemoved,
  depCandidates,
  depChipLabel,
  renderDepEditor,
  renderDepCandidates,
  type DepGraphTask,
} from "../src/depedit.ts";

// 1 <- 2 <- 3  (3 depends on 2, 2 depends on 1)
const chain: DepGraphTask[] = [
  { id: 1, title: "root", done: false },
  { id: 2, title: "mid", done: false, depends_on: [1] },
  { id: 3, title: "leaf", done: false, depends_on: [2] },
  { id: 4, title: "free", done: true },
];

test("buildAdjacency maps each id to its deps", () => {
  const adj = buildAdjacency(chain);
  assert.deepEqual(adj.get(3), [2]);
  assert.deepEqual(adj.get(1), []);
  assert.deepEqual(adj.get(4), []);
});

test("canReach follows transitive edges", () => {
  const adj = buildAdjacency(chain);
  assert.equal(canReach(adj, 3, 1), true); // 3 -> 2 -> 1
  assert.equal(canReach(adj, 1, 3), false); // not the other way
  assert.equal(canReach(adj, 2, 2), true); // reflexive
});

test("canReach tolerates unknown ids", () => {
  const adj = buildAdjacency(chain);
  assert.equal(canReach(adj, 99, 1), false);
});

test("validateAddDep rejects a self-reference", () => {
  const r = validateAddDep(chain, 2, 2);
  assert.equal(r.ok, false);
  if (!r.ok) assert.equal(r.reason, "self");
});

test("validateAddDep rejects an unknown id", () => {
  const r = validateAddDep(chain, 2, 99);
  assert.equal(r.ok, false);
  if (!r.ok) assert.equal(r.reason, "missing");
});

test("validateAddDep rejects an existing blocker", () => {
  const r = validateAddDep(chain, 3, 2); // 3 already depends on 2
  assert.equal(r.ok, false);
  if (!r.ok) assert.equal(r.reason, "duplicate");
});

test("validateAddDep rejects a cycle", () => {
  // Adding "1 depends on 3" cycles: 3 already reaches 1.
  const r = validateAddDep(chain, 1, 3);
  assert.equal(r.ok, false);
  if (!r.ok) assert.equal(r.reason, "cycle");
});

test("validateAddDep accepts a fresh, acyclic blocker", () => {
  // 1 depends on 4 (4 is free) — fine.
  const r = validateAddDep(chain, 1, 4);
  assert.equal(r.ok, true);
});

test("currentDeps returns a copy of a task's blockers", () => {
  assert.deepEqual(currentDeps(chain, 3), [2]);
  assert.deepEqual(currentDeps(chain, 1), []);
  assert.deepEqual(currentDeps(chain, 99), []);
});

test("withDepAdded / withDepRemoved are pure", () => {
  const base = [1, 2];
  assert.deepEqual(withDepAdded(base, 3), [1, 2, 3]);
  assert.deepEqual(withDepAdded(base, 2), [1, 2]); // dup ignored
  assert.deepEqual(withDepRemoved(base, 1), [2]);
  assert.deepEqual(base, [1, 2]); // unmutated
});

test("depCandidates excludes self, existing deps, and cycle-formers", () => {
  // Candidates for task 1: can't be 1 (self), can't be 2 or 3 (they'd cycle),
  // so only 4 qualifies.
  const cands = depCandidates(chain, 1);
  assert.deepEqual(
    cands.map((t) => t.id),
    [4],
  );
});

test("depCandidates ranks open before done, then by id", () => {
  const tasks: DepGraphTask[] = [
    { id: 1, title: "a", done: true },
    { id: 2, title: "b", done: false },
    { id: 3, title: "c", done: false },
    { id: 10, title: "target", done: false },
  ];
  const cands = depCandidates(tasks, 10);
  assert.deepEqual(
    cands.map((t) => t.id),
    [2, 3, 1], // open 2,3 first (by id), then done 1
  );
});

test("depCandidates filters by id or title query", () => {
  const tasks: DepGraphTask[] = [
    { id: 1, title: "buy milk", done: false },
    { id: 2, title: "walk dog", done: false },
    { id: 10, title: "target", done: false },
  ];
  assert.deepEqual(
    depCandidates(tasks, 10, "milk").map((t) => t.id),
    [1],
  );
  assert.deepEqual(
    depCandidates(tasks, 10, "2").map((t) => t.id),
    [2],
  );
});

test("depChipLabel clamps long titles", () => {
  const long = depChipLabel({ id: 5, title: "x".repeat(40), done: false });
  assert.ok(long.length < 40);
  assert.match(long, /^#5 /);
  assert.match(long, /…$/);
});

test("renderDepEditor shows chips for current blockers + an input", () => {
  const html = renderDepEditor(chain, 3); // 3 depends on 2
  assert.match(html, /data-dep-remove="2"/);
  assert.match(html, /data-dep-input/);
});

test("renderDepEditor shows an empty state when no blockers", () => {
  const html = renderDepEditor(chain, 1);
  assert.match(html, /No blockers yet/);
});

test("renderDepCandidates marks the active row + carries ids", () => {
  const cands = depCandidates(chain, 1); // [4]
  const html = renderDepCandidates(cands, 0);
  assert.match(html, /is-active/);
  assert.match(html, /data-dep-cand="4"/);
});

test("renderDepCandidates is empty for no candidates", () => {
  assert.equal(renderDepCandidates([], 0), "");
});

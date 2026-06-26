import { test } from "node:test";
import assert from "node:assert/strict";
import {
  buildCohort,
  applyCohort,
  cohortCount,
  renderCohortChipBody,
  cohortChipTitle,
  type CohortFocus,
} from "../src/cohort.ts";
import type { DepStatsTask } from "../src/deps.ts";

// A small graph: #2, #3, #4 all depend on #1 (so #1 is a chokepoint with 3
// waiters). #5 depends on #4 (a deeper waiter, NOT a direct dependent of #1).
function graph(): DepStatsTask[] {
  return [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1] },
    { id: 3, done: false, depends_on: [1] },
    { id: 4, done: false, depends_on: [1] },
    { id: 5, done: false, depends_on: [4] },
  ];
}

test("buildCohort collects the direct undone dependents of the source", () => {
  const c = buildCohort(graph(), 1);
  assert.notEqual(c, null);
  assert.equal(c!.sourceId, 1);
  // #2, #3, #4 wait directly on #1; #5 waits on #4 (not #1) so it's excluded.
  assert.deepEqual(c!.ids, [2, 3, 4]);
});

test("buildCohort preserves store order", () => {
  const ts: DepStatsTask[] = [
    { id: 10, done: false },
    { id: 7, done: false, depends_on: [10] },
    { id: 3, done: false, depends_on: [10] },
  ];
  const c = buildCohort(ts, 10);
  assert.deepEqual(c!.ids, [7, 3]); // input order, not sorted
});

test("buildCohort returns null when nothing waits on the source", () => {
  assert.equal(buildCohort(graph(), 5), null); // nothing depends on #5
  assert.equal(buildCohort(graph(), 999), null); // unknown id
});

test("buildCohort excludes done dependents", () => {
  const ts: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: true, depends_on: [1] }, // done — not waiting
    { id: 3, done: false, depends_on: [1] },
  ];
  const c = buildCohort(ts, 1);
  assert.deepEqual(c!.ids, [3]);
});

test("applyCohort narrows a list to the id set, preserving order", () => {
  const tasks = [{ id: 1 }, { id: 2 }, { id: 3 }, { id: 4 }];
  const out = applyCohort(tasks, new Set([3, 1]));
  assert.deepEqual(out.map((t) => t.id), [1, 3]); // input order, not set order
});

test("applyCohort with an empty set yields nothing", () => {
  assert.deepEqual(applyCohort([{ id: 1 }], new Set<number>()), []);
});

test("cohortCount reads the plural-agnostic 'N waiting'", () => {
  assert.equal(cohortCount({ sourceId: 1, ids: [2, 3, 4] }), "3 waiting");
  assert.equal(cohortCount({ sourceId: 1, ids: [2] }), "1 waiting");
});

test("renderCohortChipBody shows the count, source, and a clear ×", () => {
  const focus: CohortFocus = { sourceId: 7, ids: [2, 3] };
  const html = renderCohortChipBody(focus);
  assert.match(html, /2 waiting/);
  assert.match(html, /on #7/);
  assert.match(html, /lens-x/); // the trailing clear glyph
});

test("renderCohortChipBody is empty for a null focus", () => {
  assert.equal(renderCohortChipBody(null), "");
});

test("cohortChipTitle reads naturally for one vs many waiters", () => {
  assert.match(cohortChipTitle({ sourceId: 4, ids: [1] }), /the 1 task waiting on #4/);
  assert.match(cohortChipTitle({ sourceId: 4, ids: [1, 2] }), /the 2 tasks waiting on #4/);
});

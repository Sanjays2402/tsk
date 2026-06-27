import { test } from "node:test";
import assert from "node:assert/strict";
import {
  buildCohort,
  applyCohort,
  cohortCount,
  renderCohortChipBody,
  cohortChipTitle,
  reconcileCohort,
  renderCohortFocusButton,
  cohortSummary,
  renderCohortHelp,
  pushCohortHistory,
  popCohortHistory,
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

// --- F101: reconcileCohort -------------------------------------------------

test("reconcileCohort with no prior focus is a no-op", () => {
  const r = reconcileCohort(graph(), null);
  assert.deepEqual(r, { focus: null, cleared: false });
});

test("reconcileCohort refreshes the id set against the fresh graph", () => {
  // Was focused on #1 with [2,3,4]; now #3 completed externally, #6 newly waits.
  const prev: CohortFocus = { sourceId: 1, ids: [2, 3, 4] };
  const fresh: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1] },
    { id: 3, done: true, depends_on: [1] }, // completed since the snapshot
    { id: 4, done: false, depends_on: [1] },
    { id: 6, done: false, depends_on: [1] }, // newly added waiter
  ];
  const r = reconcileCohort(fresh, prev);
  assert.equal(r.cleared, false);
  assert.notEqual(r.focus, null);
  assert.equal(r.focus!.sourceId, 1);
  // #3 dropped (done), #6 picked up — the live set, not the stale snapshot.
  assert.deepEqual(r.focus!.ids, [2, 4, 6]);
});

test("reconcileCohort clears when every waiter completed", () => {
  const prev: CohortFocus = { sourceId: 1, ids: [2, 3] };
  const fresh: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: true, depends_on: [1] },
    { id: 3, done: true, depends_on: [1] },
  ];
  const r = reconcileCohort(fresh, prev);
  assert.deepEqual(r, { focus: null, cleared: true });
});

test("reconcileCohort clears when the chokepoint itself was completed", () => {
  // #1 is done now, so nothing is "waiting on" an open #1 — the focus drops.
  const prev: CohortFocus = { sourceId: 1, ids: [2, 3] };
  const fresh: DepStatsTask[] = [
    { id: 1, done: true },
    { id: 2, done: false, depends_on: [1] },
    { id: 3, done: false, depends_on: [1] },
  ];
  const r = reconcileCohort(fresh, prev);
  assert.equal(r.cleared, true);
  assert.equal(r.focus, null);
});

test("reconcileCohort clears when the chokepoint was deleted", () => {
  const prev: CohortFocus = { sourceId: 9, ids: [2] };
  const r = reconcileCohort(graph(), prev); // #9 isn't in the graph
  assert.deepEqual(r, { focus: null, cleared: true });
});

// --- F102: renderCohortFocusButton -----------------------------------------

test("renderCohortFocusButton carries the shared data-cohort-focus hook", () => {
  const html = renderCohortFocusButton(7);
  // Same hook the F96 sidebar focus button uses, so the existing wiring fires.
  assert.match(html, /data-cohort-focus="7"/);
  assert.match(html, /focus these/);
  assert.match(html, /<button/);
});

test("renderCohortFocusButton names the source task in its title", () => {
  const html = renderCohortFocusButton(42);
  assert.match(html, /waiting on #42/);
  // Title + aria-label both present for hover + a11y.
  assert.match(html, /aria-label=/);
});

// --- F103: cohortSummary ----------------------------------------------------

test("cohortSummary reads as 'N waiting on #M'", () => {
  assert.equal(cohortSummary({ sourceId: 1, ids: [2, 3, 4] }), "3 waiting on #1");
  assert.equal(cohortSummary({ sourceId: 5, ids: [9] }), "1 waiting on #5");
});

// --- F108: cohort back-stack -----------------------------------------------

test("pushCohortHistory appends a source id and returns a new array", () => {
  const a: number[] = [];
  const b = pushCohortHistory(a, 1);
  assert.deepEqual(b, [1]);
  assert.notEqual(a, b); // new array, original untouched
  assert.deepEqual(pushCohortHistory(b, 3), [1, 3]);
});

test("pushCohortHistory de-dupes a no-op push of the same top id", () => {
  // Re-focusing the cohort already on top shouldn't grow the stack.
  assert.deepEqual(pushCohortHistory([1, 3], 3), [1, 3]);
  // But a non-top repeat IS pushed (you genuinely revisited it).
  assert.deepEqual(pushCohortHistory([1, 3], 1), [1, 3, 1]);
});

test("pushCohortHistory caps the depth, dropping the oldest", () => {
  const stack = pushCohortHistory([1, 2, 3], 4, 3);
  assert.deepEqual(stack, [2, 3, 4]); // capped at 3, #1 evicted
});

test("popCohortHistory returns the most recent ancestor that still has waiters", () => {
  // Stack: focused #1 then #4. Step back -> rebuild #4's cohort from the graph.
  const back = popCohortHistory(graph(), [1, 4]);
  assert.equal(back.focus!.sourceId, 4);
  assert.deepEqual(back.focus!.ids, [5]); // #5 waits on #4 in the fixture
  assert.deepEqual(back.stack, [1]); // #4 popped, #1 remains
});

test("popCohortHistory skips dead ancestors and lands on the nearest live one", () => {
  // #3 has no dependents now (dead ancestor); #1 still has waiters. Stepping
  // back from [1, 3] skips #3 and lands on #1's live cohort.
  const back = popCohortHistory(graph(), [1, 3]);
  assert.equal(back.focus!.sourceId, 1);
  assert.deepEqual(back.focus!.ids, [2, 3, 4]);
  assert.deepEqual(back.stack, []); // both popped to reach a live one
});

test("popCohortHistory clears when no ancestor still holds a cohort", () => {
  // Neither #5 nor #3 has open dependents -> empties to a null focus.
  const back = popCohortHistory(graph(), [5, 3]);
  assert.deepEqual(back, { stack: [], focus: null });
});

test("popCohortHistory on an empty stack is a clean null", () => {
  assert.deepEqual(popCohortHistory(graph(), []), { stack: [], focus: null });
});

test("renderCohortChipBody prepends a back glyph only when history is non-empty", () => {
  const focus: CohortFocus = { sourceId: 7, ids: [2, 3] };
  // No history -> no back glyph (byte-identical to the F96 chip).
  assert.equal(renderCohortChipBody(focus, 0).includes("data-cohort-back"), false);
  assert.equal(renderCohortChipBody(focus), renderCohortChipBody(focus, 0));
  // With history -> a ‹ back affordance precedes the count.
  const withBack = renderCohortChipBody(focus, 2);
  assert.match(withBack, /data-cohort-back/);
  assert.ok(withBack.indexOf("data-cohort-back") < withBack.indexOf("2 waiting"));
});

// --- F113: cohort back-stack depth badge ------------------------------------

test("renderCohortChipBody shows a depth badge only when history is >1 deep", () => {
  const focus: CohortFocus = { sourceId: 7, ids: [2, 3] };
  // Depth 0 / 1 render the bare ‹ glyph — no depth numeral.
  assert.doesNotMatch(renderCohortChipBody(focus, 0), /cohort-back-depth/);
  assert.doesNotMatch(renderCohortChipBody(focus, 1), /cohort-back-depth/);
  // Depth 2+ surfaces the count so a multi-step drill shows how deep you are.
  const deep = renderCohortChipBody(focus, 3);
  assert.match(deep, /cohort-back-depth/);
  assert.match(deep, /cohort-back-depth[^>]*>3</); // the numeral is the depth
});

test("renderCohortChipBody depth badge sits inside the back glyph, before the count", () => {
  const focus: CohortFocus = { sourceId: 4, ids: [1, 2] };
  const html = renderCohortChipBody(focus, 5);
  // The depth numeral rides the back glyph (after data-cohort-back) and still
  // precedes the "N waiting" count — purely a readout, not a new hit target.
  assert.ok(html.indexOf("data-cohort-back") < html.indexOf("cohort-back-depth"));
  assert.ok(html.indexOf("cohort-back-depth") < html.indexOf("2 waiting"));
  // The title gains the depth so a hover explains the count.
  assert.match(html, /5 in history/);
});

test("renderCohortChipBody depth-1 keeps the single-level title", () => {
  const focus: CohortFocus = { sourceId: 4, ids: [1] };
  const html = renderCohortChipBody(focus, 1);
  assert.match(html, /Back to the previous cohort"/); // no "(N in history)" suffix
  assert.doesNotMatch(html, /in history/);
});

// --- F117: cohort breadcrumb in the help (`?`) overlay ----------------------

test("renderCohortHelp summarizes the active cohort and reuses cohortSummary", () => {
  const focus: CohortFocus = { sourceId: 1, ids: [2, 3, 4] };
  const html = renderCohortHelp(focus);
  assert.match(html, /Cohort focus:/);
  // It bolds the SAME summary the Cmd-K command + chip use, so they can't drift.
  assert.match(html, new RegExp(`<strong>${cohortSummary(focus)}</strong>`));
  assert.match(html, /<strong>3 waiting on #1<\/strong>/);
});

test("renderCohortHelp is empty with no active cohort", () => {
  assert.equal(renderCohortHelp(null), "");
  assert.equal(renderCohortHelp(null, 4), ""); // depth is irrelevant with no focus
});

test("renderCohortHelp appends the back-stack depth note when history is deep", () => {
  const focus: CohortFocus = { sourceId: 5, ids: [6, 7] };
  const html = renderCohortHelp(focus, 2);
  assert.match(html, /help-cohort-history/);
  assert.match(html, /2 in history/);
  assert.match(html, /&#8249;2/); // the ‹ back glyph echoes the F113 chip badge
});

test("renderCohortHelp omits the history note at depth 0", () => {
  const focus: CohortFocus = { sourceId: 5, ids: [6] };
  const html = renderCohortHelp(focus, 0);
  assert.doesNotMatch(html, /help-cohort-history/);
  assert.doesNotMatch(html, /in history/);
  // Byte-identical to the default-arg form so a no-history cohort reads plainly.
  assert.equal(html, renderCohortHelp(focus));
});

test("renderCohortHelp escapes nothing injectable but keeps the summary intact at depth 1", () => {
  // Depth 1 means a single back step exists but no multi-level badge note is
  // shown (mirrors the chip, which only badges depth > 1) — the line still reads
  // the plain summary plus the history note (any history is worth surfacing here).
  const focus: CohortFocus = { sourceId: 9, ids: [10] };
  const html = renderCohortHelp(focus, 1);
  assert.match(html, /<strong>1 waiting on #9<\/strong>/);
  assert.match(html, /1 in history/);
});

// --- F122: the help breadcrumb history note is an actionable button ----------

test("renderCohortHelp history note is a button carrying the shared data-cohort-back hook", () => {
  const focus: CohortFocus = { sourceId: 1, ids: [2, 3] };
  const html = renderCohortHelp(focus, 2);
  // It's a real <button> (not an inert span) so the `?` overlay can step back.
  assert.match(html, /<button[^>]*class="help-cohort-history"/);
  // Same hook the chip's ‹ glyph + Escape drive, so the existing cohortBack wiring fires.
  assert.match(html, /data-cohort-back/);
  // The visible text + glyph are unchanged from F117 so styling/readout carry over.
  assert.match(html, /&#8249;2 in history/);
});

test("renderCohortHelp emits no button when there's no history to step back to", () => {
  const focus: CohortFocus = { sourceId: 5, ids: [6] };
  const html = renderCohortHelp(focus, 0);
  // Depth 0 -> plain "Cohort focus: …" with no actionable back button.
  assert.doesNotMatch(html, /data-cohort-back/);
  assert.doesNotMatch(html, /<button/);
});

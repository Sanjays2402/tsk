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
  renderCohortPanelLine,
  renderCohortTrail,
  jumpCohortHistory,
  pushCohortHistory,
  popCohortHistory,
  cohortTrailKeyTarget,
  densestCohortAncestorIndex,
  cohortTrailCounts,
  formatCohortTrailText,
  formatCohortTrailMarkdown,
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

// --- F126: the stats-panel active-cohort line --------------------------------

test("renderCohortPanelLine summarizes the focus and reuses cohortSummary", () => {
  const focus: CohortFocus = { sourceId: 1, ids: [2, 3, 4] };
  const html = renderCohortPanelLine(focus);
  assert.match(html, /Focused/);
  // Reuses cohortSummary so the panel line can't drift from the chip / help / Cmd-K.
  assert.match(html, new RegExp(cohortSummary(focus)));
  assert.match(html, /3 waiting on #1/);
});

test("renderCohortPanelLine carries the data-cohort-clear hook + a clear glyph", () => {
  const html = renderCohortPanelLine({ sourceId: 7, ids: [8] });
  // The line is a button that clears the focus on click (panel sibling of the chip ×).
  assert.match(html, /<button/);
  assert.match(html, /data-cohort-clear/);
  assert.match(html, /&times;/);
});

test("renderCohortPanelLine reads naturally for one vs many waiters in its title", () => {
  assert.match(renderCohortPanelLine({ sourceId: 4, ids: [1] }), /the task waiting on #4/);
  assert.match(renderCohortPanelLine({ sourceId: 4, ids: [1, 2] }), /the tasks waiting on #4/);
});

test("renderCohortPanelLine is empty for a null focus", () => {
  assert.equal(renderCohortPanelLine(null), "");
  assert.equal(renderCohortPanelLine(null, 3), ""); // depth is irrelevant with no focus
});

// --- F127: the panel cohort line gains a back-step when history exists --------

test("renderCohortPanelLine omits the back button at depth 0", () => {
  const html = renderCohortPanelLine({ sourceId: 1, ids: [2, 3] }, 0);
  assert.doesNotMatch(html, /data-cohort-back/);
  assert.doesNotMatch(html, /stat-cohort-back/);
  // Byte-identical to the no-history default-arg form so a flat drill reads plainly.
  assert.equal(html, renderCohortPanelLine({ sourceId: 1, ids: [2, 3] }));
});

test("renderCohortPanelLine grows a ‹ back button when the cohort has history", () => {
  const html = renderCohortPanelLine({ sourceId: 1, ids: [2, 3] }, 1);
  // A real button carrying the SAME hook the chip's ‹ glyph + Escape + help drive.
  assert.match(html, /<button[^>]*class="stat-cohort-back"/);
  assert.match(html, /data-cohort-back/);
  assert.match(html, /&#8249;/); // the ‹ glyph
  // The back button sits BEFORE the clear readout in the row.
  assert.ok(html.indexOf("stat-cohort-back") < html.indexOf("stat-cohort-line"));
});

test("renderCohortPanelLine badges the back depth once the stack is deeper than one", () => {
  const html = renderCohortPanelLine({ sourceId: 5, ids: [6] }, 3);
  assert.match(html, /&#8249;3/); // "‹3" depth badge mirrors the F113 chip badge
  assert.match(html, /3 in history/); // and the title spells it out
});

// --- F128: the panel cohort line gains a "walk" into the chain-drill ----------

test("renderCohortPanelLine carries a walk button on the shared data-waiting-walk hook", () => {
  const html = renderCohortPanelLine({ sourceId: 7, ids: [8, 9] });
  // The walk button reuses the SAME hook the sidebar chokepoint rows use, so it
  // routes through openChainDrill(sourceId, "dependent") with zero new dispatch.
  assert.match(html, /<button[^>]*class="stat-cohort-walk"/);
  assert.match(html, /data-waiting-walk="7"/);
  // The walk button sits AFTER the clear readout in the row.
  assert.ok(html.indexOf("stat-cohort-line") < html.indexOf("stat-cohort-walk"));
});

test("renderCohortPanelLine row holds disjoint back / clear / walk buttons in order", () => {
  const html = renderCohortPanelLine({ sourceId: 2, ids: [3] }, 2);
  assert.match(html, /<div class="stat-cohort-row">/);
  // Each affordance is its own button (a button can't nest another), in order.
  const back = html.indexOf("data-cohort-back");
  const clear = html.indexOf("data-cohort-clear");
  const walk = html.indexOf("data-waiting-walk");
  assert.ok(back >= 0 && clear >= 0 && walk >= 0);
  assert.ok(back < clear && clear < walk);
});

// --- F132: cohort breadcrumb trail + multi-step jump -----------------------

test("renderCohortTrail is empty without a focus or without history", () => {
  assert.equal(renderCohortTrail(null, [1, 2]), "");
  assert.equal(renderCohortTrail({ sourceId: 3, ids: [4] }, []), "");
  // A null focus stays empty even with history.
  assert.equal(renderCohortTrail(null, []), "");
});

test("renderCohortTrail lays out ancestors then the current cohort, in order", () => {
  const html = renderCohortTrail({ sourceId: 9, ids: [10] }, [1, 4]);
  // Two ancestor step buttons indexed by position, then the current segment.
  assert.match(html, /data-cohort-jump="0"[^>]*>#1</);
  assert.match(html, /data-cohort-jump="1"[^>]*>#4</);
  assert.match(html, /class="cohort-trail-current"[^>]*aria-current="step"[^>]*>#9</);
  // Order: #1 before #4 before the current #9.
  assert.ok(html.indexOf("#1") < html.indexOf("#4"));
  assert.ok(html.indexOf("#4") < html.indexOf(">#9<"));
});

test("renderCohortTrail renders one separator fewer than segments", () => {
  // 2 ancestors + 1 current = 3 segments -> 2 separators.
  const html = renderCohortTrail({ sourceId: 9, ids: [10] }, [1, 4]);
  const seps = html.match(/cohort-trail-sep/g) ?? [];
  assert.equal(seps.length, 2);
});

test("renderCohortTrail escapes nothing odd but the current is non-interactive", () => {
  const html = renderCohortTrail({ sourceId: 2, ids: [3] }, [1]);
  // The current segment is a span, not a button (you're already on it).
  assert.match(html, /<span class="cohort-trail-current"/);
  assert.doesNotMatch(html, /data-cohort-jump="1"/); // no jump for the current
});

test("jumpCohortHistory lands on the targeted ancestor and trims the stack", () => {
  // #2,#3 wait on #1; #4 waits on #1; the stack is [1, 4] (drilled 1 then 4).
  const ts: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1] },
    { id: 4, done: false },
    { id: 5, done: false, depends_on: [4] },
  ];
  // Jump to index 0 (the #1 ancestor): focus rebuilds for #1, stack empties.
  const j = jumpCohortHistory(ts, [1, 4], 0);
  assert.notEqual(j.focus, null);
  assert.equal(j.focus!.sourceId, 1);
  assert.deepEqual(j.stack, []);
});

test("jumpCohortHistory to the top index returns that ancestor", () => {
  const ts: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1] },
    { id: 4, done: false },
    { id: 5, done: false, depends_on: [4] },
  ];
  // index 1 targets #4; the remaining stack is everything before it ([1]).
  const j = jumpCohortHistory(ts, [1, 4], 1);
  assert.equal(j.focus!.sourceId, 4);
  assert.deepEqual(j.stack, [1]);
});

test("jumpCohortHistory skips a dead targeted ancestor to the nearest live one", () => {
  // #1 is done -> its cohort is dead; jumping to it should fall back to an
  // older live ancestor (#7, which still has a waiter #8).
  const ts: DepStatsTask[] = [
    { id: 7, done: false },
    { id: 8, done: false, depends_on: [7] },
    { id: 1, done: true }, // completed chokepoint -> dead cohort
  ];
  const j = jumpCohortHistory(ts, [7, 1], 1);
  assert.notEqual(j.focus, null);
  assert.equal(j.focus!.sourceId, 7); // skipped #1, landed on #7
  assert.deepEqual(j.stack, []);
});

test("jumpCohortHistory is a no-op for an out-of-range index", () => {
  const ts: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1] },
  ];
  assert.deepEqual(jumpCohortHistory(ts, [1], -1), { stack: [1], focus: null });
  assert.deepEqual(jumpCohortHistory(ts, [1], 5), { stack: [1], focus: null });
});

// --- F133: panel pin star --------------------------------------------------

test("renderCohortPanelLine omits the pin star when pinned is undefined", () => {
  const html = renderCohortPanelLine({ sourceId: 1, ids: [2] });
  assert.doesNotMatch(html, /data-cohort-pin/);
  // Byte-identical to the historyless no-star form.
  assert.equal(html, renderCohortPanelLine({ sourceId: 1, ids: [2] }, 0));
});

test("renderCohortPanelLine shows a hollow star (un-pinned) and a filled star (pinned)", () => {
  const unpinned = renderCohortPanelLine({ sourceId: 1, ids: [2] }, 0, false);
  assert.match(unpinned, /data-cohort-pin="1"/);
  assert.match(unpinned, /\u2606/); // ☆ hollow when not pinned
  assert.match(unpinned, /aria-pressed="false"/);
  const pinned = renderCohortPanelLine({ sourceId: 1, ids: [2] }, 0, true);
  assert.match(pinned, /\u2605/); // ★ filled when pinned
  assert.match(pinned, /is-pinned/);
  assert.match(pinned, /aria-pressed="true"/);
});

test("renderCohortPanelLine pin star is the last disjoint sibling in the row", () => {
  const html = renderCohortPanelLine({ sourceId: 2, ids: [3] }, 1, false);
  // Order: back < clear < walk < pin, each its own button.
  const back = html.indexOf("data-cohort-back");
  const clear = html.indexOf("data-cohort-clear");
  const walk = html.indexOf("data-waiting-walk");
  const pin = html.indexOf("data-cohort-pin");
  assert.ok(back >= 0 && clear >= 0 && walk >= 0 && pin >= 0);
  assert.ok(back < clear && clear < walk && walk < pin);
});

// --- F137: cohort trail keyboard navigation --------------------------------

test("cohortTrailKeyTarget steps to the most-recent ancestor (one level back)", () => {
  // History [1, 4, 9] (3 ancestors) -> a single step back targets index 2 (the
  // most-recent ancestor, #9 here), the same landing Escape/cohortBack gives.
  assert.equal(cohortTrailKeyTarget(3, "step"), 2);
  assert.equal(cohortTrailKeyTarget(1, "step"), 0);
});

test("cohortTrailKeyTarget leaps to the drill root (oldest ancestor)", () => {
  // "root" always targets index 0 — the origin of the drill — regardless of depth.
  assert.equal(cohortTrailKeyTarget(5, "root"), 0);
  assert.equal(cohortTrailKeyTarget(1, "root"), 0);
});

test("cohortTrailKeyTarget returns -1 for an empty history (nothing to step)", () => {
  // No back-stack -> the caller declines to act, in both directions.
  assert.equal(cohortTrailKeyTarget(0, "step"), -1);
  assert.equal(cohortTrailKeyTarget(0, "root"), -1);
  assert.equal(cohortTrailKeyTarget(-1, "step"), -1);
});

// --- F140: copy the cohort drill chain as text -----------------------------

test("formatCohortTrailText renders the whole chain ancestors-first then current", () => {
  const focus: CohortFocus = { sourceId: 9, ids: [10] };
  // History oldest-first [1, 4] + the current #9 -> "#1 › #4 › #9".
  assert.equal(formatCohortTrailText(focus, [1, 4]), "#1 \u203a #4 \u203a #9");
});

test("formatCohortTrailText is empty without a focus or without history", () => {
  assert.equal(formatCohortTrailText(null, [1, 2]), "");
  assert.equal(formatCohortTrailText({ sourceId: 3, ids: [4] }, []), "");
  // Matches renderCohortTrail's empty conditions so the two appear/vanish together.
  assert.equal(renderCohortTrail(null, [1, 2]), formatCohortTrailText(null, [1, 2]));
});

test("formatCohortTrailText reuses the trail separator glyph so they read alike", () => {
  const focus: CohortFocus = { sourceId: 2, ids: [3] };
  const text = formatCohortTrailText(focus, [1]);
  assert.equal(text, "#1 \u203a #2");
  // The on-screen trail names the same ids in the same order (the copy is faithful).
  const html = renderCohortTrail(focus, [1]);
  assert.ok(html.indexOf("#1") < html.indexOf(">#2<"));
});

test("renderCohortTrail appends a copy-chain button carrying the data-cohort-copy hook", () => {
  const html = renderCohortTrail({ sourceId: 9, ids: [10] }, [1, 4]);
  // A real button with the shared copy hook, sitting AFTER the current segment.
  assert.match(html, /<button[^>]*class="cohort-trail-copy"[^>]*data-cohort-copy/);
  assert.ok(html.indexOf("cohort-trail-current") < html.indexOf("data-cohort-copy"));
  // Its title carries the full chain so a hover previews what'll be copied.
  assert.match(html, /Copy the drill chain \(#1 \u203a #4 \u203a #9\)/);
  // F143: the title advertises the Alt-click markdown gesture.
  assert.match(html, /Alt-click for markdown/);
});

test("renderCohortTrail emits no copy button when there's no trail to copy", () => {
  // No history -> no trail and therefore no copy affordance.
  assert.equal(renderCohortTrail({ sourceId: 3, ids: [4] }, []), "");
  assert.equal(renderCohortTrail(null, [1]), "");
});

// --- F143: copy the cohort drill chain as markdown -------------------------

test("formatCohortTrailMarkdown renders the chain with arrow joins", () => {
  const focus: CohortFocus = { sourceId: 9, ids: [10] };
  // Same segment order as the plain text, but → (U+2192) joins instead of › .
  assert.equal(formatCohortTrailMarkdown(focus, [1, 4]), "#1 \u2192 #4 \u2192 #9");
});

test("formatCohortTrailMarkdown uses a distinct glyph from the plain text", () => {
  const focus: CohortFocus = { sourceId: 2, ids: [3] };
  const md = formatCohortTrailMarkdown(focus, [1]);
  const txt = formatCohortTrailText(focus, [1]);
  // The two formats name the same ids in the same order...
  assert.match(md, /#1.*#2/);
  assert.match(txt, /#1.*#2/);
  // ...but with different separators so a reader can tell which they pasted.
  assert.ok(md.includes("\u2192"));
  assert.ok(!md.includes("\u203a"));
  assert.ok(txt.includes("\u203a"));
  assert.ok(!txt.includes("\u2192"));
});

test("formatCohortTrailMarkdown is empty under the same conditions as the text form", () => {
  // No focus OR no history -> "" so F140's plain copy + F143's markdown copy
  // appear/vanish together.
  assert.equal(formatCohortTrailMarkdown(null, [1, 2]), "");
  assert.equal(formatCohortTrailMarkdown({ sourceId: 3, ids: [4] }, []), "");
  assert.equal(formatCohortTrailMarkdown(null, []), "");
});

// --- F147: jump to the densest ancestor in the cohort drill ----------------

test("densestCohortAncestorIndex returns the heaviest ancestor's index", () => {
  // history [1, 4, 9] with waiter counts 2, 7, 3 -> #4 (index 1) is densest.
  const counts: Record<number, number> = { 1: 2, 4: 7, 9: 3 };
  assert.equal(densestCohortAncestorIndex([1, 4, 9], (id) => counts[id] ?? 0), 1);
});

test("densestCohortAncestorIndex breaks a tie toward the oldest ancestor", () => {
  // #1 and #9 both have 5 waiters; the lower index (closer to the drill root)
  // wins so repeated presses are deterministic.
  const counts: Record<number, number> = { 1: 5, 4: 2, 9: 5 };
  assert.equal(densestCohortAncestorIndex([1, 4, 9], (id) => counts[id] ?? 0), 0);
});

test("densestCohortAncestorIndex ignores dead ancestors and empty history", () => {
  // All-dead ancestry (every count 0) -> -1, the caller declines to act.
  assert.equal(densestCohortAncestorIndex([1, 4], () => 0), -1);
  // Empty history -> -1.
  assert.equal(densestCohortAncestorIndex([], () => 9), -1);
  // A single live ancestor among dead ones is picked.
  const counts: Record<number, number> = { 1: 0, 4: 3, 9: 0 };
  assert.equal(densestCohortAncestorIndex([1, 4, 9], (id) => counts[id] ?? 0), 1);
});

test("densestCohortAncestorIndex result feeds jumpCohortHistory cleanly", () => {
  // A live winner here lands on a real cohort via jumpCohortHistory (skip-dead).
  // #2,#3,#4 wait on #1 (count 3); #5 waits on #4 (count 1). History [1, 4].
  const ts = graph();
  const idx = densestCohortAncestorIndex([1, 4], (id) => {
    const c = buildCohort(ts, id);
    return c ? c.ids.length : 0;
  });
  assert.equal(idx, 0); // #1 (3 waiters) beats #4 (1 waiter)
  const jump = jumpCohortHistory(ts, [1, 4], idx);
  assert.equal(jump.focus?.sourceId, 1);
});

// --- F150: per-segment waiter counts in the cohort trail -------------------

test("cohortTrailCounts maps each ancestor to its live waiter count, in order", () => {
  const counts: Record<number, number> = { 1: 2, 4: 7, 9: 3 };
  // Same oldest-first order renderCohortTrail walks; one entry per ancestor.
  assert.deepEqual(cohortTrailCounts([1, 4, 9], (id) => counts[id] ?? 0), [2, 7, 3]);
});

test("cohortTrailCounts returns [] for an empty history (no ancestors)", () => {
  assert.deepEqual(cohortTrailCounts([], () => 5), []);
});

test("cohortTrailCounts excludes the current focus (history-only, like F147)", () => {
  // The focus isn't in `history`, so its count never appears — the array length
  // equals the ancestor count, pairing 1:1 with the trail's ancestor segments.
  const counts: Record<number, number> = { 1: 4, 4: 6 };
  assert.deepEqual(cohortTrailCounts([1, 4], (id) => counts[id] ?? 0), [4, 6]);
});

test("renderCohortTrail wears a superscript waiter-count per ancestor when supplied", () => {
  // history [1, 4] with counts [2, 7]; current #9 has no count (it's "you are here").
  const html = renderCohortTrail({ sourceId: 9, ids: [10] }, [1, 4], [2, 7]);
  assert.match(html, /#1<sup class="cohort-trail-count" aria-hidden="true">2<\/sup>/);
  assert.match(html, /#4<sup class="cohort-trail-count" aria-hidden="true">7<\/sup>/);
  // The current segment stays a plain "you are here" with no count superscript.
  assert.match(html, /cohort-trail-current[^>]*>#9</);
  assert.doesNotMatch(html, /#9<sup/);
  // The aria-label surfaces the count for assistive tech.
  assert.match(html, /\(7 waiting\)/);
});

test("renderCohortTrail omits counts when no waiterCounts array is passed (byte-identical)", () => {
  const withArg = renderCohortTrail({ sourceId: 9, ids: [10] }, [1, 4]);
  const noArg = renderCohortTrail({ sourceId: 9, ids: [10] }, [1, 4], undefined);
  assert.equal(withArg, noArg);
  assert.doesNotMatch(withArg, /cohort-trail-count/);
});

test("renderCohortTrail skips a zero/missing per-segment count gracefully", () => {
  // count 0 for #1 (dead ancestor) -> no superscript; #4 keeps its 5.
  const html = renderCohortTrail({ sourceId: 9, ids: [10] }, [1, 4], [0, 5]);
  assert.doesNotMatch(html, /#1<sup/);
  assert.match(html, /#4<sup class="cohort-trail-count"[^>]*>5<\/sup>/);
  // A SHORTER array (only one entry for two ancestors) degrades: #4 gets nothing.
  const partial = renderCohortTrail({ sourceId: 9, ids: [10] }, [1, 4], [3]);
  assert.match(partial, /#1<sup[^>]*>3<\/sup>/);
  assert.doesNotMatch(partial, /#4<sup/);
});

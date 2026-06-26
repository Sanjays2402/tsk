import { test } from "node:test";
import assert from "node:assert/strict";
import {
  matchesLens,
  applyLens,
  lensMeta,
  renderLensChipBody,
  lensForDigit,
  lensDigit,
  activeLensSummary,
  renderActiveLensHelp,
  renderLensDigitMap,
  computeLensBreakdown,
  renderLensBreakdown,
  lensBreakdownPriority,
  LENS_BD_PRIORITIES,
  LENS_ORDER,
  type LensKind,
  type LensTask,
  type LensBreakdownTask,
} from "../src/lens.ts";
import { doneIndex } from "../src/deps.ts";

// A fixed "now" so the day-window math is deterministic: Wed 2026-06-24.
const NOW = new Date(2026, 5, 24, 9, 0, 0);

function tasks(): LensTask[] {
  return [
    { id: 1, done: false, due: "2026-06-20" }, // overdue (4 days ago)
    { id: 2, done: false, due: "2026-06-24" }, // today
    { id: 3, done: false, due: "2026-06-27" }, // this week (in 3d)
    { id: 4, done: false, due: "2026-07-15" }, // upcoming, NOT this week
    { id: 5, done: false }, // no due
    { id: 6, done: true, due: "2026-06-20" }, // done overdue — excluded
    { id: 7, done: false, depends_on: [5] }, // blocked by undone #5
  ];
}

test("matchesLens overdue: undone, due before today", () => {
  const ts = tasks();
  const done = doneIndex(ts);
  assert.equal(matchesLens(ts[0], "overdue", NOW, done), true); // #1
  assert.equal(matchesLens(ts[1], "overdue", NOW, done), false); // #2 today
  assert.equal(matchesLens(ts[5], "overdue", NOW, done), false); // #6 done
});

test("matchesLens today: undone, due is today", () => {
  const ts = tasks();
  const done = doneIndex(ts);
  assert.equal(matchesLens(ts[1], "today", NOW, done), true); // #2
  assert.equal(matchesLens(ts[0], "today", NOW, done), false); // #1 overdue
});

test("matchesLens week: today..today+6 inclusive, overdue excluded", () => {
  const ts = tasks();
  const done = doneIndex(ts);
  assert.equal(matchesLens(ts[1], "week", NOW, done), true); // #2 today counts
  assert.equal(matchesLens(ts[2], "week", NOW, done), true); // #3 in 3d
  assert.equal(matchesLens(ts[3], "week", NOW, done), false); // #4 too far out
  assert.equal(matchesLens(ts[0], "week", NOW, done), false); // #1 overdue excluded
});

test("matchesLens week: boundary at today+6 is included, today+7 is not", () => {
  const done = doneIndex([]);
  const edge: LensTask = { id: 1, done: false, due: "2026-06-30" }; // today+6
  const past: LensTask = { id: 2, done: false, due: "2026-07-01" }; // today+7
  assert.equal(matchesLens(edge, "week", NOW, done), true);
  assert.equal(matchesLens(past, "week", NOW, done), false);
});

test("matchesLens nodue: undone with no (parseable) due date", () => {
  const ts = tasks();
  const done = doneIndex(ts);
  assert.equal(matchesLens(ts[4], "nodue", NOW, done), true); // #5 no due
  assert.equal(matchesLens(ts[1], "nodue", NOW, done), false); // #2 has a due
  // a malformed due string counts as no-due (can't be scheduled)
  const bad: LensTask = { id: 9, done: false, due: "not-a-date" };
  assert.equal(matchesLens(bad, "nodue", NOW, done), true);
});

test("matchesLens blocked: undone with an open blocker (cross-task)", () => {
  const ts = tasks();
  const done = doneIndex(ts);
  assert.equal(matchesLens(ts[6], "blocked", NOW, done), true); // #7 -> #5 undone
  assert.equal(matchesLens(ts[0], "blocked", NOW, done), false); // #1 no deps
});

test("matchesLens blocked: a done prereq stops blocking", () => {
  const ts: LensTask[] = [
    { id: 1, done: true },
    { id: 2, done: false, depends_on: [1] },
  ];
  const done = doneIndex(ts);
  assert.equal(matchesLens(ts[1], "blocked", NOW, done), false);
});

test("applyLens narrows the list and preserves order", () => {
  const ts = tasks();
  const done = doneIndex(ts);
  const week = applyLens(ts, "week", NOW, done).map((t) => t.id);
  assert.deepEqual(week, [2, 3]); // today + in-3d, input order
  const overdue = applyLens(ts, "overdue", NOW, done).map((t) => t.id);
  assert.deepEqual(overdue, [1]);
  const nodue = applyLens(ts, "nodue", NOW, done).map((t) => t.id);
  // #5 has no due, #7 has no due (only depends_on) — both undated
  assert.deepEqual(nodue, [5, 7]);
});

test("applyLens blocked uses the whole-list done index", () => {
  const ts = tasks();
  const done = doneIndex(ts);
  assert.deepEqual(applyLens(ts, "blocked", NOW, done).map((t) => t.id), [7]);
});

test("lensMeta carries a label, glyph, and hue per kind", () => {
  const kinds: LensKind[] = ["blocked", "overdue", "today", "week", "nodue"];
  for (const k of kinds) {
    const m = lensMeta(k);
    assert.ok(m.label.length > 0);
    assert.ok(m.glyph.length > 0);
    assert.ok(["alert", "today", "neutral"].includes(m.hue));
  }
  assert.equal(lensMeta("blocked").hue, "alert");
  assert.equal(lensMeta("today").hue, "today");
  assert.equal(lensMeta("nodue").hue, "neutral");
});

test("renderLensChipBody shows the label + a clear affordance", () => {
  const html = renderLensChipBody("overdue");
  assert.match(html, /overdue/);
  assert.match(html, /lens-x/);
  assert.match(html, /&times;/);
});

// --- F82: digit-key hint in the chip + help overlay ------------------------

test("renderLensChipBody leads with the lens digit badge", () => {
  // overdue is digit 2 (blocked=1, overdue=2, today=3, week=4, nodue=5)
  const html = renderLensChipBody("overdue");
  assert.match(html, /<kbd class="lens-chip-key"[^>]*>2<\/kbd>/);
  // blocked is digit 1
  assert.match(renderLensChipBody("blocked"), /lens-chip-key[^>]*>1</);
  // nodue is digit 5
  assert.match(renderLensChipBody("nodue"), /lens-chip-key[^>]*>5</);
});

test("renderLensChipBody digit matches lensDigit for every lens", () => {
  for (const k of LENS_ORDER) {
    const html = renderLensChipBody(k);
    assert.match(html, new RegExp(`lens-chip-key[^>]*>${lensDigit(k)}<`));
  }
});

test("renderActiveLensHelp bolds the label + shows the digit kbd", () => {
  const html = renderActiveLensHelp("today"); // digit 3
  assert.match(html, /Active lens:/);
  assert.match(html, /<kbd>3<\/kbd>/);
  assert.match(html, /<strong>due today<\/strong>/);
});

test("renderActiveLensHelp is empty with no active lens", () => {
  assert.equal(renderActiveLensHelp(null), "");
});

test("renderActiveLensHelp digit agrees with lensDigit per lens", () => {
  for (const k of LENS_ORDER) {
    assert.match(renderActiveLensHelp(k), new RegExp(`<kbd>${lensDigit(k)}</kbd>`));
  }
});

// F90 — the full lens digit-map mini-legend.
test("renderLensDigitMap lists every lens with its digit + label, in order", () => {
  const html = renderLensDigitMap(null);
  for (const k of LENS_ORDER) {
    assert.match(html, new RegExp(`data-lens-legend="${k}"`), `missing ${k}`);
    assert.match(html, new RegExp(`<kbd>${lensDigit(k)}</kbd> ${lensMeta(k).glyph} ${lensMeta(k).label}`));
  }
  // The entries appear in LENS_ORDER (blocked before overdue before today...).
  const order = LENS_ORDER.map((k) => html.indexOf(`data-lens-legend="${k}"`));
  for (let i = 1; i < order.length; i++) assert.ok(order[i] > order[i - 1]);
});

test("renderLensDigitMap marks the active lens and only it", () => {
  const html = renderLensDigitMap("overdue");
  assert.match(html, /data-lens-legend="overdue"[^>]*aria-current="true"/);
  // Exactly one is-active entry.
  assert.equal((html.match(/lens-legend-item is-active/g) ?? []).length, 1);
});

test("renderLensDigitMap with no active lens marks none", () => {
  const html = renderLensDigitMap(null);
  assert.doesNotMatch(html, /is-active/);
  assert.doesNotMatch(html, /aria-current/);
  // Still renders all five.
  assert.equal((html.match(/data-lens-legend=/g) ?? []).length, LENS_ORDER.length);
});

test("renderLensChipBody is empty when no lens is active", () => {
  assert.equal(renderLensChipBody(null), "");
});

test("renderLensChipBody escapes the (static) label safely", () => {
  // labels are static + safe, but the escape path must hold for all kinds
  for (const k of ["blocked", "overdue", "today", "week", "nodue"] as LensKind[]) {
    const html = renderLensChipBody(k);
    assert.doesNotMatch(html, /<script>/);
  }
});

// --- F71: lens keyboard shortcuts ------------------------------------------

test("LENS_ORDER is the five tile lenses in tile order", () => {
  assert.deepEqual([...LENS_ORDER], ["blocked", "overdue", "today", "week", "nodue"]);
});

test("lensForDigit maps 1-5 to the lens at that slot", () => {
  assert.equal(lensForDigit("1"), "blocked");
  assert.equal(lensForDigit("2"), "overdue");
  assert.equal(lensForDigit("3"), "today");
  assert.equal(lensForDigit("4"), "week");
  assert.equal(lensForDigit("5"), "nodue");
});

test("lensForDigit returns null for out-of-range / non-digit keys", () => {
  assert.equal(lensForDigit("6"), null); // only 5 lenses
  assert.equal(lensForDigit("0"), null);
  assert.equal(lensForDigit("9"), null);
  assert.equal(lensForDigit("a"), null);
  assert.equal(lensForDigit("Enter"), null);
  assert.equal(lensForDigit(""), null);
  assert.equal(lensForDigit("12"), null); // not a single key
});

test("lensForDigit stays in sync with LENS_ORDER", () => {
  LENS_ORDER.forEach((kind, i) => {
    assert.equal(lensForDigit(String(i + 1)), kind);
  });
});

test("activeLensSummary names the active lens with its glyph", () => {
  const s = activeLensSummary("overdue");
  assert.match(s, /overdue/);
  assert.ok(s.startsWith(lensMeta("overdue").glyph));
});

test("activeLensSummary is empty when no lens is active", () => {
  assert.equal(activeLensSummary(null), "");
});

// --- F76: lens digit hints (inverse of lensForDigit) -----------------------

test("lensDigit returns the 1-based key for each lens", () => {
  assert.equal(lensDigit("blocked"), "1");
  assert.equal(lensDigit("overdue"), "2");
  assert.equal(lensDigit("today"), "3");
  assert.equal(lensDigit("week"), "4");
  assert.equal(lensDigit("nodue"), "5");
});

test("lensDigit returns '' for non-lens keys (e.g. the open tile)", () => {
  assert.equal(lensDigit("open"), "");
  assert.equal(lensDigit(""), "");
  assert.equal(lensDigit("nonsense"), "");
});

test("lensDigit is the exact inverse of lensForDigit", () => {
  // Round-trip: every lens -> its digit -> back to the same lens.
  LENS_ORDER.forEach((kind) => {
    const d = lensDigit(kind);
    assert.notEqual(d, "");
    assert.equal(lensForDigit(d), kind);
  });
});

test("lensDigit stays in sync with LENS_ORDER positions", () => {
  LENS_ORDER.forEach((kind, i) => {
    assert.equal(lensDigit(kind), String(i + 1));
  });
});

// --- F80: lens-aware breakdown ---------------------------------------------

function bdTasks(): LensBreakdownTask[] {
  return [
    { id: 1, done: false, due: "2026-06-20", priority: "urgent" }, // overdue, urgent
    { id: 2, done: false, due: "2026-06-20", priority: "high" }, // overdue, high
    { id: 3, done: false, due: "2026-06-24", priority: "medium" }, // today, medium
    { id: 4, done: false, priority: "low" }, // no due, low
    { id: 5, done: false }, // no due, no priority -> counts toward none
    { id: 6, done: false, depends_on: [5], priority: "high" }, // blocked by undone #5
  ];
}

test("computeLensBreakdown tallies the priority split of the lensed subset", () => {
  const ts = bdTasks();
  const bd = computeLensBreakdown(ts, "overdue", NOW, doneIndex(ts));
  assert.equal(bd.total, 2); // #1 + #2
  assert.equal(bd.urgent, 1); // #1
  assert.equal(bd.high, 1); // #2
  assert.equal(bd.medium, 0);
  assert.equal(bd.low, 0);
});

test("computeLensBreakdown zeroes the cross-cut redundant with the lens", () => {
  const ts = bdTasks();
  // Under the overdue lens, the overdue cross-cut is suppressed (would be == total).
  const overdue = computeLensBreakdown(ts, "overdue", NOW, doneIndex(ts));
  assert.equal(overdue.overdue, 0);
  // Under the blocked lens, the blocked cross-cut is suppressed.
  const blocked = computeLensBreakdown(ts, "blocked", NOW, doneIndex(ts));
  assert.equal(blocked.blocked, 0);
  assert.equal(blocked.total, 1); // #6
  assert.equal(blocked.high, 1); // #6 is high priority
});

test("computeLensBreakdown reports the overdue cross-cut under a non-overdue lens", () => {
  const ts = bdTasks();
  // The nodue lens picks every undated undone task: #4 (low), #5 (none), and
  // #6 (high — it has depends_on but no due, so it's undated too). None are
  // overdue (no due date), so the overdue cross-cut is 0.
  const nodue = computeLensBreakdown(ts, "nodue", NOW, doneIndex(ts));
  assert.equal(nodue.total, 3);
  assert.equal(nodue.overdue, 0);
  assert.equal(nodue.low, 1); // #4
  assert.equal(nodue.high, 1); // #6
  // #6 is blocked (by undone #5), so the blocked cross-cut surfaces here.
  assert.equal(nodue.blocked, 1);
});

test("computeLensBreakdown: an empty subset reports all zeros", () => {
  const ts: LensBreakdownTask[] = [{ id: 1, done: true, due: "2026-06-20", priority: "urgent" }];
  const bd = computeLensBreakdown(ts, "overdue", NOW, doneIndex(ts));
  assert.equal(bd.total, 0);
  assert.equal(bd.urgent, 0);
});

test("renderLensBreakdown shows the headline + non-zero pills", () => {
  const ts = bdTasks();
  const bd = computeLensBreakdown(ts, "overdue", NOW, doneIndex(ts));
  const html = renderLensBreakdown("overdue", bd);
  assert.match(html, /In view/);
  assert.match(html, /<strong>2<\/strong>/);
  assert.match(html, /overdue/); // the lens label in the headline
  assert.match(html, /bd-urgent/);
  assert.match(html, /bd-high/);
  // zero-count levels are omitted
  assert.doesNotMatch(html, /bd-medium/);
  assert.doesNotMatch(html, /bd-low/);
});

test("renderLensBreakdown is empty with no active lens or an empty subset", () => {
  const empty = { total: 0, urgent: 0, high: 0, medium: 0, low: 0, overdue: 0, blocked: 0 };
  assert.equal(renderLensBreakdown(null, empty), "");
  assert.equal(renderLensBreakdown("overdue", empty), "");
});

test("renderLensBreakdown shows the blocked cross-cut under a schedule lens", () => {
  // A board where a due-today task is also blocked, so the today lens reports
  // a "blocked" cross-cut pill.
  const ts: LensBreakdownTask[] = [
    { id: 1, done: false }, // open blocker
    { id: 2, done: false, due: "2026-06-24", priority: "high", depends_on: [1] }, // today + blocked
  ];
  const bd = computeLensBreakdown(ts, "today", NOW, doneIndex(ts));
  assert.equal(bd.total, 1);
  assert.equal(bd.blocked, 1);
  const html = renderLensBreakdown("today", bd);
  assert.match(html, /bd-blocked/);
});

// --- F81: clickable lens-breakdown drill-downs -----------------------------

test("lensBreakdownPriority decodes the four drillable levels, rejects the rest", () => {
  assert.equal(lensBreakdownPriority("urgent"), "urgent");
  assert.equal(lensBreakdownPriority("high"), "high");
  assert.equal(lensBreakdownPriority("medium"), "medium");
  assert.equal(lensBreakdownPriority("low"), "low");
  // cross-cut pill labels are NOT facet tokens
  assert.equal(lensBreakdownPriority("overdue"), null);
  assert.equal(lensBreakdownPriority("blocked"), null);
  assert.equal(lensBreakdownPriority(""), null);
  assert.equal(lensBreakdownPriority("URGENT"), null); // case-sensitive token
});

test("LENS_BD_PRIORITIES is the urgent-first drill order", () => {
  assert.deepEqual([...LENS_BD_PRIORITIES], ["urgent", "high", "medium", "low"]);
});

test("renderLensBreakdown makes priority pills clickable buttons with a facet token", () => {
  const ts = bdTasks();
  const bd = computeLensBreakdown(ts, "overdue", NOW, doneIndex(ts));
  const html = renderLensBreakdown("overdue", bd);
  // urgent + high pills present as drill buttons carrying the facet token
  assert.match(html, /<button[^>]*data-lens-bd-prio="urgent"[^>]*>/);
  assert.match(html, /<button[^>]*data-lens-bd-prio="high"[^>]*>/);
  assert.match(html, /is-drill/);
});

test("renderLensBreakdown cross-cut pills stay plain spans (no facet token)", () => {
  // nodue lens over a board where a member is also blocked -> a blocked cross-cut.
  const ts: LensBreakdownTask[] = [
    { id: 1, done: false }, // open blocker
    { id: 2, done: false, priority: "high", depends_on: [1] }, // no due + blocked
  ];
  const bd = computeLensBreakdown(ts, "nodue", NOW, doneIndex(ts));
  const html = renderLensBreakdown("nodue", bd);
  assert.match(html, /bd-blocked/);
  // the blocked cross-cut is a span, not a drill button
  assert.doesNotMatch(html, /data-lens-bd-prio="blocked"/);
  assert.doesNotMatch(html, /data-lens-bd-prio="overdue"/);
});

test("renderLensBreakdown marks an already-active facet pill is-on + aria-pressed", () => {
  const ts = bdTasks();
  const bd = computeLensBreakdown(ts, "overdue", NOW, doneIndex(ts));
  const html = renderLensBreakdown("overdue", bd, new Set(["urgent"]));
  // the urgent pill is on; high is not
  assert.match(html, /data-lens-bd-prio="urgent"[^>]*aria-pressed="true"/);
  assert.match(html, /is-drill bd-urgent is-on/);
  assert.match(html, /data-lens-bd-prio="high"[^>]*aria-pressed="false"/);
});

test("renderLensBreakdown with no active facets marks every pill off", () => {
  const ts = bdTasks();
  const bd = computeLensBreakdown(ts, "overdue", NOW, doneIndex(ts));
  const html = renderLensBreakdown("overdue", bd);
  assert.doesNotMatch(html, /is-on/);
  assert.match(html, /aria-pressed="false"/);
});

import { test } from "node:test";
import assert from "node:assert/strict";
import {
  matchesLens,
  applyLens,
  lensMeta,
  renderLensChipBody,
  type LensKind,
  type LensTask,
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

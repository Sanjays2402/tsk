import { test } from "node:test";
import assert from "node:assert/strict";
import {
  normalizeFilter,
  filterIsEmpty,
  filtersEqual,
  normalizeViews,
  parseViews,
  serializeViews,
  addView,
  removeView,
  updateView,
  moveView,
  activeView,
  activeViewWithLens,
  lensProvenanceNote,
  pureLensViewName,
  findPureLensView,
  viewMatches,
  describeView,
  renderViewChips,
  type ViewFilter,
  type SavedView,
} from "../src/views.ts";

const EMPTY: ViewFilter = { query: "", priorities: [], tags: [], hideDone: false };

test("normalizeFilter trims query, lowercases + sorts facets", () => {
  const f = normalizeFilter({
    query: "  buy  ",
    priorities: ["high", "low"],
    tags: ["Work", "Admin"],
    hideDone: true,
  });
  assert.equal(f.query, "buy");
  assert.deepEqual(f.priorities, ["high", "low"]);
  assert.deepEqual(f.tags, ["admin", "work"]);
  assert.equal(f.hideDone, true);
});

test("filterIsEmpty is true only for a no-op filter", () => {
  assert.equal(filterIsEmpty(EMPTY), true);
  assert.equal(filterIsEmpty({ ...EMPTY, query: "x" }), false);
  assert.equal(filterIsEmpty({ ...EMPTY, hideDone: true }), false);
  assert.equal(filterIsEmpty({ ...EMPTY, tags: ["a"] }), false);
});

test("filtersEqual ignores facet order + tag case", () => {
  const a: ViewFilter = { query: "x", priorities: ["high", "low"], tags: ["Work"], hideDone: false };
  const b: ViewFilter = { query: "x", priorities: ["low", "high"], tags: ["work"], hideDone: false };
  assert.equal(filtersEqual(a, b), true);
  assert.equal(filtersEqual(a, { ...a, hideDone: true }), false);
});

test("addView appends a new named view capturing the filter", () => {
  const f: ViewFilter = { ...EMPTY, tags: ["work"], hideDone: true };
  const views = addView([], "Work focus", f);
  assert.equal(views.length, 1);
  assert.equal(views[0].name, "Work focus");
  assert.deepEqual(views[0].filter.tags, ["work"]);
});

test("addView rejects blank names and empty filters", () => {
  assert.deepEqual(addView([], "  ", { ...EMPTY, tags: ["a"] }), []);
  assert.deepEqual(addView([], "Empty", EMPTY), []);
});

test("addView overwrites a same-name (case-insensitive) view in place", () => {
  const first = addView([], "Urgent", { ...EMPTY, priorities: ["urgent"] });
  const second = addView(first, "urgent", { ...EMPTY, priorities: ["high"] });
  assert.equal(second.length, 1);
  assert.equal(second[0].id, first[0].id); // same id, updated content
  assert.deepEqual(second[0].filter.priorities, ["high"]);
  assert.equal(second[0].name, "urgent");
});

test("removeView drops by id", () => {
  const views = addView([], "A", { ...EMPTY, tags: ["a"] });
  assert.deepEqual(removeView(views, views[0].id), []);
  assert.deepEqual(removeView(views, "nope"), views);
});

test("updateView overwrites the captured filter, keeping id + name", () => {
  const views = addView([], "Work", { ...EMPTY, tags: ["work"] });
  const id = views[0].id;
  const next = updateView(views, id, { ...EMPTY, priorities: ["urgent"] });
  assert.equal(next[0].id, id);
  assert.equal(next[0].name, "Work");
  assert.deepEqual(next[0].filter.priorities, ["urgent"]);
  assert.deepEqual(next[0].filter.tags, []);
});

test("updateView rejects an empty filter and unknown ids", () => {
  const views = addView([], "Work", { ...EMPTY, tags: ["work"] });
  assert.equal(updateView(views, views[0].id, EMPTY), views); // empty -> unchanged
  const same = updateView(views, "nope", { ...EMPTY, tags: ["x"] });
  assert.deepEqual(same[0].filter.tags, ["work"]); // unknown id -> no change
});

test("moveView reorders before a target / to the end", () => {
  let v = addView([], "A", { ...EMPTY, tags: ["a"] });
  v = addView(v, "B", { ...EMPTY, tags: ["b"] });
  v = addView(v, "C", { ...EMPTY, tags: ["c"] });
  const [a, b, c] = v.map((x) => x.id);
  // Move C before A -> C, A, B
  assert.deepEqual(moveView(v, c, a).map((x) => x.name), ["C", "A", "B"]);
  // Move A to the end (beforeId null) -> B, C, A
  assert.deepEqual(moveView(v, a, null).map((x) => x.name), ["B", "C", "A"]);
  // Move B before C -> A, B, C is a no-op (already there): same ref
  assert.equal(moveView(v, b, c), v);
});

test("moveView no-ops on self-drop and unknown ids return same ref", () => {
  let v = addView([], "A", { ...EMPTY, tags: ["a"] });
  v = addView(v, "B", { ...EMPTY, tags: ["b"] });
  const [a] = v.map((x) => x.id);
  assert.equal(moveView(v, a, a), v); // self
  assert.equal(moveView(v, "nope", a), v); // unknown moved
});

test("activeView matches the live filter, null when empty or unmatched", () => {
  const views = addView([], "Hi", { ...EMPTY, priorities: ["high"] });
  assert.equal(activeView(views, { ...EMPTY, priorities: ["high"] })?.name, "Hi");
  assert.equal(activeView(views, { ...EMPTY, priorities: ["low"] }), null);
  assert.equal(activeView(views, EMPTY), null);
});

test("parse/serialize round-trips views", () => {
  const views = addView(addView([], "A", { ...EMPTY, tags: ["a"] }), "B", { ...EMPTY, query: "b" });
  const round = parseViews(serializeViews(views));
  assert.equal(round.length, 2);
  assert.equal(round[0].name, "A");
  assert.equal(round[1].filter.query, "b");
});

test("normalizeViews drops malformed entries", () => {
  const raw = [
    { name: "ok", filter: { tags: ["x"] } },
    { name: "" }, // blank name -> dropped
    "garbage", // not an object -> dropped
    { filter: { tags: ["y"] } }, // no name -> dropped
  ];
  const views = normalizeViews(raw);
  assert.equal(views.length, 1);
  assert.equal(views[0].name, "ok");
  assert.ok(views[0].id); // synthesized id
});

test("parseViews tolerates null / malformed store", () => {
  assert.deepEqual(parseViews(null), []);
  assert.deepEqual(parseViews("{not array"), []);
});

test("describeView summarizes the constraints", () => {
  const v: SavedView = {
    id: "1",
    name: "x",
    filter: { query: "buy", priorities: ["high"], tags: ["work"], hideDone: true },
  };
  const d = describeView(v);
  assert.match(d, /buy/);
  assert.match(d, /high/);
  assert.match(d, /#work/);
  assert.match(d, /hide done/);
});

test("renderViewChips marks the active view and carries recall/del hooks", () => {
  const views = addView([], "Work", { ...EMPTY, tags: ["work"] });
  const html = renderViewChips(views, { ...EMPTY, tags: ["work"] });
  assert.match(html, /is-active/);
  assert.match(html, /data-view-recall=/);
  assert.match(html, /data-view-del=/);
  // Non-matching filter -> no active marker.
  assert.doesNotMatch(renderViewChips(views, EMPTY), /is-active/);
  // Empty list collapses.
  assert.equal(renderViewChips([], EMPTY), "");
});

test("renderViewChips: draggable + update affordance (F32)", () => {
  const views = addView([], "Work", { ...EMPTY, tags: ["work"] });
  const id = views[0].id;
  // draggable adds the drag hooks.
  const drag = renderViewChips(views, EMPTY, { draggable: true });
  assert.match(drag, /draggable="true"/);
  assert.match(drag, /data-view-drag/);
  // updatableId surfaces the update button on that one chip only.
  const upd = renderViewChips(views, { ...EMPTY, tags: ["work"] }, { updatableId: id });
  assert.match(upd, /data-view-update="/);
  // No update button when updatableId doesn't match.
  assert.doesNotMatch(renderViewChips(views, EMPTY, { updatableId: "other" }), /data-view-update/);
});

test("renderViewChips escapes view names", () => {
  const views: SavedView[] = [
    { id: "1", name: "<b>x</b>", filter: { ...EMPTY, tags: ["a"] } },
  ];
  const html = renderViewChips(views, EMPTY);
  assert.doesNotMatch(html, /<b>x<\/b>/);
  assert.match(html, /&lt;b&gt;/);
});

// --- F104: the lens+facet saved-view bridge --------------------------------

test("addView captures an optional lens and stores it on the view", () => {
  const out = addView([], "Urgent overdue", { ...EMPTY, priorities: ["urgent"] }, "overdue");
  assert.equal(out.length, 1);
  assert.equal(out[0].lens, "overdue");
  assert.deepEqual(out[0].filter.priorities, ["urgent"]);
});

test("addView allows a pure-lens view even when the plain filter is empty", () => {
  const out = addView([], "Blocked", EMPTY, "blocked");
  assert.equal(out.length, 1);
  assert.equal(out[0].lens, "blocked");
  // Still rejected when there's neither a filter nor a lens.
  assert.equal(addView([], "Nope", EMPTY).length, 0);
});

test("addView without a lens stores no lens key (back-compat)", () => {
  const out = addView([], "Work", { ...EMPTY, tags: ["work"] });
  assert.equal("lens" in out[0], false);
});

test("addView overwriting a same-name view updates its lens (and strips when omitted)", () => {
  let v = addView([], "V", { ...EMPTY, priorities: ["high"] }, "today");
  assert.equal(v[0].lens, "today");
  // Re-save same name without a lens -> lens stripped, filter updated.
  v = addView(v, "V", { ...EMPTY, priorities: ["low"] });
  assert.equal(v.length, 1);
  assert.equal("lens" in v[0], false);
  assert.deepEqual(v[0].filter.priorities, ["low"]);
});

test("normalizeViews round-trips a stored lens and drops a junk one", () => {
  const stored = [
    { id: "a", name: "Lensed", filter: { priorities: ["urgent"] }, lens: "overdue" },
    { id: "b", name: "Junk lens", filter: { tags: ["x"] }, lens: 42 },
    { id: "c", name: "Empty lens", filter: {}, lens: "" },
  ];
  const out = normalizeViews(stored);
  assert.equal(out[0].lens, "overdue");
  assert.equal("lens" in out[1], false); // non-string dropped
  assert.equal("lens" in out[2], false); // empty dropped
});

test("serialize/parse preserves the captured lens", () => {
  const views = addView([], "Drill", { ...EMPTY, priorities: ["urgent"] }, "blocked");
  const round = parseViews(serializeViews(views));
  assert.equal(round[0].lens, "blocked");
});

test("viewMatches requires BOTH the facet and the lens to line up", () => {
  const v: SavedView = { id: "1", name: "UO", filter: { ...EMPTY, priorities: ["urgent"] }, lens: "overdue" };
  const f: ViewFilter = { ...EMPTY, priorities: ["urgent"] };
  assert.equal(viewMatches(v, f, "overdue"), true); // facet + lens both match
  assert.equal(viewMatches(v, f, "blocked"), false); // wrong lens
  assert.equal(viewMatches(v, f, null), false); // no live lens
  assert.equal(viewMatches(v, { ...EMPTY, priorities: ["high"] }, "overdue"), false); // wrong facet
});

test("viewMatches: a lens-less view requires NO live lens", () => {
  const v: SavedView = { id: "1", name: "W", filter: { ...EMPTY, tags: ["work"] } };
  const f = { ...EMPTY, tags: ["work"] };
  assert.equal(viewMatches(v, f, null), true);
  assert.equal(viewMatches(v, f, "blocked"), false); // a live lens means it's not this plain view
});

test("viewMatches: a pure-lens view matches on the lens with an empty filter", () => {
  const v: SavedView = { id: "1", name: "B", filter: { ...EMPTY }, lens: "blocked" };
  assert.equal(viewMatches(v, EMPTY, "blocked"), true);
  assert.equal(viewMatches(v, { ...EMPTY, tags: ["x"] }, "blocked"), false); // extra facet
});

test("activeViewWithLens finds the lens+facet combo, distinct from activeView", () => {
  const plain: SavedView = { id: "1", name: "Urgent", filter: { ...EMPTY, priorities: ["urgent"] } };
  const lensed: SavedView = { id: "2", name: "Urgent overdue", filter: { ...EMPTY, priorities: ["urgent"] }, lens: "overdue" };
  const views = [plain, lensed];
  const f: ViewFilter = { ...EMPTY, priorities: ["urgent"] };
  // With the overdue lens on, the lensed view is the active combo.
  assert.equal(activeViewWithLens(views, f, "overdue")?.id, "2");
  // With no lens, the plain view is active.
  assert.equal(activeViewWithLens(views, f, null)?.id, "1");
  // activeView (lens-blind) only ever sees the first filter match.
  assert.equal(activeView(views, f)?.id, "1");
});

test("describeView surfaces the captured lens", () => {
  const v: SavedView = { id: "1", name: "UO", filter: { ...EMPTY, priorities: ["urgent"] }, lens: "overdue" };
  assert.match(describeView(v), /lens: overdue/);
});

test("renderViewChips lens-aware highlight + is-lensed marker", () => {
  const lensed: SavedView = { id: "2", name: "UO", filter: { ...EMPTY, priorities: ["urgent"] }, lens: "overdue" };
  const f: ViewFilter = { ...EMPTY, priorities: ["urgent"] };
  // liveLens matches -> is-active; the chip is also marked is-lensed.
  const onHtml = renderViewChips([lensed], f, { liveLens: "overdue" });
  assert.match(onHtml, /is-active/);
  assert.match(onHtml, /is-lensed/);
  // liveLens mismatch -> not active (but still marked lensed).
  const offHtml = renderViewChips([lensed], f, { liveLens: null });
  assert.doesNotMatch(offHtml, /is-active/);
  assert.match(offHtml, /is-lensed/);
});

test("updateView can re-capture or strip the lens", () => {
  let v = addView([], "V", { ...EMPTY, priorities: ["urgent"] }, "overdue");
  // Update with a new lens.
  v = updateView(v, v[0].id, { ...EMPTY, priorities: ["urgent"] }, "blocked");
  assert.equal(v[0].lens, "blocked");
  // Update without a lens -> stripped.
  v = updateView(v, v[0].id, { ...EMPTY, priorities: ["high"] });
  assert.equal("lens" in v[0], false);
});

// --- F109: lens provenance --------------------------------------------------

test("lensProvenanceNote names the recalled view when its lens equals the live lens", () => {
  const recalled = addView([], "Sprint", { ...EMPTY, priorities: ["urgent"] }, "overdue")[0];
  assert.equal(lensProvenanceNote(recalled, "overdue"), "Sprint");
});

test("lensProvenanceNote is null when the recalled view captured no lens", () => {
  // A filter-only view doesn't explain a lens even if one is live (it came from
  // a digit key / stat tile, not this view).
  const recalled = addView([], "Work", { ...EMPTY, tags: ["work"] })[0];
  assert.equal(lensProvenanceNote(recalled, "overdue"), null);
});

test("lensProvenanceNote is null when the live lens diverged from the view's", () => {
  // You recalled "Sprint (overdue)" then switched the lens to blocked — the
  // view no longer explains the live lens.
  const recalled = addView([], "Sprint", { ...EMPTY, priorities: ["urgent"] }, "overdue")[0];
  assert.equal(lensProvenanceNote(recalled, "blocked"), null);
});

test("lensProvenanceNote is null with no recalled view or no live lens", () => {
  const recalled = addView([], "Sprint", { ...EMPTY, priorities: ["urgent"] }, "overdue")[0];
  assert.equal(lensProvenanceNote(null, "overdue"), null); // nothing recalled
  assert.equal(lensProvenanceNote(recalled, null), null); // no lens on
});

// --- F110: pin a lens as a pure-lens quick view -----------------------------

test("pureLensViewName is the lens label verbatim", () => {
  assert.equal(pureLensViewName("overdue"), "overdue");
  assert.equal(pureLensViewName("due this week"), "due this week");
});

test("findPureLensView matches an empty-filter view with the given lens", () => {
  const views = addView([], "overdue", EMPTY, "overdue");
  const hit = findPureLensView(views, "overdue");
  assert.notEqual(hit, null);
  assert.equal(hit!.lens, "overdue");
  assert.equal(findPureLensView(views, "blocked"), null); // different lens
});

test("findPureLensView ignores a lensed view that ALSO has a filter facet", () => {
  // "urgent (overdue)" is a lens+facet combo, NOT a pure-lens pin — so the pin
  // affordance treats the lens as unpinned and a fresh pure-lens view is savable.
  const views = addView([], "urgent (overdue)", { ...EMPTY, priorities: ["urgent"] }, "overdue");
  assert.equal(findPureLensView(views, "overdue"), null);
});

test("findPureLensView pin round-trip: addView with empty filter + lens is found", () => {
  // Mirrors what main.ts pinCurrentLens does — addView(EMPTY, lens) creates a
  // pure-lens view (allowed because a lens is supplied), and findPureLensView
  // then locates it so a second pin recalls instead of duplicating.
  let views = addView([], pureLensViewName("blocked"), EMPTY, "blocked");
  assert.equal(views.length, 1);
  const found = findPureLensView(views, "blocked");
  assert.equal(found!.name, "blocked");
  // A same-name re-pin overwrites (addView's name-collision rule) — still one.
  views = addView(views, pureLensViewName("blocked"), EMPTY, "blocked");
  assert.equal(views.length, 1);
});

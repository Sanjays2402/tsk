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
  isPureLensView,
  isCohortView,
  findCohortView,
  isStaleCohortView,
  staleCohortViewIds,
  snapshotViews,
  restoreSweptViews,
  countViewMatches,
  countViewMatchesBreakdown,
  describeViewMatchBreakdown,
  busiestViewId,
  viewsRowSummary,
  viewsRowDoneCount,
  describeViewsRowSummary,
  appendStaleSegment,
  busiestHeadlineHTML,
  staleSweepSegmentHTML,
  staleSweepTitle,
  viewsSummaryTooltip,
  peekOpenOnlyLabel,
  peekViewLabel,
  peekCommandTitle,
  peekOpenOnlyTitle,
  enrichCohortPeek,
  recallOpenOnlyTitle,
  badgeHidesDone,
  addCohortView,
  chipClippedX,
  canAnimateChipExit,
  UNPIN_EXIT_MS,
  viewMatches,
  describeView,
  renderViewChips,
  groupViewsByTag,
  describeViewGroups,
  viewGroupDividers,
  viewDividerLabelBefore,
  exportViewsDoc,
  importViewsDoc,
  previewImportViews,
  renderViewDivider,
  chipStaleTitle,
  doneSegmentHTML,
  staleSweepTitleAged,
  peekOpenRecallTitle,
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

// --- F112: pure-lens bookmarks read as lens chips, not filter chips ---------

test("isPureLensView is true only for a lens with an empty filter", () => {
  // F110's one-click pin: empty filter + a lens.
  assert.equal(isPureLensView({ id: "1", name: "overdue", filter: EMPTY, lens: "overdue" }), true);
  // A lens+facet drill (F104) is NOT a pure pin.
  assert.equal(
    isPureLensView({ id: "2", name: "UO", filter: { ...EMPTY, priorities: ["urgent"] }, lens: "overdue" }),
    false,
  );
  // A plain filter view (no lens) is not a pure-lens view.
  assert.equal(isPureLensView({ id: "3", name: "work", filter: { ...EMPTY, tags: ["work"] } }), false);
  // A lens-less empty view is not a pure-lens view either.
  assert.equal(isPureLensView({ id: "4", name: "empty", filter: EMPTY }), false);
});

test("renderViewChips gives a pure-lens chip its lens glyph + is-lens-pin", () => {
  const pin: SavedView = { id: "1", name: "overdue", filter: EMPTY, lens: "overdue" };
  const glyph = (k: string) => (k === "overdue" ? "\u26A0" : "");
  const html = renderViewChips([pin], EMPTY, { liveLens: null, lensGlyph: glyph });
  assert.match(html, /is-lens-pin/);
  assert.match(html, /view-chip-lens-glyph/);
  assert.match(html, /\u26A0/); // the lens's own glyph appears
});

test("renderViewChips: a lens+facet drill is NOT a lens-pin (no glyph)", () => {
  const drill: SavedView = {
    id: "1",
    name: "UO",
    filter: { ...EMPTY, priorities: ["urgent"] },
    lens: "overdue",
  };
  const f: ViewFilter = { ...EMPTY, priorities: ["urgent"] };
  const html = renderViewChips([drill], f, { liveLens: "overdue", lensGlyph: () => "\u26A0" });
  assert.doesNotMatch(html, /is-lens-pin/);
  assert.doesNotMatch(html, /view-chip-lens-glyph/);
  assert.match(html, /is-lensed/); // still marked lensed (the drill diamond)
});

test("renderViewChips: a pin with no glyph resolver gets the class but no glyph", () => {
  const pin: SavedView = { id: "1", name: "blocked", filter: EMPTY, lens: "blocked" };
  const html = renderViewChips([pin], EMPTY, { liveLens: null });
  assert.match(html, /is-lens-pin/);
  assert.doesNotMatch(html, /view-chip-lens-glyph/);
});

test("renderViewChips: a garbage lens resolving to '' yields no glyph span", () => {
  const pin: SavedView = { id: "1", name: "junk", filter: EMPTY, lens: "bogus" };
  const html = renderViewChips([pin], EMPTY, { liveLens: null, lensGlyph: () => "" });
  assert.match(html, /is-lens-pin/);
  assert.doesNotMatch(html, /view-chip-lens-glyph/);
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

// --- F119: flash the just-pinned chip --------------------------------------

test("renderViewChips flashes only the chip whose id matches flashId", () => {
  const a: SavedView = { id: "a", name: "alpha", filter: { ...EMPTY, tags: ["x"] } };
  const b: SavedView = { id: "b", name: "beta", filter: { ...EMPTY, tags: ["y"] } };
  const html = renderViewChips([a, b], EMPTY, { flashId: "b" });
  // The flashed chip carries is-flash; the other does not.
  assert.match(html, /data-view-id="b"[^>]*?/);
  const bChip = html.slice(html.indexOf('data-view-id="b"') - 60, html.indexOf('data-view-id="b"'));
  assert.match(bChip, /is-flash/);
  const aChip = html.slice(html.indexOf('data-view-id="a"') - 60, html.indexOf('data-view-id="a"'));
  assert.doesNotMatch(aChip, /is-flash/);
});

test("renderViewChips adds no is-flash class when flashId is absent or unknown", () => {
  const a: SavedView = { id: "a", name: "alpha", filter: { ...EMPTY, tags: ["x"] } };
  assert.doesNotMatch(renderViewChips([a], EMPTY, {}), /is-flash/);
  assert.doesNotMatch(renderViewChips([a], EMPTY, { flashId: null }), /is-flash/);
  assert.doesNotMatch(renderViewChips([a], EMPTY, { flashId: "nope" }), /is-flash/);
});

test("renderViewChips flash composes with the pure-lens-pin classes", () => {
  // The chip a pin creates is a pure-lens bookmark, so the flash must ride
  // alongside is-lens-pin (the F112 marker) on the same chip.
  const pin: SavedView = { id: "p", name: "overdue", filter: EMPTY, lens: "overdue" };
  const html = renderViewChips([pin], EMPTY, {
    liveLens: null,
    lensGlyph: () => "\u26A0",
    flashId: "p",
  });
  assert.match(html, /is-lens-pin/);
  assert.match(html, /is-flash/);
});

// --- F124: chipClippedX (scroll a just-pinned chip into view) ----------------

test("chipClippedX is false when the chip sits fully inside the container", () => {
  // container [0,200]; chip [50,120] is comfortably inside -> no scroll needed.
  assert.equal(chipClippedX({ left: 50, right: 120 }, { left: 0, right: 200 }), false);
});

test("chipClippedX is true when the chip overflows the right edge", () => {
  // chip [180,260] runs past the container's right (200) -> clipped.
  assert.equal(chipClippedX({ left: 180, right: 260 }, { left: 0, right: 200 }), true);
});

test("chipClippedX is true when the chip is before the left edge", () => {
  // A scrolled row can push a chip to negative offsets relative to the container.
  assert.equal(chipClippedX({ left: -30, right: 40 }, { left: 0, right: 200 }), true);
});

test("chipClippedX tolerates sub-pixel rounding via the epsilon", () => {
  // A chip flush with the edges (within 1px) is NOT treated as clipped, so a
  // chip exactly at the boundary doesn't trigger a needless scroll.
  assert.equal(chipClippedX({ left: -0.4, right: 200.4 }, { left: 0, right: 200 }), false);
  // Past the epsilon it IS clipped.
  assert.equal(chipClippedX({ left: 0, right: 202 }, { left: 0, right: 200 }), true);
});

// --- F129: animate a chip's exit on unpin -----------------------------------

test("canAnimateChipExit is true for a chip with a working classList.add", () => {
  // A real element shape — has a classList whose add is callable.
  const chip = { classList: { add: () => {} } };
  assert.equal(canAnimateChipExit(chip), true);
});

test("canAnimateChipExit is false in the jsdom-less / detached cases", () => {
  // Null chip (not in the row, or no DOM) -> synchronous remove, no animation.
  assert.equal(canAnimateChipExit(null), false);
  // A bare object with no classList -> can't animate, fall through.
  assert.equal(canAnimateChipExit({}), false);
  // A classList without a callable add -> can't animate.
  assert.equal(canAnimateChipExit({ classList: {} }), false);
  assert.equal(canAnimateChipExit({ classList: { add: 42 as unknown } }), false);
});

test("UNPIN_EXIT_MS is a positive duration the CSS keyframe can match", () => {
  // The deferred removeView timer keys off this; it must be a real positive ms.
  assert.equal(typeof UNPIN_EXIT_MS, "number");
  assert.ok(UNPIN_EXIT_MS > 0);
});

// --- F133: cohort bookmark views -------------------------------------------

const emptyF = (): ViewFilter => ({ query: "", priorities: [], tags: [], hideDone: false });

test("isCohortView is true for a chokepoint id with no filter or lens", () => {
  const v: SavedView = { id: "a", name: "waiting on #1", filter: emptyF(), cohort: 1 };
  assert.equal(isCohortView(v), true);
});

test("isCohortView is false when a filter or lens rides along", () => {
  // A cohort id plus a filter is not a pure cohort bookmark.
  assert.equal(
    isCohortView({ id: "a", name: "x", filter: { ...emptyF(), tags: ["dev"] }, cohort: 1 }),
    false,
  );
  // A cohort id plus a lens is not a pure cohort bookmark either.
  assert.equal(
    isCohortView({ id: "a", name: "x", filter: emptyF(), cohort: 1, lens: "overdue" }),
    false,
  );
  // No cohort id at all -> not a cohort view.
  assert.equal(isCohortView({ id: "a", name: "x", filter: emptyF() }), false);
});

test("addCohortView creates a cohort bookmark with an empty filter", () => {
  const out = addCohortView([], "waiting on #3", 3);
  assert.equal(out.length, 1);
  assert.equal(out[0].cohort, 3);
  assert.equal(out[0].name, "waiting on #3");
  assert.equal(isCohortView(out[0]), true);
  assert.equal(filterIsEmpty(out[0].filter), true);
});

test("addCohortView rejects a blank name or a non-positive id", () => {
  assert.deepEqual(addCohortView([], "  ", 3), []);
  assert.deepEqual(addCohortView([], "x", 0), []);
  assert.deepEqual(addCohortView([], "x", -2), []);
});

test("addCohortView re-pinning the same chokepoint overwrites, never duplicates", () => {
  const a = addCohortView([], "waiting on #5", 5);
  const b = addCohortView(a, "renamed #5", 5); // same chokepoint, new name
  assert.equal(b.length, 1); // overwritten in place, not appended
  assert.equal(b[0].cohort, 5);
  assert.equal(b[0].name, "renamed #5");
  assert.equal(b[0].id, a[0].id); // id preserved across the overwrite
});

test("findCohortView returns the bookmark for a chokepoint or null", () => {
  const views = addCohortView(addCohortView([], "#1", 1), "#4", 4);
  assert.equal(findCohortView(views, 4)!.cohort, 4);
  assert.equal(findCohortView(views, 1)!.cohort, 1);
  assert.equal(findCohortView(views, 9), null);
});

test("normalizeViews carries a positive cohort id and drops a bad one", () => {
  const ok = normalizeViews([{ name: "c", filter: {}, cohort: 7 }]);
  assert.equal(ok[0].cohort, 7);
  // A non-integer / non-positive cohort is dropped (degrades to a plain view).
  const bad = normalizeViews([{ name: "c", filter: {}, cohort: -1 }]);
  assert.equal(bad[0].cohort, undefined);
  const frac = normalizeViews([{ name: "c", filter: {}, cohort: 2.5 }]);
  assert.equal(frac[0].cohort, undefined);
});

test("a cohort view round-trips through serialize/parse", () => {
  const views = addCohortView([], "waiting on #2", 2);
  const back = parseViews(serializeViews(views));
  assert.equal(back[0].cohort, 2);
  assert.equal(isCohortView(back[0]), true);
});

test("describeView names the chokepoint for a cohort view", () => {
  const v = addCohortView([], "c", 8)[0];
  assert.match(describeView(v), /waiting on #8/);
});

test("renderViewChips marks a cohort chip active only when its chokepoint is focused", () => {
  const views = addCohortView(addCohortView([], "#1", 1), "#4", 4);
  // Focused on #4's cohort -> only the #4 chip is active.
  const html = renderViewChips(views, emptyF(), { activeCohort: 4, cohortGlyph: "\u2191" });
  // The #4 chip carries is-active + is-cohort-pin + the ↑ glyph.
  assert.match(html, /class="view-chip is-active is-cohort-pin"[^>]*>[\s\S]*?#4/);
  // Exactly one chip is active (the #1 chip is not).
  assert.equal((html.match(/is-active/g) ?? []).length, 1);
  // The ↑ cohort glyph is present.
  assert.match(html, /view-chip-lens-glyph[^>]*>\u2191</);
});

test("renderViewChips leaves cohort chips inactive when nothing is focused", () => {
  const views = addCohortView([], "#1", 1);
  const html = renderViewChips(views, emptyF(), { activeCohort: null, cohortGlyph: "\u2191" });
  assert.doesNotMatch(html, /is-active/);
});

// --- F138: stale cohort chips ----------------------------------------------

test("isStaleCohortView is true only when a cohort view's chokepoint has no live cohort", () => {
  const live = addCohortView([], "#1", 1)[0];
  // #1 has a live cohort -> not stale.
  assert.equal(isStaleCohortView(live, (id) => id === 1), false);
  // #1 has no live cohort (its waiters all completed / it's done) -> stale.
  assert.equal(isStaleCohortView(live, () => false), true);
});

test("isStaleCohortView never flags a non-cohort view", () => {
  // A plain filter view is never stale by the cohort measure, even if the
  // predicate would say no live cohort exists.
  const filterView: SavedView = { id: "f", name: "work", filter: { ...emptyF(), tags: ["work"] } };
  assert.equal(isStaleCohortView(filterView, () => false), false);
  // A pure-lens view likewise.
  const lensView: SavedView = { id: "l", name: "(blocked)", filter: emptyF(), lens: "blocked" };
  assert.equal(isStaleCohortView(lensView, () => false), false);
});

test("renderViewChips marks a stale cohort chip with is-stale-cohort + a tooltip note", () => {
  const views = addCohortView([], "#7", 7);
  // The chokepoint #7 is dead (no live cohort) -> the chip reads as stale.
  const html = renderViewChips(views, emptyF(), {
    activeCohort: null,
    cohortGlyph: "\u2191",
    staleCohort: () => true,
  });
  assert.match(html, /class="view-chip is-cohort-pin is-stale-cohort"/);
  assert.match(html, /stale, recall to clear/);
});

test("renderViewChips leaves a live cohort chip un-marked (no stale class)", () => {
  const views = addCohortView([], "#7", 7);
  const html = renderViewChips(views, emptyF(), {
    activeCohort: null,
    cohortGlyph: "\u2191",
    staleCohort: () => false,
  });
  assert.doesNotMatch(html, /is-stale-cohort/);
  assert.doesNotMatch(html, /stale, recall to clear/);
});

test("renderViewChips without a staleCohort resolver never marks anything stale", () => {
  // Omitting the resolver keeps existing callers byte-identical (no stale marks).
  const views = addCohortView([], "#7", 7);
  const html = renderViewChips(views, emptyF(), { activeCohort: null, cohortGlyph: "\u2191" });
  assert.doesNotMatch(html, /is-stale-cohort/);
});

// --- F141: saved-view chip match-count badges ------------------------------

interface CountTask {
  id: number;
  title: string;
  priority: string;
  tags: string[];
  done: boolean;
}

const COUNT_TASKS: CountTask[] = [
  { id: 1, title: "alpha", priority: "high", tags: ["work"], done: false },
  { id: 2, title: "beta", priority: "low", tags: ["work"], done: false },
  { id: 3, title: "gamma", priority: "high", tags: ["home"], done: true },
];

// A trivial counters bundle: filter by tag membership, a lens that only the
// high-priority tasks pass, and a fixed cohort id-set.
function counters(cohortSet: number[] = []) {
  return {
    matchesFilter: (t: CountTask, f: ViewFilter) =>
      f.tags.length === 0 || f.tags.some((tag) => t.tags.includes(tag)),
    matchesLens: (t: CountTask, _lens: string) => t.priority === "high",
    cohortIds: (_sourceId: number) => cohortSet,
  };
}

test("countViewMatches counts a plain filter view over the live tasks", () => {
  const v: SavedView = { id: "f", name: "work", filter: { ...emptyF(), tags: ["work"] } };
  // #1 and #2 carry #work -> 2 matches.
  assert.equal(countViewMatches(v, COUNT_TASKS, counters()), 2);
});

test("countViewMatches ANDs the filter with the lens for a lens+facet view", () => {
  // #work AND high-priority: only #1 (beta is low, gamma is #home) -> 1.
  const v: SavedView = { id: "lf", name: "work hi", filter: { ...emptyF(), tags: ["work"] }, lens: "blocked" };
  assert.equal(countViewMatches(v, COUNT_TASKS, counters()), 1);
});

test("countViewMatches counts everything a pure-lens view passes", () => {
  // Empty filter + a lens -> count all high-priority tasks (#1, #3) -> 2.
  const v: SavedView = { id: "l", name: "(blocked)", filter: emptyF(), lens: "blocked" };
  assert.equal(countViewMatches(v, COUNT_TASKS, counters()), 2);
});

test("countViewMatches returns a cohort view's live id-set size", () => {
  const v = addCohortView([], "#5", 5)[0];
  // The cohort id-set is injected; its length is the count (3 here).
  assert.equal(countViewMatches(v, COUNT_TASKS, counters([7, 8, 9])), 3);
  // A dead/empty cohort counts 0.
  assert.equal(countViewMatches(v, COUNT_TASKS, counters([])), 0);
});

test("renderViewChips renders a quiet ·N badge when matchCount resolves a number", () => {
  const views = addView([], "work", { ...emptyF(), tags: ["work"] });
  const html = renderViewChips(views, emptyF(), { matchCount: () => 12 });
  assert.match(html, /class="view-chip-count"[^>]*>&middot;12</);
  assert.match(html, /12 matching tasks/);
});

test("renderViewChips singularizes the badge tooltip for one match", () => {
  const views = addView([], "work", { ...emptyF(), tags: ["work"] });
  const html = renderViewChips(views, emptyF(), { matchCount: () => 1 });
  assert.match(html, /1 matching task[^s]/);
});

test("renderViewChips suppresses the badge for a null/undefined count or no resolver", () => {
  const views = addView([], "work", { ...emptyF(), tags: ["work"] });
  // null count -> no badge.
  assert.doesNotMatch(renderViewChips(views, emptyF(), { matchCount: () => null }), /view-chip-count/);
  // no resolver -> byte-identical to before (no badge).
  assert.doesNotMatch(renderViewChips(views, emptyF(), {}), /view-chip-count/);
});

// --- F145: open/done breakdown for the badge tooltip -----------------------

test("countViewMatchesBreakdown splits a filter view's matches into open/done", () => {
  const v: SavedView = { id: "f", name: "work", filter: { ...emptyF(), tags: ["work"] } };
  // #1 (open) + #2 (open) carry #work; #3 is #home (not matched). -> 2 open, 0 done.
  assert.deepEqual(
    countViewMatchesBreakdown(v, COUNT_TASKS, counters(), (t) => t.done),
    { open: 2, done: 0 },
  );
});

test("countViewMatchesBreakdown counts done matches when the filter admits them", () => {
  // A lens view that passes all high-priority tasks: #1 (open, #work) + #3
  // (done, #home) -> with an empty filter the lens alone gates -> 1 open, 1 done.
  const v: SavedView = { id: "l", name: "(hi)", filter: emptyF(), lens: "blocked" };
  assert.deepEqual(
    countViewMatchesBreakdown(v, COUNT_TASKS, counters(), (t) => t.done),
    { open: 1, done: 1 },
  );
});

test("countViewMatchesBreakdown reports a cohort as all-open by construction", () => {
  const v = addCohortView([], "#5", 5)[0];
  // A cohort's id-set is the undone dependents — open by construction, done=0.
  assert.deepEqual(
    countViewMatchesBreakdown(v, COUNT_TASKS, counters([7, 8, 9]), (t) => t.done),
    { open: 3, done: 0 },
  );
});

test("describeViewMatchBreakdown renders open-only, the split, and the empty case", () => {
  assert.equal(describeViewMatchBreakdown({ open: 12, done: 0 }), "12 open");
  assert.equal(describeViewMatchBreakdown({ open: 12, done: 3 }), "12 open \u00b7 3 done");
  assert.equal(describeViewMatchBreakdown({ open: 0, done: 3 }), "0 open \u00b7 3 done");
  assert.equal(describeViewMatchBreakdown({ open: 0, done: 0 }), "no matches");
});

test("renderViewChips uses the matchTitle breakdown for the badge tooltip", () => {
  const views = addView([], "work", { ...emptyF(), tags: ["work"] });
  const html = renderViewChips(views, emptyF(), {
    matchCount: () => 12,
    matchTitle: () => "9 open \u00b7 3 done",
  });
  // The richer breakdown replaces the bare "·N matching tasks" in title + aria.
  assert.match(html, /title="9 open \xb7 3 done"/);
  assert.match(html, /aria-label="9 open \xb7 3 done"/);
  // The number itself is still the badge text.
  assert.match(html, /&middot;12</);
});

test("renderViewChips falls back to the plain count tooltip without matchTitle", () => {
  const views = addView([], "work", { ...emptyF(), tags: ["work"] });
  const html = renderViewChips(views, emptyF(), { matchCount: () => 5 });
  assert.match(html, /5 matching tasks/);
});

// --- F142: busiest-view marker ---------------------------------------------

test("busiestViewId picks the single densest view", () => {
  const a: SavedView = { id: "a", name: "a", filter: { ...emptyF(), tags: ["a"] } };
  const b: SavedView = { id: "b", name: "b", filter: { ...emptyF(), tags: ["b"] } };
  const c: SavedView = { id: "c", name: "c", filter: { ...emptyF(), tags: ["c"] } };
  const counts: Record<string, number> = { a: 3, b: 9, c: 5 };
  assert.equal(busiestViewId([a, b, c], (v) => counts[v.id]), "b");
});

test("busiestViewId returns null on a tie for the top", () => {
  const a: SavedView = { id: "a", name: "a", filter: { ...emptyF(), tags: ["a"] } };
  const b: SavedView = { id: "b", name: "b", filter: { ...emptyF(), tags: ["b"] } };
  const counts: Record<string, number> = { a: 7, b: 7 };
  assert.equal(busiestViewId([a, b], (v) => counts[v.id]), null);
});

test("busiestViewId ignores zero/null counts and an empty list", () => {
  const a: SavedView = { id: "a", name: "a", filter: { ...emptyF(), tags: ["a"] } };
  const b: SavedView = { id: "b", name: "b", filter: { ...emptyF(), tags: ["b"] } };
  // All-zero / null -> nothing is "busy".
  assert.equal(busiestViewId([a, b], () => 0), null);
  assert.equal(busiestViewId([a, b], () => null), null);
  assert.equal(busiestViewId([], () => 5), null);
  // A single positive count beats a field of zeros.
  assert.equal(busiestViewId([a, b], (v) => (v.id === "a" ? 0 : 4)), "b");
});

test("renderViewChips marks only the busiest chip with is-busiest", () => {
  const views = [
    ...addView([], "a", { ...emptyF(), tags: ["a"] }),
  ];
  const second = addView(views, "b", { ...emptyF(), tags: ["b"] });
  const html = renderViewChips(second, emptyF(), { busiestId: second[1].id });
  // Exactly one chip carries the class.
  assert.equal((html.match(/is-busiest/g) ?? []).length, 1);
  // It's the chip with the matching id.
  assert.match(
    html,
    new RegExp(`view-chip[^"]*is-busiest"[^>]*data-view-id="${second[1].id}"`),
  );
});

test("renderViewChips omits is-busiest when busiestId is null/absent", () => {
  const views = addView([], "a", { ...emptyF(), tags: ["a"] });
  assert.doesNotMatch(renderViewChips(views, emptyF(), { busiestId: null }), /is-busiest/);
  assert.doesNotMatch(renderViewChips(views, emptyF(), {}), /is-busiest/);
});

// --- F148: Views-row coverage summary --------------------------------------

test("viewsRowSummary sums distinct views + total matched tasks", () => {
  const a: SavedView = { id: "a", name: "a", filter: { ...emptyF(), tags: ["a"] } };
  const b: SavedView = { id: "b", name: "b", filter: { ...emptyF(), tags: ["b"] } };
  const c: SavedView = { id: "c", name: "c", filter: { ...emptyF(), tags: ["c"] } };
  const counts: Record<string, number> = { a: 3, b: 9, c: 5 };
  assert.deepEqual(viewsRowSummary([a, b, c], (v) => counts[v.id]), { views: 3, tasks: 17 });
});

test("viewsRowSummary treats null/zero/negative counts as zero", () => {
  const a: SavedView = { id: "a", name: "a", filter: { ...emptyF(), tags: ["a"] } };
  const b: SavedView = { id: "b", name: "b", filter: { ...emptyF(), tags: ["b"] } };
  // null + a real count -> only the real one contributes; views still counts both.
  assert.deepEqual(viewsRowSummary([a, b], (v) => (v.id === "a" ? null : 4)), { views: 2, tasks: 4 });
  assert.deepEqual(viewsRowSummary([a, b], () => 0), { views: 2, tasks: 0 });
  assert.deepEqual(viewsRowSummary([a, b], () => -1), { views: 2, tasks: 0 });
});

test("viewsRowSummary is zeros for an empty list", () => {
  assert.deepEqual(viewsRowSummary([], () => 5), { views: 0, tasks: 0 });
});

test("describeViewsRowSummary renders the readout, singularizing both nouns", () => {
  assert.equal(describeViewsRowSummary({ views: 3, tasks: 17 }), "3 views \u00b7 17 tasks");
  assert.equal(describeViewsRowSummary({ views: 1, tasks: 1 }), "1 view \u00b7 1 task");
});

test("describeViewsRowSummary drops the task half when nothing matches", () => {
  // Views exist but match nothing -> "3 views", not "3 views · 0 tasks".
  assert.equal(describeViewsRowSummary({ views: 3, tasks: 0 }), "3 views");
  // No views at all -> empty (the row + readout hide).
  assert.equal(describeViewsRowSummary({ views: 0, tasks: 0 }), "");
});

// --- F154: views-summary names the busiest inline --------------------------

test("describeViewsRowSummary appends the busiest headline when supplied", () => {
  assert.equal(
    describeViewsRowSummary({ views: 3, tasks: 17 }, { name: "#work", count: 9 }),
    "3 views \u00b7 17 tasks \u00b7 busiest: #work (9)",
  );
});

test("describeViewsRowSummary keeps the bare readout with no busiest winner", () => {
  // null / omitted busiest -> byte-identical to the F148 readout.
  assert.equal(describeViewsRowSummary({ views: 2, tasks: 5 }, null), "2 views \u00b7 5 tasks");
  assert.equal(describeViewsRowSummary({ views: 2, tasks: 5 }), "2 views \u00b7 5 tasks");
  // An empty-name busiest (defensive) is treated as no winner.
  assert.equal(
    describeViewsRowSummary({ views: 2, tasks: 5 }, { name: "", count: 0 }),
    "2 views \u00b7 5 tasks",
  );
});

test("describeViewsRowSummary omits the busiest headline on an all-empty board", () => {
  // No task half means no pile-up to name, even if a busiest is passed.
  assert.equal(
    describeViewsRowSummary({ views: 3, tasks: 0 }, { name: "#work", count: 0 }),
    "3 views",
  );
});

// --- F175: views-row summary "· M done" segment ----------------------------

test("describeViewsRowSummary folds a done total after the task count", () => {
  assert.equal(
    describeViewsRowSummary({ views: 2, tasks: 17 }, null, 5),
    "2 views \u00b7 17 tasks \u00b7 5 done",
  );
});

test("describeViewsRowSummary drops the done segment when nothing's done", () => {
  assert.equal(describeViewsRowSummary({ views: 2, tasks: 17 }, null, 0), "2 views \u00b7 17 tasks");
  assert.equal(describeViewsRowSummary({ views: 2, tasks: 17 }, null, -3), "2 views \u00b7 17 tasks");
  assert.equal(describeViewsRowSummary({ views: 2, tasks: 17 }, null), "2 views \u00b7 17 tasks");
});

test("describeViewsRowSummary keeps done before the busiest headline", () => {
  assert.equal(
    describeViewsRowSummary({ views: 3, tasks: 17 }, { name: "#work", count: 9 }, 4),
    "3 views \u00b7 17 tasks \u00b7 4 done \u00b7 busiest: #work (9)",
  );
});

test("describeViewsRowSummary never shows done with no task half", () => {
  assert.equal(describeViewsRowSummary({ views: 3, tasks: 0 }, null, 5), "3 views");
});

test("viewsRowDoneCount sums done across views, skipping nullish/negatives", () => {
  const v = (id: string): SavedView => ({ id, name: id, filter: emptyF() });
  const list = [v("a"), v("b"), v("c"), v("d")];
  const counts: Record<string, number | null | undefined> = { a: 3, b: 0, c: null, d: -2 };
  assert.equal(viewsRowDoneCount(list, (x) => counts[x.id]), 3);
  assert.equal(viewsRowDoneCount([], () => 1), 0);
});

// --- F163: views-row summary "· N stale" segment ---------------------------

test("appendStaleSegment tacks a stale count onto the summary", () => {
  assert.equal(appendStaleSegment("3 views \u00b7 17 tasks", 2), "3 views \u00b7 17 tasks \u00b7 2 stale");
  assert.equal(appendStaleSegment("3 views", 1), "3 views \u00b7 1 stale");
});

test("appendStaleSegment leaves the summary unchanged with no stale views", () => {
  assert.equal(appendStaleSegment("3 views \u00b7 17 tasks", 0), "3 views \u00b7 17 tasks");
  assert.equal(appendStaleSegment("3 views \u00b7 17 tasks", -1), "3 views \u00b7 17 tasks");
});

test("appendStaleSegment adds nothing to an empty (no-views) summary", () => {
  assert.equal(appendStaleSegment("", 5), "");
});

test("appendStaleSegment composes after the busiest headline", () => {
  const base = describeViewsRowSummary({ views: 3, tasks: 17 }, { name: "#work", count: 9 });
  assert.equal(appendStaleSegment(base, 2), "3 views \u00b7 17 tasks \u00b7 busiest: #work (9) \u00b7 2 stale");
});

// --- F152: actionable hide-done count badge --------------------------------

test("badgeHidesDone is true for a show-all view holding done tasks", () => {
  const v: SavedView = { id: "a", name: "a", filter: { ...emptyF(), tags: ["a"] } };
  assert.equal(badgeHidesDone(v, { open: 12, done: 3 }), true);
});

test("badgeHidesDone is false when the view already hides done", () => {
  const v: SavedView = { id: "a", name: "a", filter: { ...emptyF(), tags: ["a"], hideDone: true } };
  assert.equal(badgeHidesDone(v, { open: 12, done: 3 }), false);
});

test("badgeHidesDone is false when the match set has no done tasks", () => {
  const v: SavedView = { id: "a", name: "a", filter: { ...emptyF(), tags: ["a"] } };
  assert.equal(badgeHidesDone(v, { open: 12, done: 0 }), false);
});

test("badgeHidesDone is false for a cohort bookmark (no hideDone facet)", () => {
  const v = addCohortView([], "waiting on #5", 5)[0];
  // Even with done tasks in its cohort, a cohort view has no filter to flip.
  assert.equal(badgeHidesDone(v, { open: 4, done: 2 }), false);
});

// --- F158: recall-open-only command title ----------------------------------

test("recallOpenOnlyTitle names the view as the open-only recall", () => {
  assert.equal(recallOpenOnlyTitle("work"), "Recall work (open only)");
});

test("recallOpenOnlyTitle preserves the name verbatim (unicode/parens)", () => {
  assert.equal(recallOpenOnlyTitle("(overdue)"), "Recall (overdue) (open only)");
  assert.equal(recallOpenOnlyTitle("#bug \u2192 fix"), "Recall #bug \u2192 fix (open only)");
});

// --- F159: busiest headline is a clickable recall ---------------------------

test("busiestHeadlineHTML wraps the busiest name in a recall button", () => {
  const html = busiestHeadlineHTML({ name: "#work", count: 9 }, "v1");
  assert.match(html, /busiest:/);
  assert.match(html, /data-view-recall="v1"/);
  assert.match(html, />#work<\/button> \(9\)/);
  assert.match(html, /^ \u00b7 busiest:/); // leads with the separator so it composes onto the base
});

test("busiestHeadlineHTML escapes the view name in markup", () => {
  const html = busiestHeadlineHTML({ name: "<b>x</b>", count: 3 }, "v2");
  assert.match(html, /&lt;b&gt;x&lt;\/b&gt;/);
  assert.doesNotMatch(html, /<b>x<\/b>/);
});

test("busiestHeadlineHTML is empty with no clear winner", () => {
  assert.equal(busiestHeadlineHTML(null, "v1"), "");
  assert.equal(busiestHeadlineHTML(undefined, "v1"), "");
  assert.equal(busiestHeadlineHTML({ name: "", count: 0 }, "v1"), "");
});

// --- F164: stale segment is a clickable sweep -------------------------------

test("staleSweepSegmentHTML wraps the count in a sweep button", () => {
  const html = staleSweepSegmentHTML(3);
  assert.match(html, /data-views-sweep/);
  assert.match(html, />3 stale<\/button>/);
  assert.match(html, /^ \u00b7 /); // composes after the busiest segment
});

test("staleSweepSegmentHTML is empty for zero/negative stale", () => {
  assert.equal(staleSweepSegmentHTML(0), "");
  assert.equal(staleSweepSegmentHTML(-2), "");
});

// --- F172: stale-segment tooltip names the dead cohort views ----------------

test("staleSweepTitle names the stale views when supplied", () => {
  assert.equal(staleSweepTitle(["a", "b", "c"]), "Forget: a, b, c");
});

test("staleSweepTitle falls back to the generic phrase with no names", () => {
  assert.equal(staleSweepTitle(), "Forget every stale cohort view");
  assert.equal(staleSweepTitle([]), "Forget every stale cohort view");
});

test("staleSweepSegmentHTML folds the names into the button title (escaped)", () => {
  const html = staleSweepSegmentHTML(2, ["waiting on #7", "x<y"]);
  assert.match(html, /title="Forget: waiting on #7, x&lt;y"/);
});

// --- F167: views-summary headline hover tooltip -----------------------------

test("viewsSummaryTooltip combines busiest and stale", () => {
  assert.equal(viewsSummaryTooltip({ name: "#work", count: 9 }, 2), "busiest: #work (9) \u00b7 2 stale");
});

test("viewsSummaryTooltip drops absent clauses", () => {
  assert.equal(viewsSummaryTooltip({ name: "#work", count: 9 }, 0), "busiest: #work (9)");
  assert.equal(viewsSummaryTooltip(null, 3), "3 stale");
  assert.equal(viewsSummaryTooltip(null, 0), "");
  assert.equal(viewsSummaryTooltip({ name: "", count: 0 }, 0), "");
});

// --- F176: group views by first tag -----------------------------------------

test("groupViewsByTag clusters by first tag, untagged last, order stable", () => {
  let v = addView([], "a", { ...EMPTY, tags: ["work"] });
  v = addView(v, "b", { ...EMPTY, tags: ["home"] });
  v = addView(v, "c", { ...EMPTY, query: "z" }); // untagged but non-empty
  v = addView(v, "d", { ...EMPTY, tags: ["work", "x"] }); // first tag work
  const groups = groupViewsByTag(v);
  assert.deepEqual(groups.map((g) => g.tag), ["work", "home", ""]);
  assert.deepEqual(groups[0].views.map((x) => x.name), ["a", "d"]);
  assert.equal(groups[2].views.length, 1);
});

test("groupViewsByTag returns [] for an empty list", () => {
  assert.deepEqual(groupViewsByTag([]), []);
});

test("describeViewGroups reads the cluster breakdown, empty under 2 groups", () => {
  let v = addView([], "a", { ...EMPTY, tags: ["work"] });
  v = addView(v, "b", { ...EMPTY, tags: ["work"] });
  v = addView(v, "c", { ...EMPTY, query: "z" }); // untagged but non-empty
  assert.equal(describeViewGroups(groupViewsByTag(v)), "#work: 2 \u00b7 untagged: 1");
  const one = addView([], "a", { ...EMPTY, tags: ["work"] });
  assert.equal(describeViewGroups(groupViewsByTag(one)), "");
});



// --- F165: peek open-only preview label -------------------------------------

test("peekOpenOnlyLabel renders the open count + facet summary", () => {
  const v = addView([], "work", { ...EMPTY, tags: ["work"] })[0];
  assert.match(peekOpenOnlyLabel(v, 9), /^9 open \u00b7 /);
  assert.match(peekOpenOnlyLabel(v, 1), /^1 open \u00b7 /);
});

test("peekOpenOnlyLabel reads 'all done' at zero open", () => {
  const v = addView([], "work", { ...EMPTY, tags: ["work"] })[0];
  assert.match(peekOpenOnlyLabel(v, 0), /^all done \u00b7 /);
});

test("peekOpenOnlyLabel drops the count half for a null/undefined open count", () => {
  const v = addView([], "work", { ...EMPTY, tags: ["work"] })[0];
  assert.doesNotMatch(peekOpenOnlyLabel(v, null), /open/);
  assert.doesNotMatch(peekOpenOnlyLabel(v, undefined), /open/);
});

// --- F171: peek-open title carries the open count ---------------------------

test("peekOpenOnlyTitle folds a quiet open count onto the title", () => {
  assert.equal(peekOpenOnlyTitle("work", 9), "Peek open-only (work) \u00b79");
  assert.equal(peekOpenOnlyTitle("work", 0), "Peek open-only (work) \u00b70");
});

test("peekOpenOnlyTitle keeps the plain title with no open count", () => {
  assert.equal(peekOpenOnlyTitle("work", null), "Peek open-only (work)");
  assert.equal(peekOpenOnlyTitle("work", undefined), "Peek open-only (work)");
});

// --- F166: recall-open title carries the open count -------------------------

test("recallOpenOnlyTitle folds a quiet open count onto the title", () => {
  assert.equal(recallOpenOnlyTitle("work", 9), "Recall work (open only) \u00b79");
  assert.equal(recallOpenOnlyTitle("work", 0), "Recall work (open only) \u00b70");
});

test("recallOpenOnlyTitle keeps the plain title with no open count", () => {
  assert.equal(recallOpenOnlyTitle("work"), "Recall work (open only)");
  assert.equal(recallOpenOnlyTitle("work", null), "Recall work (open only)");
  assert.equal(recallOpenOnlyTitle("work", undefined), "Recall work (open only)");
});

test("renderViewChips emits an actionable button badge only when hideDoneBadge is true", () => {
  const views = addView([], "work", { ...emptyF(), tags: ["work"] });
  // Actionable -> a <button data-view-hide-done> with the hint in the title.
  const on = renderViewChips(views, emptyF(), {
    matchCount: () => 15,
    hideDoneBadge: () => true,
  });
  assert.match(on, /<button[^>]*class="view-chip-count is-actionable"[^>]*data-view-hide-done=/);
  assert.match(on, /click to hide done/);
  // Not actionable -> the inert <span> badge, byte-compatible with F141.
  const off = renderViewChips(views, emptyF(), {
    matchCount: () => 15,
    hideDoneBadge: () => false,
  });
  assert.match(off, /<span class="view-chip-count"/);
  assert.doesNotMatch(off, /data-view-hide-done/);
  // No resolver -> inert span (existing callers unaffected).
  assert.doesNotMatch(
    renderViewChips(views, emptyF(), { matchCount: () => 15 }),
    /data-view-hide-done/,
  );
});

// --- F146: peek view (preview without recall) ------------------------------

test("peekViewLabel combines the live count with the facet description", () => {
  const v: SavedView = { id: "f", name: "work", filter: { ...emptyF(), tags: ["work"] } };
  assert.equal(peekViewLabel(v, 12), "12 tasks \u00b7 tags: #work");
  // Singularizes one task.
  assert.equal(peekViewLabel(v, 1), "1 task \u00b7 tags: #work");
});

test("peekViewLabel reads 'no matches' for a zero count", () => {
  const v: SavedView = { id: "f", name: "work", filter: { ...emptyF(), tags: ["work"] } };
  assert.equal(peekViewLabel(v, 0), "no matches \u00b7 tags: #work");
});

test("peekViewLabel drops the count half for a null/undefined count", () => {
  const v: SavedView = { id: "f", name: "work", filter: { ...emptyF(), tags: ["work"] } };
  // Description-only (describeView never returns "").
  assert.equal(peekViewLabel(v, null), "tags: #work");
  assert.equal(peekViewLabel(v, undefined), "tags: #work");
});

test("peekViewLabel describes a cohort + a lens view by their kind", () => {
  const cohort = addCohortView([], "#5", 5)[0];
  assert.equal(peekViewLabel(cohort, 3), "3 tasks \u00b7 waiting on #5");
  const lensView: SavedView = { id: "l", name: "(overdue)", filter: emptyF(), lens: "overdue" };
  assert.equal(peekViewLabel(lensView, 7), "7 tasks \u00b7 lens: overdue");
});

// --- F157: peek-view command title carries the live count ------------------

test("peekCommandTitle folds a quiet count suffix onto the peek title", () => {
  assert.equal(peekCommandTitle("work", 12), "Peek view (work) \u00b712");
  assert.equal(peekCommandTitle("urgent", 1), "Peek view (urgent) \u00b71");
});

test("peekCommandTitle renders 0 honestly rather than hiding it", () => {
  assert.equal(peekCommandTitle("empty", 0), "Peek view (empty) \u00b70");
});

test("peekCommandTitle keeps the plain title for a null/undefined count", () => {
  assert.equal(peekCommandTitle("work", null), "Peek view (work)");
  assert.equal(peekCommandTitle("work", undefined), "Peek view (work)");
});

test("peekCommandTitle preserves a name with parens or unicode verbatim", () => {
  assert.equal(peekCommandTitle("(overdue)", 4), "Peek view ((overdue)) \u00b74");
});

// --- F160: cohort busiest peek names the waiter depth -----------------------

test("enrichCohortPeek appends the waiter depth for a cohort bookmark", () => {
  const cohort = addCohortView([], "#7", 7)[0];
  assert.equal(
    enrichCohortPeek(cohort, "4 tasks \u00b7 waiting on #7", 4),
    "4 tasks \u00b7 waiting on #7 \u00b7 4 waiting",
  );
});

test("enrichCohortPeek leaves a non-cohort view's label unchanged", () => {
  const v: SavedView = { id: "f", name: "work", filter: { ...emptyF(), tags: ["work"] } };
  assert.equal(enrichCohortPeek(v, "12 tasks \u00b7 tags: #work", 12), "12 tasks \u00b7 tags: #work");
});

test("enrichCohortPeek leaves the label unchanged with no known waiter count", () => {
  const cohort = addCohortView([], "#7", 7)[0];
  assert.equal(enrichCohortPeek(cohort, "waiting on #7", null), "waiting on #7");
  assert.equal(enrichCohortPeek(cohort, "waiting on #7", undefined), "waiting on #7");
});

// --- F144: stale cohort-view bulk sweep ------------------------------------

test("staleCohortViewIds returns every dead cohort bookmark's id", () => {
  // Two cohort views (#1, #2) + a plain filter view. #2's chokepoint is dead.
  let views = addCohortView([], "wait 1", 1);
  views = addCohortView(views, "wait 2", 2);
  views = addView(views, "work", { ...emptyF(), tags: ["work"] });
  const ids = staleCohortViewIds(views, (id) => id === 1); // only #1 is live
  // Only the #2 cohort view is stale; the live one + the filter view are not.
  const dead = views.find((v) => v.cohort === 2)!;
  assert.deepEqual(ids, [dead.id]);
});

test("staleCohortViewIds returns [] when every cohort is live (or there are none)", () => {
  let views = addCohortView([], "wait 1", 1);
  views = addCohortView(views, "wait 2", 2);
  // Both chokepoints live -> nothing stale.
  assert.deepEqual(staleCohortViewIds(views, () => true), []);
  // No cohort views at all -> nothing stale.
  const filterOnly = addView([], "work", { ...emptyF(), tags: ["work"] });
  assert.deepEqual(staleCohortViewIds(filterOnly, () => false), []);
});

test("staleCohortViewIds preserves list order and agrees with isStaleCohortView", () => {
  let views = addCohortView([], "a", 1);
  views = addCohortView(views, "b", 2);
  views = addCohortView(views, "c", 3);
  // #1 and #3 are dead, #2 is live -> ids in list order.
  const dead = (id: number) => id === 2;
  const ids = staleCohortViewIds(views, dead);
  const expected = views.filter((v) => isStaleCohortView(v, dead)).map((v) => v.id);
  assert.deepEqual(ids, expected);
  assert.equal(ids.length, 2);
});

// --- F151: stale-sweep undo (snapshot + restore) ---------------------------

test("snapshotViews captures the named views as detached copies, in list order", () => {
  let views = addCohortView([], "a", 1);
  views = addCohortView(views, "b", 2);
  views = addView(views, "work", { ...emptyF(), tags: ["work"] });
  const ids = [views[2].id, views[0].id]; // out-of-order request
  const snap = snapshotViews(views, ids);
  // Returned in LIST order (#a, then work), not request order.
  assert.deepEqual(snap.map((v) => v.name), ["a", "work"]);
  // Detached: mutating the live list doesn't touch the snapshot.
  views = removeView(views, views[0].id);
  assert.equal(snap.length, 2);
  assert.equal(snap[0].name, "a");
});

test("snapshotViews returns [] for an empty id set or no matches", () => {
  const views = addCohortView([], "a", 1);
  assert.deepEqual(snapshotViews(views, []), []);
  assert.deepEqual(snapshotViews(views, ["nope"]), []);
});

test("restoreSweptViews re-appends every missing snapshot view", () => {
  let views = addCohortView([], "a", 1);
  views = addCohortView(views, "b", 2);
  const snap = snapshotViews(views, views.map((v) => v.id));
  // Sweep them, then restore from the snapshot.
  const swept = views.filter((v) => v.cohort === 1); // keep only #a
  const restored = restoreSweptViews(swept, snap);
  // Both snapshot views are present again (the surviving #a is not duplicated).
  assert.equal(restored.length, 2);
  assert.deepEqual(new Set(restored.map((v) => v.name)), new Set(["a", "b"]));
});

test("restoreSweptViews is idempotent on id (no duplicates if already present)", () => {
  let views = addCohortView([], "a", 1);
  views = addCohortView(views, "b", 2);
  const snap = snapshotViews(views, views.map((v) => v.id));
  // Restoring into the SAME list (nothing swept) adds nothing — same reference.
  const restored = restoreSweptViews(views, snap);
  assert.equal(restored, views);
  assert.equal(restored.length, 2);
});

test("restoreSweptViews is a no-op for an empty snapshot", () => {
  const views = addCohortView([], "a", 1);
  assert.equal(restoreSweptViews(views, []), views);
});

test("snapshotViews + restoreSweptViews round-trip a stale-sweep faithfully", () => {
  // The real flow: two stale cohorts get swept, then undone.
  let views = addCohortView([], "wait 1", 1);
  views = addCohortView(views, "wait 2", 2);
  views = addView(views, "work", { ...emptyF(), tags: ["work"] });
  const dead = (id: number) => id === 99; // both cohorts stale
  const staleIds = staleCohortViewIds(views, dead);
  assert.equal(staleIds.length, 2);
  const snap = snapshotViews(views, staleIds);
  // Sweep: drop the stale ids.
  let swept = staleIds.reduce((acc, id) => removeView(acc, id), views);
  assert.equal(swept.length, 1); // only the filter view remains
  // Undo: restore.
  swept = restoreSweptViews(swept, snap);
  assert.equal(swept.length, 3);
  assert.deepEqual(new Set(swept.map((v) => v.cohort)), new Set([1, 2, undefined]));
});

// F170: portable export/import doc.
test("exportViewsDoc wraps views in a versioned envelope", () => {
  const views = addView([], "Work", { ...EMPTY, tags: ["work"] });
  const doc = JSON.parse(exportViewsDoc(views));
  assert.equal(doc.tsk, "tsk.views");
  assert.equal(doc.v, 1);
  assert.equal(doc.views.length, 1);
  assert.equal(doc.views[0].name, "Work");
});

test("importViewsDoc merges new views with fresh ids", () => {
  const mine = addView([], "Mine", { ...EMPTY, tags: ["a"] });
  const theirs = addView([], "Theirs", { ...EMPTY, tags: ["b"] });
  const merged = importViewsDoc(mine, exportViewsDoc(theirs));
  assert.equal(merged.length, 2);
  assert.deepEqual(new Set(merged.map((v) => v.name)), new Set(["Mine", "Theirs"]));
  assert.notEqual(merged[1].id, theirs[0].id); // fresh id, no collision
});

test("importViewsDoc drops name dups (yours wins) and rejects garbage", () => {
  const mine = addView([], "Work", { ...EMPTY, tags: ["mine"] });
  const dup = addView([], "WORK", { ...EMPTY, tags: ["theirs"] });
  assert.equal(importViewsDoc(mine, exportViewsDoc(dup)).length, 1);
  assert.equal(importViewsDoc(mine, "not json").length, 1);
  assert.equal(importViewsDoc(mine, JSON.stringify({ tsk: "other", v: 1, views: [] })).length, 1);
  assert.equal(importViewsDoc(mine, JSON.stringify({ tsk: "tsk.views", v: 2, views: [] })).length, 1);
});

// F177: chip cluster dividers.
test("viewGroupDividers labels each cluster with its ids", () => {
  let vs = addView([], "A", { ...EMPTY, tags: ["work"] });
  vs = addView(vs, "B", { ...EMPTY, tags: ["home"] });
  const div = viewGroupDividers(groupViewsByTag(vs));
  assert.equal(div.length, 2);
  assert.equal(div[0].label, "#work");
  assert.equal(div[1].label, "#home");
  assert.equal(div[0].ids.length, 1);
});

test("viewGroupDividers empty under 2 groups; untagged reads untagged", () => {
  let vs = addView([], "A", { ...EMPTY, tags: ["work"] });
  assert.deepEqual(viewGroupDividers(groupViewsByTag(vs)), []);
  vs = addView(vs, "B", { ...EMPTY, query: "x" });
  const div = viewGroupDividers(groupViewsByTag(vs));
  assert.equal(div[div.length - 1].label, "untagged");
});

// F178: clickable done segment.
test("doneSegmentHTML wraps the done total in a recall button", () => {
  const html = doneSegmentHTML(5);
  assert.match(html, /data-views-done/);
  assert.match(html, /5 done/);
  assert.equal(doneSegmentHTML(0), "");
  assert.equal(doneSegmentHTML(-2), "");
});

// F179: aged stale sweep title.
test("staleSweepTitleAged names each dead view with its age", () => {
  assert.equal(staleSweepTitleAged([{ name: "a", days: 3 }]), "Forget: a (dead 3d)");
  assert.equal(staleSweepTitleAged([{ name: "a", days: 0 }]), "Forget: a (dead today)");
  assert.equal(staleSweepTitleAged([{ name: "a", days: -1 }]), "Forget: a");
  assert.equal(staleSweepTitleAged([]), "Forget every stale cohort view");
  assert.equal(staleSweepTitleAged(), "Forget every stale cohort view");
});

// F180: peek-open keyboard recall title.
test("peekOpenRecallTitle folds the open count when present", () => {
  assert.equal(peekOpenRecallTitle("Work", 9), "Recall open-only (Work) \u00b79");
  assert.equal(peekOpenRecallTitle("Work", 0), "Recall open-only (Work) \u00b70");
  assert.equal(peekOpenRecallTitle("Work", null), "Recall open-only (Work)");
});

// F183: per-chip divider label resolver — only the first id of each cluster.
test("viewDividerLabelBefore labels only the first chip of each cluster", () => {
  let vs = addView([], "A", { ...EMPTY, tags: ["work"] });
  vs = addView(vs, "B", { ...EMPTY, tags: ["work"] });
  vs = addView(vs, "C", { ...EMPTY, tags: ["home"] });
  const groups = groupViewsByTag(vs);
  const ids = groups.flatMap((g) => g.views.map((v) => v.id));
  const at = viewDividerLabelBefore(groups);
  assert.equal(at(ids[0]), "#work"); // first work chip leads
  assert.equal(at(ids[1]), ""); // second work chip no divider
  assert.equal(at(ids[2]), "#home"); // first home chip leads
});

test("viewDividerLabelBefore answers empty under 2 groups", () => {
  const vs = addView([], "A", { ...EMPTY, tags: ["work"] });
  const at = viewDividerLabelBefore(groupViewsByTag(vs));
  assert.equal(at(vs[0].id), "");
});

// F184: aged sweep title override on the summary segment.
test("staleSweepSegmentHTML accepts an aged title override", () => {
  const aged = staleSweepTitleAged([{ name: "a", days: 3 }]);
  const html = staleSweepSegmentHTML(1, ["a"], aged);
  assert.match(html, /Forget: a \(dead 3d\)/);
  assert.match(html, />1 stale<\/button>/);
});

test("staleSweepSegmentHTML falls back to names when no title given", () => {
  const html = staleSweepSegmentHTML(2, ["a", "b"]);
  assert.match(html, /Forget: a, b/);
});

// F187: previewImportViews counts genuinely-new views without committing.
test("previewImportViews counts only the genuinely-new views", () => {
  let mine = addView([], "A", { ...EMPTY, tags: ["work"] });
  const theirs = addView([], "A", { ...EMPTY, tags: ["work"] }); // name dup
  const more = addView(theirs, "B", { ...EMPTY, tags: ["home"] });
  const doc = exportViewsDoc(more);
  // mine has A; doc has A (dup, skip) + B (new) => +1
  assert.equal(previewImportViews(mine, doc), 1);
  // matches importViewsDoc's actual append count
  assert.equal(importViewsDoc(mine, doc).length - mine.length, 1);
});

test("previewImportViews returns 0 for garbage / all-dup docs", () => {
  const mine = addView([], "A", { ...EMPTY, tags: ["work"] });
  assert.equal(previewImportViews(mine, "not json"), 0);
  assert.equal(previewImportViews(mine, exportViewsDoc(mine)), 0); // all dups
  assert.equal(previewImportViews([], '{"tsk":"nope","v":1,"views":[]}'), 0);
});

// F189: clickable cluster divider — "#tag" -> recall button, untagged stays inert.
test("renderViewDivider makes a #tag heading a recall button", () => {
  const html = renderViewDivider("#work");
  assert.match(html, /data-divider-tag="work"/);
  assert.match(html, /button/);
  assert.match(html, />#work</);
});

test("renderViewDivider keeps untagged label inert", () => {
  const html = renderViewDivider("untagged");
  assert.match(html, /<span/);
  assert.doesNotMatch(html, /data-divider-tag/);
});

// F190: chip stale age phrase.
test("chipStaleTitle folds the age in when known", () => {
  assert.equal(chipStaleTitle(3), " \u2014 stale 3d, recall to clear");
  assert.equal(chipStaleTitle(0), " \u2014 stale today, recall to clear");
});

test("chipStaleTitle degrades to bare phrase without an age", () => {
  assert.equal(chipStaleTitle(null), " \u2014 stale, recall to clear");
  assert.equal(chipStaleTitle(-1), " \u2014 stale, recall to clear");
});

// F190: staleCohortAge resolver threads "stale Nd" into a stale cohort chip title.
test("renderViewChips folds chip stale age into the title", () => {
  const vs = addCohortView([], "Blocked", 7);
  const html = renderViewChips(vs, EMPTY, {
    activeCohort: null,
    staleCohort: () => true,
    staleCohortAge: () => 2,
  });
  assert.match(html, /stale 2d, recall to clear/);
});

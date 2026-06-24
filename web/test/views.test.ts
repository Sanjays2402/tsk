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
  activeView,
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

test("renderViewChips escapes view names", () => {
  const views: SavedView[] = [
    { id: "1", name: "<b>x</b>", filter: { ...EMPTY, tags: ["a"] } },
  ];
  const html = renderViewChips(views, EMPTY);
  assert.doesNotMatch(html, /<b>x<\/b>/);
  assert.match(html, /&lt;b&gt;/);
});

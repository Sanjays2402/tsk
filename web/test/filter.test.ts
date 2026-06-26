import { test } from "node:test";
import assert from "node:assert/strict";
import {
  emptyFilter,
  isFilterActive,
  isSubsequence,
  fuzzyMatch,
  matchesFilter,
  applyFilter,
  collectTags,
  toggleMember,
  renderPriorityPills,
  renderTagChips,
  filterSummary,
  renderPriorityScopeNote,
  renderTagScopeNote,
  priorityGlyph,
  type FilterableTask,
  type FilterState,
} from "../src/filter.ts";

function task(p: Partial<FilterableTask> = {}): FilterableTask {
  return { title: "a task", priority: "medium", tags: [], done: false, ...p };
}

test("emptyFilter is inactive and matches everything", () => {
  const f = emptyFilter();
  assert.equal(isFilterActive(f), false);
  assert.equal(matchesFilter(task(), f), true);
  assert.equal(matchesFilter(task({ done: true, priority: "urgent", tags: ["x"] }), f), true);
});

test("isSubsequence respects order", () => {
  assert.equal(isSubsequence("abc", "aXbXc"), true);
  assert.equal(isSubsequence("", "anything"), true);
  assert.equal(isSubsequence("cba", "abc"), false);
  assert.equal(isSubsequence("abcd", "abc"), false);
});

test("fuzzyMatch: every token must subsequence-match, case-insensitive", () => {
  assert.equal(fuzzyMatch("buy mlk", "buy milk grocery"), true);
  assert.equal(fuzzyMatch("BUY", "buy milk"), true);
  assert.equal(fuzzyMatch("", "anything"), true);
  assert.equal(fuzzyMatch("   ", "anything"), true);
  assert.equal(fuzzyMatch("xyz", "buy milk"), false);
  // Both tokens required:
  assert.equal(fuzzyMatch("buy zzz", "buy milk"), false);
});

test("priority facet is OR within, AND across facets", () => {
  const f: FilterState = { ...emptyFilter(), priorities: ["high", "urgent"] };
  assert.equal(matchesFilter(task({ priority: "high" }), f), true);
  assert.equal(matchesFilter(task({ priority: "urgent" }), f), true);
  assert.equal(matchesFilter(task({ priority: "low" }), f), false);
});

test("tag facet is OR within", () => {
  const f: FilterState = { ...emptyFilter(), tags: ["work", "home"] };
  assert.equal(matchesFilter(task({ tags: ["work"] }), f), true);
  assert.equal(matchesFilter(task({ tags: ["home", "x"] }), f), true);
  assert.equal(matchesFilter(task({ tags: ["errand"] }), f), false);
  assert.equal(matchesFilter(task({ tags: [] }), f), false);
});

test("hideDone drops completed tasks", () => {
  const f: FilterState = { ...emptyFilter(), hideDone: true };
  assert.equal(isFilterActive(f), true);
  assert.equal(matchesFilter(task({ done: true }), f), false);
  assert.equal(matchesFilter(task({ done: false }), f), true);
});

test("query matches against title AND tags combined", () => {
  const f: FilterState = { ...emptyFilter(), query: "grocery" };
  assert.equal(matchesFilter(task({ title: "buy milk", tags: ["grocery"] }), f), true);
  assert.equal(matchesFilter(task({ title: "buy milk", tags: [] }), f), false);
});

test("facets combine with AND across each other", () => {
  const f: FilterState = { ...emptyFilter(), priorities: ["high"], tags: ["work"] };
  assert.equal(matchesFilter(task({ priority: "high", tags: ["work"] }), f), true);
  assert.equal(matchesFilter(task({ priority: "high", tags: ["home"] }), f), false);
  assert.equal(matchesFilter(task({ priority: "low", tags: ["work"] }), f), false);
});

test("applyFilter preserves input order and narrows", () => {
  const tasks = [
    task({ title: "one", priority: "urgent" }),
    task({ title: "two", priority: "low" }),
    task({ title: "three", priority: "urgent" }),
  ];
  const f: FilterState = { ...emptyFilter(), priorities: ["urgent"] };
  assert.deepEqual(
    applyFilter(tasks, f).map((t) => t.title),
    ["one", "three"],
  );
});

test("collectTags counts + sorts by count desc then name asc", () => {
  const tasks = [
    task({ tags: ["work", "urgent"] }),
    task({ tags: ["work"] }),
    task({ tags: ["home", "work"] }),
    task({ tags: ["home"] }),
  ];
  assert.deepEqual(collectTags(tasks), [
    { tag: "work", count: 3 },
    { tag: "home", count: 2 },
    { tag: "urgent", count: 1 },
  ]);
});

test("toggleMember adds then removes", () => {
  assert.deepEqual(toggleMember<string>([], "a"), ["a"]);
  assert.deepEqual(toggleMember(["a", "b"], "a"), ["b"]);
  assert.deepEqual(toggleMember(["a"], "b"), ["a", "b"]);
});

test("priorityGlyph maps each priority", () => {
  assert.equal(priorityGlyph("urgent"), "U");
  assert.equal(priorityGlyph("high"), "H");
  assert.equal(priorityGlyph("medium"), "M");
  assert.equal(priorityGlyph("low"), "L");
});

test("renderPriorityPills marks active + carries data-prio", () => {
  const html = renderPriorityPills({ ...emptyFilter(), priorities: ["urgent"] });
  assert.match(html, /data-prio="urgent"/);
  assert.match(html, /prio-urgent is-active/);
  assert.match(html, /aria-pressed="true"/);
  // Non-selected stay inactive
  assert.match(html, /data-prio="low"[^>]*aria-pressed="false"/);
});

test("renderTagChips renders counts + active state, empty when no tags", () => {
  assert.equal(renderTagChips([], emptyFilter()), "");
  const html = renderTagChips(
    [{ tag: "work", count: 3 }],
    { ...emptyFilter(), tags: ["work"] },
  );
  assert.match(html, /data-tag="work"/);
  assert.match(html, /is-active/);
  assert.match(html, /fchip-n">3</);
});

test("renderTagChips escapes tag names", () => {
  const html = renderTagChips([{ tag: "a<b", count: 1 }], emptyFilter());
  assert.match(html, /a&lt;b/);
  assert.doesNotMatch(html, /a<b/);
});

test("filterSummary phrasing", () => {
  assert.equal(filterSummary(5, 5), "5 tasks");
  assert.equal(filterSummary(1, 1), "1 task");
  assert.equal(filterSummary(2, 7), "2 of 7 shown");
});

// F86 — "in <lens>" qualifier beside the priority pills.
test("renderPriorityScopeNote shows the lens label when a priority facet is active under a lens", () => {
  const f = { ...emptyFilter(), priorities: ["urgent" as const] };
  const html = renderPriorityScopeNote("overdue", f);
  assert.match(html, /in overdue/);
  assert.match(html, /fprio-scope/);
});

test("renderPriorityScopeNote is empty without a lens label", () => {
  const f = { ...emptyFilter(), priorities: ["high" as const] };
  assert.equal(renderPriorityScopeNote(null, f), "");
  assert.equal(renderPriorityScopeNote("", f), "");
});

test("renderPriorityScopeNote is empty when no priority facet is selected", () => {
  assert.equal(renderPriorityScopeNote("overdue", emptyFilter()), "");
});

test("renderPriorityScopeNote escapes the lens label", () => {
  const f = { ...emptyFilter(), priorities: ["low" as const] };
  const html = renderPriorityScopeNote("a<b", f);
  assert.match(html, /a&lt;b/);
  assert.doesNotMatch(html, /in a<b</);
});

// F94 — "in <lens>" qualifier beside the tag chips (sister of F86).
test("renderTagScopeNote shows the lens label when a tag facet is active under a lens", () => {
  const f = { ...emptyFilter(), tags: ["work"] };
  const html = renderTagScopeNote("overdue", f);
  assert.match(html, /in overdue/);
  assert.match(html, /ftag-scope/);
});

test("renderTagScopeNote is empty without a lens label", () => {
  const f = { ...emptyFilter(), tags: ["home"] };
  assert.equal(renderTagScopeNote(null, f), "");
  assert.equal(renderTagScopeNote("", f), "");
});

test("renderTagScopeNote is empty when no tag facet is selected", () => {
  assert.equal(renderTagScopeNote("overdue", emptyFilter()), "");
  // A priority facet alone (no tag) must NOT trigger the tag note.
  assert.equal(renderTagScopeNote("overdue", { ...emptyFilter(), priorities: ["high"] }), "");
});

test("renderTagScopeNote escapes the lens label", () => {
  const f = { ...emptyFilter(), tags: ["x"] };
  const html = renderTagScopeNote("a<b", f);
  assert.match(html, /a&lt;b/);
  assert.doesNotMatch(html, /in a<b</);
});

// --- F99: the scope notes are clickable buttons that clear the lens ---------

test("renderPriorityScopeNote is a button carrying the lens-scope-clear hook", () => {
  const f = { ...emptyFilter(), priorities: ["urgent" as const] };
  const html = renderPriorityScopeNote("overdue", f);
  assert.match(html, /<button[^>]*data-lens-scope-clear/);
  assert.match(html, /type="button"/);
  // Still wears its class + reads "in <lens>" so the F86 styling/tests carry.
  assert.match(html, /fprio-scope/);
  assert.match(html, /in overdue/);
});

test("renderTagScopeNote is a button carrying the lens-scope-clear hook", () => {
  const f = { ...emptyFilter(), tags: ["work"] };
  const html = renderTagScopeNote("due today", f);
  assert.match(html, /<button[^>]*data-lens-scope-clear/);
  assert.match(html, /ftag-scope/);
  assert.match(html, /in due today/);
});

test("F99 scope-clear buttons stay empty without a lens (no dangling button)", () => {
  const fp = { ...emptyFilter(), priorities: ["high" as const] };
  const ft = { ...emptyFilter(), tags: ["home"] };
  assert.equal(renderPriorityScopeNote(null, fp), "");
  assert.equal(renderTagScopeNote(null, ft), "");
});


import { test } from "node:test";
import assert from "node:assert/strict";
import {
  doneIndex,
  openBlockers,
  isBlocked,
  blockerLabel,
  blockedToggleConfirm,
  needsBlockedConfirm,
  blockedInBulkToggle,
  bulkBlockedConfirm,
  computeDepStats,
  filterBlocked,
  deepestChainFrom,
  hasWalkableChain,
  renderBlockedBadge,
  blockedClass,
  type DepTask,
  type DepStatsTask,
} from "../src/deps.ts";

const tasks: DepTask[] = [
  { id: 1, done: true }, // a completed prereq
  { id: 2, done: false }, // an open prereq
  { id: 3, done: false, depends_on: [1] }, // blocked only by a DONE task -> not blocked
  { id: 4, done: false, depends_on: [2] }, // blocked by an OPEN task -> blocked
  { id: 5, done: false, depends_on: [1, 2] }, // mixed -> blocked by #2
  { id: 6, done: true, depends_on: [2] }, // done itself -> never blocked
  { id: 7, done: false, depends_on: [99] }, // dep was deleted -> not blocked
];

test("doneIndex maps every id to its done state", () => {
  const idx = doneIndex(tasks);
  assert.equal(idx.get(1), true);
  assert.equal(idx.get(2), false);
  assert.equal(idx.has(99), false);
});

test("openBlockers ignores done prereqs", () => {
  const idx = doneIndex(tasks);
  assert.deepEqual(openBlockers(tasks[2], idx), []); // #3 deps on done #1
});

test("openBlockers returns open prereqs only", () => {
  const idx = doneIndex(tasks);
  assert.deepEqual(openBlockers(tasks[4], idx), [2]); // #5 deps on [1 done, 2 open]
});

test("openBlockers drops deleted (unknown) dep ids", () => {
  const idx = doneIndex(tasks);
  assert.deepEqual(openBlockers(tasks[6], idx), []); // dep #99 no longer exists
});

test("openBlockers preserves declared order", () => {
  const idx = doneIndex([
    { id: 1, done: false },
    { id: 2, done: false },
    { id: 3, done: false, depends_on: [2, 1] },
  ]);
  assert.deepEqual(openBlockers({ id: 3, done: false, depends_on: [2, 1] }, idx), [2, 1]);
});

test("isBlocked: open prereq blocks, done prereq does not", () => {
  const idx = doneIndex(tasks);
  assert.equal(isBlocked(tasks[3], idx), true); // #4 -> open #2
  assert.equal(isBlocked(tasks[2], idx), false); // #3 -> done #1
});

test("isBlocked: a done task is never blocked", () => {
  const idx = doneIndex(tasks);
  assert.equal(isBlocked(tasks[5], idx), false); // #6 is done
});

test("isBlocked: no deps means not blocked", () => {
  const idx = doneIndex(tasks);
  assert.equal(isBlocked(tasks[1], idx), false);
});

test("blockerLabel reads naturally", () => {
  assert.equal(blockerLabel([]), "");
  assert.equal(blockerLabel([3]), "blocked by #3");
  assert.equal(blockerLabel([3, 7]), "blocked by #3, #7");
});

// --- F45: blocked-toggle confirm guard -------------------------------------

test("needsBlockedConfirm: true only when completing a blocked, undone task", () => {
  const idx = doneIndex(tasks);
  assert.equal(needsBlockedConfirm(tasks[3], idx), true); // #4 -> open #2, undone
  assert.equal(needsBlockedConfirm(tasks[2], idx), false); // #3 -> only done #1
});

test("needsBlockedConfirm: re-opening a done task never prompts", () => {
  const idx = doneIndex(tasks);
  // #6 is done and depends on open #2; toggling it re-opens, no confirm.
  assert.equal(needsBlockedConfirm(tasks[5], idx), false);
});

test("needsBlockedConfirm: an unblocked task never prompts", () => {
  const idx = doneIndex(tasks);
  assert.equal(needsBlockedConfirm(tasks[1], idx), false); // #2 has no deps
});

test("blockedToggleConfirm names the open blockers", () => {
  const idx = doneIndex(tasks);
  assert.equal(blockedToggleConfirm(tasks[3], idx), "#4 is blocked by #2 — complete anyway?");
});

test("blockedToggleConfirm lists multiple blockers", () => {
  const idx = doneIndex([
    { id: 1, done: false },
    { id: 2, done: false },
    { id: 3, done: false, depends_on: [1, 2] },
  ]);
  assert.equal(
    blockedToggleConfirm({ id: 3, done: false, depends_on: [1, 2] }, idx),
    "#3 is blocked by #1, #2 — complete anyway?",
  );
});

test("blockedToggleConfirm is empty when nothing blocks", () => {
  const idx = doneIndex(tasks);
  assert.equal(blockedToggleConfirm(tasks[2], idx), ""); // #3 only deps on done #1
});

// --- F60: bulk blocked-toggle guard ----------------------------------------

test("blockedInBulkToggle flags only the ids that would complete while blocked", () => {
  // From the fixture: #4 and #5 are blocked + undone; #2 is undone but
  // unblocked; #6 is done (re-opening, never flagged); #3 deps only on done #1.
  const flagged = blockedInBulkToggle([2, 3, 4, 5, 6], tasks);
  assert.deepEqual(flagged, [4, 5]);
});

test("blockedInBulkToggle follows the input id order", () => {
  const flagged = blockedInBulkToggle([5, 4], tasks);
  assert.deepEqual(flagged, [5, 4]);
});

test("blockedInBulkToggle skips ids not in the list (deleted)", () => {
  const flagged = blockedInBulkToggle([4, 999], tasks);
  assert.deepEqual(flagged, [4]);
});

test("blockedInBulkToggle returns [] when nothing in the selection is blocked", () => {
  assert.deepEqual(blockedInBulkToggle([1, 2, 3, 6], tasks), []);
});

test("bulkBlockedConfirm summarizes how many of the selection are blocked", () => {
  assert.equal(
    bulkBlockedConfirm([4, 5], 5),
    "2 of 5 tasks are blocked (#4, #5) — complete all anyway?",
  );
});

test("bulkBlockedConfirm uses singular grammar for one blocked task", () => {
  assert.equal(
    bulkBlockedConfirm([4], 3),
    "1 of 3 task is blocked (#4) — complete all anyway?",
  );
});

test("bulkBlockedConfirm is empty when none are blocked", () => {
  assert.equal(bulkBlockedConfirm([], 4), "");
});

test("renderBlockedBadge is empty when nothing blocks", () => {
  const idx = doneIndex(tasks);
  assert.equal(renderBlockedBadge(tasks[2], idx), ""); // #3 not blocked
});

test("renderBlockedBadge carries the first blocker as a jump target", () => {
  const idx = doneIndex(tasks);
  const html = renderBlockedBadge(tasks[4], idx); // #5 -> blocked by #2
  assert.match(html, /data-dep-jump="2"/);
  assert.match(html, /#2/);
  assert.match(html, /class="dep-badge"/);
});

// --- F61: chain-walk button on the badge ------------------------------------

test("renderBlockedBadge adds a chain-walk button carrying this task's id", () => {
  const idx = doneIndex(tasks);
  const html = renderBlockedBadge(tasks[3], idx); // #4 -> blocked by #2
  assert.match(html, /dep-badge-group/);
  assert.match(html, /data-chain-from="4"/);
  assert.match(html, /dep-chain-btn/);
});

test("blockedClass returns the css flag only when blocked", () => {
  const idx = doneIndex(tasks);
  assert.equal(blockedClass(tasks[3], idx), "is-blocked");
  assert.equal(blockedClass(tasks[2], idx), "");
});

// --- F64: blocked-only lens -------------------------------------------------

test("filterBlocked keeps only the currently-blocked tasks", () => {
  const idx = doneIndex(tasks);
  const blocked = filterBlocked(tasks, idx);
  // #4 (open #2) and #5 (open #2) are the only blocked ones in the fixture.
  assert.deepEqual(blocked.map((t) => t.id), [4, 5]);
});

test("filterBlocked preserves input order", () => {
  const list: DepTask[] = [
    { id: 3, done: false, depends_on: [1] },
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1] },
  ];
  const idx = doneIndex(list);
  // #3 and #2 both depend on open #1; result keeps the [3, 2] input order.
  assert.deepEqual(filterBlocked(list, idx).map((t) => t.id), [3, 2]);
});

test("filterBlocked returns [] when nothing is blocked", () => {
  const list: DepTask[] = [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1, 99] }, // #1 open -> actually blocked
  ];
  // Make #1 done so #2 is unblocked.
  const idx = doneIndex([
    { id: 1, done: true },
    { id: 2, done: false, depends_on: [1, 99] },
  ]);
  assert.deepEqual(filterBlocked(list, idx), []);
});

// --- F46: dependency stats aggregate ---------------------------------------

test("computeDepStats counts blocked + pinned", () => {
  const list: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1], pinned: true },
    { id: 3, done: false, depends_on: [2] },
    { id: 4, done: true, pinned: true }, // done -> not blocked, still pinned
  ];
  const s = computeDepStats(list);
  assert.equal(s.blocked, 2); // #2 (open #1) and #3 (open #2)
  assert.equal(s.pinned, 2); // #2 and #4
});

test("computeDepStats measures the longest open-blocker chain", () => {
  // 4 -> 3 -> 2 -> 1, all open -> depth at #4 is 3
  const list: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1] },
    { id: 3, done: false, depends_on: [2] },
    { id: 4, done: false, depends_on: [3] },
  ];
  assert.equal(computeDepStats(list).longestChain, 3);
});

test("computeDepStats: a done blocker truncates the chain", () => {
  // #2 depends on done #1 -> #2 not blocked; #3 -> open #2 -> depth 1.
  const list: DepStatsTask[] = [
    { id: 1, done: true },
    { id: 2, done: false, depends_on: [1] },
    { id: 3, done: false, depends_on: [2] },
  ];
  const s = computeDepStats(list);
  assert.equal(s.blocked, 1); // only #3
  assert.equal(s.longestChain, 1);
});

test("computeDepStats: flat board has zero everything", () => {
  const list: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false },
  ];
  assert.deepEqual(computeDepStats(list), { blocked: 0, pinned: 0, longestChain: 0 });
});

test("computeDepStats: a dependency cycle is bounded, not infinite", () => {
  // 1 -> 2 -> 1 cycle. Both are blocked; the depth DFS must terminate.
  const list: DepStatsTask[] = [
    { id: 1, done: false, depends_on: [2] },
    { id: 2, done: false, depends_on: [1] },
  ];
  const s = computeDepStats(list);
  assert.equal(s.blocked, 2);
  assert.ok(Number.isFinite(s.longestChain));
});

test("computeDepStats: deleted blocker ids don't extend depth", () => {
  const list: DepStatsTask[] = [{ id: 5, done: false, depends_on: [99] }];
  const s = computeDepStats(list);
  assert.equal(s.blocked, 0); // dep #99 doesn't exist
  assert.equal(s.longestChain, 0);
});

// --- F61: deepestChainFrom (per-task chain walk) ----------------------------

test("deepestChainFrom walks the chain from a specific task to its root", () => {
  // #4 -> #3 -> #2 -> #1; starting at #3 yields just #3 -> #2 -> #1.
  const list: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1] },
    { id: 3, done: false, depends_on: [2] },
    { id: 4, done: false, depends_on: [3] },
  ];
  assert.deepEqual(deepestChainFrom(list, 3), [3, 2, 1]);
  assert.deepEqual(deepestChainFrom(list, 4), [4, 3, 2, 1]);
});

test("deepestChainFrom picks the deeper branch at a fork from the start", () => {
  // #5 -> #4 (deep: ->#3->#2) and #1 (shallow). From #5 the deep branch wins.
  const list: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false },
    { id: 3, done: false, depends_on: [2] },
    { id: 4, done: false, depends_on: [3] },
    { id: 5, done: false, depends_on: [1, 4] },
  ];
  assert.deepEqual(deepestChainFrom(list, 5), [5, 4, 3, 2]);
});

test("deepestChainFrom is empty for an unblocked / done / missing start", () => {
  const list: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1] },
    { id: 3, done: true, depends_on: [1] },
  ];
  assert.deepEqual(deepestChainFrom(list, 1), []); // #1 has no blockers
  assert.deepEqual(deepestChainFrom(list, 3), []); // #3 is done
  assert.deepEqual(deepestChainFrom(list, 99), []); // not in the list
});

test("deepestChainFrom skips a done blocker (only open links extend)", () => {
  // #3 -> done #2 and open #1; the only open step is to #1.
  const list: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: true },
    { id: 3, done: false, depends_on: [1, 2] },
  ];
  assert.deepEqual(deepestChainFrom(list, 3), [3, 1]);
});

test("deepestChainFrom terminates on a cycle", () => {
  const list: DepStatsTask[] = [
    { id: 1, done: false, depends_on: [2] },
    { id: 2, done: false, depends_on: [1] },
  ];
  const path = deepestChainFrom(list, 1);
  assert.ok(path.length <= 2);
  assert.equal(new Set(path).size, path.length); // no id repeats
  assert.equal(path[0], 1); // starts where asked
});

// --- F74: hasWalkableChain (dep-editor chain preview gating) ----------------

test("hasWalkableChain is true when a task chains into >= 1 open blocker", () => {
  const list: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1] },
    { id: 3, done: false, depends_on: [2] },
  ];
  assert.equal(hasWalkableChain(list, 3), true); // 3 -> 2 -> 1
  assert.equal(hasWalkableChain(list, 2), true); // 2 -> 1
});

test("hasWalkableChain is false for a leaf / done / missing task", () => {
  const list: DepStatsTask[] = [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1] },
    { id: 3, done: true, depends_on: [1] },
  ];
  assert.equal(hasWalkableChain(list, 1), false); // #1 has no blockers (leaf)
  assert.equal(hasWalkableChain(list, 3), false); // #3 is done
  assert.equal(hasWalkableChain(list, 99), false); // not in the list
});

test("hasWalkableChain is false when the only blocker is already done", () => {
  const list: DepStatsTask[] = [
    { id: 1, done: true },
    { id: 2, done: false, depends_on: [1] }, // blocked only by a DONE task
  ];
  assert.equal(hasWalkableChain(list, 2), false);
});

import { test } from "node:test";
import assert from "node:assert/strict";
import {
  doneIndex,
  openBlockers,
  isBlocked,
  blockerLabel,
  blockedToggleConfirm,
  needsBlockedConfirm,
  renderBlockedBadge,
  blockedClass,
  type DepTask,
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

test("blockedClass returns the css flag only when blocked", () => {
  const idx = doneIndex(tasks);
  assert.equal(blockedClass(tasks[3], idx), "is-blocked");
  assert.equal(blockedClass(tasks[2], idx), "");
});

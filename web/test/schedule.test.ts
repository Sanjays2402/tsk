import { test } from "node:test";
import assert from "node:assert/strict";
import { computeScheduleStats, type ScheduleTask } from "../src/schedule.ts";
import { renderScheduleStats } from "../src/stats.ts";

// A fixed "now" so the day-window math is deterministic: Wed 2026-06-24.
const NOW = new Date(2026, 5, 24, 10, 0, 0);

/** Build a YYYY-MM-DD string offset from NOW by `days`. */
function dayOffset(days: number): string {
  const d = new Date(2026, 5, 24 + days);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${dd}`;
}

test("computeScheduleStats counts undone tasks due in the next 7 days", () => {
  const tasks: ScheduleTask[] = [
    { done: false, due: dayOffset(0) }, // today -> in week
    { done: false, due: dayOffset(3) }, // +3d -> in week
    { done: false, due: dayOffset(6) }, // +6d -> in week (last day)
    { done: false, due: dayOffset(7) }, // +7d -> just outside
  ];
  assert.equal(computeScheduleStats(tasks, NOW).dueThisWeek, 3);
});

test("computeScheduleStats excludes overdue tasks from the week count", () => {
  const tasks: ScheduleTask[] = [
    { done: false, due: dayOffset(-1) }, // yesterday -> overdue, not "on deck"
    { done: false, due: dayOffset(-30) },
    { done: false, due: dayOffset(2) }, // the only one in the coming week
  ];
  assert.equal(computeScheduleStats(tasks, NOW).dueThisWeek, 1);
});

test("computeScheduleStats counts undone tasks with no due date", () => {
  const tasks: ScheduleTask[] = [
    { done: false }, // no due
    { done: false, due: "" }, // empty -> no due
    { done: false, due: dayOffset(1) }, // scheduled -> not no-due
  ];
  const s = computeScheduleStats(tasks, NOW);
  assert.equal(s.noDue, 2);
  assert.equal(s.dueThisWeek, 1);
});

test("computeScheduleStats ignores done tasks for both lenses", () => {
  const tasks: ScheduleTask[] = [
    { done: true, due: dayOffset(1) }, // done + soon -> not counted
    { done: true }, // done + no due -> not counted
    { done: false, due: dayOffset(1) },
  ];
  const s = computeScheduleStats(tasks, NOW);
  assert.equal(s.dueThisWeek, 1);
  assert.equal(s.noDue, 0);
});

test("computeScheduleStats treats a malformed due string as no-due", () => {
  const tasks: ScheduleTask[] = [{ done: false, due: "not-a-date" }];
  const s = computeScheduleStats(tasks, NOW);
  assert.equal(s.noDue, 1);
  assert.equal(s.dueThisWeek, 0);
});

test("computeScheduleStats on an empty list is all zero", () => {
  assert.deepEqual(computeScheduleStats([], NOW), { dueThisWeek: 0, noDue: 0 });
});

// --- renderScheduleStats ----------------------------------------------------

test("renderScheduleStats shows both tiles with their counts", () => {
  const html = renderScheduleStats({ dueThisWeek: 4, noDue: 7 });
  assert.match(html, /Schedule/);
  assert.match(html, /Due this week/);
  assert.match(html, /No due/);
  assert.match(html, /stat-num">4</);
  assert.match(html, /stat-num">7</);
});

test("renderScheduleStats collapses to empty when both counts are zero", () => {
  assert.equal(renderScheduleStats({ dueThisWeek: 0, noDue: 0 }), "");
});

test("renderScheduleStats still renders when only one lens is non-zero", () => {
  assert.match(renderScheduleStats({ dueThisWeek: 0, noDue: 3 }), /No due/);
  assert.match(renderScheduleStats({ dueThisWeek: 2, noDue: 0 }), /Due this week/);
});

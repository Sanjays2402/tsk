import { test } from "node:test";
import assert from "node:assert/strict";
import {
  sectionFor,
  groupIntoSections,
  flattenSections,
  type TaskLike,
} from "../src/sections.ts";

// Fixed reference day: 2026-06-24.
const NOW = new Date(2026, 5, 24, 9, 0, 0);

function task(p: Partial<TaskLike> & { id: number }): TaskLike {
  return { done: false, priority: "medium", ...p };
}

test("classifies each section relative to now", () => {
  assert.equal(sectionFor(task({ id: 1, due: "2026-06-20" }), NOW), "overdue");
  assert.equal(sectionFor(task({ id: 2, due: "2026-06-24" }), NOW), "today");
  assert.equal(sectionFor(task({ id: 3, due: "2026-06-30" }), NOW), "upcoming");
  assert.equal(sectionFor(task({ id: 4 }), NOW), "nodue");
  assert.equal(sectionFor(task({ id: 5, done: true, due: "2026-06-20" }), NOW), "done");
});

test("done overrides due — an overdue completed task is in Done", () => {
  assert.equal(sectionFor(task({ id: 9, done: true, due: "2026-01-01" }), NOW), "done");
});

test("groups in fixed display order, omitting empty sections", () => {
  const tasks = [
    task({ id: 1, due: "2026-06-30" }), // upcoming
    task({ id: 2, done: true }), // done
    task({ id: 3, due: "2026-06-20" }), // overdue
  ];
  const sections = groupIntoSections(tasks, NOW);
  assert.deepEqual(
    sections.map((s) => s.key),
    ["overdue", "upcoming", "done"],
  );
  // No "today" / "nodue" headers since they're empty.
  assert.equal(sections.find((s) => s.key === "today"), undefined);
});

test("undone sections sort by priority then id", () => {
  const tasks = [
    task({ id: 1, priority: "low" }),
    task({ id: 2, priority: "urgent" }),
    task({ id: 3, priority: "urgent" }),
    task({ id: 4, priority: "high" }),
  ];
  const [nodue] = groupIntoSections(tasks, NOW);
  assert.equal(nodue.key, "nodue");
  assert.deepEqual(
    nodue.tasks.map((t) => t.id),
    [2, 3, 4, 1],
  );
});

test("done section sorts most-recently-completed first", () => {
  const tasks = [
    task({ id: 1, done: true, completed: "2026-06-24T08:00:00Z" }),
    task({ id: 2, done: true, completed: "2026-06-24T10:00:00Z" }),
    task({ id: 3, done: true }), // no timestamp -> sinks below timestamped
  ];
  const [done] = groupIntoSections(tasks, NOW);
  assert.equal(done.key, "done");
  assert.deepEqual(
    done.tasks.map((t) => t.id),
    [2, 1, 3],
  );
});

test("flattenSections yields visible top-to-bottom order", () => {
  const tasks = [
    task({ id: 1, done: true }),
    task({ id: 2, due: "2026-06-20" }), // overdue
    task({ id: 3, due: "2026-06-24" }), // today
  ];
  const flat = flattenSections(groupIntoSections(tasks, NOW));
  assert.deepEqual(
    flat.map((t) => t.id),
    [2, 3, 1],
  );
});

test("empty input yields no sections", () => {
  assert.deepEqual(groupIntoSections([], NOW), []);
});

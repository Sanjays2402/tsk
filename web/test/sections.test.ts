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

// --- F27: Pinned section -----------------------------------------------------

test("a pinned undone task floats into the Pinned section regardless of due", () => {
  assert.equal(sectionFor(task({ id: 1, pinned: true, due: "2026-06-30" }), NOW), "pinned");
  assert.equal(sectionFor(task({ id: 2, pinned: true }), NOW), "pinned");
  assert.equal(sectionFor(task({ id: 3, pinned: true, due: "2026-06-20" }), NOW), "pinned");
});

test("a pinned DONE task stays in Done (a finished pin isn't surfaced)", () => {
  assert.equal(sectionFor(task({ id: 1, pinned: true, done: true }), NOW), "done");
});

test("Pinned renders first, above Overdue", () => {
  const tasks = [
    task({ id: 1, due: "2026-06-20" }), // overdue
    task({ id: 2, pinned: true, due: "2026-06-30" }), // pinned (would be upcoming)
    task({ id: 3, done: true }), // done
  ];
  const sections = groupIntoSections(tasks, NOW);
  assert.deepEqual(
    sections.map((s) => s.key),
    ["pinned", "overdue", "done"],
  );
  assert.deepEqual(
    sections.find((s) => s.key === "pinned")!.tasks.map((t) => t.id),
    [2],
  );
});

test("Pinned section preserves file order (F40: hand-curated, drag-reorderable)", () => {
  // Pins are NOT priority-sorted — they keep the order they appear in the file
  // so a manual drag-reorder sticks. Here urgent #2 comes AFTER low #1 in file
  // order and must stay there.
  const tasks = [
    task({ id: 1, pinned: true, priority: "low" }),
    task({ id: 2, pinned: true, priority: "urgent" }),
    task({ id: 3, pinned: true, priority: "high" }),
  ];
  const [pinned] = groupIntoSections(tasks, NOW);
  assert.equal(pinned.key, "pinned");
  assert.deepEqual(
    pinned.tasks.map((t) => t.id),
    [1, 2, 3], // file order, not priority order
  );
});

test("pinned task is pulled out of its due bucket (not double-counted)", () => {
  const tasks = [
    task({ id: 1, pinned: true, due: "2026-06-20" }), // overdue + pinned
    task({ id: 2, due: "2026-06-20" }), // overdue
  ];
  const sections = groupIntoSections(tasks, NOW);
  const overdue = sections.find((s) => s.key === "overdue")!;
  assert.deepEqual(overdue.tasks.map((t) => t.id), [2]); // #1 moved to Pinned
  const pinned = sections.find((s) => s.key === "pinned")!;
  assert.deepEqual(pinned.tasks.map((t) => t.id), [1]);
});

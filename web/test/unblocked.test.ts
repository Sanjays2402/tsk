import { test } from "node:test";
import assert from "node:assert/strict";
import { newlyUnblocked, unblockedMessage, type DepTask } from "../src/deps.ts";

// Scenario: #3 depends on #1; #4 depends on #1 AND #2. Completing #1 unblocks
// #3 (its only blocker) but NOT #4 (still blocked by open #2).
const before: DepTask[] = [
  { id: 1, done: false },
  { id: 2, done: false },
  { id: 3, done: false, depends_on: [1] },
  { id: 4, done: false, depends_on: [1, 2] },
];

test("newlyUnblocked finds a task whose last blocker just completed", () => {
  const after: DepTask[] = [
    { id: 1, done: true }, // just completed
    { id: 2, done: false },
    { id: 3, done: false, depends_on: [1] },
    { id: 4, done: false, depends_on: [1, 2] },
  ];
  assert.deepEqual(newlyUnblocked(before, after), [3]);
});

test("newlyUnblocked ignores tasks still blocked by another open prereq", () => {
  const after: DepTask[] = [
    { id: 1, done: true },
    { id: 2, done: false },
    { id: 3, done: true, depends_on: [1] }, // also got completed -> not "to start"
    { id: 4, done: false, depends_on: [1, 2] }, // still blocked by #2
  ];
  // #3 is now done (excluded), #4 still blocked -> nothing to announce.
  assert.deepEqual(newlyUnblocked(before, after), []);
});

test("newlyUnblocked reports several when one completion frees many", () => {
  const b: DepTask[] = [
    { id: 1, done: false },
    { id: 2, done: false, depends_on: [1] },
    { id: 3, done: false, depends_on: [1] },
  ];
  const a: DepTask[] = [
    { id: 1, done: true },
    { id: 2, done: false, depends_on: [1] },
    { id: 3, done: false, depends_on: [1] },
  ];
  assert.deepEqual(newlyUnblocked(b, a), [2, 3]);
});

test("newlyUnblocked returns nothing when nothing changed", () => {
  assert.deepEqual(newlyUnblocked(before, before), []);
});

test("newlyUnblocked ignores a task that was never blocked", () => {
  const b: DepTask[] = [
    { id: 1, done: false },
    { id: 2, done: false }, // free-standing, no deps
  ];
  const a: DepTask[] = [
    { id: 1, done: true },
    { id: 2, done: false },
  ];
  assert.deepEqual(newlyUnblocked(b, a), []);
});

test("newlyUnblocked ignores tasks that didn't exist before", () => {
  const b: DepTask[] = [{ id: 1, done: false }];
  const a: DepTask[] = [
    { id: 1, done: true },
    { id: 9, done: false, depends_on: [1] }, // new, no "before"
  ];
  assert.deepEqual(newlyUnblocked(b, a), []);
});

test("unblockedMessage invites a start for a single id", () => {
  assert.equal(unblockedMessage([7]), "#7 is now unblocked — start it?");
});

test("unblockedMessage lists several ids", () => {
  assert.equal(unblockedMessage([2, 3]), "#2, #3 are now unblocked");
});

test("unblockedMessage is empty for no ids", () => {
  assert.equal(unblockedMessage([]), "");
});

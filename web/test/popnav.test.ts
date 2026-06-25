import { test } from "node:test";
import assert from "node:assert/strict";
import {
  keyToPopNavAction,
  nextPopNavIndex,
  type PopNavAction,
} from "../src/popnav.ts";

test("keyToPopNavAction maps arrows + vim keys", () => {
  assert.equal(keyToPopNavAction("ArrowDown"), "next");
  assert.equal(keyToPopNavAction("j"), "next");
  assert.equal(keyToPopNavAction("ArrowUp"), "prev");
  assert.equal(keyToPopNavAction("k"), "prev");
});

test("keyToPopNavAction maps Home/End and g/G to the ends", () => {
  assert.equal(keyToPopNavAction("Home"), "first");
  assert.equal(keyToPopNavAction("g"), "first");
  assert.equal(keyToPopNavAction("End"), "last");
  assert.equal(keyToPopNavAction("G"), "last");
});

test("keyToPopNavAction maps Enter + Escape", () => {
  assert.equal(keyToPopNavAction("Enter"), "activate");
  assert.equal(keyToPopNavAction("Escape"), "close");
});

test("keyToPopNavAction returns none for unmapped keys", () => {
  assert.equal(keyToPopNavAction("x"), "none");
  assert.equal(keyToPopNavAction("Tab"), "none");
  assert.equal(keyToPopNavAction(" "), "none");
});

test("nextPopNavIndex next/prev wrap past the ends", () => {
  assert.equal(nextPopNavIndex(0, 3, "next"), 1);
  assert.equal(nextPopNavIndex(2, 3, "next"), 0); // wrap forward
  assert.equal(nextPopNavIndex(0, 3, "prev"), 2); // wrap backward
  assert.equal(nextPopNavIndex(1, 3, "prev"), 0);
});

test("nextPopNavIndex first/last jump to the ends", () => {
  assert.equal(nextPopNavIndex(2, 4, "first"), 0);
  assert.equal(nextPopNavIndex(0, 4, "last"), 3);
});

test("nextPopNavIndex leaves the index put for activate/close/none", () => {
  const actions: PopNavAction[] = ["activate", "close", "none"];
  for (const a of actions) {
    assert.equal(nextPopNavIndex(2, 5, a), 2);
  }
});

test("nextPopNavIndex clamps an out-of-range current before moving", () => {
  // current beyond the end clamps to len-1 first, then steps
  assert.equal(nextPopNavIndex(9, 3, "next"), 0); // clamp to 2, +1 wraps to 0
  assert.equal(nextPopNavIndex(-5, 3, "prev"), 2); // clamp to 0, -1 wraps to 2
  assert.equal(nextPopNavIndex(9, 3, "none"), 2); // just clamps
});

test("nextPopNavIndex is safe on an empty list", () => {
  assert.equal(nextPopNavIndex(0, 0, "next"), 0);
  assert.equal(nextPopNavIndex(3, 0, "last"), 0);
});

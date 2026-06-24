import { test } from "node:test";
import assert from "node:assert/strict";
import {
  PRIORITY_LADDER,
  priorityRank,
  nextPriority,
  prevPriority,
  priorityGlyph,
} from "../src/priority.ts";

test("ladder is low -> medium -> high -> urgent", () => {
  assert.deepEqual([...PRIORITY_LADDER], ["low", "medium", "high", "urgent"]);
});

test("priorityRank places each level", () => {
  assert.equal(priorityRank("low"), 0);
  assert.equal(priorityRank("medium"), 1);
  assert.equal(priorityRank("high"), 2);
  assert.equal(priorityRank("urgent"), 3);
});

test("priorityRank clamps unknowns to medium", () => {
  assert.equal(priorityRank("bogus"), 1);
  assert.equal(priorityRank(""), 1);
});

test("nextPriority climbs the ladder", () => {
  assert.equal(nextPriority("low"), "medium");
  assert.equal(nextPriority("medium"), "high");
  assert.equal(nextPriority("high"), "urgent");
});

test("nextPriority wraps urgent -> low", () => {
  assert.equal(nextPriority("urgent"), "low");
});

test("prevPriority descends the ladder", () => {
  assert.equal(prevPriority("urgent"), "high");
  assert.equal(prevPriority("high"), "medium");
  assert.equal(prevPriority("medium"), "low");
});

test("prevPriority wraps low -> urgent", () => {
  assert.equal(prevPriority("low"), "urgent");
});

test("up then down returns to start", () => {
  for (const p of PRIORITY_LADDER) {
    assert.equal(prevPriority(nextPriority(p)), p);
  }
});

test("glyphs match the chip letters", () => {
  assert.equal(priorityGlyph("low"), "L");
  assert.equal(priorityGlyph("medium"), "M");
  assert.equal(priorityGlyph("high"), "H");
  assert.equal(priorityGlyph("urgent"), "U");
  assert.equal(priorityGlyph("???"), "M");
});

import { test } from "node:test";
import assert from "node:assert/strict";
import { emptyNav, reconcile, move, select, type NavState } from "../src/keynav.ts";

const IDS = [10, 20, 30, 40];

test("empty state has no selection", () => {
  assert.equal(emptyNav().selectedId, null);
});

test("move next from nothing selects first; prev selects last", () => {
  assert.equal(move(emptyNav(), IDS, "next").selectedId, 10);
  assert.equal(move(emptyNav(), IDS, "prev").selectedId, 40);
});

test("next/prev walk the list and clamp at the ends", () => {
  let s: NavState = { selectedId: 10 };
  s = move(s, IDS, "next");
  assert.equal(s.selectedId, 20);
  s = move(s, IDS, "next");
  assert.equal(s.selectedId, 30);
  // clamp at bottom
  s = { selectedId: 40 };
  assert.equal(move(s, IDS, "next").selectedId, 40);
  // clamp at top
  s = { selectedId: 10 };
  assert.equal(move(s, IDS, "prev").selectedId, 10);
});

test("first/last jump to the ends", () => {
  assert.equal(move({ selectedId: 30 }, IDS, "first").selectedId, 10);
  assert.equal(move({ selectedId: 10 }, IDS, "last").selectedId, 40);
});

test("move on empty list clears selection", () => {
  assert.equal(move({ selectedId: 10 }, [], "next").selectedId, null);
});

test("reconcile keeps a still-visible selection", () => {
  assert.equal(reconcile({ selectedId: 20 }, IDS).selectedId, 20);
});

test("reconcile holds index position when the selected id vanishes", () => {
  // Was on id 30 (index 2); 30 deleted -> new list, hold index 2 => 40.
  const prev = [10, 20, 30, 40];
  const next = [10, 20, 40, 50];
  assert.equal(reconcile({ selectedId: 30 }, next, prev).selectedId, 40);
});

test("reconcile clamps to last when index overruns the shorter list", () => {
  const prev = [10, 20, 30, 40];
  const next = [10, 20]; // deleted last two; was on 40 (idx 3)
  assert.equal(reconcile({ selectedId: 40 }, next, prev).selectedId, 20);
});

test("reconcile falls back to first when no prior index is known", () => {
  assert.equal(reconcile({ selectedId: 99 }, IDS).selectedId, 10);
});

test("reconcile on empty list clears", () => {
  assert.equal(reconcile({ selectedId: 10 }, []).selectedId, null);
});

test("select only accepts a visible id", () => {
  assert.equal(select({ selectedId: 10 }, IDS, 30).selectedId, 30);
  assert.equal(select({ selectedId: 10 }, IDS, 999).selectedId, 10);
});

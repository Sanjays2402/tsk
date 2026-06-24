import { test } from "node:test";
import assert from "node:assert/strict";
import { matchedIndices, highlightTitle } from "../src/highlight.ts";

test("matchedIndices marks a contiguous substring", () => {
  // "buy" appears at indices 0,1,2 of "buy milk"
  assert.deepEqual([...matchedIndices("buy", "buy milk")].sort((a, b) => a - b), [0, 1, 2]);
});

test("matchedIndices walks a subsequence greedily, earliest hits", () => {
  // "bm" -> b@0, m@4 in "buy milk"
  assert.deepEqual([...matchedIndices("bm", "buy milk")].sort((a, b) => a - b), [0, 4]);
});

test("matchedIndices is case-insensitive", () => {
  assert.deepEqual([...matchedIndices("BUY", "buy milk")].sort((a, b) => a - b), [0, 1, 2]);
});

test("a token that does not fully match the title contributes nothing", () => {
  // "xyz" never matches "buy milk" -> no marks
  assert.equal(matchedIndices("xyz", "buy milk").size, 0);
});

test("multi-token query unions the per-token hits", () => {
  // "buy milk" both match in full -> indices 0,1,2 + 4,5,6,7
  const marks = matchedIndices("buy milk", "buy milk");
  assert.deepEqual([...marks].sort((a, b) => a - b), [0, 1, 2, 4, 5, 6, 7]);
});

test("partial token among full tokens still highlights the matching ones", () => {
  // "buy" matches the title; "zzz" doesn't -> only buy's chars highlighted.
  const marks = matchedIndices("buy zzz", "buy milk");
  assert.deepEqual([...marks].sort((a, b) => a - b), [0, 1, 2]);
});

test("empty query yields no marks", () => {
  assert.equal(matchedIndices("", "buy milk").size, 0);
  assert.equal(matchedIndices("   ", "buy milk").size, 0);
});

test("highlightTitle wraps matched runs in <mark>", () => {
  assert.equal(highlightTitle("buy milk", "buy"), "<mark>buy</mark> milk");
});

test("highlightTitle coalesces adjacent matched chars into one mark", () => {
  // "bm" -> b@0 and m@4 are non-adjacent -> two marks
  assert.equal(highlightTitle("buy milk", "bm"), "<mark>b</mark>uy <mark>m</mark>ilk");
});

test("highlightTitle escapes HTML in the title", () => {
  const out = highlightTitle("<b>x</b>", "x");
  assert.ok(!out.includes("<b>"));
  assert.ok(out.includes("&lt;b&gt;"));
  assert.ok(out.includes("<mark>x</mark>"));
});

test("highlightTitle with no match returns the plain escaped title", () => {
  assert.equal(highlightTitle("a & b", "zzz"), "a &amp; b");
});

test("highlightTitle with empty query returns escaped title, no marks", () => {
  const out = highlightTitle("a <i>b</i>", "");
  assert.ok(!out.includes("<mark>"));
  assert.ok(out.includes("&lt;i&gt;"));
});

test("highlight does not leak an unclosed mark at end of string", () => {
  // last char matches -> mark must be closed
  const out = highlightTitle("milk", "k");
  assert.equal(out, "mil<mark>k</mark>");
});

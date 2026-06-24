import { test } from "node:test";
import assert from "node:assert/strict";
import { parseQuickAdd, isSubmittable } from "../src/quickadd.ts";

test("plain title, no tokens", () => {
  const p = parseQuickAdd("buy milk");
  assert.equal(p.title, "buy milk");
  assert.equal(p.priority, undefined);
  assert.equal(p.due, undefined);
  assert.deepEqual(p.tags, []);
});

test("extracts priority short + long forms", () => {
  assert.equal(parseQuickAdd("x !h").priority, "high");
  assert.equal(parseQuickAdd("x !high").priority, "high");
  assert.equal(parseQuickAdd("x !u").priority, "urgent");
  assert.equal(parseQuickAdd("x !critical").priority, "urgent");
  assert.equal(parseQuickAdd("x !l").priority, "low");
  assert.equal(parseQuickAdd("x !med").priority, "medium");
});

test("unknown !word stays in the title", () => {
  const p = parseQuickAdd("ship it !now");
  assert.equal(p.title, "ship it !now");
  assert.equal(p.priority, undefined);
});

test("extracts a single-token due", () => {
  assert.equal(parseQuickAdd("pay rent @tomorrow").due, "tomorrow");
  assert.equal(parseQuickAdd("pay rent @2026-07-04").due, "2026-07-04");
  assert.equal(parseQuickAdd("pay rent @3d").due, "3d");
  assert.equal(parseQuickAdd("pay rent @eow").due, "eow");
});

test("extracts and de-dupes tags, lower-cased, first-seen order", () => {
  const p = parseQuickAdd("plan #Work #home #work #HOME");
  assert.deepEqual(p.tags, ["work", "home"]);
  assert.equal(p.title, "plan");
});

test("all token kinds together, order-independent", () => {
  const p = parseQuickAdd("review PR #dev !urgent @fri #ci");
  assert.equal(p.title, "review PR");
  assert.equal(p.priority, "urgent");
  assert.equal(p.due, "fri");
  assert.deepEqual(p.tags, ["dev", "ci"]);
});

test("tokens interspersed with title words", () => {
  const p = parseQuickAdd("call !high the @mon plumber #house");
  assert.equal(p.title, "call the plumber");
  assert.equal(p.priority, "high");
  assert.equal(p.due, "mon");
  assert.deepEqual(p.tags, ["house"]);
});

test("last priority and due win", () => {
  const p = parseQuickAdd("x !low !urgent @today @fri");
  assert.equal(p.priority, "urgent");
  assert.equal(p.due, "fri");
});

test("bare sigils are literal title text", () => {
  const p = parseQuickAdd("ship it ! @ #");
  assert.equal(p.title, "ship it ! @ #");
  assert.equal(p.priority, undefined);
  assert.equal(p.due, undefined);
  assert.deepEqual(p.tags, []);
});

test("mid-word sigils are not tokens", () => {
  const p = parseQuickAdd("email bob@acme.com about C# done!");
  assert.equal(p.title, "email bob@acme.com about C# done!");
  assert.equal(p.due, undefined);
  assert.deepEqual(p.tags, []);
});

test("collapses internal whitespace", () => {
  const p = parseQuickAdd("  spaced    out   task  ");
  assert.equal(p.title, "spaced out task");
});

test("isSubmittable reflects non-empty title", () => {
  assert.equal(isSubmittable(parseQuickAdd("real task")), true);
  assert.equal(isSubmittable(parseQuickAdd("  ")), false);
  // metadata-only input has no title and is not submittable
  assert.equal(isSubmittable(parseQuickAdd("!high #tag @today")), false);
});

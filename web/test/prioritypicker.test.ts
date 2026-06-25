import { test } from "node:test";
import assert from "node:assert/strict";
import { priorityOptions, renderPriorityPicker } from "../src/prioritypicker.ts";

test("priorityOptions lists all four levels, urgent first", () => {
  const opts = priorityOptions("medium");
  assert.deepEqual(
    opts.map((o) => o.priority),
    ["urgent", "high", "medium", "low"],
  );
});

test("priorityOptions marks the current priority active", () => {
  const opts = priorityOptions("high");
  const active = opts.filter((o) => o.active);
  assert.equal(active.length, 1);
  assert.equal(active[0].priority, "high");
});

test("priorityOptions carries the correct glyphs and labels", () => {
  const opts = priorityOptions("low");
  const byPrio = Object.fromEntries(opts.map((o) => [o.priority, o]));
  assert.equal(byPrio.urgent.glyph, "U");
  assert.equal(byPrio.high.glyph, "H");
  assert.equal(byPrio.medium.glyph, "M");
  assert.equal(byPrio.low.glyph, "L");
  assert.equal(byPrio.urgent.label, "Urgent");
  assert.equal(byPrio.low.label, "Low");
});

test("priorityOptions marks nothing active for an unknown current value", () => {
  const opts = priorityOptions("bogus");
  assert.equal(opts.filter((o) => o.active).length, 0);
  assert.equal(opts.length, 4); // still a usable setter
});

test("renderPriorityPicker emits a data-set-prio hook per level", () => {
  const html = renderPriorityPicker("medium");
  for (const p of ["urgent", "high", "medium", "low"]) {
    assert.match(html, new RegExp(`data-set-prio="${p}"`));
  }
});

test("renderPriorityPicker marks the active option with aria-checked", () => {
  const html = renderPriorityPicker("urgent");
  assert.match(html, /data-set-prio="urgent"[^>]*aria-checked="true"|aria-checked="true"[^>]*data-set-prio="urgent"/);
  // exactly one active item
  assert.equal((html.match(/is-active/g) ?? []).length, 1);
});

test("renderPriorityPicker reuses the .priority color classes on the glyph", () => {
  const html = renderPriorityPicker("low");
  assert.match(html, /class="prio-pick-glyph priority urgent"/);
  assert.match(html, /class="prio-pick-glyph priority low"/);
});

test("renderPriorityPicker has the menu role + aria-label", () => {
  const html = renderPriorityPicker("medium");
  assert.match(html, /role="menu"/);
  assert.match(html, /aria-label="Set priority"/);
  assert.match(html, /role="menuitemradio"/);
});

test("renderPriorityPicker marks the current level with a check (F55)", () => {
  const html = renderPriorityPicker("high");
  // exactly one check glyph, on the active row
  assert.equal((html.match(/prio-pick-check/g) ?? []).length, 1);
});

test("renderPriorityPicker shows no check for an unknown current value (F55)", () => {
  const html = renderPriorityPicker("bogus");
  assert.doesNotMatch(html, /prio-pick-check/);
});

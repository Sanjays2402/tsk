import { test } from "node:test";
import assert from "node:assert/strict";
import {
  menuItemsFor,
  renderContextMenu,
  clampMenuPosition,
  type MenuTask,
} from "../src/contextmenu.ts";

const undone: MenuTask = { id: 1, done: false };
const done: MenuTask = { id: 2, done: true };
const pinned: MenuTask = { id: 3, done: false, pinned: true };

test("menuItemsFor lists every per-row action in order", () => {
  const actions = menuItemsFor(undone).map((i) => i.action);
  assert.deepEqual(actions, [
    "toggle",
    "edit",
    "due",
    "notes",
    "pin",
    "prio-up",
    "prio-down",
    "delete",
  ]);
});

test("toggle label reflects done state", () => {
  assert.equal(menuItemsFor(undone)[0].label, "Mark done");
  assert.equal(menuItemsFor(done)[0].label, "Mark not done");
});

test("pin label reflects pinned state", () => {
  const pinItem = (t: MenuTask) => menuItemsFor(t).find((i) => i.action === "pin")!;
  assert.equal(pinItem(undone).label, "Pin to top");
  assert.equal(pinItem(pinned).label, "Unpin");
});

test("delete is danger and divided", () => {
  const del = menuItemsFor(undone).find((i) => i.action === "delete")!;
  assert.equal(del.danger, true);
  assert.equal(del.divider, true);
});

test("renderContextMenu carries an action hook per item", () => {
  const html = renderContextMenu(undone);
  for (const a of ["toggle", "edit", "due", "notes", "pin", "prio-up", "prio-down", "delete"]) {
    assert.ok(html.includes(`data-row-action="${a}"`), `missing ${a}`);
  }
  assert.match(html, /role="menu"/);
  assert.match(html, /Task #1 actions/);
});

test("renderContextMenu marks the danger item", () => {
  assert.match(renderContextMenu(undone), /ctxmenu-item[^"]*is-danger/);
});

test("clampMenuPosition keeps a menu inside the viewport", () => {
  // Anchor near the bottom-right; menu should be nudged up + left.
  const { left, top } = clampMenuPosition(790, 590, 200, 300, 800, 600);
  assert.equal(left, 800 - 200 - 8);
  assert.equal(top, 600 - 300 - 8);
});

test("clampMenuPosition leaves a comfortably-placed menu where it is", () => {
  const { left, top } = clampMenuPosition(100, 120, 200, 300, 800, 600);
  assert.equal(left, 100);
  assert.equal(top, 120);
});

test("clampMenuPosition never goes past the top-left margin", () => {
  const { left, top } = clampMenuPosition(-50, -50, 200, 300, 800, 600);
  assert.equal(left, 8);
  assert.equal(top, 8);
});

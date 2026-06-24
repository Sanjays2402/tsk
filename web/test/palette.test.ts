import { test } from "node:test";
import assert from "node:assert/strict";
import {
  isSubseq,
  scoreCommand,
  filterCommands,
  moveIndex,
  clampIndex,
  renderPaletteList,
  type Command,
} from "../src/palette.ts";

function cmd(p: Partial<Command> & { id: string; title: string }): Command {
  return { ...p };
}

const COMMANDS: Command[] = [
  cmd({ id: "add", title: "Add task", group: "Task", keywords: ["new", "create"] }),
  cmd({ id: "toggle", title: "Toggle done", group: "Task" }),
  cmd({ id: "delete", title: "Delete task", group: "Task", keywords: ["remove"] }),
  cmd({ id: "stats", title: "Toggle stats", group: "View", keywords: ["metrics"] }),
  cmd({ id: "theme", title: "Cycle theme", group: "View" }),
  cmd({ id: "undo", title: "Undo delete", group: "Task", disabled: true }),
];

test("isSubseq respects order, case-insensitively via caller", () => {
  assert.equal(isSubseq("adt", "add task"), true);
  assert.equal(isSubseq("", "anything"), true);
  assert.equal(isSubseq("tda", "add task"), false);
});

test("scoreCommand: exact > prefix > substring > fuzzy > miss", () => {
  const c = cmd({ id: "x", title: "Add task", keywords: ["create"] });
  assert.equal(scoreCommand(c, "add task"), 100); // exact
  assert.ok(scoreCommand(c, "add") >= 80); // prefix
  assert.ok(scoreCommand(c, "task") >= 40 && scoreCommand(c, "task") < 80); // substring
  assert.ok(scoreCommand(c, "create") > 0); // keyword fuzzy
  assert.equal(scoreCommand(c, "zzz"), -1); // miss
});

test("empty query matches everything with neutral score", () => {
  assert.equal(scoreCommand(COMMANDS[0], ""), 0);
  assert.equal(filterCommands(COMMANDS, "").length, COMMANDS.length);
});

test("filterCommands ranks prefix above substring", () => {
  // "to" prefixes "Toggle done" + "Toggle stats", substring-hits nothing else.
  const out = filterCommands(COMMANDS, "to");
  assert.equal(out[0].title.toLowerCase().startsWith("to"), true);
});

test("filterCommands drops non-matches", () => {
  const out = filterCommands(COMMANDS, "theme");
  assert.deepEqual(out.map((c) => c.id), ["theme"]);
});

test("filterCommands matches via keywords", () => {
  const out = filterCommands(COMMANDS, "metrics");
  assert.equal(out.length, 1);
  assert.equal(out[0].id, "stats");
});

test("filterCommands preserves declared order on score ties", () => {
  // Empty query -> all neutral; order must equal declaration order.
  const out = filterCommands(COMMANDS, "");
  assert.deepEqual(
    out.map((c) => c.id),
    COMMANDS.map((c) => c.id),
  );
});

test("disabled commands sink slightly under equal-score live ones", () => {
  const two: Command[] = [
    cmd({ id: "live", title: "Delete task" }),
    cmd({ id: "dead", title: "Delete task", disabled: true }),
  ];
  // Same exact-match score; the live one should come first.
  const out = filterCommands(two, "delete task");
  assert.deepEqual(out.map((c) => c.id), ["live", "dead"]);
});

test("moveIndex wraps both directions", () => {
  assert.equal(moveIndex(0, 3, 1), 1);
  assert.equal(moveIndex(2, 3, 1), 0); // wrap forward
  assert.equal(moveIndex(0, 3, -1), 2); // wrap backward
  assert.equal(moveIndex(0, 0, 1), 0); // empty list safe
});

test("clampIndex keeps the index in range without wrapping", () => {
  assert.equal(clampIndex(5, 3), 2);
  assert.equal(clampIndex(-1, 3), 0);
  assert.equal(clampIndex(1, 0), 0);
});

test("renderPaletteList marks the active row and carries dispatch ids", () => {
  const html = renderPaletteList(COMMANDS.slice(0, 3), 1);
  assert.match(html, /data-cmd-id="toggle"/);
  assert.match(html, /is-active/);
  assert.match(html, /aria-selected="true"/);
});

test("renderPaletteList greys disabled commands", () => {
  const html = renderPaletteList([COMMANDS[5]], 0);
  assert.match(html, /is-disabled/);
});

test("renderPaletteList shows an empty state", () => {
  const html = renderPaletteList([], 0);
  assert.match(html, /No matching commands/);
});

test("renderPaletteList escapes HTML in titles", () => {
  const html = renderPaletteList([cmd({ id: "x", title: "<script>" })], 0);
  assert.doesNotMatch(html, /<script>/);
  assert.match(html, /&lt;script&gt;/);
});

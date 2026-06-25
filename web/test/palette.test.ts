import { test } from "node:test";
import assert from "node:assert/strict";
import {
  isSubseq,
  scoreCommand,
  filterCommands,
  moveIndex,
  clampIndex,
  renderPaletteList,
  buildPriorityCommands,
  buildDueCommands,
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

// --- F57: highlight the matched subsequence in palette titles ---------------

test("renderPaletteList highlights the matched chars when a query is passed", () => {
  const html = renderPaletteList([cmd({ id: "theme", title: "Cycle theme" })], 0, "theme");
  // the contiguous "theme" run is wrapped in a single <mark>
  assert.match(html, /<mark>theme<\/mark>/i);
});

test("renderPaletteList highlights a subsequence, not just substrings", () => {
  const html = renderPaletteList([cmd({ id: "td", title: "Toggle done" })], 0, "td");
  // T...d each marked (subsequence across the title)
  assert.match(html, /<mark>T<\/mark>/);
  assert.match(html, /<mark>d<\/mark>/);
});

test("renderPaletteList marks nothing in the title when the query missed it", () => {
  // "metrics" matches this command via keywords, not the visible title.
  const html = renderPaletteList(
    [cmd({ id: "stats", title: "Toggle stats", keywords: ["metrics"] })],
    0,
    "metrics",
  );
  assert.doesNotMatch(html, /<mark>/);
});

test("renderPaletteList without a query has no marks (back-compat)", () => {
  const html = renderPaletteList(COMMANDS.slice(0, 3), 0);
  assert.doesNotMatch(html, /<mark>/);
});

test("renderPaletteList still escapes while highlighting", () => {
  const html = renderPaletteList([cmd({ id: "x", title: "<b> tag" })], 0, "tag");
  assert.doesNotMatch(html, /<b>/);
  assert.match(html, /&lt;b&gt;/);
  assert.match(html, /<mark>tag<\/mark>/);
});

// --- F63: "Set priority" palette command group ------------------------------

test("buildPriorityCommands emits the four levels, urgent-first", () => {
  const cmds = buildPriorityCommands(true, "medium");
  assert.deepEqual(
    cmds.map((c) => c.id),
    ["prio-set-urgent", "prio-set-high", "prio-set-medium", "prio-set-low"],
  );
  assert.deepEqual(
    cmds.map((c) => c.title),
    ["Set priority: Urgent", "Set priority: High", "Set priority: Medium", "Set priority: Low"],
  );
  // all share the Priority group
  assert.ok(cmds.every((c) => c.group === "Priority"));
});

test("buildPriorityCommands disables the level already in effect", () => {
  const cmds = buildPriorityCommands(true, "high");
  const high = cmds.find((c) => c.id === "prio-set-high")!;
  const low = cmds.find((c) => c.id === "prio-set-low")!;
  assert.equal(high.disabled, true); // current -> disabled
  assert.equal(low.disabled, false); // a different level -> enabled
});

test("buildPriorityCommands disables everything with no selection", () => {
  const cmds = buildPriorityCommands(false, undefined);
  assert.ok(cmds.every((c) => c.disabled === true));
});

test("buildPriorityCommands enables all four when nothing matches current", () => {
  // An undefined current (shouldn't happen with a selection, but be safe) keeps
  // every level actionable.
  const cmds = buildPriorityCommands(true, undefined);
  assert.ok(cmds.every((c) => c.disabled === false));
});

test("buildPriorityCommands rows fuzzy-match via filterCommands", () => {
  // The group is reachable by typing a single letter mnemonic (keyword).
  const cmds = buildPriorityCommands(true, "low");
  const out = filterCommands(cmds, "urgent");
  assert.equal(out[0].id, "prio-set-urgent");
});

// --- F67: "Set due" palette command group -----------------------------------

test("buildDueCommands emits the presets plus a clear, in order", () => {
  const cmds = buildDueCommands(true);
  assert.deepEqual(
    cmds.map((c) => c.id),
    ["due-set-today", "due-set-tomorrow", "due-set-eow", "due-set-1w", "due-set-eom", "due-set-clear"],
  );
  // all share the Due group
  assert.ok(cmds.every((c) => c.group === "Due"));
  // the clear command reads as "Set due: Clear"
  assert.equal(cmds[cmds.length - 1].title, "Set due: Clear");
});

test("buildDueCommands carries the natural-language token per command", () => {
  const cmds = buildDueCommands(true);
  const byId = new Map(cmds.map((c) => [c.id, c]));
  assert.equal(byId.get("due-set-today")!.token, "today");
  assert.equal(byId.get("due-set-1w")!.token, "1w");
  assert.equal(byId.get("due-set-eom")!.token, "eom");
  // the clear command's token is the empty string (server treats "" as clear)
  assert.equal(byId.get("due-set-clear")!.token, "");
});

test("buildDueCommands disables everything with no selection", () => {
  const cmds = buildDueCommands(false);
  assert.ok(cmds.every((c) => c.disabled === true));
});

test("buildDueCommands enables everything with a selection", () => {
  const cmds = buildDueCommands(true);
  assert.ok(cmds.every((c) => c.disabled === false));
});

test("buildDueCommands rows fuzzy-match via filterCommands keywords", () => {
  const cmds = buildDueCommands(true);
  // "weekend" reaches the eow preset via its keyword, even though the title
  // says "This weekend" too.
  const out = filterCommands(cmds, "weekend");
  assert.equal(out[0].id, "due-set-eow");
  // "tomorrow" matches the tomorrow preset
  const out2 = filterCommands(cmds, "tomorrow");
  assert.equal(out2[0].id, "due-set-tomorrow");
});

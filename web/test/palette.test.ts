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
  dueTokenForCommandId,
  priorityForCommandId,
  priorityPreviewVM,
  renderPriorityPreview,
  setCommandDisabledReason,
  commandDisabledReason,
  clearLensCommand,
  SELECTION_GATED_COMMANDS,
  renderDisabledReason,
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

// --- F73: due-preview token decoding ---------------------------------------

test("dueTokenForCommandId decodes the NL token a due command carries", () => {
  assert.equal(dueTokenForCommandId("due-set-today"), "today");
  assert.equal(dueTokenForCommandId("due-set-tomorrow"), "tomorrow");
  assert.equal(dueTokenForCommandId("due-set-eow"), "eow");
  assert.equal(dueTokenForCommandId("due-set-1w"), "1w");
  assert.equal(dueTokenForCommandId("due-set-eom"), "eom");
});

test("dueTokenForCommandId returns '' for the clear command (distinct from null)", () => {
  assert.equal(dueTokenForCommandId("due-set-clear"), "");
});

test("dueTokenForCommandId returns null for non-due commands", () => {
  assert.equal(dueTokenForCommandId("prio-set-urgent"), null);
  assert.equal(dueTokenForCommandId("toggle"), null);
  assert.equal(dueTokenForCommandId("view:abc"), null);
  assert.equal(dueTokenForCommandId(""), null);
});

test("dueTokenForCommandId round-trips every buildDueCommands id to its token", () => {
  for (const c of buildDueCommands(true)) {
    assert.equal(dueTokenForCommandId(c.id), c.token);
  }
});

// --- F77: priority-preview decoding + view-model ---------------------------

test("priorityForCommandId decodes the level a prio command carries", () => {
  assert.equal(priorityForCommandId("prio-set-urgent"), "urgent");
  assert.equal(priorityForCommandId("prio-set-high"), "high");
  assert.equal(priorityForCommandId("prio-set-medium"), "medium");
  assert.equal(priorityForCommandId("prio-set-low"), "low");
});

test("priorityForCommandId returns null for non-priority / malformed commands", () => {
  assert.equal(priorityForCommandId("due-set-today"), null);
  assert.equal(priorityForCommandId("prio-up"), null); // the cycle command, not a set
  assert.equal(priorityForCommandId("prio-set-bogus"), null);
  assert.equal(priorityForCommandId("toggle"), null);
  assert.equal(priorityForCommandId(""), null);
});

test("priorityForCommandId round-trips every buildPriorityCommands id", () => {
  for (const c of buildPriorityCommands(true, undefined)) {
    const lvl = priorityForCommandId(c.id);
    assert.notEqual(lvl, null);
    assert.equal(`prio-set-${lvl}`, c.id);
  }
});

test("priorityPreviewVM shows a current -> new transition with an arrow", () => {
  const vm = priorityPreviewVM("medium", "urgent");
  assert.equal(vm.state, "valid");
  assert.match(vm.text, /Medium/);
  assert.match(vm.text, /Urgent/);
  assert.match(vm.text, /\u2192/); // the arrow
});

test("priorityPreviewVM reads 'Already X' when the level is unchanged", () => {
  const vm = priorityPreviewVM("high", "high");
  assert.equal(vm.state, "empty");
  assert.equal(vm.text, "Already High");
});

test("priorityPreviewVM renders an em dash when there's no current priority", () => {
  const vm = priorityPreviewVM(undefined, "low");
  assert.equal(vm.state, "valid");
  assert.match(vm.text, /\u2014/); // em dash on the left
  assert.match(vm.text, /Low/);
});

test("renderPriorityPreview reuses the .due-preview state classes", () => {
  assert.match(renderPriorityPreview({ state: "valid", text: "Medium \u2192 Urgent" }), /due-preview is-valid/);
  assert.match(renderPriorityPreview({ state: "empty", text: "Already Urgent" }), /due-preview is-empty/);
});

test("renderPriorityPreview escapes its text", () => {
  const html = renderPriorityPreview({ state: "valid", text: "<x> & y" });
  assert.match(html, /&lt;x&gt; &amp; y/);
  assert.doesNotMatch(html, /<x>/);
});

// --- F83: disabled-reason hints for set commands ---------------------------

test("setCommandDisabledReason: no selection -> 'select a task first' for set commands", () => {
  assert.equal(setCommandDisabledReason("prio-set-urgent", false), "select a task first");
  assert.equal(setCommandDisabledReason("due-set-today", false), "select a task first");
  assert.equal(setCommandDisabledReason("due-set-clear", false), "select a task first");
});

test("setCommandDisabledReason: with a selection -> null (the normal preview takes over)", () => {
  assert.equal(setCommandDisabledReason("prio-set-urgent", true), null);
  assert.equal(setCommandDisabledReason("due-set-today", true), null);
});

test("setCommandDisabledReason: null for non-set commands either way", () => {
  assert.equal(setCommandDisabledReason("add", false), null);
  assert.equal(setCommandDisabledReason("toggle", false), null);
  assert.equal(setCommandDisabledReason("export-csv", false), null);
  assert.equal(setCommandDisabledReason("view:abc", false), null);
});

test("setCommandDisabledReason covers every priority + due preset id", () => {
  for (const lvl of ["urgent", "high", "medium", "low"]) {
    assert.equal(setCommandDisabledReason(`prio-set-${lvl}`, false), "select a task first");
  }
  for (const tok of ["today", "tomorrow", "eow", "1w", "eom"]) {
    assert.equal(setCommandDisabledReason(`due-set-${tok}`, false), "select a task first");
  }
});

test("renderDisabledReason reuses the faint .due-preview is-empty style", () => {
  const html = renderDisabledReason("select a task first");
  assert.match(html, /due-preview is-empty/);
  assert.match(html, /select a task first/);
});

test("renderDisabledReason escapes its text", () => {
  const html = renderDisabledReason("<b>pick</b> & go");
  assert.match(html, /&lt;b&gt;pick&lt;\/b&gt; &amp; go/);
  assert.doesNotMatch(html, /<b>pick/);
});

// --- F89: general disabled-reason hints for every gated command ------------

const FULL_CTX = { hasSel: true, canUndo: true, hasTasks: true, onTag: true, hasLens: true };

test("commandDisabledReason: selection-gated verbs say 'select a task first' with no selection", () => {
  const ctx = { ...FULL_CTX, hasSel: false };
  for (const id of SELECTION_GATED_COMMANDS) {
    assert.equal(commandDisabledReason(id, ctx), "select a task first", `for ${id}`);
  }
  // The F83 set-command group is still covered (delegated through).
  assert.equal(commandDisabledReason("prio-set-urgent", ctx), "select a task first");
  assert.equal(commandDisabledReason("due-set-today", ctx), "select a task first");
});

test("commandDisabledReason: undo with nothing pending -> 'nothing to undo'", () => {
  assert.equal(commandDisabledReason("undo", { ...FULL_CTX, canUndo: false }), "nothing to undo");
  // With something to undo it's not disabled -> no reason.
  assert.equal(commandDisabledReason("undo", FULL_CTX), null);
});

test("commandDisabledReason: filter with no tasks -> 'no tasks to filter'", () => {
  assert.equal(commandDisabledReason("filter", { ...FULL_CTX, hasTasks: false }), "no tasks to filter");
  assert.equal(commandDisabledReason("filter", FULL_CTX), null);
});

test("commandDisabledReason: all-tasks while not on a tag -> 'already on all tasks'", () => {
  assert.equal(commandDisabledReason("alltasks", { ...FULL_CTX, onTag: false }), "already on all tasks");
  assert.equal(commandDisabledReason("alltasks", FULL_CTX), null);
});

test("commandDisabledReason: null for commands with no known reason", () => {
  assert.equal(commandDisabledReason("add", { ...FULL_CTX, hasSel: false }), null);
  assert.equal(commandDisabledReason("stats", { ...FULL_CTX, hasSel: false }), null);
  assert.equal(commandDisabledReason("theme", FULL_CTX), null);
});

test("SELECTION_GATED_COMMANDS holds the per-task verbs, not the global ones", () => {
  assert.ok(SELECTION_GATED_COMMANDS.has("toggle"));
  assert.ok(SELECTION_GATED_COMMANDS.has("delete"));
  assert.ok(!SELECTION_GATED_COMMANDS.has("add"));
  assert.ok(!SELECTION_GATED_COMMANDS.has("undo"));
});

// --- F98: the "Clear lens" palette command ---------------------------------

test("clearLensCommand names the active lens and is enabled when one is on", () => {
  const c = clearLensCommand("overdue");
  assert.equal(c.id, "lens-clear");
  assert.equal(c.title, "Clear lens (overdue)");
  assert.equal(c.disabled, false);
  assert.equal(c.group, "View");
});

test("clearLensCommand reads plainly and is disabled when no lens is active", () => {
  const c = clearLensCommand(null);
  assert.equal(c.id, "lens-clear");
  assert.equal(c.title, "Clear lens");
  assert.equal(c.disabled, true);
});

test("commandDisabledReason: clear-lens with no lens -> 'no lens active'", () => {
  assert.equal(commandDisabledReason("lens-clear", { ...FULL_CTX, hasLens: false }), "no lens active");
  // With a lens active it isn't disabled -> no reason.
  assert.equal(commandDisabledReason("lens-clear", FULL_CTX), null);
});

test("clearLensCommand is fuzzy-findable by 'lens' and 'clear'", () => {
  const list = filterCommands([clearLensCommand("blocked")], "clear lens");
  assert.equal(list.length, 1);
  assert.equal(list[0].id, "lens-clear");
});


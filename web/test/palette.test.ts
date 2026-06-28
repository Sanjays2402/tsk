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
  clearCohortCommand,
  pinLensCommand,
  unpinLensCommand,
  pinCohortCommand,
  unpinCohortCommand,
  forgetStaleCohortsCommand,
  recallBusiestViewCommand,
  focusChokepointCommand,
  buildChokepointFocusCommands,
  focusShiftedChokepointCommand,
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

const FULL_CTX = { hasSel: true, canUndo: true, hasTasks: true, onTag: true, hasLens: true, lensPinned: true, hasCohort: true, hasChokepoint: true, cohortPinned: true, staleCohortCount: 2, hasBusiestView: true };

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

// --- F115: the "Pin lens" palette command ----------------------------------

test("pinLensCommand reads 'Pin lens (<label>)' when a lens is active + not pinned", () => {
  const c = pinLensCommand("overdue", false);
  assert.equal(c.id, "lens-pin");
  assert.equal(c.title, "Pin lens (overdue)");
  assert.equal(c.disabled, false);
  assert.equal(c.group, "View");
});

test("pinLensCommand flips to 'Recall pinned lens (<label>)' when already pinned", () => {
  const c = pinLensCommand("blocked", true);
  assert.equal(c.id, "lens-pin");
  assert.equal(c.title, "Recall pinned lens (blocked)");
  assert.equal(c.disabled, false);
});

test("pinLensCommand reads plainly and is disabled with no lens (pinned irrelevant)", () => {
  for (const pinned of [false, true]) {
    const c = pinLensCommand(null, pinned);
    assert.equal(c.title, "Pin lens");
    assert.equal(c.disabled, true);
  }
});

test("commandDisabledReason: pin-lens with no lens -> 'no lens active'", () => {
  assert.equal(commandDisabledReason("lens-pin", { ...FULL_CTX, hasLens: false }), "no lens active");
  // With a lens active it isn't disabled -> no reason.
  assert.equal(commandDisabledReason("lens-pin", FULL_CTX), null);
});

test("pinLensCommand is fuzzy-findable by 'pin' and 'lens'", () => {
  const byPin = filterCommands([pinLensCommand("today", false)], "pin lens");
  assert.equal(byPin.length, 1);
  assert.equal(byPin[0].id, "lens-pin");
  // And by 'recall' once pinned, so the recall affordance is discoverable.
  const byRecall = filterCommands([pinLensCommand("today", true)], "recall");
  assert.equal(byRecall.length, 1);
});

// --- F125: the "Unpin lens" palette command --------------------------------

test("unpinLensCommand reads 'Unpin lens (<label>)' and is enabled only when pinned", () => {
  const c = unpinLensCommand("overdue", true);
  assert.equal(c.id, "lens-unpin");
  assert.equal(c.title, "Unpin lens (overdue)");
  assert.equal(c.disabled, false);
  assert.equal(c.group, "View");
});

test("unpinLensCommand is disabled when the active lens isn't pinned", () => {
  const c = unpinLensCommand("blocked", false);
  assert.equal(c.title, "Unpin lens (blocked)");
  assert.equal(c.disabled, true); // nothing to unpin yet
});

test("unpinLensCommand reads plainly and is disabled with no lens", () => {
  for (const pinned of [false, true]) {
    const c = unpinLensCommand(null, pinned);
    assert.equal(c.title, "Unpin lens");
    assert.equal(c.disabled, true);
  }
});

test("commandDisabledReason: unpin-lens distinguishes no-lens from not-pinned", () => {
  // No lens at all.
  assert.equal(commandDisabledReason("lens-unpin", { ...FULL_CTX, hasLens: false }), "no lens active");
  // A lens, but it isn't pinned -> a distinct reason so the user knows why.
  assert.equal(commandDisabledReason("lens-unpin", { ...FULL_CTX, lensPinned: false }), "lens not pinned");
  // A pinned lens -> not disabled, no reason.
  assert.equal(commandDisabledReason("lens-unpin", FULL_CTX), null);
});

test("unpinLensCommand is fuzzy-findable by 'unpin' and 'remove'", () => {
  const byUnpin = filterCommands([unpinLensCommand("today", true)], "unpin");
  assert.equal(byUnpin.length, 1);
  assert.equal(byUnpin[0].id, "lens-unpin");
  // 'remove' is a keyword so the command surfaces for that search too.
  const byRemove = filterCommands([unpinLensCommand("today", true)], "remove");
  assert.equal(byRemove.length, 1);
  assert.equal(byRemove[0].id, "lens-unpin");
});

test("unpin and pin commands never share an id (distinct dispatch)", () => {
  assert.notEqual(unpinLensCommand("today", true).id, pinLensCommand("today", true).id);
});

test("pinLensCommand id is distinct from the clear-lens command", () => {
  // The two share the same lens context but route to opposite actions
  // (pinCurrentLens vs setLens(null)), so their ids must not collide.
  assert.notEqual(pinLensCommand("overdue", false).id, clearLensCommand("overdue").id);
});

// --- F103: the cohort-focus palette commands -------------------------------

test("clearCohortCommand names the active cohort and is enabled when one is on", () => {
  const c = clearCohortCommand("3 waiting on #1");
  assert.equal(c.id, "cohort-clear");
  assert.equal(c.title, "Clear cohort focus (3 waiting on #1)");
  assert.equal(c.disabled, false);
  assert.equal(c.group, "View");
});

test("clearCohortCommand reads plainly and is disabled when no cohort is active", () => {
  const c = clearCohortCommand(null);
  assert.equal(c.id, "cohort-clear");
  assert.equal(c.title, "Clear cohort focus");
  assert.equal(c.disabled, true);
});

// --- F118: depth-aware "Clear cohort" label --------------------------------

test("clearCohortCommand warns about the drill depth when a back-stack exists", () => {
  const c = clearCohortCommand("3 waiting on #1", 2);
  assert.equal(c.title, "Clear cohort focus (3 waiting on #1) + 2-step history");
  assert.equal(c.disabled, false);
});

test("clearCohortCommand at depth 0 is byte-identical to the no-depth form", () => {
  // The default arg keeps existing call sites + snapshots unchanged.
  assert.equal(
    clearCohortCommand("2 on #5", 0).title,
    clearCohortCommand("2 on #5").title,
  );
  assert.doesNotMatch(clearCohortCommand("2 on #5", 0).title, /history/);
});

test("clearCohortCommand ignores depth when no cohort is active", () => {
  // No cohort summary means nothing to clear, so a stray depth must not leak a
  // "+ N-step history" suffix onto the plain disabled label.
  const c = clearCohortCommand(null, 4);
  assert.equal(c.title, "Clear cohort focus");
  assert.equal(c.disabled, true);
});

test("clearCohortCommand is fuzzy-findable by 'history' and 'drill'", () => {
  const c = clearCohortCommand("3 waiting on #1", 2);
  assert.equal(filterCommands([c], "history").length, 1);
  assert.equal(filterCommands([c], "drill").length, 1);
});

test("focusChokepointCommand names the chokepoint and is enabled when one exists", () => {
  const c = focusChokepointCommand(7);
  assert.equal(c.id, "cohort-focus-biggest");
  assert.equal(c.title, "Focus biggest chokepoint (#7)");
  assert.equal(c.disabled, false);
});

test("focusChokepointCommand reads plainly and is disabled on a flat board", () => {
  const c = focusChokepointCommand(null);
  assert.equal(c.id, "cohort-focus-biggest");
  assert.equal(c.title, "Focus biggest chokepoint");
  assert.equal(c.disabled, true);
});

test("commandDisabledReason: cohort commands explain their absent precondition", () => {
  assert.equal(commandDisabledReason("cohort-clear", { ...FULL_CTX, hasCohort: false }), "no cohort active");
  assert.equal(commandDisabledReason("cohort-clear", FULL_CTX), null);
  assert.equal(commandDisabledReason("cohort-focus-biggest", { ...FULL_CTX, hasChokepoint: false }), "no chokepoint");
  assert.equal(commandDisabledReason("cohort-focus-biggest", FULL_CTX), null);
  // F139: pin/unpin cohort reasons.
  assert.equal(commandDisabledReason("cohort-pin", { ...FULL_CTX, hasCohort: false }), "no cohort active");
  assert.equal(commandDisabledReason("cohort-pin", FULL_CTX), null);
  assert.equal(commandDisabledReason("cohort-unpin", { ...FULL_CTX, hasCohort: false }), "no cohort active");
  assert.equal(commandDisabledReason("cohort-unpin", { ...FULL_CTX, cohortPinned: false }), "cohort not pinned");
  assert.equal(commandDisabledReason("cohort-unpin", FULL_CTX), null);
});

// --- F139: cohort pin/unpin palette commands -------------------------------

test("pinCohortCommand reads 'Pin cohort (<summary>)' when focused + not pinned", () => {
  const c = pinCohortCommand("3 waiting on #1", false);
  assert.equal(c.id, "cohort-pin");
  assert.match(c.title, /Pin cohort \(3 waiting on #1\)/);
  assert.equal(c.disabled, false);
});

test("pinCohortCommand flips to 'Recall pinned cohort (<summary>)' when already pinned", () => {
  const c = pinCohortCommand("2 waiting on #4", true);
  assert.equal(c.id, "cohort-pin");
  assert.match(c.title, /Recall pinned cohort \(2 waiting on #4\)/);
  assert.equal(c.disabled, false);
});

test("pinCohortCommand reads plainly and is disabled with no cohort (pinned irrelevant)", () => {
  for (const pinned of [true, false]) {
    const c = pinCohortCommand(null, pinned);
    assert.equal(c.title, "Pin cohort");
    assert.equal(c.disabled, true);
  }
  assert.equal(commandDisabledReason("cohort-pin", { ...FULL_CTX, hasCohort: false }), "no cohort active");
});

test("pinCohortCommand is fuzzy-findable by 'pin' and 'cohort'", () => {
  const byPin = filterCommands([pinCohortCommand("1 waiting on #9", false)], "pin cohort");
  assert.equal(byPin.length, 1);
  assert.equal(byPin[0].id, "cohort-pin");
  const byRecall = filterCommands([pinCohortCommand("1 waiting on #9", true)], "recall");
  assert.equal(byRecall.length, 1);
});

test("unpinCohortCommand reads 'Unpin cohort (<summary>)' and is enabled only when pinned", () => {
  const c = unpinCohortCommand("3 waiting on #1", true);
  assert.equal(c.id, "cohort-unpin");
  assert.match(c.title, /Unpin cohort \(3 waiting on #1\)/);
  assert.equal(c.disabled, false);
});

test("unpinCohortCommand is disabled when the focused cohort isn't pinned", () => {
  const c = unpinCohortCommand("3 waiting on #1", false);
  assert.equal(c.disabled, true);
});

test("unpinCohortCommand reads plainly and is disabled with no cohort", () => {
  for (const pinned of [true, false]) {
    const c = unpinCohortCommand(null, pinned);
    assert.equal(c.title, "Unpin cohort");
    assert.equal(c.disabled, true);
  }
});

test("unpinCohortCommand is fuzzy-findable by 'unpin' and 'forget'", () => {
  const byUnpin = filterCommands([unpinCohortCommand("2 waiting on #4", true)], "unpin");
  assert.equal(byUnpin.length, 1);
  assert.equal(byUnpin[0].id, "cohort-unpin");
});

// --- F144: forget all stale cohort views -----------------------------------

test("forgetStaleCohortsCommand names the stale count and is enabled when > 0", () => {
  const c = forgetStaleCohortsCommand(3);
  assert.equal(c.id, "cohort-forget-stale");
  assert.match(c.title, /Forget all stale cohort views \(3\)/);
  assert.equal(c.disabled, false);
});

test("forgetStaleCohortsCommand reads plainly and is disabled at zero stale", () => {
  const c = forgetStaleCohortsCommand(0);
  assert.equal(c.title, "Forget all stale cohort views");
  assert.equal(c.disabled, true);
});

test("forgetStaleCohortsCommand is fuzzy-findable by 'stale' and 'sweep'", () => {
  assert.equal(filterCommands([forgetStaleCohortsCommand(2)], "stale")[0].id, "cohort-forget-stale");
  assert.equal(filterCommands([forgetStaleCohortsCommand(2)], "sweep")[0].id, "cohort-forget-stale");
});

test("commandDisabledReason explains a zero-stale forget command", () => {
  assert.equal(
    commandDisabledReason("cohort-forget-stale", { ...FULL_CTX, staleCohortCount: 0 }),
    "no stale cohort views",
  );
  // With stale views present the command is live (no reason).
  assert.equal(commandDisabledReason("cohort-forget-stale", FULL_CTX), null);
});

// --- F149: recall busiest view command -------------------------------------

test("recallBusiestViewCommand names the winner + count and is enabled", () => {
  const cmd = recallBusiestViewCommand("#work", 12);
  assert.equal(cmd.id, "view-busiest");
  assert.equal(cmd.title, "Recall busiest view (#work, 12)");
  assert.equal(cmd.group, "Views");
  assert.equal(cmd.disabled, false);
});

test("recallBusiestViewCommand is a disabled placeholder with no clear winner", () => {
  // No winner (null name/count) -> a plain, discoverable-but-disabled command.
  const cmd = recallBusiestViewCommand(null, null);
  assert.equal(cmd.title, "Recall busiest view");
  assert.equal(cmd.disabled, true);
  // A half-null pair (name but no count, or vice versa) is also treated as no winner.
  assert.equal(recallBusiestViewCommand("#work", null).disabled, true);
  assert.equal(recallBusiestViewCommand(null, 5).disabled, true);
});

test("recallBusiestViewCommand is fuzzy-findable by 'busiest' and 'densest'", () => {
  const cmd = recallBusiestViewCommand("#work", 9);
  assert.equal(filterCommands([cmd], "busiest").length, 1);
  assert.equal(filterCommands([cmd], "densest").length, 1);
});

test("commandDisabledReason explains an absent busiest view", () => {
  assert.equal(
    commandDisabledReason("view-busiest", { ...FULL_CTX, hasBusiestView: false }),
    "no busiest view",
  );
  // With a winner present the command is live (no reason).
  assert.equal(commandDisabledReason("view-busiest", FULL_CTX), null);
});

test("cohort commands are fuzzy-findable by 'cohort' and 'chokepoint'", () => {
  const cmds = [clearCohortCommand("2 on #5"), focusChokepointCommand(5)];
  assert.equal(filterCommands(cmds, "cohort").length, 2);
  assert.equal(filterCommands(cmds, "chokepoint")[0].id, "cohort-focus-biggest");
});

// --- F107: per-chokepoint focus commands for the runners-up ----------------

test("buildChokepointFocusCommands skips the biggest and emits one per runner-up", () => {
  const cmds = buildChokepointFocusCommands([
    { id: 1, count: 4 }, // biggest — owned by focusChokepointCommand, skipped
    { id: 2, count: 2 },
    { id: 5, count: 1 },
  ]);
  assert.equal(cmds.length, 2);
  assert.deepEqual(
    cmds.map((c) => c.id),
    ["cohort-focus-2", "cohort-focus-5"],
  );
  assert.equal(cmds[0].title, "Focus chokepoint #2 (2 waiting)");
  assert.equal(cmds[1].title, "Focus chokepoint #5 (1 waiting)");
  assert.equal(cmds[0].group, "View");
});

test("buildChokepointFocusCommands returns [] when there are no runners-up", () => {
  assert.deepEqual(buildChokepointFocusCommands([]), []); // flat board
  assert.deepEqual(buildChokepointFocusCommands([{ id: 1, count: 3 }]), []); // only the biggest
});

test("buildChokepointFocusCommands ids never collide with cohort-focus-biggest", () => {
  // The decoder in main.ts routes "cohort-focus-biggest" to the static command
  // and "cohort-focus-<N>" to setCohort(N); a numeric id can never spell
  // "biggest", so the two dispatch paths stay disjoint.
  const cmds = buildChokepointFocusCommands([
    { id: 1, count: 9 },
    { id: 42, count: 3 },
  ]);
  assert.ok(cmds.every((c) => c.id !== "cohort-focus-biggest"));
  assert.equal(cmds[0].id, "cohort-focus-42");
});

test("buildChokepointFocusCommands rows are fuzzy-findable by id and 'bottleneck'", () => {
  const cmds = buildChokepointFocusCommands([
    { id: 1, count: 4 },
    { id: 8, count: 2 },
  ]);
  // The "#8" keyword lets you jump straight to a known chokepoint by id.
  assert.equal(filterCommands(cmds, "#8")[0].id, "cohort-focus-8");
  assert.equal(filterCommands(cmds, "bottleneck").length, 1);
});

// --- F114: "Focus the new biggest chokepoint" lead command on a shift --------

test("focusShiftedChokepointCommand leads with the shift only when it changed", () => {
  // A real shift (#7 -> #3) yields the lead command naming both ids.
  const c = focusShiftedChokepointCommand(3, 7);
  assert.notEqual(c, null);
  assert.equal(c!.id, "cohort-focus-new");
  assert.equal(c!.title, "Focus the new biggest chokepoint (#3, was #7)");
  assert.equal(c!.group, "View");
});

test("focusShiftedChokepointCommand is null on a steady board (same id)", () => {
  assert.equal(focusShiftedChokepointCommand(5, 5), null);
});

test("focusShiftedChokepointCommand is null when either id is absent", () => {
  assert.equal(focusShiftedChokepointCommand(null, 7), null); // flat board now
  assert.equal(focusShiftedChokepointCommand(3, null), null); // no prior to compare
  assert.equal(focusShiftedChokepointCommand(null, null), null);
});

test("focusShiftedChokepointCommand id is disjoint from biggest + per-chokepoint ids", () => {
  // The decoder routes "cohort-focus-new" through the switch (not the numeric
  // path), so it must differ from both "cohort-focus-biggest" and any
  // "cohort-focus-<N>". "new" is non-numeric, so it can never collide.
  const c = focusShiftedChokepointCommand(3, 7)!;
  assert.notEqual(c.id, "cohort-focus-biggest");
  assert.ok(!Number.isFinite(Number(c.id.slice("cohort-focus-".length))));
});

test("focusShiftedChokepointCommand is fuzzy-findable by 'new' and the new id", () => {
  const c = focusShiftedChokepointCommand(9, 2)!;
  assert.equal(filterCommands([c], "new")[0].id, "cohort-focus-new");
  assert.equal(filterCommands([c], "#9").length, 1);
  assert.equal(filterCommands([c], "shift").length, 1);
});


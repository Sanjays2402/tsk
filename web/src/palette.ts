/**
 * Command palette model (F18) — a pure, testable core for a Cmd-K palette that
 * fuzzy-finds every action in the app and runs it keyboard-only.
 *
 * main.ts owns the overlay DOM, the global Cmd-K shortcut, and the actual
 * action callbacks; this module owns the data: what a Command is, how the
 * query filters + ranks the list, and how the highlighted index moves. Keeping
 * it pure means the fuzzy ranking and selection math are unit-tested with zero
 * DOM.
 *
 * Ranking (best first), all case-insensitive:
 *   1. exact title match
 *   2. title starts with the query
 *   3. query is a substring of the title
 *   4. query is a subsequence of "title + keywords" (fuzzy)
 * Ties break by the command's declared order, so the list is stable.
 */

import { highlightText } from "./highlight.ts";

export interface Command {
  /** Stable id used as the action key. */
  id: string;
  /** Human label shown in the list. */
  title: string;
  /** Optional section grouping label (e.g. "Task", "View"). */
  group?: string;
  /** Extra words that should match the query but aren't shown. */
  keywords?: string[];
  /** Optional shortcut hint string shown on the right (e.g. "n"). */
  hint?: string;
  /** True to grey it out + skip running (e.g. "undo" with nothing to undo). */
  disabled?: boolean;
}

export interface ScoredCommand {
  cmd: Command;
  score: number;
}

/** Case-insensitive subsequence test. */
export function isSubseq(needle: string, hay: string): boolean {
  if (needle === "") return true;
  let ni = 0;
  for (let hi = 0; hi < hay.length && ni < needle.length; hi++) {
    if (hay[hi] === needle[ni]) ni++;
  }
  return ni === needle.length;
}

/**
 * Score a command against a query. Returns a number where higher is better, or
 * -1 when it doesn't match at all. An empty query matches everything with a
 * neutral score so the full list shows in declared order.
 */
export function scoreCommand(cmd: Command, query: string): number {
  const q = query.trim().toLowerCase();
  if (q === "") return 0;
  const title = cmd.title.toLowerCase();
  if (title === q) return 100;
  if (title.startsWith(q)) return 80;
  const idx = title.indexOf(q);
  if (idx >= 0) return 60 - Math.min(idx, 20); // earlier substring ranks higher
  // Fuzzy over title + keywords.
  const hay = (cmd.title + " " + (cmd.keywords ?? []).join(" ")).toLowerCase();
  if (isSubseq(q, hay)) return 30;
  return -1;
}

/**
 * Filter + rank commands for a query, preserving declared order on ties.
 * Disabled commands still appear (greyed) but sink slightly so live actions
 * surface first at equal score.
 */
export function filterCommands(commands: Command[], query: string): Command[] {
  const scored: Array<{ cmd: Command; score: number; order: number }> = [];
  commands.forEach((cmd, order) => {
    const score = scoreCommand(cmd, query);
    if (score < 0) return;
    scored.push({ cmd, score: cmd.disabled ? score - 0.5 : score, order });
  });
  scored.sort((a, b) => (b.score !== a.score ? b.score - a.score : a.order - b.order));
  return scored.map((s) => s.cmd);
}

/** Clamp the highlighted index into [0, len), wrapping past the ends. */
export function moveIndex(current: number, len: number, delta: number): number {
  if (len <= 0) return 0;
  return (((current + delta) % len) + len) % len;
}

/** Clamp an index into range without wrapping (used after the list shrinks). */
export function clampIndex(current: number, len: number): number {
  if (len <= 0) return 0;
  return Math.max(0, Math.min(current, len - 1));
}

/** Escape strings before injecting into innerHTML. Local copy keeps this pure. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/**
 * Render the results list. Pure -> unit-tested. The highlighted row carries
 * `is-active` + aria-selected; each row carries data-cmd-id for dispatch.
 * Renders an empty-state row when nothing matches.
 *
 * F57: when a `query` is passed, the matched subsequence in each command title
 * is wrapped in <mark> (reusing the generic highlightText engine that powers
 * the row title / tag / notes highlight), so it's obvious why a fuzzy match
 * surfaced. The title is HTML-escaped by highlightText; without a query it
 * falls back to plain escaping.
 */
export function renderPaletteList(
  commands: Command[],
  activeIndex: number,
  query = "",
): string {
  if (commands.length === 0) {
    return `<li class="cmdk-empty" role="option" aria-disabled="true">No matching commands</li>`;
  }
  return commands
    .map((cmd, i) => {
      const active = i === activeIndex ? " is-active" : "";
      const disabled = cmd.disabled ? " is-disabled" : "";
      const group = cmd.group
        ? `<span class="cmdk-group">${escapeHTML(cmd.group)}</span>`
        : "";
      const hint = cmd.hint
        ? `<kbd class="cmdk-hint">${escapeHTML(cmd.hint)}</kbd>`
        : "";
      const title = highlightText(cmd.title, query);
      return `<li class="cmdk-item${active}${disabled}" role="option" aria-selected="${i === activeIndex}" data-cmd-id="${escapeHTML(cmd.id)}">
        <span class="cmdk-title">${title}</span>
        ${group}${hint}
      </li>`;
    })
    .join("");
}

/**
 * F63: the four "Set priority: X" palette commands, in urgent-first order. Pure
 * so the id/title/disabled shape is unit-tested without the app. `hasSel` gates
 * them on a selection existing; `current` (the selected task's priority)
 * disables the level already in effect so the group also reads as a "current
 * priority" indicator. The ids are the same `prio-set-<level>` keys runCommand
 * dispatches on via setPriority.
 */
export type PriorityLevel = "urgent" | "high" | "medium" | "low";

export function buildPriorityCommands(
  hasSel: boolean,
  current: PriorityLevel | undefined,
): Command[] {
  const levels: ReadonlyArray<{ level: PriorityLevel; label: string; keywords: string[] }> = [
    { level: "urgent", label: "Urgent", keywords: ["priority", "u", "important", "now"] },
    { level: "high", label: "High", keywords: ["priority", "h"] },
    { level: "medium", label: "Medium", keywords: ["priority", "m", "normal"] },
    { level: "low", label: "Low", keywords: ["priority", "l", "later", "someday"] },
  ];
  return levels.map(({ level, label, keywords }) => ({
    id: `prio-set-${level}`,
    title: `Set priority: ${label}`,
    group: "Priority",
    keywords,
    disabled: !hasSel || current === level,
  }));
}

/**
 * F67: the "Set due ▸" palette command group — pick a due date keyboard-only
 * from Cmd-K without opening the date picker. Mirrors F63's priority group:
 * pure so the id/title/keyword shape is unit-tested without the app. Each
 * command's `token` is the natural-language string handed verbatim to the
 * server's PATCH `due` field (the same dateparse the picker uses), so "today",
 * "tomorrow", "fri", "1w" resolve identically; the "clear" command sends "" to
 * drop the date. `hasSel` gates them all on a selection existing. The ids are
 * `due-set-<token>` keys runCommand dispatches on via the existing commitDue.
 */
export interface DueCommand extends Command {
  /** The natural-language string sent to the server (or "" to clear). */
  token: string;
}

export function buildDueCommands(hasSel: boolean): DueCommand[] {
  const presets: ReadonlyArray<{ token: string; label: string; keywords: string[] }> = [
    { token: "today", label: "Today", keywords: ["due", "date", "deadline", "now"] },
    { token: "tomorrow", label: "Tomorrow", keywords: ["due", "date", "deadline", "tmrw"] },
    { token: "eow", label: "This weekend", keywords: ["due", "date", "weekend", "sat", "sun", "end of week"] },
    { token: "1w", label: "Next week", keywords: ["due", "date", "week", "7d"] },
    { token: "eom", label: "End of month", keywords: ["due", "date", "month", "eom"] },
    { token: "", label: "Clear due date", keywords: ["due", "date", "remove", "none", "unset"] },
  ];
  return presets.map(({ token, label, keywords }) => ({
    id: token === "" ? "due-set-clear" : `due-set-${token}`,
    title: token === "" ? "Set due: Clear" : `Set due: ${label}`,
    group: "Due",
    keywords,
    token,
    disabled: !hasSel,
  }));
}

/**
 * F73: decode the natural-language due token a `due-set-<token>` command id
 * carries, so the palette can live-preview the resolved date when the command
 * is highlighted (mirroring the F47 bulk-due preview). Pure → unit-tested.
 *   - "due-set-today"  -> "today"
 *   - "due-set-1w"     -> "1w"
 *   - "due-set-clear"  -> ""   (the clear command — caller shows "Clears…")
 *   - anything else    -> null (not a due command; no preview)
 * The "" return for clear is intentionally distinct from null: clear IS a due
 * command (it previews "Clears the due date"), it just carries no NL token.
 */
export function dueTokenForCommandId(id: string): string | null {
  if (id === "due-set-clear") return "";
  if (id.startsWith("due-set-")) return id.slice("due-set-".length);
  return null;
}

/**
 * F77: decode the priority level a `prio-set-<level>` command id carries, so
 * the palette can live-preview the "current -> new" transition when the command
 * is highlighted (the sync sibling of F73's due preview, which needs a server
 * parse). Pure → unit-tested. Returns null for any non-priority command (no
 * preview line).
 */
export function priorityForCommandId(id: string): PriorityLevel | null {
  if (!id.startsWith("prio-set-")) return null;
  const level = id.slice("prio-set-".length);
  if (level === "urgent" || level === "high" || level === "medium" || level === "low") {
    return level;
  }
  return null;
}

/** F77: the preview view-model for a highlighted "Set priority" command. */
export interface PriorityPreview {
  /** "valid" when the level would change; "empty" when it's already in effect. */
  state: "valid" | "empty";
  /** Human text, e.g. "Medium \u2192 Urgent" or "Already Urgent". */
  text: string;
}

/** Title-case a priority level for display ("urgent" -> "Urgent"). */
function capLevel(level: string): string {
  return level.charAt(0).toUpperCase() + level.slice(1);
}

/**
 * F77: shape the "current -> new" priority preview for the palette. Pure →
 * unit-tested. When the selected task is already at the target level the
 * command is disabled (buildPriorityCommands handles that), so the preview
 * reads "Already <Level>"; otherwise it shows the transition with an arrow. A
 * task with no explicit priority renders the left side as an em dash.
 */
export function priorityPreviewVM(
  current: PriorityLevel | undefined,
  target: PriorityLevel,
): PriorityPreview {
  if (current === target) {
    return { state: "empty", text: `Already ${capLevel(target)}` };
  }
  const from = current ? capLevel(current) : "\u2014";
  return { state: "valid", text: `${from} \u2192 ${capLevel(target)}` };
}

/**
 * F77: render the priority preview line. Reuses the F12/F47/F73 `.due-preview`
 * state classes (is-valid -> accent, is-empty -> faint) so it reads identically
 * to the due preview that shares the same palette slot. Pure → unit-tested.
 */
export function renderPriorityPreview(vm: PriorityPreview): string {
  return `<span class="due-preview is-${vm.state}">${escapeHTML(vm.text)}</span>`;
}

/**
 * F83: the human reason a "Set due ▸" / "Set priority ▸" palette command is
 * disabled, so a greyed command can explain itself in the preview slot instead
 * of just dimming. Pure → unit-tested.
 *
 * Only the "no selection" case is surfaced here: a set command needs a target
 * task, and without one every set command is disabled — \"select a task first\"
 * tells you the one thing to do. The OTHER disabled case ("already urgent") is
 * already handled by priorityPreviewVM's empty state, which renders its own
 * "Already <Level>" text, so it's intentionally left to that path. Returns null
 * for any non-set command (no hint) or when a selection exists (the normal
 * preview takes over).
 */
export function setCommandDisabledReason(id: string, hasSel: boolean): string | null {
  const isSet = priorityForCommandId(id) !== null || dueTokenForCommandId(id) !== null;
  if (!isSet) return null;
  if (!hasSel) return "select a task first";
  return null;
}

/**
 * F83: render a disabled-reason hint into the shared palette preview slot,
 * reusing the `.due-preview is-empty` faint style so it reads as a quiet
 * "can't do this yet" note rather than a live preview. Pure → unit-tested.
 */
export function renderDisabledReason(reason: string): string {
  return `<span class="due-preview is-empty">${escapeHTML(reason)}</span>`;
}

/**
 * F89: the per-task command ids that need a SELECTION to act on, so a disabled
 * one (no row selected) can explain itself with "select a task first" — the
 * same reason F83 gives the Set due/priority commands. Exported as a single
 * source of truth so the reason map can't drift from buildCommands' disabled
 * gating (these are the ids whose `disabled` is `!hasSel`).
 */
export const SELECTION_GATED_COMMANDS: ReadonlySet<string> = new Set([
  "toggle",
  "edit",
  "due",
  "notes",
  "deps",
  "pin",
  "prio-up",
  "prio-down",
  "delete",
]);

/** F89: the live context a disabled command needs to explain WHY it's greyed. */
export interface CommandReasonContext {
  /** Is a task currently selected? */
  hasSel: boolean;
  /** Is there a pending delete to undo? */
  canUndo: boolean;
  /** Are there any tasks to filter (the filter bar is shown)? */
  hasTasks: boolean;
  /** Is the board currently on a tag page (so "all tasks" is meaningful)? */
  onTag: boolean;
}

/**
 * F89: the human reason ANY disabled palette command is greyed — the general
 * form of F83, which only covered the Set due/priority group. So every greyed
 * command explains itself in the preview slot:
 *   - the selection-gated per-task verbs (toggle/edit/due/notes/deps/pin/
 *     prio-up/prio-down/delete) + the Set due/priority group, with no selection
 *     -> "select a task first";
 *   - "Undo last delete" with nothing pending -> "nothing to undo";
 *   - "Focus filter / search" with no tasks   -> "no tasks to filter";
 *   - "Go to all tasks" when already there     -> "already on all tasks".
 * Returns null for a command with no known disabled-reason (the slot hides).
 * Pure → unit-tested. Delegates to setCommandDisabledReason first so the F83
 * set-command behaviour is preserved exactly. The caller only consults this for
 * a command it already knows is disabled, so the context conditions here line
 * up with buildCommands' `disabled` gates.
 */
export function commandDisabledReason(id: string, ctx: CommandReasonContext): string | null {
  const setReason = setCommandDisabledReason(id, ctx.hasSel);
  if (setReason !== null) return setReason;
  if (SELECTION_GATED_COMMANDS.has(id) && !ctx.hasSel) return "select a task first";
  if (id === "undo" && !ctx.canUndo) return "nothing to undo";
  if (id === "filter" && !ctx.hasTasks) return "no tasks to filter";
  if (id === "alltasks" && !ctx.onTag) return "already on all tasks";
  return null;
}

/**
 * Cohort focus (F96) — a pure, testable model for "show me exactly THESE tasks"
 * by a set of ids, the render-pipeline sibling of a lens.
 *
 * A LENS (lens.ts) narrows to a derived predicate (blocked / overdue / a time
 * window). A COHORT narrows to an explicit id set — the concrete tasks waiting
 * on a chokepoint. F92 surfaces the biggest chokepoint (#N with K waiters) and
 * F85/F87 let you WALK its dependent chain; F96 adds the other half: drop the
 * whole board down to exactly those K undone dependents so you can ACT on the
 * cohort (bulk-select, bulk-edit, export) rather than only walk it.
 *
 * Like a lens, a cohort is NOT a FilterState facet: it doesn't serialize into
 * saved views (the id set is a momentary "what waits on #N right now" snapshot,
 * meaningless once those tasks complete), and it's mutually exclusive with a
 * lens (both are "special narrowings" layered on top of the text/facet filter).
 * main.ts owns the single `focusCohort` slot, the chip, and the clear wiring;
 * this module owns the data — the id-set narrowing and the chip markup — kept
 * pure so it's unit-tested with zero browser.
 */

import { openDependents, doneIndex, type DepStatsTask } from "./deps.ts";

/**
 * A focus on the undone tasks waiting on one chokepoint. `ids` is the snapshot
 * of open dependents at the moment the cohort was built (store order); main.ts
 * re-derives a fresh cohort whenever it re-focuses, so a stale set never sticks.
 */
export interface CohortFocus {
  /** The chokepoint task the cohort is waiting on. */
  sourceId: number;
  /** The undone tasks directly waiting on sourceId, in store order. */
  ids: number[];
}

/**
 * Build a cohort focus for `sourceId` — the undone tasks directly waiting on it
 * (its open dependents, via deps.openDependents over the whole live list).
 * Returns null when nothing waits on it (no cohort to show) so the caller can
 * skip focusing an empty set. Pure → unit-tested.
 */
export function buildCohort(tasks: DepStatsTask[], sourceId: number): CohortFocus | null {
  const ids = openDependents(tasks, sourceId, doneIndex(tasks));
  if (ids.length === 0) return null;
  return { sourceId, ids };
}

/**
 * Narrow a task list to the cohort's id set, preserving input order. Generic so
 * it keeps the caller's concrete task type (it runs over the live `Task[]` in
 * the render pipeline). A Set keeps the membership test O(1) over big boards.
 */
export function applyCohort<T extends { id: number }>(tasks: T[], ids: ReadonlySet<number>): T[] {
  return tasks.filter((t) => ids.has(t.id));
}

/** Escape strings before injecting into innerHTML. Local copy keeps this pure. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/** The plural-aware noun for K waiting tasks ("1 waiting" / "3 waiting"). */
export function cohortCount(focus: CohortFocus): string {
  const n = focus.ids.length;
  return `${n} waiting`;
}

/**
 * Render the inner markup for the cohort chip in the filter bar — mirrors the
 * lens chip (renderLensChipBody): an up-arrow glyph echoing the F87 "N waiting"
 * badge, the count + the chokepoint id, and a trailing × that signals a click
 * clears the focus. Returns "" for a null focus so the chip element hides.
 *
 * F108: when `historyDepth` > 0 (you drilled into this cohort FROM another), a
 * leading ‹ back glyph (data-cohort-back) is prepended — clicking it steps back
 * to the previous cohort instead of clearing. The glyph is a non-interactive
 * span inside the chip button (mouse-only, mirroring the decorative × clear);
 * Escape is the keyboard path. Omitting / zero keeps the chip byte-identical.
 *
 * F113: when the back-stack is more than one deep (`historyDepth` > 1), the back
 * glyph also shows the depth as a tiny superscript-ish count ("‹2") so a
 * multi-step drill reveals HOW deep you are, not just that there's somewhere to
 * go back to. The Escape / glyph behaviour is unchanged — it still steps exactly
 * ONE level (popCohortHistory) regardless of the badge; the count is purely a
 * readout. A depth of 0 or 1 renders the bare ‹ glyph as before.
 */
export function renderCohortChipBody(focus: CohortFocus | null, historyDepth = 0): string {
  if (focus === null) return "";
  let back = "";
  if (historyDepth > 0) {
    // F113: surface the stack depth once it's more than one level deep so a
    // multi-step drill shows its depth. Single-level history stays the bare ‹.
    const depthBadge =
      historyDepth > 1
        ? `<span class="cohort-back-depth" aria-hidden="true">${historyDepth}</span>`
        : "";
    const title =
      historyDepth > 1
        ? `Back to the previous cohort (${historyDepth} in history)`
        : "Back to the previous cohort";
    back = `<span class="cohort-back" data-cohort-back aria-hidden="true" title="${title}">&#8249;${depthBadge}</span> `;
  }
  return `${back}&#8593; ${escapeHTML(cohortCount(focus))} on #${focus.sourceId} <span class="lens-x" aria-hidden="true">&times;</span>`;
}

/** The hover/aria title for the active cohort chip. */
export function cohortChipTitle(focus: CohortFocus): string {
  const n = focus.ids.length;
  const noun = n === 1 ? "task" : "tasks";
  return `Showing only the ${n} ${noun} waiting on #${focus.sourceId} — click to clear`;
}

/**
 * F103: a one-line human summary of the active cohort for the Cmd-K palette
 * label (e.g. "3 waiting on #1"), so a "Clear cohort focus (<summary>)" command
 * doubles as a "what am I focused on?" readout — the cohort sibling of F98's
 * lens-label-in-the-command pattern. Pure → unit-tested.
 */
export function cohortSummary(focus: CohortFocus): string {
  return `${cohortCount(focus)} on #${focus.sourceId}`;
}

/**
 * F101: the outcome of re-deriving a cohort focus against a fresh task list.
 * A cohort's `ids` are a snapshot of who waited on #sourceId at click time; an
 * external edit (CLI / TUI / hand / another tab) can complete those waiters,
 * add new ones, or finish #sourceId itself. After a refresh the focus must
 * track reality rather than show a stale set.
 */
export interface CohortReconcile {
  /** The refreshed cohort (updated id set), or null when it no longer holds. */
  focus: CohortFocus | null;
  /** True when a previously-held cohort lost ALL its waiters — caller toasts. */
  cleared: boolean;
}

/**
 * F101: re-derive a cohort focus against the current graph. Given the previous
 * focus and the fresh task list, rebuild the cohort for the SAME sourceId via
 * buildCohort:
 *   - no prior focus           -> { focus: null, cleared: false }  (nothing to do)
 *   - sourceId still has waiters-> { focus: <fresh>, cleared: false } (id set updated)
 *   - sourceId done / gone /
 *     all waiters completed     -> { focus: null,  cleared: true }  (focus dropped)
 * Pure → unit-tested. main.ts calls this on every refresh so a focused cohort
 * survives external edits (with a live id set) instead of silently going stale,
 * and drops cleanly (with a quiet toast) when its blocked cohort is all done.
 */
export function reconcileCohort(
  tasks: DepStatsTask[],
  prev: CohortFocus | null,
): CohortReconcile {
  if (prev === null) return { focus: null, cleared: false };
  const fresh = buildCohort(tasks, prev.sourceId);
  if (fresh === null) return { focus: null, cleared: true };
  return { focus: fresh, cleared: false };
}

/**
 * F102: render the "focus these" affordance for the dependent chain-drill
 * popover head — it narrows the board to exactly the cohort of undone tasks
 * waiting on `sourceId` (the same setCohort path F96's sidebar button drives),
 * so you can drop into a cohort from ANY chokepoint you're walking, not just
 * the single biggest one F92/F96 surface in the sidebar. Carries
 * `data-cohort-focus="<id>"` — the SAME hook the sidebar focus button uses —
 * so a delegated click routes through the existing cohort wiring with no new
 * dispatch path. Pure → unit-tested.
 */
export function renderCohortFocusButton(sourceId: number): string {
  const title = `Focus the board on the tasks waiting on #${sourceId}`;
  return `<button type="button" class="chain-pop-focus" data-cohort-focus="${sourceId}" title="${escapeHTML(title)}" aria-label="${escapeHTML(title)}">focus these</button>`;
}

/**
 * F108: push a source id onto the cohort back-stack, returning a NEW stack.
 * setCohort replaces the single focus slot; this records where you came FROM so
 * a later "back" can return to it. De-dupes a no-op push (the same id already on
 * top — re-focusing the current cohort shouldn't grow the stack) and caps the
 * depth so a long focus chain can't grow unbounded over a session. Pure →
 * unit-tested. The stack is momentary (per-session, never persisted) — cohorts
 * are snapshots, so the history is too.
 */
export function pushCohortHistory(stack: readonly number[], sourceId: number, cap = 20): number[] {
  if (stack.length > 0 && stack[stack.length - 1] === sourceId) return [...stack];
  const next = [...stack, sourceId];
  return next.length > cap ? next.slice(next.length - cap) : next;
}

/** F108: the outcome of stepping the cohort back-stack one level. */
export interface CohortBack {
  /** The remaining stack after the pop(s). */
  stack: number[];
  /** The rebuilt focus to land on, or null when no live ancestor remains. */
  focus: CohortFocus | null;
}

/**
 * F108: pop the cohort back-stack to the most recent ancestor that STILL has a
 * live cohort. Pops ids off the END (most-recent-first), rebuilding each via
 * buildCohort against the FRESH graph, until one holds (returns it + the
 * remaining stack) or the stack empties (returns { stack: [], focus: null }).
 * Skipping dead ancestors (the chokepoint completed, or all its waiters are
 * done) means a back-step never lands on an empty focus — it transparently
 * walks past stale history to the nearest still-meaningful cohort. Pure →
 * unit-tested. main.ts calls this on Escape / the chip's back glyph.
 */
export function popCohortHistory(
  tasks: DepStatsTask[],
  stack: readonly number[],
): CohortBack {
  const next = [...stack];
  while (next.length > 0) {
    const id = next.pop()!;
    const focus = buildCohort(tasks, id);
    if (focus !== null) return { stack: next, focus };
  }
  return { stack: [], focus: null };
}

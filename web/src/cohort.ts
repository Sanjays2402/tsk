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
 */
export function renderCohortChipBody(focus: CohortFocus | null): string {
  if (focus === null) return "";
  return `&#8593; ${escapeHTML(cohortCount(focus))} on #${focus.sourceId} <span class="lens-x" aria-hidden="true">&times;</span>`;
}

/** The hover/aria title for the active cohort chip. */
export function cohortChipTitle(focus: CohortFocus): string {
  const n = focus.ids.length;
  const noun = n === 1 ? "task" : "tasks";
  return `Showing only the ${n} ${noun} waiting on #${focus.sourceId} — click to clear`;
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

/**
 * Render-pipeline lenses (F66) — a pure, testable model for the one-click
 * "show me only these" narrowings the stats sidebar drives.
 *
 * A LENS is deliberately NOT a FilterState facet. Facets (query / priority /
 * tag / hide-done) are per-task, stable, and serialize into saved views. A
 * lens is a DERIVED subset that is either cross-task (blocked depends on OTHER
 * tasks' done state) or TIME-relative (overdue / due-today / due-this-week /
 * no-due all shift as the clock moves). Saving "overdue" into a named view
 * would be meaningless tomorrow, so lenses live OUTSIDE FilterState and run as
 * a render-pipeline step after the text/facet filter — exactly where F64's
 * blocked-only lens already sat. This module generalizes that single boolean
 * into a small enum so every stats tile can drive the list the same way.
 *
 * main.ts owns the single `activeLens` slot, the stats-tile click wiring, and
 * the filter-bar chip; this module owns the data: the predicate per lens, the
 * list narrowing, and the chip label/markup. Keeping it pure means the
 * day-window math + the cross-task blocked predicate are unit-tested with zero
 * browser. Exactly ONE lens is active at a time (they are mutually-exclusive
 * subsets), so a single chip + a toggle-on-reclick model is all the UI needs.
 */

import { isBlocked, type DepTask } from "./deps.ts";

/**
 * The lenses the stats sidebar can drive. `blocked` is cross-task; the other
 * four are time-relative to the client's "today". `open` (hide-done) is NOT a
 * lens — it maps to the real `hideDone` filter facet, which DOES serialize, so
 * main.ts routes it there instead.
 */
export type LensKind = "blocked" | "overdue" | "today" | "week" | "nodue";

/** The minimal shape a task needs to be matched by a lens. */
export interface LensTask extends DepTask {
  /** YYYY-MM-DD, or missing/"" for no due date. */
  due?: string;
}

/** Parse a YYYY-MM-DD string to a local-midnight day-number, or null if absent/bad. */
function dayNum(due: string | undefined): number | null {
  if (!due) return null;
  const [y, m, d] = due.split("-").map((n) => parseInt(n, 10));
  if (!y || !m || !d) return null;
  return Math.floor(new Date(y, m - 1, d).getTime() / 86_400_000);
}

/** The local-midnight day-number for a reference `now`. */
function todayNum(now: Date): number {
  return Math.floor(
    new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime() / 86_400_000,
  );
}

/**
 * Does a task satisfy a lens? Pure → unit-tested.
 *   - blocked: an undone task with at least one OPEN blocker (uses the done
 *     index built over the WHOLE live list, so a blocker hidden by another
 *     filter still counts).
 *   - overdue / today / week / nodue: an UNDONE task whose due date lands in
 *     the named window. A done task never matches a schedule lens (you already
 *     finished it). A malformed due string counts as "no due" since it can't be
 *     scheduled — matching how section grouping treats it.
 *     - overdue: due strictly before today.
 *     - today:   due is today.
 *     - week:    due in [today, today+6] (the coming 7 days INCLUDING today;
 *                overdue is excluded — it has its own lens).
 *     - nodue:   no (parseable) due date.
 */
export function matchesLens(
  task: LensTask,
  kind: LensKind,
  now: Date,
  done: Map<number, boolean>,
): boolean {
  if (kind === "blocked") return isBlocked(task, done);
  if (task.done) return false; // schedule lenses are about outstanding work
  const today = todayNum(now);
  const day = dayNum(task.due);
  switch (kind) {
    case "nodue":
      return day === null;
    case "overdue":
      return day !== null && day < today;
    case "today":
      return day !== null && day === today;
    case "week":
      return day !== null && day >= today && day <= today + 6;
  }
  return false;
}

/**
 * Narrow a task list to a single lens, preserving input order. Generic so it
 * keeps the caller's concrete task type (it runs over the live `Task[]` in the
 * render pipeline). The done-index should be built over the WHOLE live list.
 */
export function applyLens<T extends LensTask>(
  tasks: T[],
  kind: LensKind,
  now: Date,
  done: Map<number, boolean>,
): T[] {
  return tasks.filter((t) => matchesLens(t, kind, now, done));
}

/** Display metadata for the active-lens chip. */
export interface LensMeta {
  /** Short chip label, lower-case to match the hide-done pill. */
  label: string;
  /** A leading glyph echoing the stat tile the lens came from. */
  glyph: string;
  /** The hue class the chip wears so it echoes its source tile. */
  hue: "alert" | "today" | "neutral";
}

const LENS_META: Readonly<Record<LensKind, LensMeta>> = {
  blocked: { label: "blocked", glyph: "\u26D4", hue: "alert" }, // ⛔
  overdue: { label: "overdue", glyph: "\u26A0", hue: "alert" }, // ⚠
  today: { label: "due today", glyph: "\u25F7", hue: "today" }, // ◷
  week: { label: "due this week", glyph: "\u25A6", hue: "today" }, // ▦
  nodue: { label: "no due date", glyph: "\u2205", hue: "neutral" }, // ∅
};

/** The chip metadata for a lens kind. */
export function lensMeta(kind: LensKind): LensMeta {
  return LENS_META[kind];
}

/**
 * F71: the canonical lens order for the number-key shortcuts (and the help
 * overlay row). Digit 1 -> blocked, 2 -> overdue, 3 -> today, 4 -> week,
 * 5 -> no-due — the same left-to-right order the stats tiles read in. Kept as a
 * single source of truth so the keyboard map and any "lens N" labelling can't
 * drift from the tile order.
 */
export const LENS_ORDER: ReadonlyArray<LensKind> = [
  "blocked",
  "overdue",
  "today",
  "week",
  "nodue",
];

/**
 * F71: map a KeyboardEvent.key ("1".."5") to the lens at that 1-based slot, or
 * null for anything else. Pure → unit-tested. main.ts toggles the returned lens
 * (pressing the active lens's digit again clears it) without opening the
 * sidebar, so the whole stats-lens set is reachable keyboard-only.
 */
export function lensForDigit(key: string): LensKind | null {
  if (key.length !== 1 || key < "1" || key > "9") return null;
  const idx = key.charCodeAt(0) - "1".charCodeAt(0);
  return idx < LENS_ORDER.length ? LENS_ORDER[idx] : null;
}

/**
 * F71: a one-line, human summary of the active lens for the help overlay's
 * "current view" line. Returns "" when no lens is active so the line can hide.
 */
export function activeLensSummary(kind: LensKind | null): string {
  if (kind === null) return "";
  const meta = LENS_META[kind];
  return `${meta.glyph} ${meta.label}`;
}

/** Escape strings before injecting into innerHTML. Local copy keeps this pure. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/**
 * Render the inner markup for the single active-lens chip in the filter bar.
 * Returns "" when no lens is active (the chip element hides). The chip carries
 * the lens glyph + label and a trailing × to signal that clicking it clears
 * the lens. The `hue-<kind>` class lets the chip echo its source tile's color.
 */
export function renderLensChipBody(kind: LensKind | null): string {
  if (kind === null) return "";
  const meta = LENS_META[kind];
  return `${meta.glyph} ${escapeHTML(meta.label)} <span class="lens-x" aria-hidden="true">&times;</span>`;
}

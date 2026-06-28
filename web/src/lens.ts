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

/**
 * F80: the richer task shape the lens breakdown needs — a lens task plus its
 * priority, so the sidebar can show how the lensed subset distributes across
 * urgency levels.
 */
export interface LensBreakdownTask extends LensTask {
  /** "low" | "medium" | "high" | "urgent"; absent counts toward none. */
  priority?: string;
}

/**
 * F80: a small distribution of the tasks matched by the active lens, so the
 * stats sidebar can read as "what am I actually looking at?" rather than always
 * reporting whole-board numbers. Carries the lensed total, the per-priority
 * split, and two cross-cuts (how many of the lensed subset are ALSO overdue /
 * blocked) that the renderer suppresses when they'd be redundant with the lens
 * itself.
 */
export interface LensBreakdown {
  total: number;
  urgent: number;
  high: number;
  medium: number;
  low: number;
  /** How many of the lensed tasks are overdue (redundant under the overdue lens). */
  overdue: number;
  /** How many of the lensed tasks are blocked (redundant under the blocked lens). */
  blocked: number;
}

/**
 * F80: compute the breakdown of the subset a lens selects. Pure → unit-tested.
 * Applies the lens first (reusing applyLens + the same whole-list done-index),
 * then tallies the per-priority counts and the overdue / blocked cross-cuts.
 * The cross-cuts reuse matchesLens so they stay perfectly consistent with the
 * lenses the user can switch to.
 */
export function computeLensBreakdown(
  tasks: LensBreakdownTask[],
  kind: LensKind,
  now: Date,
  done: Map<number, boolean>,
): LensBreakdown {
  const subset = applyLens(tasks, kind, now, done);
  const out: LensBreakdown = {
    total: subset.length,
    urgent: 0,
    high: 0,
    medium: 0,
    low: 0,
    overdue: 0,
    blocked: 0,
  };
  for (const t of subset) {
    switch (t.priority) {
      case "urgent":
        out.urgent++;
        break;
      case "high":
        out.high++;
        break;
      case "medium":
        out.medium++;
        break;
      case "low":
        out.low++;
        break;
    }
    if (kind !== "overdue" && matchesLens(t, "overdue", now, done)) out.overdue++;
    if (kind !== "blocked" && matchesLens(t, "blocked", now, done)) out.blocked++;
  }
  return out;
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

/**
 * F76: the 1-based digit shortcut for a lens (the inverse of lensForDigit), as
 * a string ready to drop into a kbd badge — "1".."5" for a known lens, "" for
 * anything not in LENS_ORDER (e.g. the stats "open" tile, which maps to the
 * hideDone facet rather than a numbered lens). Accepting a plain string lets
 * the stats tiles — which carry their lens as a `data-lens-drill` string — ask
 * for a badge without a cast. Reusing LENS_ORDER keeps the hint in lock-step
 * with lensForDigit so a tile's badge can never point at the wrong key.
 */
export function lensDigit(kind: string): string {
  const idx = (LENS_ORDER as ReadonlyArray<string>).indexOf(kind);
  return idx >= 0 ? String(idx + 1) : "";
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
 *
 * F82: a leading digit badge echoes the number-key shortcut that toggles this
 * lens (the same "1".."5" the F76 stat tiles wear), so the active shortcut is
 * visible right on the chip — "2 ⚠ overdue ×". Reuses lensDigit so the badge
 * can never drift from lensForDigit / LENS_ORDER.
 */
export function renderLensChipBody(kind: LensKind | null): string {
  if (kind === null) return "";
  const meta = LENS_META[kind];
  const digit = lensDigit(kind);
  const key = digit
    ? `<kbd class="lens-chip-key" aria-hidden="true">${digit}</kbd> `
    : "";
  return `${key}${meta.glyph} ${escapeHTML(meta.label)} <span class="lens-x" aria-hidden="true">&times;</span>`;
}

/**
 * F82: render the help overlay's "active lens" line as HTML (vs the plain-text
 * activeLensSummary). Bolds the lens label and shows its number-key shortcut as
 * a <kbd>, so the `?` overlay surfaces both WHAT you're looking at and the
 * digit that toggles it — the same shortcut the chip and the stat tiles show.
 * Returns "" when no lens is active so the line collapses. Pure → unit-tested.
 *
 * F116: when the active lens was re-applied by recalling a lensed saved view
 * (F104), an optional `provenance` (the recalled view's NAME, from
 * lensProvenanceNote) appends a quiet "from <view>" note — so the keyboard-only
 * `?` summary answers "why is this lens on?" without looking at the filter bar,
 * mirroring F109's chip-side readout. Omitting it (or passing null) keeps the
 * line byte-identical, so a digit-key / stat-tile lens (no source view) reads
 * exactly as before.
 *
 * F120: when the active lens is ALSO pinned (it has a pure-lens saved view —
 * the F110 star is filled, the F115 palette command reads "Recall pinned lens"),
 * an optional `pinned` flag appends a small "★ pinned" marker so the `?` summary
 * reports the pin state too. The chip star (F110), the Cmd-K command (F115), and
 * this overlay line all read the SAME findPureLensView result in main.ts, so the
 * three surfaces can never disagree about whether the active lens is pinned.
 * Defaults to false, keeping the line byte-identical for an unpinned lens.
 */
export function renderActiveLensHelp(
  kind: LensKind | null,
  provenance: string | null = null,
  pinned = false,
): string {
  if (kind === null) return "";
  const meta = LENS_META[kind];
  const digit = lensDigit(kind);
  const key = digit ? `<kbd>${digit}</kbd> ` : "";
  const from = provenance
    ? ` <span class="help-lens-from">from ${escapeHTML(provenance)}</span>`
    : "";
  const pin = pinned
    ? ` <span class="help-lens-pinned" title="This lens is pinned as a saved view">\u2605 pinned</span>`
    : "";
  return `Active lens: ${key}${meta.glyph} <strong>${escapeHTML(meta.label)}</strong>${from}${pin}`;
}

/**
 * F131: the action a "toggle pin on the active lens" keyboard shortcut should
 * take, given the active lens and whether it's already pinned. The whole pin
 * lifecycle (pin F110/F115, recall, unpin F125) was reachable by mouse (the chip
 * star) and Cmd-K, but there was no DIRECT key. This makes the binding's decision
 * a pure, unit-tested function so main.ts's keydown handler stays declarative:
 *   - no active lens   -> "none"   (nothing to pin; the key is a no-op)
 *   - active + unpinned -> "pin"    (route through pinCurrentLens)
 *   - active + pinned   -> "unpin"  (route through unpinCurrentLens)
 * Pure → unit-tested; mirrors the star's pin-vs-recall/unpin state so the key,
 * the star, and the palette commands can't disagree about what a toggle does.
 */
export function lensPinToggleAction(
  kind: LensKind | null,
  pinned: boolean,
): "none" | "pin" | "unpin" {
  if (kind === null) return "none";
  return pinned ? "unpin" : "pin";
}

/**
 * F136: the action the lens-pin STAR's keyboard handler should take, given the
 * key, the shift modifier, the active lens, and whether it's already pinned. The
 * star is a real <button> so Enter/Space already fire a native click (→ pin /
 * recall), but there's no keyboard equivalent of the mouse's right-click /
 * long-press UNPIN. This makes the star's keys explicit + unit-tested, mirroring
 * the mouse affordances rather than the global `*` toggle (F131):
 *   - no active lens                 -> "none"  (the star is hidden anyway)
 *   - a non-activation key           -> "none"  (let it bubble)
 *   - Shift + Enter/Space, pinned    -> "unpin" (the keyboard right-click)
 *   - Shift + Enter/Space, unpinned  -> "none"  (nothing to unpin)
 *   - Enter/Space (no shift)         -> "pin"   (pinCurrentLens recalls if pinned)
 * Distinct from lensPinToggleAction: the star pins on plain activation and unpins
 * only with Shift (matching click vs right-click), where `*` toggles. main.ts
 * preventDefaults on a handled key so the native button click can't double-fire.
 * Pure → unit-tested.
 */
export function lensStarKeyAction(
  key: string,
  shift: boolean,
  kind: LensKind | null,
  pinned: boolean,
): "none" | "pin" | "unpin" {
  if (kind === null) return "none";
  const activate = key === "Enter" || key === " " || key === "Spacebar";
  if (!activate) return "none";
  if (shift) return pinned ? "unpin" : "none";
  return "pin";
}

/**
 * F90: render the FULL lens digit map as a live mini-legend for the help
 * overlay — every lens with its number-key shortcut inline ("1 ⛔ blocked ·
 * 2 ⚠ overdue · …"), in LENS_ORDER so it can't drift from the keyboard map.
 * The currently-active lens (if any) is marked with `is-active` + aria-current
 * so the overlay teaches the WHOLE digit map at a glance AND shows where you
 * are. Pure → unit-tested. Unlike renderActiveLensHelp (which shows only the one
 * active lens), this always renders all five so the shortcuts are discoverable
 * even when no lens is active. Each entry carries `data-lens-legend="<kind>"`.
 *
 * F91: the entries are real `<button>`s (not inert spans), so the `?` overlay
 * doubles as an actionable lens switcher — a delegated click toggles that lens
 * the same way its number-key / stat tile does, then closes the overlay. The
 * markup keeps the same classes + `data-lens-legend` hook F90 shipped, so the
 * existing legend tests and styling carry over unchanged.
 *
 * F120: an optional `pinnedKind` marks the ACTIVE lens's legend entry with an
 * `is-pinned` class + a ★ when that lens is pinned (has a pure-lens saved view).
 * Only the active+pinned entry is marked, and `pinnedKind` should equal `active`
 * when set (main.ts derives both from the same state), so the digit map agrees
 * with the active-lens line's "★ pinned" marker and the chip star — the third of
 * the three surfaces F120 keeps in sync. Null (the default) keeps the legend
 * byte-identical, so an unpinned / no-lens map reads exactly as before.
 */
export function renderLensDigitMap(active: LensKind | null, pinnedKind: LensKind | null = null): string {
  const items = LENS_ORDER.map((kind) => {
    const meta = LENS_META[kind];
    const digit = lensDigit(kind);
    const on = kind === active ? " is-active" : "";
    const isPinned = kind === active && kind === pinnedKind;
    const pinnedCls = isPinned ? " is-pinned" : "";
    const current = kind === active ? ' aria-current="true"' : "";
    const star = isPinned ? ` <span class="lens-legend-pin" aria-hidden="true">\u2605</span>` : "";
    const title = kind === active
      ? isPinned
        ? `Clear the ${meta.label} lens (pinned)`
        : `Clear the ${meta.label} lens`
      : `Show only ${meta.label} (key ${digit})`;
    return `<button type="button" class="lens-legend-item${on}${pinnedCls}" data-lens-legend="${kind}"${current} title="${escapeHTML(title)}"><kbd>${digit}</kbd> ${meta.glyph} ${escapeHTML(meta.label)}${star}</button>`;
  }).join('<span class="lens-legend-sep" aria-hidden="true"> &middot; </span>');
  return `<div class="lens-legend">${items}</div>`;
}

/**
 * F93: sessionStorage key for the per-tab active lens. A lens is time-relative
 * (overdue / today / week shift as the clock moves) and cross-task (blocked
 * depends on other tasks' done state), so it must NOT leak across sessions the
 * way a saved view does — sessionStorage (per-tab, cleared on close) is the
 * right scope, mirroring F88's export-scope persistence. Exported so the reader
 * and writer can't drift on the key string.
 */
export const LENS_KEY = "tsk.lens";

/**
 * F93: decode a persisted lens value into a LensKind, validating it against
 * LENS_ORDER so a stale/garbage stored value (e.g. a lens kind removed in a
 * later version, or a hand-poked sessionStorage) can never wedge the board into
 * an unknown state — it simply falls back to "no lens". Pure → unit-tested.
 * Returns null for null/empty/unknown input, the matching LensKind otherwise.
 */
export function parseLens(raw: string | null): LensKind | null {
  if (!raw) return null;
  return (LENS_ORDER as ReadonlyArray<string>).includes(raw) ? (raw as LensKind) : null;
}

/**
 * F97: sessionStorage key for the priority FACET drilled on top of an active
 * lens (the F81 breakdown-pill drill: lens AND urgent). F93 persists the lens
 * itself per-tab; this persists the facet that rides it, so reloading restores
 * the WHOLE lens+facet drill, not just the lens. Like the lens, it's per-tab
 * (sessionStorage) and scoped to the lens lifecycle — written only while a lens
 * is active, cleared when the lens clears — so a plain priority facet (no lens)
 * keeps its existing non-persisted behaviour. Exported so the reader/writer
 * can't drift on the key string.
 */
export const LENS_FACET_KEY = "tsk.lens.facet";

/** The four priority levels a lens-drill facet can hold, for validation. */
const FACET_LEVELS: ReadonlyArray<string> = ["urgent", "high", "medium", "low"];

/**
 * F97: decode a persisted lens-facet value (a JSON array of priority levels)
 * into a clean string[] of known levels, in the stored order, de-duplicated.
 * Any non-array, non-string entry, or unknown level is dropped — a stale or
 * hand-poked value can never inject a bad facet. Returns [] for null/empty/
 * garbage. Pure → unit-tested. main.ts casts the result to its Priority[] type
 * (the level strings are identical) and only applies it when a lens restored.
 */
export function parseLensFacet(raw: string | null): string[] {
  if (!raw) return [];
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return [];
  }
  if (!Array.isArray(parsed)) return [];
  const out: string[] = [];
  const seen = new Set<string>();
  for (const v of parsed) {
    if (typeof v !== "string") continue;
    if (!FACET_LEVELS.includes(v) || seen.has(v)) continue;
    seen.add(v);
    out.push(v);
  }
  return out;
}

/**
 * F97: serialize a lens-drill priority facet for sessionStorage — a compact
 * JSON array of the known levels (unknown entries dropped so a round-trip is
 * idempotent). Pure → unit-tested; pairs with parseLensFacet.
 */
export function serializeLensFacet(priorities: ReadonlyArray<string>): string {
  return JSON.stringify(priorities.filter((p) => FACET_LEVELS.includes(p)));
}

/**
 * F81: the priority levels a lens-breakdown pill can drill into, in the same
 * urgent-first order the pills render. The cross-cut pills (overdue / blocked)
 * are NOT in this set — they're derived facets, not the `priorities` filter
 * facet, so they read as static counts while only these four are clickable
 * drill-downs. Exported so the click handler can validate a pill's token
 * against the same source of truth the renderer uses.
 */
export const LENS_BD_PRIORITIES: ReadonlyArray<"urgent" | "high" | "medium" | "low"> = [
  "urgent",
  "high",
  "medium",
  "low",
];

/**
 * F81: decode a lens-breakdown pill's `data-lens-bd-prio` token into a priority
 * level the `priorities` filter facet understands, or null for anything that
 * isn't one of the four drillable levels (e.g. the overdue / blocked cross-cut
 * pills, which carry no token). Pure → unit-tested. Keeps the click handler
 * from trusting raw DOM strings.
 */
export function lensBreakdownPriority(token: string): "urgent" | "high" | "medium" | "low" | null {
  return (LENS_BD_PRIORITIES as ReadonlyArray<string>).includes(token)
    ? (token as "urgent" | "high" | "medium" | "low")
    : null;
}

/**
 * F80: render the lensed-subset breakdown for the stats sidebar. Pure →
 * unit-tested. Shown only while a lens is active so the sidebar reflects "what
 * I'm looking at": a headline ("12 blocked") plus a row of small count pills for
 * the priority split (urgent/high/medium/low, each shown only when non-zero)
 * and the overdue / blocked cross-cuts (suppressed when they'd be redundant with
 * the active lens, which computeLensBreakdown already zeroes). Returns "" when
 * no lens is active or the subset is empty so the section collapses cleanly.
 *
 * F81: the four priority pills are now interactive drill-downs — each is a
 * <button data-lens-bd-prio> that layers the matching `priorities` filter facet
 * on top of the active lens (lens AND urgent), turning the readout into a
 * one-click narrowing. A pill whose priority is already in `active` wears
 * `is-on` so the breakdown doubles as a "what facet am I drilled into?"
 * indicator and a second click toggles it back off. The overdue / blocked
 * cross-cuts stay plain spans (they map to other lenses, not this facet).
 */
export function renderLensBreakdown(
  kind: LensKind | null,
  bd: LensBreakdown,
  active: ReadonlySet<string> = new Set(),
): string {
  if (kind === null || bd.total === 0) return "";
  const meta = LENS_META[kind];
  const pills: Array<{ cls: string; label: string; n: number; prio?: string }> = [
    { cls: "bd-urgent", label: "urgent", n: bd.urgent, prio: "urgent" },
    { cls: "bd-high", label: "high", n: bd.high, prio: "high" },
    { cls: "bd-medium", label: "medium", n: bd.medium, prio: "medium" },
    { cls: "bd-low", label: "low", n: bd.low, prio: "low" },
    { cls: "bd-overdue", label: "overdue", n: bd.overdue },
    { cls: "bd-blocked", label: "blocked", n: bd.blocked },
  ];
  const body = pills
    .filter((p) => p.n > 0)
    .map((p) => {
      const inner = `<span class="lens-bd-n">${p.n}</span> ${escapeHTML(p.label)}`;
      if (p.prio) {
        const on = active.has(p.prio) ? " is-on" : "";
        return `<button type="button" class="lens-bd-pill is-drill ${p.cls}${on}" data-lens-bd-prio="${p.prio}" aria-pressed="${active.has(p.prio)}" title="Filter the ${escapeHTML(meta.label)} view to ${escapeHTML(p.label)}">${inner}</button>`;
      }
      return `<span class="lens-bd-pill ${p.cls}">${inner}</span>`;
    })
    .join("");
  const pillRow = body ? `<div class="lens-bd-pills">${body}</div>` : "";
  return `
    <div class="stats-section-label">In view</div>
    <div class="lens-bd">
      <div class="lens-bd-head">${meta.glyph} <strong>${bd.total}</strong> ${escapeHTML(meta.label)}</div>
      ${pillRow}
    </div>`;
}

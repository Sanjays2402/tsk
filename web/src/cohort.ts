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
 * F117: render the help (`?`) overlay's "active cohort" line as HTML — the
 * cohort sibling of F116's active-lens line. F108/F113 track a per-session
 * cohort back-stack with a chip depth badge, but the keyboard-only `?` summary
 * never mentions the cohort at all, so a keyboard user can't answer "what am I
 * focused on, and how deep did I drill?" without looking at the filter bar.
 *
 * Returns the cohort summary (reusing cohortSummary so it can't drift from the
 * Cmd-K command label / chip) bolded, e.g. "Cohort focus: <strong>3 waiting on
 * #1</strong>". When the back-stack is non-empty (`historyDepth` > 0) it appends
 * a quiet "(‹K in history)" note echoing the F113 chip depth badge, so the
 * overlay reports the drill depth too. Returns "" for a null focus so the line
 * collapses (mirroring renderActiveLensHelp). Pure → unit-tested; the ‹ glyph
 * matches the chip's back affordance so the two readouts read consistently.
 *
 * F122: the history note is a real `<button data-cohort-back>` (not inert text),
 * so the keyboard user can step back through the drill from the `?` summary
 * itself — a delegated click in the overlay runs the SAME cohortBack the chip's
 * ‹ glyph and Escape already drive (zero new dispatch path, mirroring F91's
 * actionable legend). The button keeps the F117 `help-cohort-history` class +
 * the "‹K in history" text/glyph so existing styling and tests carry over; only
 * a null/zero history renders no button (nothing to step back to). The button is
 * emitted only when there's history, so a depth-0 cohort line stays plain text.
 */
export function renderCohortHelp(focus: CohortFocus | null, historyDepth = 0): string {
  if (focus === null) return "";
  const history =
    historyDepth > 0
      ? ` <button type="button" class="help-cohort-history" data-cohort-back title="Step back to the previous cohort">(&#8249;${historyDepth} in history)</button>`
      : "";
  return `Cohort focus: <strong>${escapeHTML(cohortSummary(focus))}</strong>${history}`;
}

/**
 * F126: render the stats panel's "active cohort" line — the mouse-surface
 * sibling of F117's `?`-overlay cohort breadcrumb. The stats panel shows the
 * biggest-chokepoint line but never the ACTIVE cohort focus, so when you've
 * dropped the board onto "3 waiting on #1" the panel doesn't reflect it. This
 * adds a small "Focused: <summary>" readout (reusing cohortSummary so it can't
 * drift from the chip / Cmd-K command / help line) as a button carrying
 * `data-cohort-clear` — a click clears the focus through the existing clearCohort
 * wiring (the panel sibling of the filter-bar clear chip). Returns "" for a null
 * focus so the line collapses on an unfocused board. Pure → unit-tested.
 *
 * F127: when the cohort has back-history (`historyDepth` > 0, from F108/F113's
 * back-stack), a leading ‹ back button (data-cohort-back) is prepended — the
 * SAME hook the chip's ‹ glyph, Escape, and the F122 help breadcrumb drive — so
 * a mouse user can step back through the drill from the panel too, not only
 * clear it. Mirroring the chip, the depth is shown as a tiny "‹N" badge once
 * the stack is more than one deep; a single level shows the bare ‹. The button
 * is emitted ONLY when there's history (depth 0 renders no back affordance).
 *
 * F128: a trailing "walk" button (data-waiting-walk="<sourceId>") jumps from the
 * active-cohort readout straight into the F85 dependent chain-drill for the
 * chokepoint — the panel sibling of the chip's implicit "what waits?" question,
 * reusing the SAME data-waiting-walk hook the sidebar chokepoint rows use (so it
 * routes through main.ts's existing openChainDrill(sourceId, "dependent") with
 * zero new dispatch). Present whenever a cohort is focused.
 *
 * The three affordances are sibling buttons inside a flex row (a button can't
 * nest another button), so each click target is disjoint — back / clear / walk
 * never overlap. main.ts wraps the whole return in the `.stats-cohort` block.
 *
 * F133: an optional trailing PIN star (data-cohort-pin) lets you bookmark the
 * focused chokepoint as a saved "cohort view" — re-focus "what waits on #N" in
 * one click next session (the id-set is re-derived live on recall). The star is
 * ★ (filled) when this chokepoint is already pinned and ☆ (hollow) when not, so
 * it doubles as a pinned-state readout; main.ts passes `pinned` from
 * findCohortView. Omitted (`pinned` undefined) renders no star, keeping older
 * callers/tests byte-identical. The star sits after the walk button as a fourth
 * disjoint sibling.
 */
export function renderCohortPanelLine(
  focus: CohortFocus | null,
  historyDepth = 0,
  pinned?: boolean,
): string {
  if (focus === null) return "";
  const summary = escapeHTML(cohortSummary(focus));
  const title = `Focused on the ${focus.ids.length === 1 ? "task" : "tasks"} waiting on #${focus.sourceId} — click to clear`;
  // F127: a back-step button, only when there's history to step back through.
  // The depth badge mirrors the chip (bare ‹ at depth 1, ‹N once deeper).
  let back = "";
  if (historyDepth > 0) {
    const depthBadge = historyDepth > 1 ? `${historyDepth}` : "";
    const backTitle =
      historyDepth > 1
        ? `Step back to the previous cohort (${historyDepth} in history)`
        : `Step back to the previous cohort`;
    back = `<button type="button" class="stat-cohort-back" data-cohort-back title="${escapeHTML(backTitle)}" aria-label="${escapeHTML(backTitle)}">&#8249;${depthBadge}</button>`;
  }
  const clear = `<button type="button" class="stat-cohort-line" data-cohort-clear title="${escapeHTML(title)}" aria-label="${escapeHTML(title)}"><span class="stat-cohort-label">Focused</span> <span class="stat-cohort-val">${summary}</span> <span class="stat-cohort-x" aria-hidden="true">&times;</span></button>`;
  // F128: the walk affordance — open the dependent chain-drill for the chokepoint.
  const walkTitle = `Walk the tasks waiting on #${focus.sourceId}`;
  const walk = `<button type="button" class="stat-cohort-walk" data-waiting-walk="${focus.sourceId}" title="${escapeHTML(walkTitle)}" aria-label="${escapeHTML(walkTitle)}">walk</button>`;
  // F133: the pin star — bookmark / un-bookmark this chokepoint as a cohort view.
  let pin = "";
  if (pinned !== undefined) {
    const star = pinned ? "\u2605" : "\u2606"; // ★ / ☆
    const pinTitle = pinned
      ? `Unpin the cohort waiting on #${focus.sourceId}`
      : `Pin the cohort waiting on #${focus.sourceId} as a saved view`;
    const pinClass = pinned ? "stat-cohort-pin is-pinned" : "stat-cohort-pin";
    pin = `<button type="button" class="${pinClass}" data-cohort-pin="${focus.sourceId}" title="${escapeHTML(pinTitle)}" aria-label="${escapeHTML(pinTitle)}" aria-pressed="${pinned ? "true" : "false"}">${star}</button>`;
  }
  return `<div class="stat-cohort-row">${back}${clear}${walk}${pin}</div>`;
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

/**
 * F132: jump STRAIGHT to a specific ancestor in the cohort back-stack, not just
 * one level. F108's popCohortHistory steps back exactly one cohort; the F132
 * breadcrumb trail renders the WHOLE stack so a click can leap to any ancestor
 * directly. Given a target index into `stack` (0 = oldest), this rebuilds the
 * cohort for that ancestor against the FRESH graph and returns it plus the
 * remaining stack (everything BEFORE the landed ancestor). It reuses
 * popCohortHistory's skip-dead-ancestor walk by running it over the truncated
 * prefix `stack.slice(0, targetIndex + 1)`: if the targeted ancestor has since
 * gone dead (its chokepoint completed, or all its waiters finished), the jump
 * transparently lands on the nearest still-live ancestor at-or-before it rather
 * than on an empty focus — exactly mirroring a multi-tap back. An out-of-range
 * index is a no-op (returns the stack unchanged + a null focus) so the caller
 * can decline to act. Pure → unit-tested; main.ts calls this from the trail.
 */
export function jumpCohortHistory(
  tasks: DepStatsTask[],
  stack: readonly number[],
  targetIndex: number,
): CohortBack {
  if (targetIndex < 0 || targetIndex >= stack.length) {
    return { stack: [...stack], focus: null };
  }
  return popCohortHistory(tasks, stack.slice(0, targetIndex + 1));
}

/**
 * F137: the cohort-history index a trail keyboard step should jump to — the
 * keyboard sibling of F132's mouse-only breadcrumb trail (click a segment to
 * leap to an ancestor). The trail renders ancestors oldest-first then the
 * current cohort; a keyboard user couldn't walk that ancestry without the mouse.
 *
 * Given the back-stack length and a direction, return the index to hand to
 * cohortJumpTo (which routes through jumpCohortHistory, skipping dead ancestors):
 *   - "step" -> historyLength - 1: the MOST-RECENT ancestor, i.e. exactly one
 *     level back (the same landing cohortBack/F108 produces — jumping to the top
 *     of the stack pops just the last entry). Repeated steps walk back one
 *     ancestor at a time because the stack trims on each jump.
 *   - "root" -> 0: the OLDEST ancestor, the drill origin — a one-press leap all
 *     the way back to where the cohort drill started.
 * Both directions walk TOWARD the root (the only direction a back-stack
 * supports — there's no redo once you've stepped back). An empty history returns
 * -1 so the caller declines to act (nothing to step through). Pure → unit-tested;
 * main.ts maps Alt+Left -> "step" and Alt+Right -> "root", then cohortJumpTo.
 */
export function cohortTrailKeyTarget(historyLength: number, dir: "step" | "root"): number {
  if (historyLength <= 0) return -1;
  return dir === "root" ? 0 : historyLength - 1;
}

/**
 * F147: the back-stack index of the DENSEST ancestor in the cohort drill — the
 * ancestor whose chokepoint has the most live waiters (the worst bottleneck you
 * walked through). F132's trail lets you eyeball the ancestry and F137 walks it
 * step/root; this lets a key (Alt+D) leap STRAIGHT to the heaviest one, which is
 * usually the real thing to go fix.
 *
 * `waiterCount(sourceId)` is injected (the live cohort size lives in the graph;
 * keeping cohort.ts decoupled mirrors the buildCohort-via-main.ts pattern) —
 * main.ts backs it with buildCohort(...).ids.length over the live tasks. We scan
 * only `history` (the ancestors), NOT the current focus: you're already ON the
 * current cohort, so "jump to densest" means densest ANCESTOR. The first (oldest,
 * lowest-index) ancestor wins a tie — the ancestor closer to the drill root is
 * the more fundamental bottleneck when two are equally heavy, and a stable rule
 * keeps repeated presses deterministic. A dead ancestor (its chokepoint cleared,
 * count 0) is never a candidate; if EVERY ancestor is dead/empty (or the history
 * is empty), returns -1 so the caller declines to act. Pure → unit-tested; the
 * returned index is handed to cohortJumpTo (which itself skips-dead, so a live
 * winner here always lands cleanly).
 */
export function densestCohortAncestorIndex(
  history: readonly number[],
  waiterCount: (sourceId: number) => number,
): number {
  let bestIndex = -1;
  let bestCount = 0;
  for (let i = 0; i < history.length; i++) {
    const c = waiterCount(history[i]);
    if (c > bestCount) {
      bestCount = c;
      bestIndex = i;
    }
  }
  return bestIndex;
}

/**
 * F140: format the cohort drill path as plain copyable text — "#A › #B ›
 * #current" — for the F132 trail's "copy chain" affordance. The trail buttons
 * let you JUMP through the ancestry; this lets you lift the whole bottleneck
 * walk out as text to paste into a standup note or an issue ("blocked chain:
 * #1 › #4 › #9"). Mirrors renderCohortTrail's segment order (ancestors
 * oldest-first via `history`, then the current focus as the terminal segment)
 * and reuses the same › separator glyph the trail renders, so the copied text
 * reads exactly like the on-screen breadcrumb. Returns "" when there's no focus
 * OR no history (a depth-0 cohort has no chain to copy — the single current
 * cohort isn't a path), matching renderCohortTrail's empty conditions so the
 * copy button and the trail appear/vanish together. Pure → unit-tested.
 */
export function formatCohortTrailText(focus: CohortFocus | null, history: readonly number[]): string {
  if (focus === null || history.length === 0) return "";
  const ids = [...history, focus.sourceId];
  return ids.map((id) => `#${id}`).join(" \u203a "); // › with surrounding spaces
}

/**
 * F143: format the cohort drill path as a MARKDOWN task-link chain — the richer
 * sibling of F140's plain "#A › #B › #current". F140 copies bare text good for a
 * quick paste; this lifts the same walk as a GitHub-flavoured chain of issue
 * references joined by arrows ("#1 \u2192 #4 \u2192 #9"), so pasting into a
 * markdown issue / PR / standup doc renders each segment as a live cross-link
 * (GitHub auto-links bare #N), and the arrows survive as plain text. Mirrors
 * formatCohortTrailText's segment order (ancestors oldest-first via `history`,
 * then the current focus) and its empty conditions (no focus OR no history ->
 * "") so F140's plain copy and F143's markdown copy appear/vanish together. The
 * → glyph (U+2192) is intentionally distinct from F140's › (U+203A) so a reader
 * can tell at a glance which format they pasted. Pure → unit-tested.
 */
export function formatCohortTrailMarkdown(focus: CohortFocus | null, history: readonly number[]): string {
  if (focus === null || history.length === 0) return "";
  const ids = [...history, focus.sourceId];
  return ids.map((id) => `#${id}`).join(" \u2192 "); // → with surrounding spaces
}

/**
 * F132: render the cohort back-stack as a compact breadcrumb TRAIL for the stats
 * panel — the multi-step sibling of F127's single back-step button. F108/F113
 * track a per-session cohort history but the chip / F127 panel button only ever
 * reveal the NEXT step back; this lays out the whole ancestry as
 * "#A › #B › #(current)" so you can see how deep the drill went AND leap to any
 * ancestor in one click.
 *
 * Each ancestor in `history` (oldest first, the same order pushCohortHistory
 * appends) renders as a `<button data-cohort-jump="<index>">#<id></button>` so a
 * delegated click routes through main.ts's cohortJumpTo(index) → jumpCohortHistory
 * (which skips dead ancestors). The CURRENT focus is the terminal segment — a
 * non-interactive span marked aria-current="step" ("you are here"), since you're
 * already on it. Segments are separated by an inert "›" glyph echoing the chip's
 * ‹ back affordance so the two readouts read consistently.
 *
 * Returns "" when there's no focus OR no history (a depth-0 cohort has no trail
 * to show — the F126 line already names the single current cohort), so the trail
 * only appears once you've actually drilled. Pure → unit-tested; the ids need no
 * escaping (they're numbers) but the helper stays consistent with the module.
 *
 * F140: a trailing "copy chain" button (data-cohort-copy) lifts the whole drill
 * path out as text ("#A › #B › #current") to the clipboard — useful for pasting
 * a bottleneck walk into a standup note. main.ts wires navigator.clipboard with
 * a guarded fallback (test env / no clipboard API → a status hint). The button
 * is the terminal segment after the current cohort, a disjoint sibling so its
 * click never overlaps a jump segment. Present whenever the trail renders (i.e.
 * whenever there's history), so it appears/vanishes with the trail itself.
 */
export function renderCohortTrail(focus: CohortFocus | null, history: readonly number[]): string {
  if (focus === null || history.length === 0) return "";
  const sep = `<span class="cohort-trail-sep" aria-hidden="true">&#8250;</span>`;
  const steps = history.map((id, i) => {
    const title = `Jump back to the cohort waiting on #${id}`;
    return `<button type="button" class="cohort-trail-step" data-cohort-jump="${i}" title="${escapeHTML(title)}" aria-label="${escapeHTML(title)}">#${id}</button>`;
  });
  const current = `<span class="cohort-trail-current" aria-current="step" title="Current cohort">#${focus.sourceId}</span>`;
  // F140: a copy-chain affordance after the current segment. Its hook carries
  // the full path text so the click handler doesn't re-derive it (single source
  // of truth = formatCohortTrailText, the same string this trail reads).
  // F143: Alt-click copies the same walk as a markdown task-link chain
  // ("#1 → #4 → #9") instead of the plain "›" text, so a paste into an issue /
  // PR renders live cross-links. The title advertises both gestures; main.ts
  // reads e.altKey to pick formatCohortTrailText vs formatCohortTrailMarkdown.
  const chainText = formatCohortTrailText(focus, history);
  const copyTitle = `Copy the drill chain (${chainText}) \u2014 Alt-click for markdown`;
  const copy = `<button type="button" class="cohort-trail-copy" data-cohort-copy title="${escapeHTML(copyTitle)}" aria-label="${escapeHTML(copyTitle)}">&#10697;</button>`;
  return `<div class="cohort-trail" role="group" aria-label="Cohort drill history">${[...steps, current].join(sep)}${copy}</div>`;
}

/**
 * Stats panel renderers (F13) — pure functions from a Stats DTO to the HTML
 * for the at-a-glance sidebar. Kept dependency-free so the number formatting,
 * the donut geometry, and the top-tags list are unit-tested without a browser.
 *
 * The data comes from GET /api/stats (computeStatsDTO on the server, which
 * reuses the same logic as the CLI `tsk stats`). main.ts owns the fetch, the
 * collapse toggle, and click-to-filter wiring on the tag rows.
 */

import type { Stats } from "./api";
import type { DepStats, Chokepoint } from "./deps";
import type { ScheduleStats } from "./schedule";
import { lensDigit } from "./lens.ts";

/** Escape strings before injecting into innerHTML. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/**
 * F76: a small keyboard-shortcut badge for a lens tile, rendered inside the
 * tile button so the digit shortcut (the F71 number keys) is discoverable at
 * the point of use, not just buried in the `?` overlay. Returns "" for a tile
 * with no numbered lens (the "open" tile maps to the hideDone facet, not a
 * digit), so only the five real lenses wear a badge. The digit comes from
 * lensDigit(LENS_ORDER) so it can't drift from lensForDigit.
 */
function lensKeyBadge(lens: string): string {
  const digit = lensDigit(lens);
  if (digit === "") return "";
  return `<kbd class="lens-key" aria-hidden="true">${digit}</kbd>`;
}

/** Round a completion percentage to a whole number for display. */
export function formatPct(completion: number): string {
  if (!Number.isFinite(completion)) return "0%";
  return `${Math.round(completion)}%`;
}

/** Pluralize a day-streak into "3 day streak" / "1 day streak" / "no streak". */
export function streakLabel(streak: number): string {
  if (streak <= 0) return "no streak";
  return `${streak} day${streak === 1 ? "" : "s"}`;
}

/**
 * Geometry for an SVG progress ring. Returns the dash offset for a given
 * completion (0-100) over a circle of radius r, where offset 0 = full ring
 * and offset=circumference = empty.
 */
export function ringDash(completion: number, r: number): { circumference: number; offset: number } {
  const circumference = 2 * Math.PI * r;
  const pct = Math.max(0, Math.min(100, Number.isFinite(completion) ? completion : 0));
  const offset = circumference * (1 - pct / 100);
  return { circumference, offset };
}

/** Render the completion donut as inline SVG. r=26 keeps it crisp at 64px. */
export function renderDonut(stats: Stats): string {
  const r = 26;
  const { circumference, offset } = ringDash(stats.completion, r);
  return `
    <svg class="stats-donut" viewBox="0 0 64 64" width="64" height="64" role="img"
         aria-label="${formatPct(stats.completion)} complete">
      <circle class="donut-track" cx="32" cy="32" r="${r}" fill="none" stroke-width="6"></circle>
      <circle class="donut-fill" cx="32" cy="32" r="${r}" fill="none" stroke-width="6"
              stroke-linecap="round"
              stroke-dasharray="${circumference.toFixed(2)}"
              stroke-dashoffset="${offset.toFixed(2)}"
              transform="rotate(-90 32 32)"></circle>
      <text class="donut-label" x="32" y="36" text-anchor="middle">${formatPct(stats.completion)}</text>
    </svg>`;
}

/** Render a single metric tile (big number + label), highlighting overdue. */
export function renderMetric(value: number, label: string, kind = ""): string {
  const cls = kind ? ` metric-${kind}` : "";
  const alert = kind === "overdue" && value > 0 ? " is-alert" : "";
  return `
    <div class="stat-metric${cls}${alert}">
      <span class="stat-num">${value}</span>
      <span class="stat-label">${escapeHTML(label)}</span>
    </div>`;
}

/**
 * F69: render a time-metric tile that, when it has any tasks, becomes a button
 * driving a render-pipeline lens (Open -> hide-done, Due today -> today-only,
 * Overdue -> overdue-only). Mirrors F64's blocked-tile pattern so the WHOLE
 * sidebar reads as "click a number to see those tasks". `lens` is the
 * `data-lens-drill` value main.ts dispatches on; a zero count stays a static
 * tile (no point filtering to an empty set). `kind` keeps the existing hue
 * classes (today / overdue alert). Pure → unit-tested.
 */
export function renderLensMetric(
  value: number,
  label: string,
  lens: string,
  kind = "",
): string {
  const cls = kind ? ` metric-${kind}` : "";
  const alert = kind === "overdue" && value > 0 ? " is-alert" : "";
  if (value <= 0) {
    return `
    <div class="stat-metric${cls}${alert}">
      <span class="stat-num">${value}</span>
      <span class="stat-label">${escapeHTML(label)}</span>
    </div>`;
  }
  return `
    <button type="button" class="stat-metric${cls}${alert}" data-lens-drill="${escapeHTML(lens)}" title="Show only ${escapeHTML(label.toLowerCase())} tasks (key ${lensDigit(lens) || "-"})">
      <span class="stat-num">${value}</span>
      <span class="stat-label">${escapeHTML(label)}</span>
      ${lensKeyBadge(lens)}
    </button>`;
}

/**
 * Render the top-tags list with proportional bars. Each row carries
 * data-stat-tag so a click can drive the F11 filter. Returns a small
 * empty-state when there are no tags.
 */
export function renderTopTags(stats: Stats): string {
  if (stats.top_tags.length === 0) {
    return `<div class="stats-empty">No tags yet</div>`;
  }
  const max = Math.max(...stats.top_tags.map((t) => t.count), 1);
  const rows = stats.top_tags
    .map((t) => {
      const w = Math.round((t.count / max) * 100);
      return `
      <button type="button" class="stat-tag-row" data-stat-tag="${escapeHTML(t.tag)}" title="Filter by #${escapeHTML(t.tag)}">
        <span class="stat-tag-name">#${escapeHTML(t.tag)}</span>
        <span class="stat-tag-bar"><span class="stat-tag-fill" style="width:${w}%"></span></span>
        <span class="stat-tag-count">${t.count}</span>
      </button>`;
    })
    .join("");
  return `<div class="stat-tags">${rows}</div>`;
}

/**
 * F46: render the dependency-health row — blocked + pinned counts and the
 * longest open-blocker chain (dependency depth). Pure → unit-tested. Returns
 * "" when there's nothing dependency-related to show (no blocked, no pinned,
 * flat graph) so the section collapses on simple boards. The blocked tile
 * lights up as an alert when anything is blocked.
 */
export function renderDepStats(dep: DepStats, choke?: Chokepoint | null, trendPrev: number | null = null): string {
  if (dep.blocked === 0 && dep.pinned === 0 && dep.longestChain === 0) return "";
  const blockedAlert = dep.blocked > 0 ? " is-alert" : "";
  // F64/F66: when anything is blocked, the Blocked tile becomes a button that
  // filters the list to just those tasks via the unified lens dispatch
  // (data-lens-drill="blocked"); when nothing is blocked it stays a static tile
  // (no point filtering to an empty set).
  const blockedTile =
    dep.blocked > 0
      ? `<button type="button" class="stat-metric metric-blocked${blockedAlert}" data-lens-drill="blocked" title="Show only blocked tasks (key ${lensDigit("blocked")})">
        <span class="stat-num">${dep.blocked}</span>
        <span class="stat-label">Blocked</span>
        ${lensKeyBadge("blocked")}
      </button>`
      : `<div class="stat-metric metric-blocked">
        <span class="stat-num">${dep.blocked}</span>
        <span class="stat-label">Blocked</span>
      </div>`;
  const chainTile =
    dep.longestChain > 0
      ? `<button type="button" class="stat-metric metric-chain" data-chain-drill title="Longest chain of open blockers — click to walk it">
      <span class="stat-num">${dep.longestChain}</span>
      <span class="stat-label">Chain depth</span>
    </button>`
      : "";
  return `
    <div class="stats-section-label">Dependencies</div>
    <div class="stats-grid stats-grid-dep">
      ${blockedTile}
      <div class="stat-metric metric-pinned">
        <span class="stat-num">${dep.pinned}</span>
        <span class="stat-label">Pinned</span>
      </div>
      ${chainTile}
    </div>
    ${renderChokepoint(choke, trendPrev)}`;
}

/**
 * F92: render the single worst bottleneck — the "biggest chokepoint" line for
 * the dep-stats sidebar, naming the undone task the most others are waiting on
 * and how many wait (the aggregate of F87's per-row "N waiting" badges). Pure →
 * unit-tested. Returns "" when there's no chokepoint (a flat board, choke ===
 * null/undefined) so the line collapses. The line is a button carrying
 * `data-waiting-walk="<id>"` — the SAME hook the row badge uses — so clicking it
 * opens the F85 dependent chain-drill for that task, jumping straight from "what's
 * the worst bottleneck?" to "what exactly waits on it?".
 *
 * F96: a secondary "focus" button (`data-cohort-focus="<id>"`) sits beside the
 * walk affordance — it narrows the WHOLE board down to exactly the K undone
 * tasks waiting on #N (a cohort focus), so you can act on the blocked cohort
 * (bulk-select, bulk-edit, export) rather than only walk the chain. The walk
 * affordance answers "what waits?"; the focus affordance lets you DO something
 * about it.
 */
export function renderChokepoint(choke?: Chokepoint | null, trendPrev: number | null = null): string {
  if (!choke || choke.count === 0) return "";
  const noun = choke.count === 1 ? "task waits" : "tasks wait";
  const walkTitle = `#${choke.id} is the biggest chokepoint — ${choke.count} ${noun} on it; click to see what`;
  const focusTitle = `Focus the board on the ${choke.count} ${choke.count === 1 ? "task" : "tasks"} waiting on #${choke.id}`;
  return `<div class="stat-chokepoint-row">
      <button type="button" class="stat-chokepoint" data-waiting-walk="${choke.id}" title="${escapeHTML(walkTitle)}" aria-label="${escapeHTML(walkTitle)}">
        <span class="chokepoint-label">Biggest chokepoint</span>
        <span class="chokepoint-val">#${choke.id} <span class="chokepoint-n">&#8593; ${choke.count} waiting</span>${renderChokepointTrend(trendPrev)}</span>
      </button>
      <button type="button" class="stat-chokepoint-focus" data-cohort-focus="${choke.id}" title="${escapeHTML(focusTitle)}" aria-label="${escapeHTML(focusTitle)}">focus</button>
    </div>`;
}

/**
 * F111: decide whether the biggest chokepoint CHANGED across a live-reload, so
 * the sidebar can show a quiet "was #M" delta instead of the bottleneck shifting
 * silently. Pure → unit-tested. `prev` is the previous refresh's biggest-
 * chokepoint id (held in a module slot in main.ts); `curr` is the fresh one.
 * Returns the prior id to surface ONLY when both exist and they differ — the
 * one case worth a hint:
 *   - no previous id (first paint)        -> null (nothing to compare)
 *   - no current chokepoint (board flat)  -> null (renderChokepoint hides anyway)
 *   - same id                             -> null (no change, no noise)
 *   - changed (#7 -> #3)                  -> 7   (so the hint reads "was #7")
 * A completed chokepoint (F101 reconcile) that shifts the worst bottleneck to
 * another task is exactly the shift this makes visible.
 */
export function chokepointTrend(prev: number | null, curr: number | null): number | null {
  if (prev === null || curr === null) return null;
  return prev === curr ? null : prev;
}

/**
 * F111: render the "was #M" trend hint appended to the biggest-chokepoint line.
 * Returns "" when there's nothing to show (trendPrev null), so the line stays
 * byte-identical on a steady board. A small muted note — it's a passive "heads
 * up the bottleneck moved", not an action. Pure → unit-tested.
 */
export function renderChokepointTrend(trendPrev: number | null): string {
  if (trendPrev === null) return "";
  return ` <span class="chokepoint-trend" title="The biggest chokepoint changed since the last refresh (was #${trendPrev})">was #${trendPrev}</span>`;
}

/**
 * F121: the one-shot toast message for a biggest-chokepoint SHIFT on a live
 * reload. F114 leads Cmd-K with the shift and F111 shows a sidebar "was #M"
 * hint, but with BOTH the palette closed AND the stats panel closed, a shifting
 * bottleneck moves silently. This builds the "Biggest chokepoint moved: #M ->
 * #N" notice main.ts shows as an info toast so the shift is noticed even with
 * every panel closed. Returns "" when there's no real shift to announce (`prev`
 * or `curr` null, or they're equal) so the caller can skip toasting — the same
 * "only on a genuine change" guard chokepointTrend uses. Pure → unit-tested.
 */
export function chokepointShiftMessage(prev: number | null, curr: number | null): string {
  if (prev === null || curr === null || prev === curr) return "";
  return `Biggest chokepoint moved: #${prev} \u2192 #${curr}`;
}

/**
 * F123: the view-model for the F121 chokepoint-shift toast, upgraded from a bare
 * message to a message + an optional FOCUS target. F121's toast is purely
 * informational ("Biggest chokepoint moved: #M -> #N"); F114 already leads Cmd-K
 * with the same shift, but a shift you notice via the toast still costs a trip to
 * the palette to act on. This bundles the new chokepoint's id alongside the
 * message so main.ts can hang a "Focus" action button (reusing showInfoToast's
 * F42 action slot) that drops straight into the new bottleneck's cohort
 * (setCohort) — the toast sibling of F114's keyboard lead.
 *
 * Returns `{ message: "", focusId: null }` when there's no genuine shift to
 * announce (the same `prev`/`curr`-null/equal guard chokepointShiftMessage uses),
 * so the caller skips both the toast AND the action. On a real shift, `focusId`
 * is the CURRENT biggest chokepoint (#N) — the task you'd want to focus, not the
 * one that left. Pure → unit-tested.
 */
export interface ChokepointShiftToast {
  message: string;
  focusId: number | null;
}

export function chokepointShiftToast(prev: number | null, curr: number | null): ChokepointShiftToast {
  const message = chokepointShiftMessage(prev, curr);
  return { message, focusId: message === "" ? null : curr };
}

/**
 * F130: should the shift-toast's "Focus" action (F123) ALSO open the stats panel?
 * The action drops into the new chokepoint's cohort (setCohort), but if the panel
 * is closed the F126 "Focused" line + the breakdown aren't visible — so the
 * cohort you just focused isn't legible where it matters. This returns true only
 * when the panel is currently CLOSED, so the action opens it exactly once and an
 * already-open panel isn't needlessly re-toggled (which would close it). Pure →
 * unit-tested; main.ts chains setCohort + a guarded toggleStats(true) on this.
 */
export function shouldRevealStatsOnFocus(statsOpen: boolean): boolean {
  return !statsOpen;
}

/**
 * F135: the plan for acting on a live chokepoint-shift toast — shared by the
 * toast's "Focus" button (F123/F130) AND the new `f` keyboard mirror, so the two
 * surfaces can't drift in what "Focus" does. Given the toast's tracked focus id
 * (the new chokepoint to drop into, or null when no toast/shift is live) and
 * whether the stats panel is currently open, this returns either:
 *   - null               → nothing to do (no live focus id), so the `f` key
 *                          falls through to its other handling / is a no-op.
 *   - { focusId, revealPanel } → setCohort(focusId), and toggleStats(true) iff
 *                          revealPanel (the panel was closed — F130's reveal).
 * Pure → unit-tested; main.ts runs setCohort + a guarded toggleStats(true) on
 * the result for BOTH the toast button and the `f` key.
 */
export interface ToastFocusAction {
  focusId: number;
  revealPanel: boolean;
}

export function toastFocusAction(
  focusId: number | null,
  statsOpen: boolean,
): ToastFocusAction | null {
  if (focusId === null) return null;
  return { focusId, revealPanel: shouldRevealStatsOnFocus(statsOpen) };
}

/**
 * F106: render the "Other bottlenecks" list — the runner-up chokepoints below
 * the single biggest one F92/renderChokepoint surfaces. On a board with several
 * bottlenecks, the second/third worst are invisible without scanning every row
 * for F87 badges; this lists them compactly so you can triage the whole set.
 *
 * `chokes` is the full ranked list from topChokepoints (biggest first); this
 * renderer SKIPS the first (already shown above) and emits the rest. Each row
 * reuses the SAME hooks the sidebar's big line + the row badges use:
 * `data-waiting-walk` (open the dependent chain-drill) on the id/count, and a
 * compact `data-cohort-focus` "focus" button — so clicking routes through the
 * existing wiring with zero new dispatch. Returns "" when there are no
 * runners-up (0 or 1 chokepoint) so the section collapses. Pure → unit-tested.
 */
export function renderOtherChokepoints(chokes: Chokepoint[]): string {
  if (chokes.length <= 1) return "";
  const rows = chokes
    .slice(1)
    .map((c) => {
      const noun = c.count === 1 ? "task waits" : "tasks wait";
      const walkTitle = `${c.count} ${noun} on #${c.id} — click to see what`;
      const focusTitle = `Focus the board on the ${c.count} ${c.count === 1 ? "task" : "tasks"} waiting on #${c.id}`;
      return `<div class="stat-choke-row">
        <button type="button" class="stat-choke-walk" data-waiting-walk="${c.id}" title="${escapeHTML(walkTitle)}" aria-label="${escapeHTML(walkTitle)}">
          <span class="stat-choke-id">#${c.id}</span>
          <span class="stat-choke-n">&#8593; ${c.count} waiting</span>
        </button>
        <button type="button" class="stat-choke-focus" data-cohort-focus="${c.id}" title="${escapeHTML(focusTitle)}" aria-label="${escapeHTML(focusTitle)}">focus</button>
      </div>`;
    })
    .join("");
  return `
    <div class="stats-section-label">Other bottlenecks</div>
    <div class="stat-choke-list">${rows}</div>`;
}

/**
 * F59: render the schedule lens — a "Due this week" tile (undone work on deck
 * in the next 7 days) and a "No due" tile (undone backlog with no date). Pure →
 * unit-tested. Returns "" when both are zero so the row collapses on an empty /
 * fully-scheduled board. The "due this week" tile lights up `metric-week` (a
 * calm accent, distinct from the overdue alert) so the coming week reads as
 * planning, not urgency.
 */
export function renderScheduleStats(sched: ScheduleStats): string {
  if (sched.dueThisWeek === 0 && sched.noDue === 0) return "";
  // F66: each tile becomes a click-to-filter lens when it has tasks (Due this
  // week -> the 7-day window, No due -> undated backlog), mirroring F64's
  // blocked tile and F69's time metrics. A zero count stays a static tile.
  const weekTile =
    sched.dueThisWeek > 0
      ? `<button type="button" class="stat-metric metric-week" data-lens-drill="week" title="Show only tasks due this week (key ${lensDigit("week")})">
        <span class="stat-num">${sched.dueThisWeek}</span>
        <span class="stat-label">Due this week</span>
        ${lensKeyBadge("week")}
      </button>`
      : `<div class="stat-metric metric-week">
        <span class="stat-num">${sched.dueThisWeek}</span>
        <span class="stat-label">Due this week</span>
      </div>`;
  const noDueTile =
    sched.noDue > 0
      ? `<button type="button" class="stat-metric metric-nodue" data-lens-drill="nodue" title="Show only tasks with no due date (key ${lensDigit("nodue")})">
        <span class="stat-num">${sched.noDue}</span>
        <span class="stat-label">No due</span>
        ${lensKeyBadge("nodue")}
      </button>`
      : `<div class="stat-metric metric-nodue">
        <span class="stat-num">${sched.noDue}</span>
        <span class="stat-label">No due</span>
      </div>`;
  return `
    <div class="stats-section-label">Schedule</div>
    <div class="stats-grid stats-grid-sched">
      ${weekTile}
      ${noDueTile}
    </div>`;
}

/** Render the whole stats panel body. Pure → unit-tested.
 *
 * F106: an optional `chokes` (the ranked top-N chokepoints from topChokepoints)
 * renders an "Other bottlenecks" list of the runners-up below the dep stats, so
 * the second/third worst chokepoints are visible alongside the single biggest
 * one (`choke`). Omitting it keeps the panel byte-identical for older callers.
 */
export function renderStatsPanel(
  stats: Stats,
  dep?: DepStats,
  sched?: ScheduleStats,
  choke?: Chokepoint | null,
  chokes?: Chokepoint[],
  trendPrev: number | null = null,
): string {
  return `
    <div class="stats-top">
      ${renderDonut(stats)}
      <div class="stats-headline">
        <div class="stats-streak" title="Consecutive days with a completed task">
          <span class="streak-flame" aria-hidden="true">&#9650;</span>${escapeHTML(streakLabel(stats.streak))}
        </div>
        <div class="stats-sub">${stats.done} of ${stats.total} done</div>
      </div>
    </div>
    <div class="stats-grid">
      ${renderLensMetric(stats.undone, "Open", "open")}
      ${renderLensMetric(stats.today, "Due today", "today", "today")}
      ${renderLensMetric(stats.overdue, "Overdue", "overdue", "overdue")}
    </div>
    ${sched ? renderScheduleStats(sched) : ""}
    ${dep ? renderDepStats(dep, choke, trendPrev) : ""}
    ${chokes ? renderOtherChokepoints(chokes) : ""}
    <div class="stats-section-label">Top tags</div>
    ${renderTopTags(stats)}`;
}

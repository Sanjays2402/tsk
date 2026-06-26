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
export function renderDepStats(dep: DepStats, choke?: Chokepoint | null): string {
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
    ${renderChokepoint(choke)}`;
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
 */
export function renderChokepoint(choke?: Chokepoint | null): string {
  if (!choke || choke.count === 0) return "";
  const noun = choke.count === 1 ? "task waits" : "tasks wait";
  const title = `#${choke.id} is the biggest chokepoint — ${choke.count} ${noun} on it; click to see what`;
  return `<button type="button" class="stat-chokepoint" data-waiting-walk="${choke.id}" title="${escapeHTML(title)}" aria-label="${escapeHTML(title)}">
      <span class="chokepoint-label">Biggest chokepoint</span>
      <span class="chokepoint-val">#${choke.id} <span class="chokepoint-n">&#8593; ${choke.count} waiting</span></span>
    </button>`;
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

/** Render the whole stats panel body. Pure → unit-tested. */
export function renderStatsPanel(
  stats: Stats,
  dep?: DepStats,
  sched?: ScheduleStats,
  choke?: Chokepoint | null,
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
    ${dep ? renderDepStats(dep, choke) : ""}
    <div class="stats-section-label">Top tags</div>
    ${renderTopTags(stats)}`;
}

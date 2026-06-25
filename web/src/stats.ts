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
import type { DepStats } from "./deps";
import type { ScheduleStats } from "./schedule";

/** Escape strings before injecting into innerHTML. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
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
export function renderDepStats(dep: DepStats): string {
  if (dep.blocked === 0 && dep.pinned === 0 && dep.longestChain === 0) return "";
  const blockedAlert = dep.blocked > 0 ? " is-alert" : "";
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
      <div class="stat-metric metric-blocked${blockedAlert}">
        <span class="stat-num">${dep.blocked}</span>
        <span class="stat-label">Blocked</span>
      </div>
      <div class="stat-metric metric-pinned">
        <span class="stat-num">${dep.pinned}</span>
        <span class="stat-label">Pinned</span>
      </div>
      ${chainTile}
    </div>`;
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
  return `
    <div class="stats-section-label">Schedule</div>
    <div class="stats-grid stats-grid-sched">
      <div class="stat-metric metric-week">
        <span class="stat-num">${sched.dueThisWeek}</span>
        <span class="stat-label">Due this week</span>
      </div>
      <div class="stat-metric metric-nodue">
        <span class="stat-num">${sched.noDue}</span>
        <span class="stat-label">No due</span>
      </div>
    </div>`;
}

/** Render the whole stats panel body. Pure → unit-tested. */
export function renderStatsPanel(stats: Stats, dep?: DepStats, sched?: ScheduleStats): string {
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
      ${renderMetric(stats.undone, "Open")}
      ${renderMetric(stats.today, "Due today", "today")}
      ${renderMetric(stats.overdue, "Overdue", "overdue")}
    </div>
    ${sched ? renderScheduleStats(sched) : ""}
    ${dep ? renderDepStats(dep) : ""}
    <div class="stats-section-label">Top tags</div>
    ${renderTopTags(stats)}`;
}

import { test } from "node:test";
import assert from "node:assert/strict";
import {
  formatPct,
  streakLabel,
  ringDash,
  renderDonut,
  renderMetric,
  renderLensMetric,
  renderTopTags,
  renderDepStats,
  renderChokepoint,
  renderScheduleStats,
  renderStatsPanel,
} from "../src/stats.ts";
import type { Stats } from "../src/api.ts";

function stats(over: Partial<Stats> = {}): Stats {
  return {
    total: 10,
    done: 4,
    undone: 6,
    overdue: 2,
    today: 1,
    completion: 40,
    streak: 3,
    top_tags: [
      { tag: "work", count: 5 },
      { tag: "home", count: 2 },
    ],
    ...over,
  };
}

test("formatPct rounds and guards non-finite", () => {
  assert.equal(formatPct(40), "40%");
  assert.equal(formatPct(66.6), "67%");
  assert.equal(formatPct(0), "0%");
  assert.equal(formatPct(NaN), "0%");
});

test("streakLabel pluralizes", () => {
  assert.equal(streakLabel(0), "no streak");
  assert.equal(streakLabel(1), "1 day");
  assert.equal(streakLabel(5), "5 days");
  assert.equal(streakLabel(-1), "no streak");
});

test("ringDash: 0% is empty ring, 100% is full", () => {
  const r = 26;
  const circ = 2 * Math.PI * r;
  const empty = ringDash(0, r);
  assert.ok(Math.abs(empty.offset - circ) < 1e-6); // full offset = empty visual
  const full = ringDash(100, r);
  assert.ok(Math.abs(full.offset) < 1e-6); // zero offset = full visual
  const half = ringDash(50, r);
  assert.ok(Math.abs(half.offset - circ / 2) < 1e-6);
});

test("ringDash clamps out-of-range completion", () => {
  const r = 10;
  const circ = 2 * Math.PI * r;
  assert.ok(Math.abs(ringDash(150, r).offset) < 1e-6); // clamps to 100
  assert.ok(Math.abs(ringDash(-20, r).offset - circ) < 1e-6); // clamps to 0
  assert.ok(Math.abs(ringDash(NaN, r).offset - circ) < 1e-6); // guards NaN -> 0
});

test("renderDonut shows the percentage label + aria", () => {
  const html = renderDonut(stats({ completion: 40 }));
  assert.match(html, /40%/);
  assert.match(html, /aria-label="40% complete"/);
  assert.match(html, /donut-fill/);
});

test("renderMetric: overdue with count gets an alert class", () => {
  assert.match(renderMetric(2, "Overdue", "overdue"), /is-alert/);
  assert.doesNotMatch(renderMetric(0, "Overdue", "overdue"), /is-alert/);
  assert.match(renderMetric(6, "Open"), /stat-num">6</);
});

test("renderTopTags renders rows with data-stat-tag + proportional bars", () => {
  const html = renderTopTags(stats());
  assert.match(html, /data-stat-tag="work"/);
  assert.match(html, /data-stat-tag="home"/);
  // The top tag (count 5, max 5) should be at 100%.
  assert.match(html, /width:100%/);
  // home is 2/5 = 40%.
  assert.match(html, /width:40%/);
});

test("renderTopTags empty state", () => {
  const html = renderTopTags(stats({ top_tags: [] }));
  assert.match(html, /No tags yet/);
  assert.doesNotMatch(html, /data-stat-tag/);
});

test("renderTopTags escapes tag names", () => {
  const html = renderTopTags(stats({ top_tags: [{ tag: "a<b", count: 1 }] }));
  assert.match(html, /a&lt;b/);
  assert.doesNotMatch(html, /data-stat-tag="a<b"/);
});

test("renderStatsPanel includes donut, metrics, streak, and tags", () => {
  const html = renderStatsPanel(stats());
  assert.match(html, /stats-donut/);
  assert.match(html, /3 days/); // streak
  assert.match(html, /Overdue/);
  assert.match(html, /Due today/);
  assert.match(html, /Top tags/);
  assert.match(html, /4 of 10 done/);
});

// --- F46: dependency-health row --------------------------------------------

test("renderDepStats shows blocked, pinned, and chain depth", () => {
  const html = renderDepStats({ blocked: 2, pinned: 3, longestChain: 4 });
  assert.match(html, /Dependencies/);
  assert.match(html, /Blocked/);
  assert.match(html, /Pinned/);
  assert.match(html, /Chain depth/);
  assert.match(html, /metric-blocked is-alert/); // alert since blocked > 0
});

// --- F64: blocked tile click-to-filter -------------------------------------

test("renderDepStats: the Blocked tile is a drill button when anything is blocked", () => {
  const html = renderDepStats({ blocked: 2, pinned: 0, longestChain: 0 });
  assert.match(html, /data-lens-drill="blocked"/);
  assert.match(html, /button[^>]*metric-blocked/);
});

test("renderDepStats: the Blocked tile is static (no drill) when nothing is blocked", () => {
  const html = renderDepStats({ blocked: 0, pinned: 2, longestChain: 0 });
  assert.doesNotMatch(html, /data-lens-drill/);
  // still shows the (zero) Blocked tile as a plain div
  assert.match(html, /Blocked/);
});

test("renderDepStats omits the chain tile when the graph is flat", () => {
  const html = renderDepStats({ blocked: 0, pinned: 2, longestChain: 0 });
  assert.doesNotMatch(html, /Chain depth/);
  assert.match(html, /Pinned/);
  assert.doesNotMatch(html, /is-alert/); // nothing blocked
});

test("renderDepStats collapses to empty when there's nothing to show", () => {
  assert.equal(renderDepStats({ blocked: 0, pinned: 0, longestChain: 0 }), "");
});

test("renderStatsPanel includes the dep row only when dep stats are passed", () => {
  const withDep = renderStatsPanel(stats(), { blocked: 1, pinned: 0, longestChain: 0 });
  assert.match(withDep, /Dependencies/);
  // Without the dep arg, the panel renders exactly as before (back-compat).
  assert.doesNotMatch(renderStatsPanel(stats()), /Dependencies/);
});

// --- F69: time-metric tiles click-to-filter --------------------------------

test("renderLensMetric is a drill button when it has tasks", () => {
  const html = renderLensMetric(3, "Overdue", "overdue", "overdue");
  assert.match(html, /data-lens-drill="overdue"/);
  assert.match(html, /button[^>]*metric-overdue/);
  assert.match(html, /is-alert/); // overdue + count keeps the alert hue
  assert.match(html, /stat-num">3</);
});

test("renderLensMetric is a static tile at zero (nothing to filter to)", () => {
  const html = renderLensMetric(0, "Overdue", "overdue", "overdue");
  assert.doesNotMatch(html, /data-lens-drill/);
  assert.doesNotMatch(html, /<button/);
  assert.match(html, /stat-num">0</);
});

test("renderLensMetric escapes the lens + label", () => {
  const html = renderLensMetric(1, "Due today", "today", "today");
  assert.match(html, /data-lens-drill="today"/);
  assert.match(html, /Show only due today tasks/);
});

test("renderStatsPanel wires the Open / today / overdue tiles to lenses", () => {
  const html = renderStatsPanel(stats({ undone: 6, today: 1, overdue: 2 }));
  assert.match(html, /data-lens-drill="open"/);
  assert.match(html, /data-lens-drill="today"/);
  assert.match(html, /data-lens-drill="overdue"/);
});

// --- F66: schedule tiles click-to-filter -----------------------------------

test("renderScheduleStats: tiles are drill buttons when they have tasks", () => {
  const html = renderScheduleStats({ dueThisWeek: 2, noDue: 3 });
  assert.match(html, /data-lens-drill="week"/);
  assert.match(html, /data-lens-drill="nodue"/);
  assert.match(html, /Due this week/);
  assert.match(html, /No due/);
});

test("renderScheduleStats: a zero tile stays static while the other drills", () => {
  const html = renderScheduleStats({ dueThisWeek: 0, noDue: 4 });
  // week is zero -> static div; nodue has tasks -> button
  assert.doesNotMatch(html, /data-lens-drill="week"/);
  assert.match(html, /data-lens-drill="nodue"/);
});

test("renderScheduleStats collapses to empty when both are zero", () => {
  assert.equal(renderScheduleStats({ dueThisWeek: 0, noDue: 0 }), "");
});

// --- F76: lens digit-key badges on the tiles -------------------------------

test("renderLensMetric carries the digit-key badge for a real lens", () => {
  // overdue is lens #2 -> badge shows "2"
  const html = renderLensMetric(3, "Overdue", "overdue", "overdue");
  assert.match(html, /class="lens-key"[^>]*>2</);
  assert.match(html, /key 2\)/); // also in the title tooltip
});

test("renderLensMetric badge digit matches the lens slot", () => {
  assert.match(renderLensMetric(1, "Due today", "today", "today"), /lens-key"[^>]*>3</);
});

test("renderLensMetric: the open tile has no digit badge (it's a facet, not a lens)", () => {
  // "open" maps to hideDone, not a numbered lens -> no badge, title shows "-"
  const html = renderLensMetric(6, "Open", "open");
  assert.doesNotMatch(html, /lens-key/);
  assert.match(html, /key -\)/);
});

test("renderLensMetric: a zero (static) tile carries no badge", () => {
  assert.doesNotMatch(renderLensMetric(0, "Overdue", "overdue", "overdue"), /lens-key/);
});

test("renderDepStats: the blocked drill tile shows the '1' key badge", () => {
  const html = renderDepStats({ blocked: 2, pinned: 0, longestChain: 0 });
  assert.match(html, /lens-key"[^>]*>1</);
});

test("renderDepStats: a static (zero) blocked tile has no badge", () => {
  const html = renderDepStats({ blocked: 0, pinned: 2, longestChain: 0 });
  assert.doesNotMatch(html, /lens-key/);
});

test("renderScheduleStats: week + nodue drill tiles show keys 4 and 5", () => {
  const html = renderScheduleStats({ dueThisWeek: 2, noDue: 3 });
  assert.match(html, /lens-key"[^>]*>4</);
  assert.match(html, /lens-key"[^>]*>5</);
});

// --- F92: biggest-chokepoint line ------------------------------------------

test("renderChokepoint names the worst bottleneck and its waiter count", () => {
  const html = renderChokepoint({ id: 7, count: 3 });
  assert.match(html, /Biggest chokepoint/);
  assert.match(html, /#7/);
  assert.match(html, /3 waiting/);
  // It shares the F87 dependent-walk hook so a click opens "what waits on it?".
  assert.match(html, /data-waiting-walk="7"/);
});

test("renderChokepoint uses the singular noun for a single waiter", () => {
  const html = renderChokepoint({ id: 2, count: 1 });
  assert.match(html, /1 task waits on it/);
  assert.doesNotMatch(html, /tasks wait/);
});

test("renderChokepoint collapses to empty for null / zero", () => {
  assert.equal(renderChokepoint(null), "");
  assert.equal(renderChokepoint(undefined), "");
  assert.equal(renderChokepoint({ id: 1, count: 0 }), "");
});

test("renderDepStats appends the chokepoint line only when one is passed", () => {
  const dep = { blocked: 2, pinned: 0, longestChain: 1 };
  assert.match(renderDepStats(dep, { id: 4, count: 2 }), /data-waiting-walk="4"/);
  // Back-compat: the one-arg call (and a null choke) render no chokepoint line.
  assert.doesNotMatch(renderDepStats(dep), /stat-chokepoint/);
  assert.doesNotMatch(renderDepStats(dep, null), /stat-chokepoint/);
});

test("renderStatsPanel threads the chokepoint through to the dep row", () => {
  const html = renderStatsPanel(
    stats(),
    { blocked: 1, pinned: 0, longestChain: 0 },
    undefined,
    { id: 9, count: 5 },
  );
  assert.match(html, /Biggest chokepoint/);
  assert.match(html, /data-waiting-walk="9"/);
});

// --- F96: chokepoint "focus the cohort" affordance -------------------------

test("renderChokepoint adds a focus button carrying the cohort source id", () => {
  const html = renderChokepoint({ id: 7, count: 3 });
  // The walk button (what waits?) AND the focus button (narrow to the cohort).
  assert.match(html, /data-waiting-walk="7"/);
  assert.match(html, /data-cohort-focus="7"/);
  assert.match(html, /stat-chokepoint-focus/);
  assert.match(html, />focus</);
});

test("renderChokepoint focus button is absent when there's no chokepoint", () => {
  assert.doesNotMatch(renderChokepoint(null) || "x", /data-cohort-focus/);
  assert.doesNotMatch(renderChokepoint({ id: 1, count: 0 }) || "x", /data-cohort-focus/);
});

test("renderChokepoint focus title reads naturally for one vs many waiters", () => {
  assert.match(renderChokepoint({ id: 4, count: 1 }), /waiting on #4/);
  assert.match(renderChokepoint({ id: 4, count: 2 }), /2 tasks waiting on #4/);
});
